# Brainstorm Summary

- Change: docker-image-support
- Date: 2026-10-27

## 确认的技术方案

- 根目录多阶段 Dockerfile：`golang:1.26-bookworm` 构建阶段按 `TARGETARCH` 交叉编译 `combined_refactor`；`alpine:3.22` 运行阶段安装 CA/时区，以 UID/GID 10001 非 root 运行。
- entrypoint 启动时将 `/app/cfdata` 原子同步到 `/data/cfdata` 后 `exec`，配置、`.bak`、缓存、ASN 库与日志统一落在 `/data` 卷。
- 健康检查使用未认证的 `/favicon.png`，端口由 `CFDATA_HEALTHCHECK_PORT` 覆盖，默认 13335。
- 独立 `.github/workflows/docker.yml`：`push.tags: v*` 与 `workflow_dispatch`；使用 `GITHUB_TOKEN` 推送 GHCR `ghcr.io/tunecc/cfdata-web`；tag 构建发布版本 tag、`latest`（仅稳定版）与 `sha-<短哈希>`；手动构建发布 `latest`、`edge`、`sha-<短哈希>`；QEMU + buildx 构建 amd64/arm64，GHA 缓存，按 ref 并发控制，输出摘要。
- 新增 `docker-compose.yml`、`docs/docker.md` 与 README Docker 小节；`AGENTS.md` 仅增加 `.github/workflows/docker.yml` 例外。
- Design Doc：`docs/openspec/changes/docker-image-support/design.md`。

## 关键取舍与风险

- Alpine 兼顾镜像体积与健康检查工具；entrypoint 复制而非软链接，避免 `/app` 下 `.bak`/迁移备份写入失败。
- 手动构建按已确认策略覆盖 `latest`，任意分支构建都会影响 `latest`，文档中明确警示。
- GHCR 包首次发布后需手动设为 public；host bind mount 需 `chown 10001:10001`；多架构构建增加 Actions 分钟数。
- 构建阶段不联网下载 CA，使用仓库内 `ca-certificates.crt` 保证可复现；运行镜像安装系统 CA 包。

## 测试策略

- 本地 `docker build` 与 `docker run`：验证非 root、默认端口、健康检查、认证参数、卷持久化、版本输出与多架构清单。
- 静态检查：YAML 解析、`sh -n` 脚本语法、无未替换占位符。
- 合并后由用户手动触发一次 Docker 工作流并推送 `v*` tag，验证 GHCR 标签与现有 Release 流程并存。

## Spec Patch

无。
