---
change: docker-image-support
design-doc: docs/openspec/changes/docker-image-support/design.md
base-ref: 49d5571f4e44d1fe265c44f9fa289bc296b865f3
archived-with: 2026-10-27-docker-image-support
---

<!-- comet-task-authority: docs/openspec/changes/docker-image-support/tasks.md -->

# Docker 镜像支持实施计划

## 背景与约束

- 设计依据：`docs/openspec/changes/docker-image-support/design.md`；行为验收见 `specs/container-image/spec.md` 与 `specs/image-publishing/spec.md`。
- 应用代码零改动：只新增根目录 `Dockerfile`、`.dockerignore`、`docker/docker-entrypoint.sh`、`docker-compose.yml`、`.github/workflows/docker.yml`、`docs/docker.md`，并最小化修改 `README.md` 与 `AGENTS.md`。
- 不修改 `.github/workflows/build.yml`、`combined_refactor/` 任何源码、顶层原版源码文件。
- 执行配置：autonomous / direct / standard；每个任务完成后以真实命令验证，不接受未运行的口头结论。
- 本地 Docker 29.8.2 + buildx 0.37.2（darwin/arm64）；Go 1.27.2；Ruby 可用于 YAML 语法解析；`jq` 可用。
- base-ref 固定为 `49d5571f4e44d1fe265c44f9fa289bc296b865f3`；提交与推送须另行获得用户许可。

## 1. 镜像构建基础

### 1.1 根目录多阶段 Dockerfile

<!-- comet-task-ref:ec3b550a-da8c-45aa-9bb4-90eaa1b4e8b8 -->

- 构建阶段 `golang:1.26-bookworm`，固定 `--platform=$BUILDPLATFORM`，`GOOS/GOARCH` 取自 `TARGETOS/TARGETARCH`，`CGO_ENABLED=0`、`-trimpath`、`-ldflags "-s -w -X main.appVersion=${VERSION}"`；先复制 `go.mod/go.sum` 执行 `go mod download`，再复制 `combined_refactor/` 源码。
- 运行阶段 `alpine:3.22`：`apk add --no-cache ca-certificates tzdata`，创建 `10001:10001` 用户，`/app` 只读存放二进制、entrypoint 与 `LICENSE`，`/data` 为数据目录（不声明 `VOLUME`，未挂载时数据写入容器可写层）。
- 构建参数 `VERSION`（默认 `dev`）与 `REVISION`（默认 `unknown`）写入 OCI LABEL：title、description、source=`https://github.com/tunecc/CFData-docker`、version、revision、licenses=`GPL-3.0-or-later`；同时保留 metadata-action 注入的标签。
- 运行配置：`WORKDIR /data`、`USER 10001:10001`、`EXPOSE 13335`、`ENTRYPOINT ["/app/docker-entrypoint.sh"]`、`CMD []`；`HEALTHCHECK` 在 2.2 接入。
- 依赖：无。
- 约束：不联网下载 CA，直接使用仓库内 `combined_refactor/ca-certificates.crt`（已跟踪）。
- 验收：`docker build --build-arg VERSION=v0.0.0-test -t cfdata-web:dev .` 成功；`docker run --rm cfdata-web:dev -h` 退出码为 0。

### 1.2 数据目录同步 entrypoint

<!-- comet-task-ref:181cdb6d-dbbb-4117-823c-34b0d14e738d -->

- `docker/docker-entrypoint.sh`：`#!/bin/sh` + `set -eu`；目标缺失、无可执行位或内容不同时，复制到 `mktemp` 唯一临时文件、`chmod 0755`、再 `mv` 原子替换；`trap` 清理临时文件；进入 `/data` 后 `exec /data/cfdata "$@"`。
- 依赖：1.1。
- 约束：不在 `/app` 写任何文件；同步失败时以非零退出并保留原始错误；一个数据卷只运行一个容器的建议写入文档。
- 验收：`docker run --rm cfdata-web:dev -h` 正常；`docker exec` 检查 `/data/cfdata` 存在；连续两次启动不重复写入（第二次无临时文件残留）；传入参数原样到达主程序。

### 1.3 .dockerignore

<!-- comet-task-ref:07224830-c719-4e19-8915-75a86651856d -->

- 排除 `.git`、`.github`、`docs`、`app`、`img`、根目录原版源码（`cfdata.go`、`index.html`、`go.mod`、`go.sum`、Gradle 文件）、`release_assets`、Comet/Qoder/Claude 状态目录、运行期缓存与日志；保留 `LICENSE`、`combined_refactor/**`（含 CA 与嵌入资源）。
- 依赖：1.1。
- 约束：不得排除 `LICENSE` 或 `combined_refactor/ca-certificates.crt`。
- 验收：`docker build --no-cache -t cfdata-web:dev .` 仍成功，`--progress=plain` 中构建上下文体积明显小于仓库总大小；`docker run --rm cfdata-web:dev test -f /app/LICENSE` 成功。

### 1.4 基础运行契约验证

<!-- comet-task-ref:5968c313-223c-409f-be6a-767172d5c00f -->

