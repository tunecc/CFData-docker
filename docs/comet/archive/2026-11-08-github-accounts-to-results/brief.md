# 目标

edgetunnel 移除后「上传账号」页只剩 GitHub 一个表单，独立占一个配置 tab 浪费空间。把 GitHub 账号配置完整并入「结果操作」页已有的「GitHub 上传」分组，删除「上传账号」tab，右侧配置区从 5 个 tab 缩为 4 个（官方优选、自动化、结果操作、设置）。

调查确认的现状基线：

- 配置 tab 栏（index.html L207-212）有 5 个 tab；`cfgTabAccounts` → `cfgPanelAccounts`（L563-585）持有 GitHub 账号管理表单（账户下拉+删除、账户名称、Token、仓库、分支、目标路径、提交信息、保存 token 复选框、加密存储 hint）与页面级保存按钮（`saveAccountsPage`）。
- 「结果操作」页（`cfgPanelResults`，L659 起）已含 `resultFoldGitHub` 分组（选择已保存账号下拉 `githubAccountPick`、快速上传文件、上传状态）与页面级保存按钮（`saveResultsPage`，当前保存结果面板设置与 R2 配置）。
- `renderGitHubAccounts()` 同时渲染 `githubAccountSelect`（管理面板）与 `githubAccountPick`（结果操作）两个下拉并保持同步；`nzShowConfigPanel('accounts')` 是 accounts 页的唯一入口。

# 范围

纯前端 UI 重组（index.html / web/app.js），无后端改动。两个账号下拉合并为一个：管理下拉与上传选择统一由保留的下拉承担（选择即编辑对象，同时是上传使用的账号），账号存储、WebCrypto 加密、上传逻辑全部不变。

## 目录结构

### 修改

- combined_refactor/index.html — 删除 `cfgTabAccounts` tab 按钮与 `cfgPanelAccounts` 整个面板；`resultFoldGitHub` 分组扩展承载完整 GitHub 账号管理表单（并入原上传账号页全部字段与 hint）；账号下拉合并为单个（保留 `githubAccountSelect` 语义并移除冗余的 `githubAccountPick`，或保留单一 id 以最小化 JS 改动——以实现时引用最少者为准，行为为两处合一）。
- combined_refactor/web/app.js — `saveResultsPage` 并入 GitHub 账号保存（调用现有 `saveAccountsPage` 的 GitHub 逻辑，空白表单跳过、失败提示语义不变）；`nzShowConfigPanel('accounts')` 与 accounts 面板相关引用清理；`renderGitHubAccounts` 等下拉同步逻辑适配单下拉；其余账号管理/上传函数不变。

### 明确不建

- 不改后端（server.go/protocol.go/session.go）、不改账号存储与 WebCrypto 加密逻辑、不改 GitHub/R2 上传与导出逻辑。
- 不新建独立组件/模块；沿用现有 cfg-group 分组样式。

# 非目标

- 不改账号加密存储、上传协议与导出行为（纯 UI 位置重组）。
- 不动设置页、自动化页等其他配置页。
- 不修改 docs/ 下文档。

# 验收示例

- A1: 配置 tab 栏仅剩 4 个 tab（官方优选、自动化、结果操作、设置），无「上传账号」；点击无残留引用错误。
- A2: 「结果操作」页的 GitHub 上传分组包含完整账号管理字段（账户选择/删除、名称、Token、仓库、分支、目标路径、提交信息、保存 token），可新增、切换、删除账号，保存后加密存储行为与移除前一致（localStorage `cfdata-github-accounts`）。
- A3: 结果操作页「💾 保存」覆盖 GitHub 账号保存：填写账号后点击保存出现既有保存提示；空白表单跳过不报错。
- A4: 上传 GitHub 流程正常：选择已保存账号后字段回显，「上传 GitHub」提交请求语义不变。
- A5: 页面加载无 JS 错误；`go build ./...`、`go vet ./...`、`go test ./...` 与 web node:test 全绿（后端无改动，构建验证兜底）。

# Decisions

- D1: 账号下拉合一——管理选择与上传选择使用同一下拉，消除两处冗余（用户需求「GitHub 配置项放进结果操作」的自然结果）。
- D2: 页面保存入口统一——结果操作页既有「💾 保存」按钮扩展覆盖 GitHub 账号保存，不再保留独立的 accounts 页保存按钮（遵循 config-page-save 既有统一保存模式）。
- D3: 不做 Supervisor 拆分——改动集中在两个前端文件，单 change 可独立验收。
