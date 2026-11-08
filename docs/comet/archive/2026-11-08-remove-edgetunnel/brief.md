# 目标

把项目（`combined_refactor/`）中所有 edgetunnel 有关的功能代码、CLI 参数与配置条目整体移除，减少代码体积与维护负担。调查确认 edgetunnel 仅以「上传结果到 edgetunnel（Worker 管理面板 ADD.txt）」这一条链路存在，无其他依赖方。

调查到的事实基线（grep 全仓库，排除 `.git/`、`node_modules/`、`docs/`）：

- 前端 `index.html`：上传账号页 edgetunnel 表单块（L586-603）、结果操作弹窗 `resultFoldEdgetunnel` 折叠区（L859-878）、tab 与保存按钮 title 文案（L211、L566）。
- 前端 `web/app.js`：账号存储 key `cfdata-edgetunnel-accounts` 与会话密码 Map（L83-84）、账号管理全套函数（保存/渲染/加载/删除/WebCrypto 加密存储，L3853-3997 及 L960-1022）、结果弹窗上传函数（L4817-4895）、WS 消息处理 `edgetunnel_upload_result/status/error`（L5300-5317）、两处 `renderEdgetunnelAccounts()` 调用（L3360、L5733）。
- 前端 `web/styles.css` L1222：`.github-field, .edgetunnel-field` 共享规则（拆分保留 `.github-field`）。
- 后端 `server.go`：WS handler `edgetunnel_upload`（L734-752）与 `normalizeEdgetunnelHost`/`edgetunnelLogin`/`edgetunnelRead`/`edgetunnelWrite`/`uploadToEdgetunnel` 五个函数（L944-1069）。
- `protocol.go` L107-113：`edgetunnelUploadRequest` 结构体。
- `session.go` L83-85：消息分类中的 `edgetunnel_upload_error`/`edgetunnel_upload_status`。
- CLI `cli.go`：`EdgeTunnel`/`EdgeTunnelSet` 字段与默认值（L87-88、L147、L307、L312）、导出映射与合并逻辑（L316、L390-392）、参数定义 `edgetunnel`/`ethost`/`etpassword`/`etmode`（L608-611）、导出流程上传调用（L1023-1024）、`uploadCLIExportToEdgetunnel`（L1687-1710）。
- 配置 `cfdata-config.json`：`edgetunnel`/`ethost`/`etpassword`/`etmode` 配置键（L56-59 附近）与四个参数定义条目（L414-436）。该文件是 CLI 运行时读写的配置文件（`cli.go:737-739` 解析为默认配置路径）。
- 测试文件中无 edgetunnel 引用；顶层 `index.html`、Docker 相关文件无引用。

# 范围

删除上述全部 edgetunnel 链路；GitHub、R2 上传与其他功能不受影响。用户原始请求「所有的edgetunnel 有关的东西都可以删掉，节省资源」已明确授权删除 CLI 参数与配置键（不另行询问）。

## 目录结构

### 修改

- combined_refactor/index.html — 删除 edgetunnel 上传账号表单块、结果操作弹窗 edgetunnel 折叠区，修正「上传账号」tab 与保存按钮 title 文案。
- combined_refactor/web/app.js — 删除 edgetunnel 账号管理/加密存储/上传/WS 处理函数与存储常量；`saveAccountsPage` 仅保留 GitHub 分支及对应提示；两处 `renderEdgetunnelAccounts()` 调用移除。
- combined_refactor/web/styles.css — 从共享规则中移除 `.edgetunnel-field` 选择器。
- combined_refactor/server.go — 删除 `edgetunnel_upload` WS handler 与 5 个 edgetunnel 上传函数。
- combined_refactor/protocol.go — 删除 `edgetunnelUploadRequest` 结构体。
- combined_refactor/session.go — 消息分类移除 edgetunnel 两项。
- combined_refactor/cli.go — 删除 `EdgeTunnel` 等字段、默认值、映射、合并分支、4 个参数定义与 `uploadCLIExportToEdgetunnel`。
- combined_refactor/cfdata-config.json — 删除 edgetunnel/ethost/etpassword/etmode 配置键与 4 个参数定义条目。

### 明确不建

- 不新建迁移/兼容层模块，不保留任何 edgetunnel 兼容入口。
- 不新增测试文件；验证依赖现有 `go test`、web `node:test` 与本地运行走查。

# 非目标

- 不删除文档中的历史提及（Q2 已确认保留）：`docs/openspec/**`（含归档与当前 `specs/web-console/spec.md`）、`docs/analyses/**`、`docs/superpowers/**`、`docs/comet/**` 均为历史记录或工作流产物，叙述性提及不改写。
- 不重构 GitHub/R2 上传、任务执行等其他功能；`managementHTTPClient` 等共享设施保留。
- 不修改 `docs/comet/specs/config-page-save/spec.md` 等已发布 capability 规格中提及 edgetunnel 表单的历史要求（属历史能力记录）。

# 验收示例

- A1: 前端无残留：`grep -rni edgetunnel combined_refactor/index.html combined_refactor/web/` 零命中；上传账号页仅剩 GitHub 表单且保存流程正常，页面加载无 JS 错误。
- A2: 后端与 CLI 无残留：`grep -rniE "edgetunnel|EdgeTunnel" combined_refactor/*.go` 零命中；`--help` 参数列表与 `cfdata-config.json` 中均无 edgetunnel/ethost/etpassword/etmode。
- A3: 构建与测试：`combined_refactor` 下 `go build ./...`、`go vet ./...`、`go test ./...` 全部通过；web 侧 node:test 现有用例通过。
- A4: 行为回归：本地运行页面，结果操作弹窗导出、GitHub 上传入口、R2 上传入口行为与删除前一致（编译 + 走查证据）。
- A5: 页面加载后浏览器 localStorage 中 `cfdata-edgetunnel-accounts` 键被一次性移除。

# Decisions

- D1: CLI 参数（-edgetunnel/-ethost/-etpassword/-etmode）、`cfdata-config.json` 配置键与参数定义一并删除——用户原始请求「所有edgetunnel 有关的东西都可以删掉」已明确覆盖。
- D2: 不做 Supervisor 拆分——删除目标单一、改动区域集中（同一条上传链路），单 change 可独立完成与验收。
- D3: Q1 已确认——前端初始化阶段一次性清理 localStorage 键 `cfdata-edgetunnel-accounts`（用户选择「一次性清理」）。
- D4: Q2 已确认——docs/ 下文档提及全部保留不动，不修改任何文档（用户选择「全部保留不动」）。
