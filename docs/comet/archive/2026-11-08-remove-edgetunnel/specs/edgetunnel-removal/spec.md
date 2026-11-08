# Capability: edgetunnel-removal

## Purpose

CFData-WEB 不再包含任何 edgetunnel 上传链路：结果上传到 edgetunnel（Worker 管理面板 ADD.txt 读写）这一功能整体移除，以减少代码体积、二进制体积与维护负担。本 capability 描述移除后的完整系统行为。

## Requirements

### Requirement: 前端无 edgetunnel 功能

Web 前端（`index.html`、`web/app.js`、`web/styles.css`）SHALL 不再包含任何 edgetunnel 相关内容：

- 「上传账号」页面仅保留 GitHub 账号表单与其保存流程；不再出现 edgetunnel 账号区块（账户选择/删除、账户名称、域名、密码、上传模式、保存密码复选框及其提示）。
- 结果操作弹窗不再包含「edgetunnel 上传」折叠区（账号选择、上传状态、追加/覆盖按钮），不再有「上传 edgetunnel-追加 / 覆盖」按钮及其确认提示。
- 不再有 edgetunnel 账号管理函数、WebCrypto 加密存储函数、WS 消息处理（`edgetunnel_upload_result`/`edgetunnel_upload_status`/`edgetunnel_upload_error`）与样式选择器（`.edgetunnel-field`）。
- 与 GitHub 上传、导出、R2 上传相关的全部现有行为保持不变。

### Requirement: 后端与 CLI 无 edgetunnel 功能

服务端（`server.go`、`protocol.go`、`session.go`）与 CLI（`cli.go`）SHALL 不再包含任何 edgetunnel 相关代码：

- WebSocket 不再支持 `edgetunnel_upload` 请求消息，不再发送 `edgetunnel_upload_result`/`edgetunnel_upload_status`/`edgetunnel_upload_error` 响应；session 消息分类不再引用 edgetunnel 消息类型。
- CLI 不再提供 `-edgetunnel`、`-ethost`、`-etpassword`、`-etmode` 参数及其默认值、合并逻辑与上传函数 `uploadCLIExportToEdgetunnel`。
- 配置文件 `cfdata-config.json` 不再包含 `edgetunnel`/`ethost`/`etpassword`/`etmode` 配置键与对应参数定义条目。
- GitHub、R2 上传与任务执行等其他后端行为保持不变；共享设施（如 `managementHTTPClient`）保留。

### Requirement: 旧数据一次性清理

前端初始化阶段 SHALL 一次性移除浏览器 localStorage 键 `cfdata-edgetunnel-accounts`（历史版本保存的 edgetunnel 账号数据与加密密码不再被读取，也不残留）。

### Requirement: 文档保留

`docs/` 下所有历史记录与工作流产物（openspec 归档与规格、分析报告、comet 归档与已发布 capability 规格）SHALL 保持原样，不因本 capability 改写。

## Scenarios

### Scenario: 前端无残留且账号页回归

打开 Web 控制台，「上传账号」页面仅显示 GitHub 表单；填写并保存 GitHub 账号行为正常，页面加载无 JS 错误；结果操作弹窗的导出、GitHub 上传、R2 上传入口行为与移除前一致。
验收：A1

### Scenario: 后端与 CLI 无残留

`grep -rniE "edgetunnel|EdgeTunnel" combined_refactor/*.go` 与前端 grep 均零命中；`cfdata-config.json` 与 CLI 参数列表（`--help`）中无 edgetunnel/ethost/etpassword/etmode。
验收：A2

### Scenario: 构建与测试通过

`combined_refactor` 下 `go build ./...`、`go vet ./...`、`go test ./...` 全部通过；web 侧 node:test 现有用例通过。
验收：A3

### Scenario: 旧 localStorage 数据被清理

曾使用过 edgetunnel 账号保存的浏览器打开页面后，localStorage 中 `cfdata-edgetunnel-accounts` 键被移除。
验收：A5

### Scenario: 行为回归

本地运行页面，除 edgetunnel 入口消失外，任务执行、结果导出、GitHub 上传、R2 上传等行为与移除前一致。
验收：A4
