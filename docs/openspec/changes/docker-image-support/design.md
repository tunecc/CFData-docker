---
comet_change: docker-image-support
role: technical-design
canonical_spec: openspec
---

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

- 在“禁止修改的文件”中增加例外：允许维护 `.github/workflows/docker.yml` 及其 Docker 配套文件，其余 `.github/` 文件仍禁止修改。
- 不改动 AGENTS.md 中现有的编译与 Git 规则。

## Risks / Trade-offs

- 多架构构建耗时增加 → 仅在 tag 与手动触发时构建，启用 GHA 缓存；arm64 运行阶段由 QEMU 执行少量包安装。
- 手动构建会把任意分支的代码推为 `latest` → 文档明确提示；需要稳定版本时使用版本 tag，测试用途使用 `edge` 或 `sha-*`。
- entrypoint 复制二进制存在多容器共享同一卷时的竞争 → 使用临时文件 + 原子 `mv`，文档建议一个数据卷只运行一个容器。
- 宿主机 bind mount 目录可能为 root 属主 → 文档说明需 `chown 10001:10001`；命名卷会自动继承镜像内属主。
- GHCR 包首次发布默认私有 → 首次手动构建后由仓库所有者将包可见性改为 public，文档记录该步骤。
- 仓库内 CA 包可能随时间过期 → 同步上游或定期刷新 `combined_refactor/ca-certificates.crt`；运行镜像的系统 CA 包随基础镜像更新。
- 非标准 tag（含 Docker 不允许的字符）会导致推送失败 → 沿用现有 `v*` 语义化 tag 约定。
- 修改 `-port` 但未设置 `CFDATA_HEALTHCHECK_PORT` 会显示 unhealthy → 文档与 compose 注释中同时给出。
- README 与 AGENTS.md 的小改动可能与上游冲突 → 保持最小段落，冲突时保留双方内容即可。

## Migration Plan

1. 在 change 分支完成实现并通过本地构建、运行、健康检查、数据持久化与版本验证。
2. 合并后在 Actions 页面手动运行一次 Docker 工作流，确认 GHCR 出现 `latest`、`edge` 与 `sha-*`，随后把包可见性设为 public。
3. 推送下一个 `v*` tag，确认版本 tag、`latest` 与现有 Release 流程同时正常。
4. 回滚策略：禁用或回退工作流即可停止发布；已发布镜像可保留或删除；不影响应用数据与现有 Release 资产。
5. 上游同步：新增文件不参与冲突；仅需处理 README 与 AGENTS.md 的少量上下文冲突。

## Open Questions

无阻塞问题；镜像签名、SBOM 与 Docker Hub 发布留作后续独立需求。