- 验证项：进程 UID 10001（`docker exec <c> id -u`）、PID 1 为 cfdata、`0.0.0.0:13335` 监听并可访问、`GET /favicon.png` 返回 200、日志版本号等于构建参数。
- 依赖：1.1、1.2、1.3。
- 验收：按上述命令逐项记录真实输出；版本号使用 `--build-arg VERSION=v0.0.0-test` 构建并比对。

## 2. 数据持久化与运行体验

### 2.1 命名卷持久化

<!-- comet-task-ref:6b370e12-0c87-4467-9ee6-1ccaf2267c68 -->

- 使用命名卷 `cfdata-data-test` 启动容器，写入 `/data/cfdata-config.json` 与缓存探测文件；删除容器后用同一卷重建，确认文件仍存在且服务可启动。
- 未挂载卷时数据写入容器可写层，确认服务仍可访问且不产生匿名卷。
- 依赖：1.4。
- 验收：两轮容器 `docker exec cat /data/...` 输出一致；无卷启动健康检查通过。

### 2.2 HEALTHCHECK 与端口覆盖

<!-- comet-task-ref:b596cf2d-2ebc-4418-8350-0f6c5111b8e8 -->

- Dockerfile 增加 `HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 CMD wget -q -O /dev/null "http://${CFDATA_HEALTHCHECK_HOST}:${CFDATA_HEALTHCHECK_PORT}/favicon.png" || exit 1`，并声明 `ENV CFDATA_HEALTHCHECK_HOST=127.0.0.1` 与 `ENV CFDATA_HEALTHCHECK_PORT=13335`。
- 依赖：1.1、1.4。
- 验收：默认启动轮询 `docker inspect --format '{{.State.Health.Status}}'` 到达 `healthy`；以 `-port 8080` 且不设环境变量时到达 `unhealthy`；以 `-port 8080 -e CFDATA_HEALTHCHECK_PORT=8080` 到达 `healthy`。

### 2.3 docker-compose.yml

<!-- comet-task-ref:a1dadd33-a714-4c56-87c5-6dcca82dfc3e -->

- 新增根目录 `docker-compose.yml`：`image: ghcr.io/tunecc/cfdata-web:latest`、端口 `13335:13335`、命名卷 `cfdata-data:/data`、`restart: unless-stopped`；注释给出认证参数示例，端口调整指向 `docs/docker.md` 的完整示例。
- 依赖：1.1、1.2、2.2。
- 验收：`docker compose config` 通过；本地将 `cfdata-web:dev` 标记为同名镜像后 `docker compose up -d`，访问 13335 成功，`docker compose down -v` 清理。

### 2.4 多架构构建

<!-- comet-task-ref:40fce18e-30ff-43f0-9a30-b657e889c01a -->

- 使用 `docker buildx build --platform linux/amd64,linux/arm64 --output type=oci,dest=/tmp/cfdata-multi.tar .` 构建（不推送）。
- 依赖：1.1、1.2。
- 验收：命令成功；从 OCI tar 的 `index.json` 读到的清单包含 `amd64` 与 `arm64` 两个平台条目。

## 3. GHCR 发布流水线

### 3.1 docker.yml 骨架

<!-- comet-task-ref:82a3e4ba-2016-4aeb-b25d-454912409f6b -->

- 新增 `.github/workflows/docker.yml`：`name: Docker Image`；触发 `push.tags: ["v*"]` 与 `workflow_dispatch`；顶层 `permissions: contents: read, packages: write`；`concurrency: group: docker-${{ github.ref }}, cancel-in-progress: false`。
- 依赖：1.1。
- 验收：Ruby YAML 解析成功；文本断言触发条件与权限块完整，且未声明 `packages: write` 以外的写权限。

### 3.2 标签与 VERSION 逻辑

<!-- comet-task-ref:507cdac1-b67d-4ba8-b9f1-940362604253 -->

- 使用 `docker/metadata-action` 并设置 `flavor: latest=false`，标签规则：
  - `type=ref,event=tag` → `v1.2.3`；
  - `type=sha,format=short,prefix=sha-` → `sha-<7 位>`；
  - `type=raw,value=latest,enable=<手动触发，或 tag 推送且 tag 名不含 `-`>`；
  - `type=raw,value=edge,enable=<仅手动触发>`。
- `VERSION`：tag 推送取 `github.ref_name`，手动触发取 `edge-<7 位提交哈希>`；`REVISION` 始终为完整 SHA。
- 依赖：3.1。
- 验收：用 shell 复刻四个条件（稳定 tag、预发布 tag、手动默认分支、手动功能分支），输出与规格逐项比对；确认任何情况下都不生成/覆盖 `v*` 之外的版本标签。

### 3.3 构建与缓存

<!-- comet-task-ref:a98e1a93-0f9f-4236-8784-4b728f2fb554 -->

