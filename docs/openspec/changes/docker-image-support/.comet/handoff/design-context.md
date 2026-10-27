# Comet Design Handoff

- Change: docker-image-support
- Phase: design
- Mode: compact
- Context hash: bd71ff59ecff9a79096bf69e6ed32362d97ac1af9e235a350afb0c01a3c9fc19

Generated-by: comet-handoff.sh
Task hash policy: task-content-v1. Read tasks.md for live completion; excerpts are design-time context.

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## docs/openspec/changes/docker-image-support/proposal.md

- Source: docs/openspec/changes/docker-image-support/proposal.md
- Lines: 1-44
- SHA256: 8611c1b44862bae9d98980f23ff36a48ab54cdb869975fbe2193feef25ada5c4

```md
# Proposal（提案）

## Why

CFData-WEB 目前通过 GitHub Releases 分发各平台二进制与 APK，缺少容器化运行方式；在 NAS、服务器等场景部署需要手动放置二进制并自行处理开机启动与数据持久化。本仓库作为长期维护、持续同步上游代码的 Docker 化分支，需要一条稳定的镜像生产链路：发布 tag 时自动产出可复现镜像，同时允许在 Actions 页面手动以最新代码构建，便于快速验证与滚动更新。

## What Changes

- 新增多阶段 `Dockerfile`：在 Go 1.26 构建阶段编译 `combined_refactor` 静态二进制，运行阶段使用精简基础镜像，非 root 运行，内置健康检查，镜像版本与构建来源一致。
- 新增 `.dockerignore`，缩小构建上下文，避免把 Android、文档与 Comet 运行数据带入镜像。
- 约定容器数据目录 `/data`，通过卷持久化配置文件、缓存与 ASN 数据库；提供 `docker-compose.yml` 示例，暴露 `13335` 端口。
- 新增独立 GitHub Actions 工作流 `.github/workflows/docker.yml`：
  - 推送 `v*` tag 时构建 `linux/amd64` 与 `linux/arm64` 镜像并推送 GHCR，生成版本号、`latest` 与 `sha-<短哈希>` 标签；
  - `workflow_dispatch` 手动触发时以所选分支/提交的最新代码构建，推送 `latest`、`edge` 与 `sha-<短哈希>` 标签。
- 在 README 中补充镜像拉取、运行、数据卷与升级说明（详细说明放在独立文档，减少上游同步冲突）。
- 在 `AGENTS.md` 中为 Docker 工作流增加 `.github/` 例外，其余 CI 约束保持不变。
- 不修改现有 `build.yml` 的二进制/APK 发布流程。

## Non-Goals

- 不改变 CFData-WEB 程序本身的 Web/CLI 行为、启动参数与数据格式。
- 不引入 Docker Hub 或其它镜像仓库（仅 GHCR）。
- 不提供 Kubernetes/Helm 清单，不构建 Android 镜像。
- 不调整上游源码同步方式，不修改标注"原版源码"的顶层文件。
- 不把镜像构建并入现有 Release 工作流，保持独立文件以降低上游冲突。

## Capabilities

### New Capabilities

- `container-image`: 容器镜像的构建与运行契约，包括多阶段构建、多架构、非 root、数据持久化、健康检查与版本注入。
- `image-publishing`: 镜像发布流水线契约，包括 GHCR 推送、tag 触发、手动触发与标签生成规则。

### Modified Capabilities

（无）

## Impact

- 新增文件：`Dockerfile`、`.dockerignore`、`docker-compose.yml`、`.github/workflows/docker.yml`、容器使用文档。
- 修改文件：`README.md`（少量链接与说明）、`AGENTS.md`（增加 `.github/` 例外）。
- 应用代码：无改动；镜像复用现有 `-X main.appVersion` 版本注入与 `:13335` 监听默认值。
- 基础设施：需要 GHCR 写权限（使用 `GITHUB_TOKEN`）；首次发布后需仓库所有者手动将 GHCR 包可见性设为 public，否则匿名拉取会失败。
- CI 成本：tag 与手动触发的多架构构建会增加 Actions 分钟数；Go 交叉编译负责产物架构，运行阶段安装依赖需要 QEMU 支持（由 Actions 的 setup-qemu 提供）。

```

