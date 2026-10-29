# 手动任务 R2 自动上传

## Purpose

让 Web 手动触发的扫描、详细测试、测速与非标任务，以及自动调度扫描，在启用 R2 上传且配置完整时，由服务端自动把符合当前测试阶段筛选规则的结果以 Sub-Store 目标列表上传到配置的 R2 Worker 文件名。手动任务在结果页通过「⬆ 自动上传」下拉框选择是否上传 R2；上传内容开启测速时只包含测速达标行，未开启测速时只包含通过延迟筛选的行。

## Requirements

### Requirement: R2 配置与启用

系统 SHALL 从「自动化」区块保存的 `r2enabled`、`r2workerurl`、`r2workertoken`、`r2filename` 读取 R2 上传配置，保存后无需重启即可生效。Worker token MUST NOT 回传前端。文件名 SHALL 取 `r2filename`，为空时使用 `cfdata-targets.txt`。`r2enabled` 为关闭时，手动任务与自动调度 MUST NOT 发起任何 R2 请求。

### Requirement: 手动任务的结果页选择入口

结果页官方与 NSB 的「⬆ 自动上传」下拉框 SHALL 新增「R2 上传」选项，与 GitHub 账户、「关闭自动上传」同级单选。R2 已启用且 Worker 地址与 token 已保存时，该选项 SHALL 默认选中；用户可改选 GitHub 账户或关闭。只有选中「R2 上传」时，该任务完成节点才执行 R2 上传；选择 GitHub 账户或关闭时 MUST NOT 发起 R2 请求。选择状态 SHALL 随任务参数提交并在服务端任务期间保持，页面关闭或任务转入后台后仍然生效。

### Requirement: 手动任务的完成节点

手动任务的 R2 上传 SHALL 跟随现有完成节点：官方详细测试完成、官方批量测速完成、非标结果生成完成（非标优选、链路复验）、非标批量测速完成。官方优选扫描开启自动测速时，详细测试阶段 MUST NOT 单独上传 R2，只在随后的批量测速完成节点上传一次。每个完成节点 SHALL 只发起一次上传；页面重连或跟随后台任务时的消息重放 MUST NOT 触发重复上传。官方优选扫描未开启自动测速（仅扫描）时 MUST NOT 触发 R2 上传。

### Requirement: 上传内容规则

上传内容 SHALL 为当前任务中符合筛选规则的结果行，固定转换为 Sub-Store 目标列表格式 `host[:port]#alias`：

- 开启测速时，只包含测速达标行（官方 `speed ≥ offspeedmin`，非标下载速度 `≥ nsbspeedmin`）；低于阈值、测速失败与未测速的行 MUST NOT 被上传。
- 未开启测速时，包含全部通过延迟筛选（官方详细测试的 `delay` 阈值或非标扫描的 `nsbdelay` 阈值）与链路复验筛选的结果行。

手动任务上传 MUST NOT 应用结果导出弹窗中的格式、字段、IP 类型、达标筛选或行数设置。当前任务没有任何符合规则的结果行时，系统 MUST NOT 发起上传，R2 上的旧内容 SHALL 保持不变。

### Requirement: 自动调度扫描的统一规则

自动调度官方与非标扫描的 R2 上传 SHALL 应用与手动任务相同的筛选规则：测速开启时只上传达标行，测速未开启时上传通过延迟筛选的详细测试或扫描结果。调度路径的 R2 内容 MUST NOT 受 `outqualified`、`outstartrow`、`outendrow` 影响，同次导出的文件与 GitHub/edgetunnel 上传内容保持原有过滤行为不变。调度的 R2 上传仍由 `r2enabled` 开关控制，与结果页下拉框无关。

### Requirement: 上传失败与空结果处理

R2 上传 SHALL 复用现有 Worker 请求的重试、超时与管理面代理行为。重试后仍失败时，系统 SHALL 记录失败提示并保留 R2 上的旧内容，MUST NOT 改变任务状态、扫描/测试结果或阻塞后续结果展示。已启用 R2 但 Worker 地址或 token 缺失时，系统 SHALL 跳过上传并记录明确提示，不阻塞任务完成。

### Requirement: 敏感信息保护

上传日志与前端提示 MUST NOT 包含 Worker token 明文；token 不得回传前端。

### Requirement: 页面说明

「自动化」区块 SHALL 说明 R2 上传的内容规则（开启测速只传达标行，未开测速只传延迟筛选行）以及结果页「⬆ 自动上传」可选择 R2 的关系。本次变更 MUST NOT 新增 GitHub 账号配置入口，也不改变 GitHub 账户存储方式与 GitHub 自动上传行为。

## 验收映射

本规格的验收项定义在 `brief.md` 的「验收示例」中（A1–A11）：A1 自动测速流程只上传一次、A2 详细测试未测速上传全部延迟筛选行、A3 官方批量测速只上传达标行、A4 非标测速任务只上传达标行、A5 非标未测速上传筛选后全部行、A6 无符合行不覆盖旧文件、A7 下拉框 R2 选项与默认选择、A8 格式/失败/token 保护、A9 调度官方规则与导出解耦、A10 调度非标规则、A11 仅扫描不上传。
