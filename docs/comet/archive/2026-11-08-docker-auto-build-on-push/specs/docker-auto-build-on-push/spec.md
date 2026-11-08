# Capability: docker-auto-build-on-push

## Purpose

CFData-WEB 的 GHCR 镜像流水线（`.github/workflows/docker.yml`）在每次推送到 `main` 分支时自动构建并发布镜像，与既有 `v*` 标签发布、手动 workflow_dispatch 并存。动机：用户直接推 main 的开发节奏下，无需打标签或手动触发即可让 GHCR 上的 `edge` 镜像跟随最新提交。完整动机与取舍见 `docs/comet/changes/docker-auto-build-on-push/brief.md`。

## Requirements

### Requirement: push 触发自动构建

工作流 SHALL 在 `on.push` 中同时声明 `branches: [main]` 与 `tags: ["v*"]`，并保留 `workflow_dispatch`；推送到 main 的每次提交 SHALL 触发一次构建发布流程，非 main 分支推送不触发。

### Requirement: 推 main 的版本与标签语义

推送到 main 时 SHALL：

- 版本号计算为 `edge-<提交短 SHA 前 7 位>`（与 workflow_dispatch 语义一致）；
- 启用 `edge` 镜像标签（enable 条件 = 手动触发或推送 main）与 `sha-<short>` 标签；
- 不启用 `latest` 与 `type=ref,event=tag`（二者维持现有条件：latest 仅手动触发或无连字符稳定标签；ref 标签仅标签推送）。

### Requirement: 既有行为保持

除触发与版本/标签条件外，工作流 SHALL 保持不变：

- 质量门禁（`go vet ./...` + `go test -count=1 ./...`，不通过不构建）；
- QEMU/Buildx 初始化、GHCR 登录、双平台 linux/amd64+linux/arm64、gha 构建缓存；
- `permissions`、`concurrency`（`docker-${{ github.ref }}`，不取消进行中构建）与构建摘要步骤；
- 不升级任何 action 版本。

## Scenarios

### Scenario: 触发配置正确

工作流 `on.push` 同时声明 `branches: [main]` 与 `tags: ["v*"]`，`workflow_dispatch` 保留，YAML 解析通过。
验收：A1

### Scenario: 推送 main 自动构建

推送到 `refs/heads/main` 触发工作流：质量门禁通过后构建 `linux/amd64,linux/arm64` 镜像并发布，版本为 `edge-<short7-sha>`，标签含 `edge` 与 `sha-<short>`、不含 `latest` 与 ref 标签；推送 `v*` 标签仍按原语义构建（ref 标签 + 稳定标签含 latest）。
验收：A2

### Scenario: 其余配置保持

质量门禁、QEMU/Buildx、GHCR 登录、双平台、缓存、permissions、concurrency、摘要步骤与改动前一致，不升级任何 action 版本。
验收：A3

### Scenario: 改动范围受控

改动仅涉及 `.github/workflows/docker.yml` 一个文件。
验收：A4