## docs/openspec/changes/docker-image-support/design.md

- Source: docs/openspec/changes/docker-image-support/design.md
- Lines: 1-107
- SHA256: 9e6d09740a966394fe7645d5f3f61d4aaad88ea37a97089e43f0a3f6ef432a84

[TRUNCATED]

```md
# Design（技术设计）

## Context

- 主程序位于 `combined_refactor/`（Go 1.26），`index.html`、`login.html`、`favicon.png` 与 CA 证书通过 `go:embed` 打进二进制；版本由 `-X main.appVersion` 注入。
- 程序默认监听 `0.0.0.0:13335`，配置文件按“可执行文件所在目录”定位，IP 列表、位置信息、ASN 数据库与调试日志按“当前工作目录”写入；仓库没有单元测试。
- 现有 `.github/workflows/build.yml` 在 `v*` tag 与手动触发时构建全平台二进制与 APK 并发布 Release；本次新增镜像流水线必须与其共存。
- 本仓库长期维护并同步上游 `PoemMisty/CFData-WEB`，因此应用代码零改动是硬约束，新增能力应尽量落在新文件。
- 已确认环境：本地 Docker 29.8.2、buildx 0.37.2，可执行本地构建验证；CI 通过 `combined_refactor/go.mod` 安装 Go。

## Goals / Non-Goals

**Goals：**

- 一份 `Dockerfile` 同时服务本地构建与 GitHub Actions，产出 linux/amd64 与 linux/arm64 镜像。
- 所有运行期可变状态集中在单一数据目录，容器重建后配置、缓存与日志不丢失。
- tag 发布与手动构建的标签策略固定、可追溯，构建失败不污染 `latest` 与版本标签。
- 新增文件为主，`README.md` 与 `AGENTS.md` 仅做最小改动，降低上游同步冲突。

**Non-Goals：**

- 不增加环境变量配置层或 entrypoint 参数翻译，容器仍使用程序现有命令行参数。
- 不构建 Windows/macOS 容器，不做镜像内自动更新或自升级。
- 不修改应用代码、现有 `build.yml` 与二进制/APK 发布语义。
- 不引入镜像签名、SBOM 或漏洞扫描流水线（可作为后续独立需求）。

## Decisions

### D1：多阶段构建 + Alpine 运行镜像

- 构建阶段使用 `golang:1.26-bookworm`，固定 `--platform=$BUILDPLATFORM`，按 `TARGETARCH` 交叉编译，`CGO_ENABLED=0`、`-trimpath`、`-ldflags "-s -w -X main.appVersion=<VERSION>"`。
- 运行阶段使用 `alpine:3.22`，安装 `ca-certificates`、`tzdata`，以 UID/GID `10001` 非 root 运行。
- 备选：`scratch`/distroless 体积更小，但缺少 shell 与 `wget`，健康检查和目录属主处理需要额外二进制；`debian-slim` 体积明显更大；单阶段会把 Go 工具链带进运行镜像。
- 代价：运行阶段的 `apk add` 在目标架构执行，CI 需要 QEMU；Go 编译本身不经过仿真。

### D2：二进制落位与数据持久化

程序把配置写在可执行文件旁，把缓存写在当前工作目录；为让二者都落在卷内：

- 镜像内二进制只读存放于 `/app/cfdata`，`WORKDIR` 为 `/data`，`/data` 属主为 `10001`。
- entrypoint 脚本启动时用 `cmp` 比较 `/app/cfdata` 与 `/data/cfdata`；不一致时先复制到临时文件再 `mv` 覆盖，最后 `exec /data/cfdata "$@"`。这样镜像升级后卷内二进制自动更新，配置、`.bak`、缓存与日志全部位于 `/data`。
- 备选：给 `/app/cfdata-config.json` 建软链接指向 `/data`；但程序还会在配置路径旁写 `.bak` 和迁移备份，这些写入落在只读的 `/app` 会失败。修改程序增加配置路径参数会破坏上游同步；把整个 `/app` 挂成卷会遮蔽镜像内二进制，导致升级失效。
- 代价：二进制随卷保存（约十几 MB），升级时多一次复制；`cmp` 命中时无额外开销。

### D3：非 root 与健康检查

- 运行用户 `10001:10001`，`HOME=/data`，`/app` 由 root 持有且不可写。
- 健康检查使用未认证的 `/favicon.png`，默认请求 `http://127.0.0.1:${CFDATA_HEALTHCHECK_PORT:-13335}`；间隔 30s、超时 5s、启动宽限 15s、重试 3 次。
- 用户通过 `-port` 改端口时，同时设置 `CFDATA_HEALTHCHECK_PORT`；文档中说明。
- 备选：TCP 端口探测无法验证 HTTP 服务是否真正可用；BusyBox `wget` 已随 Alpine 提供，无需额外依赖。