- 步骤顺序：checkout → 镜像名小写 → VERSION/REVISION → `docker/setup-qemu-action` → `docker/setup-buildx-action` → `docker/login-action`（`GITHUB_TOKEN`）→ `docker/metadata-action` → `docker/build-push-action` → 摘要。
- `build-push-action`：`platforms: linux/amd64,linux/arm64`、`push: true`、`cache-from: type=gha`、`cache-to: type=gha,mode=max`、`build-args` 传入 VERSION/REVISION。
- 依赖：3.1、3.2。
- 验收：Ruby 解析后按文本核对步骤顺序与关键参数齐全；YAML 中不存在硬编码镜像标签列表。

### 3.4 工作流摘要

<!-- comet-task-ref:04ef6beb-605d-4f01-a184-0d69558fdb09 -->

- `if: always()` 的摘要步骤输出镜像地址、`metadata-action` 标签、`build.outputs.digest`、VERSION、提交 SHA 与结果状态；失败时注明失败阶段（依赖安装/构建/推送）。
- 依赖：3.3。
- 验收：断言摘要引用 `steps.build.outputs.digest`、`steps.meta.outputs.tags`、`steps.build-meta.outputs.version` 与 `github.sha`，无硬编码镜像或标签。

### 3.5 镜像名小写

<!-- comet-task-ref:35eb4632-e512-4b06-93fd-cc884e1c8634 -->

- 用独立步骤生成 `name=ghcr.io/${OWNER,,}/cfdata-web`，后续 `metadata-action`、`login` 与摘要统一引用该输出。
- 依赖：3.1。
- 验收：`OWNER=TuneCC` 的 shell 模拟输出全小写 `ghcr.io/tunecc/cfdata-web`；工作流中所有镜像引用均来自该步骤输出。

## 4. 文档与仓库约定

### 4.1 README 与 docs/docker.md

<!-- comet-task-ref:c741f9e3-2e36-4cb9-ad22-c559c1c2094d -->

- README 增加简短「Docker」小节：拉取、默认运行、compose、指向 `docs/docker.md`；保持最小段落以减少上游冲突。
- `docs/docker.md` 覆盖：拉取与运行、compose、认证参数、端口与 `CFDATA_HEALTHCHECK_PORT`、数据卷与 bind mount 属主、升级方式、GHCR 首次发布设为 public、手动构建覆盖 `latest` 的提醒、多架构说明。
- 依赖：1.1–2.4、3.1–3.5。
- 验收：文档中命令逐条可执行（已在 1–3 与 5.1 验证）；无占位符。

### 4.2 AGENTS.md 例外

<!-- comet-task-ref:70dda3d1-3aee-4132-a08c-c6c9eac47f56 -->

- 在「禁止修改的文件」中增加：`.github/workflows/docker.yml` 允许在 Docker 镜像相关变更中维护，其余 `.github/` 文件仍禁止修改。
- 依赖：3.1。
- 验收：文字只新增例外，不改变原有 Git/编译规则。

### 4.3 GPL 与 OCI 标签

<!-- comet-task-ref:3a7d3af6-8ccd-419d-a9af-136e090f436c -->

- 镜像内 `/app/LICENSE`；镜像 OCI 标签包含 source、revision、version、licenses。
- 依赖：1.1、3.1。
- 验收：`docker inspect` 输出上述四项；`docker run --rm --entrypoint sh cfdata-web:dev -c 'test -f /app/LICENSE'` 成功。

## 5. 集成验证

### 5.1 端到端本地验证

<!-- comet-task-ref:f63d6308-dea4-4b30-b0d2-2c15dbc27bfa -->

- 串联：干净构建 → 启动 → 健康检查 → 认证参数（`-user admin -password secret` 时未登录访问 `/` 跳转登录）→ 卷持久化 → 版本输出 → 多架构清单。
- 依赖：1.1–4.3。
- 验收：全部命令真实通过并记录关键输出。

### 5.2 现有发布流程未受影响

<!-- comet-task-ref:25e460e3-4bc9-46c4-a964-c9bca06e937b -->

- 验收：`git diff --exit-code 49d5571f4e44d1fe265c44f9fa289bc296b865f3 -- .github/workflows/build.yml` 无输出且退出码为 0。

### 5.3 静态检查

<!-- comet-task-ref:50f141cd-5a2b-4ed7-8651-3808c06ef95b -->

- 验收：`docker.yml`、`docker-compose.yml` 通过 Ruby YAML 解析；`sh -n docker/docker-entrypoint.sh` 通过；全仓库新增文件 `grep -R "TODO\|<VERSION>\|REPLACE_ME"` 无命中。

## 独立审查与收尾

- 任务 1.1–4.3 完成后，由独立审查者（子代理）审查 `Dockerfile`、`docker/docker-entrypoint.sh`、`.dockerignore`、`docker-compose.yml`、`.github/workflows/docker.yml` 与文档，重点：非 root、卷内写入、健康检查、标签策略、最小权限、上游同步风险。
- CRITICAL/IMPORTANT 必须修复并复验；非关键偏差记录依据。
- Build 阶段结束前运行 `comet check run docker-image-support build --local -- ...` 记录构建证据，再运行 `comet guard docker-image-support build --apply`。
- 最终集成审查留给 Verify 阶段；提交与推送须获得用户明确许可。
