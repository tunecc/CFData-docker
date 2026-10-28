# 手动任务 R2 自动上传

## Purpose

让 Web 页面手动触发并产生最终结果的扫描、详细测试、测速与非标任务，在启用 R2 上传且配置完整时，由服务端自动把当前任务的全部结果以 Sub-Store 目标列表上传到配置的 R2 Worker 文件名；同时向用户明确 R2 开关与 GitHub「自动上传」下拉框各自的作用范围。

## Requirements

### Requirement: 服务端在手动任务完成时上传

启用 R2 上传且配置完整时，系统 SHALL 在 Web 手动任务的完成节点由服务端直接执行 R2 上传，不依赖页面保持打开或前端发起请求。任务转入后台运行后，上传 MUST 仍然生效。

### Requirement: 完成节点与触发次数

手动任务的 R2 上传 SHALL 跟随现有 GitHub 自动上传的完成节点：官方详细测试完成（`test_complete`）、官方批量测速完成（`official_speed_complete`）、非标结果生成完成（`nsb_csv_complete`）、非标批量测速完成（`nsb_speed_complete`）。每个完成节点 SHALL 只发起一次上传；页面重连或跟随后台任务时的消息重放 MUST NOT 触发重复上传。

### Requirement: 上传内容为全部结果的 Sub-Store 目标列表

手动任务上传的内容 SHALL 是当前任务的全部结果行，固定转换为 Sub-Store 目标列表格式 `host[:port]#alias`。上传 MUST NOT 应用结果导出弹窗中的格式、字段、IP 类型、达标筛选或行数设置。文件名 SHALL 取配置的 `r2filename`，为空时使用 `cfdata-targets.txt`，成功时覆盖 R2 上的同名文件。

### Requirement: 开关与配置判断

`r2enabled` 为关闭时，手动任务 MUST NOT 发起任何 R2 请求，任务行为与现状一致。已启用但 Worker 地址或 token 缺失时，系统 SHALL 跳过上传并记录明确提示，不阻塞任务完成。R2 配置 SHALL 与「自动化」区块保存的同一配置一致，保存后无需重启即可生效。

### Requirement: 上传失败与空结果处理

R2 上传 SHALL 复用现有 Worker 请求的重试、超时与管理面代理行为。重试后仍失败时，系统 SHALL 记录失败提示并保留 R2 上的旧内容，MUST NOT 改变任务状态、扫描/测试结果或阻塞后续结果展示。当前任务没有结果行时，系统 MUST NOT 发起上传。

### Requirement: 敏感信息保护

上传日志与前端提示 MUST NOT 包含 Worker token 明文；token 不得回传前端。

### Requirement: 自动调度路径保持

自动调度扫描的 R2 上传内容、文件名与触发时机 MUST NOT 因本次变更改变；手动任务上传 MUST NOT 与调度路径叠加产生重复请求。

### Requirement: 页面说明与 GitHub 入口保持

「自动化」区块 SHALL 说明 R2 自动上传同时覆盖自动调度与手动任务。结果页「⬆ 自动上传」下拉框 SHALL 继续仅列出浏览器 localStorage 中的 GitHub 账户，由 GitHub 账户决定是否自动上传及目标；R2 上传由「启用 R2 上传」开关控制，两者互不依赖。本次变更 MUST NOT 新增 GitHub 账号配置入口，也不改变 GitHub 账户存储方式。

## 验收映射

本规格的验收项定义在 `brief.md` 的「验收示例」中（A1–A8），分别对应官方扫描测试完成上传、官方测速完成上传、非标扫描与测速上传、未启用跳过、配置缺失跳过、上传失败保留旧内容、调度路径不重复、下拉框与说明文案保持。