### D4：版本与可追溯性

- 新增构建参数 `VERSION`、`REVISION`：tag 构建取 `github.ref_name`，手动构建取 `edge-<7 位提交哈希>`；`REVISION` 始终为完整提交 SHA。
- 通过 `docker/metadata-action` 写入 OCI 标签：`title`、`description`、`source`、`version`、`revision`、`licenses=GPL-3.0-or-later`；镜像内附带 `LICENSE`。
- 工作流摘要输出镜像地址、标签列表、digest 与版本，失败时指出失败阶段。

### D5：独立的 GHCR 发布工作流

- 新增 `.github/workflows/docker.yml`，触发条件为 `push.tags: v*` 与 `workflow_dispatch`；权限仅 `contents: read`、`packages: write`。
- 镜像名 `ghcr.io/<owner 小写>/cfdata-web`；使用 `GITHUB_TOKEN` 登录 GHCR，不引入额外 secrets。
- 标签策略：
  - tag 推送：`v1.2.3`、`sha-<短哈希>`；tag 名不含 `-` 时额外更新 `latest`。
  - 手动触发：`latest`、`edge`、`sha-<短哈希>`，所选分支或提交由 Actions 页面决定。
- 使用 `setup-qemu` + `setup-buildx` + `build-push-action` 构建 `linux/amd64,linux/arm64`，启用 GHA 构建缓存。
- `concurrency` 按 ref 分组且不取消进行中的运行，避免同一 ref 的并发推送互相覆盖。
- 选择独立文件而不是并入 `build.yml`：`AGENTS.md` 的 `.github/` 例外可精确限定到新文件，现有 Release 流程零改动，上游同步冲突最小。

### D6：构建上下文与可复现性

- `Dockerfile` 放在仓库根目录；`.dockerignore` 排除 `.git`、`.github`、`docs`、`app`、Gradle 文件、根目录原版源码、`release_assets`、Comet/Qoder 配置与运行期缓存，仅保留 `combined_refactor` 源码、嵌入资源、CA 证书与 `LICENSE`。
- 构建阶段直接使用仓库内 `combined_refactor/ca-certificates.crt`，不在镜像构建时联网下载 CA；运行镜像同时安装系统 CA 包。现有 Release 流程仍按原方式刷新 CA 打包进二进制。
- 基础镜像固定到次版本标签（`golang:1.26-bookworm`、`alpine:3.22`）；digest 锁定暂不引入，避免每次上游安全更新都要手动改文件。

### D7：compose 示例与文档

