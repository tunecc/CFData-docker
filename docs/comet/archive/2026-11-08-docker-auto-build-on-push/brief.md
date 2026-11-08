# Outcome

`.github/workflows/docker.yml` 增加自动构建触发：每次推送到 `main` 分支都自动构建并发布 GHCR 镜像（与现有 `v*` 标签触发、手动 workflow_dispatch 并存）。用户推送惯例为直接推 main（git 历史全部为 main 单分支），"每一次推送"即每次推 main。

具体行为：

- 触发：`push` 增加 `branches: [main]`，保留 `tags: ["v*"]` 与 `workflow_dispatch`。
- 推 main 的版本号与手动触发一致：`edge-<short-sha>`；镜像标签复用现有体系：`edge` + `sha-XXXXXXX`，不移动 `latest`（latest 仍仅在稳定版标签 `v*` 无连字符时与手动触发时更新）。
- 质量门禁保持：vet + test 不通过则不构建镜像（现有步骤不变）。
- 其余（QEMU/Buildx、GHCR 登录、双平台、缓存、摘要输出）全部不变。

# Scope

## 目录结构

### 修改

- `.github/workflows/docker.yml` — push 触发增加 `branches: [main]`；Compute version 步骤为 push-to-main 生成 `edge-<short-sha>`；metadata-action 的 `edge` 标签 enable 条件扩展为「手动触发或推送 main」。

### 明确不建

- 不为非 main 分支添加构建（用户无分支工作流，避免 GHCR 被分支标签污染）。
- 不改动 latest 标签语义、并发组、双平台、缓存与摘要步骤。

# Non-goals

- 不修改其他 `.github/` 文件（AGENTS.md 约束：仅 docker.yml 可维护）。
- 不改变 Dockerfile、镜像名、双平台矩阵与质量门禁逻辑。

# Acceptance examples

- **A1 触发配置**：`on.push` 同时包含 `branches: [main]` 与 `tags: ["v*"]`，`workflow_dispatch` 保留；YAML 语法有效（解析通过）。
- **A2 版本与标签**：推 main 时 version 计算 `edge-<short7-sha>`（与 workflow_dispatch 分支一致）；`edge` 标签在「手动触发或推送 main」时启用；`latest` 启用条件不变（手动触发或无连字符稳定标签）；`type=ref,event=tag` 仅标签推送启用；`sha-` 短标签不受影响。
- **A3 其余不变**：质量门禁（vet+test）、QEMU/Buildx、GHCR 登录、双平台 amd64+arm64、gha 缓存、构建摘要步骤与现有完全一致；`permissions`、`concurrency` 不变。
- **A4 结构核对**：改动仅涉及 `.github/workflows/docker.yml` 一个文件（AGENTS.md 例外范围）。

# Constraints and invariants

- AGENTS.md：仅 `.github/workflows/docker.yml` 可维护，其余 `.github/` 禁止修改。
- workflow YAML 需保持与现有 action 版本（actions/checkout@v7、setup-go@v6、docker/* v4/v6/v7）兼容，不升级 action 版本。

# Decisions

- **D1 推送范围 = main 分支**：用户 git 历史全部为 main 直接推送，"每一次推送"按推 main 理解；非 main 分支不构建，避免 GHCR 分支标签污染（Shape 确认时向用户明示）。
- **D2 推 main 版本号沿用 edge 语义**：`edge-<short-sha>`，与手动触发一致；`latest` 不随每次推送移动，仍绑定稳定标签与手动触发。
- **D3 并发组不变**：`docker-${{ github.ref }}` 且不取消进行中构建——同一 ref 顺序排队，保持既有行为。
