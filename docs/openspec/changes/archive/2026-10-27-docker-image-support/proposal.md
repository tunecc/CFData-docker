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