- 新增根目录 `docker-compose.yml`：使用 `latest` 镜像、映射 `13335`、命名卷挂载 `/data`、`restart: unless-stopped`，并保留认证参数的注释示例。
- README 增加简短 Docker 小节，指向 `docs/docker.md`；详细文档覆盖拉取、运行、compose、认证、端口与健康检查、数据卷、升级、GHCR 可见性、手动构建覆盖 `latest` 的提醒。

### D8：AGENTS.md 例外

```

Full source: docs/openspec/changes/docker-image-support/design.md

## docs/openspec/changes/docker-image-support/tasks.md

- Source: docs/openspec/changes/docker-image-support/tasks.md
- Lines: 1-41
- SHA256: 9d5248240683c7d962a6b2614d3ac8b29c876f6b6ed78893b2b3b39797483c7b

```md
# Tasks

## 1. 镜像构建基础

- [ ] 1.1 新增根目录多阶段 `Dockerfile`：Go 1.26 构建阶段按 `TARGETARCH` 交叉编译 `combined_refactor`，Alpine 运行阶段安装 CA/时区、创建 UID/GID 10001 用户；验证 `docker build -t cfdata-web:dev .` 成功且镜像可启动 <!-- comet-task:ec3b550a-da8c-45aa-9bb4-90eaa1b4e8b8 -->
- [ ] 1.2 新增 `docker/docker-entrypoint.sh`：启动时比较并原子同步 `/app/cfdata` 到 `/data/cfdata`，再 `exec` 传递全部参数；验证镜像内 `/data/cfdata` 存在、重复启动不重复复制、传入 `-h` 能正常退出 <!-- comet-task:181cdb6d-dbbb-4117-823c-34b0d14e738d -->
- [ ] 1.3 新增 `.dockerignore`：排除 `.git`、`.github`、`docs`、`app`、Gradle、根目录原版源码、Comet/Qoder 状态与运行期缓存；验证构建上下文不含这些目录且构建仍成功 <!-- comet-task:07224830-c719-4e19-8915-75a86651856d -->
- [ ] 1.4 验证镜像基础运行契约：进程 UID 为 10001、默认监听 `0.0.0.0:13335`、`GET /favicon.png` 返回 200、日志中版本号与构建参数一致 <!-- comet-task:5968c313-223c-409f-be6a-767172d5c00f -->

## 2. 数据持久化与运行体验

- [ ] 2.1 验证命名卷持久化：挂载卷启动容器，写入/修改配置并生成缓存文件，删除并重建容器后配置与缓存仍存在；验证未挂载卷时服务仍可启动 <!-- comet-task:6b370e12-0c87-4467-9ee6-1ccaf2267c68 -->
- [ ] 2.2 配置镜像 `HEALTHCHECK` 与 `CFDATA_HEALTHCHECK_PORT` 覆盖逻辑；验证默认端口下状态为 healthy，停止服务后变为 unhealthy，改 `-port 8080` 且设置环境变量后仍为 healthy <!-- comet-task:b596cf2d-2ebc-4418-8350-0f6c5111b8e8 -->
- [ ] 2.3 新增 `docker-compose.yml` 示例（`latest` 镜像、端口 13335、命名卷、`restart: unless-stopped`）；验证 `docker compose config` 通过且 `docker compose up -d` 后服务可访问 <!-- comet-task:a1dadd33-a714-4c56-87c5-6dcca82dfc3e -->
- [ ] 2.4 验证多架构构建：执行 `docker buildx build --platform linux/amd64,linux/arm64`（不推送），确认两个平台均构建成功并能生成多架构清单 <!-- comet-task:40fce18e-30ff-43f0-9a30-b657e889c01a -->

## 3. GHCR 发布流水线

