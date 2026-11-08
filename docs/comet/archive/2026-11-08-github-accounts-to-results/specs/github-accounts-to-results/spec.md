# Capability: github-accounts-to-results

## Purpose

「上传账号」配置页并入「结果操作」页：GitHub 账号配置完整呈现于结果操作页的「GitHub 上传」分组，「上传账号」tab 取消，右侧配置区由 5 个 tab 缩为 4 个。纯前端 UI 位置重组，账号存储、加密与上传行为不变。

## Requirements

### Requirement: 配置导航精简

配置 tab 栏 SHALL 只保留 4 个 tab：官方优选、自动化、结果操作、设置；「上传账号」tab 与其面板 SHALL 移除，页面内不再有任何指向 accounts 面板的入口或残留引用。

### Requirement: 结果操作页承载完整 GitHub 账号管理

「结果操作」页的「GitHub 上传」分组 SHALL 包含完整 GitHub 账号管理表单：账户选择与删除、账户名称、GitHub Token、仓库 owner/repo、分支、目标路径、提交信息、保存 token 复选框与加密存储说明；原有「选择已保存账号」与账号管理两处下拉 SHALL 合并为单一账号下拉（选择即编辑对象，同时为上传使用的账号）；账号选择与字段回显、新增、删除、保存行为与合并前一致。

### Requirement: 统一保存入口

「结果操作」页右上角「💾 保存」SHALL 同时保存结果面板设置、R2 配置与已填写的 GitHub 账号（空白表单跳过、校验失败提示语义与合并前一致）；保存成功提示语义不变。

### Requirement: 上传行为不变

「上传 GitHub」流程 SHALL 与合并前一致：使用当前账号表单信息（或已保存账号）上传，上传状态展示、失败重试提示不变；账号存储键 `cfdata-github-accounts` 与 WebCrypto 加密存储逻辑不变。

## Scenarios

### Scenario: 导航精简

打开页面，配置 tab 栏只有官方优选、自动化、结果操作、设置 4 个 tab；点击各 tab 正常切换，无 JS 错误。
验收：A1

### Scenario: 账号管理在结果操作页可用

在结果操作页 GitHub 上传分组填写新账号（名称/Token/仓库）并保存，账号写入 `cfdata-github-accounts` 加密存储；下拉切换账号时字段回显；删除账号后下拉更新。
验收：A2

### Scenario: 统一保存

填写账号后点击「💾 保存」，出现与合并前一致的保存提示；空表单点击保存不报错且提示「请先填写要保存的 GitHub 账号信息」语义不变。
验收：A3

### Scenario: 上传回归

选择已保存账号，「上传 GitHub」提交请求（Token/仓库/路径/提交信息语义不变），上传状态正常展示。
验收：A4

### Scenario: 构建与运行健康

页面加载无 JS 错误；`go build ./...`、`go vet ./...`、`go test ./...` 通过，web node:test 现有用例通过。
验收：A5