- [ ] 3.1 新增 `.github/workflows/docker.yml`：`push.tags: v*` 与 `workflow_dispatch` 触发，声明 `contents: read`、`packages: write`，使用 `GITHUB_TOKEN` 登录 GHCR；验证 YAML 可解析且触发与权限声明符合设计 <!-- comet-task:82a3e4ba-2016-4aeb-b25d-454912409f6b -->
- [ ] 3.2 实现 tag、`latest`、`edge`、`sha-<短哈希>` 标签与 `VERSION` 生成逻辑，预发布 tag 不更新 `latest`；用本地 shell 模拟 tag 推送与手动触发两种上下文，核对输出与规格一致 <!-- comet-task:507cdac1-b67d-4ba8-b9f1-940362604253 -->
- [ ] 3.3 接入 `setup-qemu`、`setup-buildx`、`build-push-action`，构建 `linux/amd64,linux/arm64`，配置 GHA 缓存与按 ref 的 `concurrency`；验证工作流步骤顺序与参数完整 <!-- comet-task:a98e1a93-0f9f-4236-8784-4b728f2fb554 -->
- [ ] 3.4 输出工作流摘要（镜像地址、标签、digest、版本、提交哈希），失败时步骤日志可定位阶段；验证摘要步骤使用实际构建输出而非硬编码 <!-- comet-task:04ef6beb-605d-4f01-a184-0d69558fdb09 -->
- [ ] 3.5 核对镜像名小写处理与 GHCR 地址生成；用大小写混合的 owner 模拟脚本，确认最终镜像名全小写 <!-- comet-task:35eb4632-e512-4b06-93fd-cc884e1c8634 -->

## 4. 文档与仓库约定

- [ ] 4.1 在 `README.md` 增加简短 Docker 小节并新增 `docs/docker.md`，覆盖拉取、运行、compose、认证、端口与健康检查、数据卷、升级、GHCR 可见性、手动构建覆盖 `latest` 的提醒；验证文档中的命令按原样可执行 <!-- comet-task:c741f9e3-2e36-4cb9-ad22-c559c1c2094d -->
- [ ] 4.2 更新 `AGENTS.md`：为 `.github/workflows/docker.yml` 增加例外，其余 `.github/` 禁止规则保持不变；验证文字与范围一致 <!-- comet-task:70dda3d1-3aee-4132-a08c-c6c9eac47f56 -->
- [ ] 4.3 核对 GPL 合规与 OCI 标签：镜像内包含 `LICENSE`，标签含源码地址、revision、version、licenses；验证 `docker inspect` 输出 <!-- comet-task:3a7d3af6-8ccd-419d-a9af-136e090f436c -->

## 5. 集成验证

- [ ] 5.1 执行端到端本地验证：构建镜像 → 启动 → 健康检查 → 认证参数 → 卷持久化 → 版本输出 → 多架构清单，全部通过 <!-- comet-task:f63d6308-dea4-4b30-b0d2-2c15dbc27bfa -->
- [ ] 5.2 核对 `.github/workflows/build.yml` 未改动，现有二进制/APK Release 触发与产物语义保持不变 <!-- comet-task:25e460e3-4bc9-46c4-a964-c9bca06e937b -->
- [ ] 5.3 静态检查全部新增 YAML 与脚本：YAML 解析通过、脚本 `sh -n` 通过、无未替换占位符 <!-- comet-task:50f141cd-5a2b-4ed7-8651-3808c06ef95b -->

## Workflow follow-up

- 合并后在 Actions 页面手动运行 Docker 工作流，确认 GHCR 出现 `latest`、`edge`、`sha-*`，并将包可见性设为 public。
- 推送 `v*` tag，确认版本 tag、`latest` 与现有 Release 流程并行正常。
- 完成后归档本 change。

```

## docs/openspec/changes/docker-image-support/.openspec.yaml

- Source: docs/openspec/changes/docker-image-support/.openspec.yaml
- Lines: 1-2
- SHA256: 89901217d56bdfb49c20b541962dc5db50e8161beb30e5c2cae1d189e5d53eb5

```md
schema: spec-driven
created: 2026-10-27

```

## docs/openspec/changes/docker-image-support/specs/container-image/spec.md

- Source: docs/openspec/changes/docker-image-support/specs/container-image/spec.md
- Lines: 1-114
- SHA256: fcb0dbc1d747dec7c9efa28d6ecc780a4abf8a32e1b5785364227d1bb7e506d8

[TRUNCATED]

```md
# Spec Delta

## Purpose

定义 CFData-WEB 容器镜像的构建与运行契约，使镜像可在 amd64/arm64 主机上以非 root 方式稳定运行 Web 服务，并在容器重建后保留配置与缓存数据。

## ADDED Requirements

### Requirement: 可复现的镜像构建

系统 MUST 能从仓库源码使用标准 Docker 构建命令生成可运行镜像，构建过程不依赖仓库外的本地构建产物，也不要求构建机预装 Go 工具链。

#### Scenario: 从干净克隆构建镜像

- **WHEN** 在干净克隆的仓库根目录执行标准 Docker 构建命令
- **THEN** 构建成功并生成可用于启动 Web 服务的镜像

#### Scenario: 构建失败时给出明确错误

- **WHEN** 源码编译失败或构建依赖不可用
- **THEN** 构建以非零状态结束，且日志包含失败阶段与原因

### Requirement: 多架构镜像

镜像 MUST 同时提供 linux/amd64 与 linux/arm64 两种架构，并在对应架构主机上以原生方式运行。

#### Scenario: 在 amd64 主机运行

- **WHEN** 在 linux/amd64 主机拉取并运行镜像
- **THEN** 容器正常启动且 Web 服务可访问

#### Scenario: 在 arm64 主机运行

- **WHEN** 在 linux/arm64 主机拉取并运行镜像
- **THEN** 容器正常启动且 Web 服务可访问

### Requirement: 默认 Web 运行模式

容器 MUST 默认以 Web 模式启动，监听容器内所有地址的 13335 端口，并允许通过附加启动参数覆盖端口、监听地址、Web 认证等运行选项。

#### Scenario: 默认启动

- **WHEN** 启动容器且不附加任何参数
- **THEN** Web 服务在容器内 `0.0.0.0:13335` 监听，访问 `/` 返回 Web 界面

#### Scenario: 覆盖端口与认证

- **WHEN** 启动容器时传入 `-port 8080 -user admin -password secret`
- **THEN** Web 服务监听 8080 端口，且未认证请求被引导至登录页

### Requirement: 数据持久化

容器 MUST 将配置文件、IP 列表、位置信息、ASN 数据库与调试日志写入可挂载的数据目录；挂载卷后，这些数据在容器删除或重建后仍保留。

#### Scenario: 重建后保留配置

- **WHEN** 挂载数据卷启动容器，修改配置后删除并重新创建容器
- **THEN** 新容器读取到修改后的配置

#### Scenario: 重建后保留缓存

- **WHEN** 挂载数据卷完成一次扫描与测速后重新创建容器
- **THEN** 已有 IP 列表、位置信息与 ASN 数据库无需重新下载即可使用

#### Scenario: 未挂载卷时仍可运行

- **WHEN** 启动容器时不挂载数据卷
- **THEN** Web 服务仍能启动并完成基本功能，数据仅保存在容器可写层

### Requirement: 非 root 运行与健康检查

容器 MUST 以非 root 用户运行；镜像 MUST 提供健康检查，用于判断 Web 服务是否可用。

#### Scenario: 以非 root 用户运行

- **WHEN** 检查容器内运行进程的用户
- **THEN** 进程 UID 不为 0

#### Scenario: 服务正常时健康


```

Full source: docs/openspec/changes/docker-image-support/specs/container-image/spec.md

## docs/openspec/changes/docker-image-support/specs/image-publishing/spec.md

- Source: docs/openspec/changes/docker-image-support/specs/image-publishing/spec.md
- Lines: 1-90
- SHA256: 858396cca5d6d61b79f11ce0d9001c518d1385867dc0920460cae1f9aa1f144c

[TRUNCATED]

```md
# Spec Delta

## Purpose

定义 CFData-WEB 镜像的发布契约，使仓库在推送版本 tag 时自动发布可追溯的 GHCR 镜像，并允许维护者从 Actions 页面以最新代码手动构建。

## ADDED Requirements

### Requirement: tag 推送自动发布

当推送匹配 `v*` 的 tag 时，发布流水线 MUST 构建 linux/amd64 与 linux/arm64 镜像并推送到 GHCR，标签 MUST 包含该 tag 名称与基于提交的 `sha-<短哈希>`；不含 `-` 的稳定版本 tag MUST 同时更新 `latest`。

#### Scenario: 推送稳定版本 tag

- **WHEN** 仓库推送 tag `v1.2.3`
- **THEN** GHCR 中出现 `v1.2.3`、`latest` 与 `sha-<短哈希>` 三个镜像标签，且均指向同一多架构镜像索引

#### Scenario: 预发布 tag 不更新 latest

- **WHEN** 仓库推送 tag `v1.3.0-beta.1`
- **THEN** GHCR 中出现 `v1.3.0-beta.1` 与 `sha-<短哈希>`，`latest` 保持不变

#### Scenario: tag 构建失败

- **WHEN** tag 推送后构建或测试失败
- **THEN** 工作流失败，且不更新 `latest` 与版本标签

### Requirement: 手动触发最新代码构建

维护者 MUST 能在 GitHub Actions 页面手动触发镜像构建，构建所选分支或提交的最新代码，无需修改仓库文件。

#### Scenario: 手动构建默认分支

- **WHEN** 在 Actions 页面选择默认分支并触发 Docker 工作流
- **THEN** 流水线使用该分支最新提交构建镜像并推送成功

#### Scenario: 手动构建指定分支

- **WHEN** 在 Actions 页面选择功能分支并触发
- **THEN** 流水线使用该分支最新提交构建，并在结果中记录提交哈希

### Requirement: 手动构建标签策略

手动构建 MUST 推送 `latest`、`edge` 与 `sha-<短哈希>` 标签，且 MUST NOT 覆盖任何 `v*` 版本标签。

#### Scenario: 手动构建标签

- **WHEN** 手动触发构建成功
- **THEN** `latest`、`edge` 与 `sha-<短哈希>` 指向本次构建镜像，所有 `v*` 版本标签保持不变

### Requirement: 免额外凭据发布

流水线 MUST 使用仓库自带的 `GITHUB_TOKEN` 完成 GHCR 登录与推送，不要求维护者配置额外的镜像仓库凭据。

#### Scenario: 未配置额外 secrets

- **WHEN** 仓库仅启用默认 `GITHUB_TOKEN` 且未配置 Docker Hub 等凭据
- **THEN** tag 与手动构建均能成功推送 GHCR

### Requirement: 构建来源可追溯

每次发布 MUST 记录源码仓库、提交修订与构建版本，并写入镜像 OCI 标签与工作流摘要，便于从镜像回溯到提交。

#### Scenario: 从镜像回溯提交

- **WHEN** 检查已发布镜像的 OCI 标签
- **THEN** 可读到源码地址、提交 SHA 与构建版本

### Requirement: 与现有发布流程共存

镜像发布 MUST NOT 影响现有 `build.yml` 的二进制、APK 构建与 GitHub Release 发布流程。

#### Scenario: tag 同时触发两条流水线

- **WHEN** 推送 `v*` tag
- **THEN** 二进制/APK Release 流程与镜像发布流程均正常完成，互不阻塞

### Requirement: 发布结果可见

流水线 MUST 在工作流摘要中列出本次发布的镜像地址、标签与提交哈希，失败时说明失败阶段。

```

Full source: docs/openspec/changes/docker-image-support/specs/image-publishing/spec.md
