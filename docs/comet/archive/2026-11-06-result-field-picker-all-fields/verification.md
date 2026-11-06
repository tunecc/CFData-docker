---
generated_from_state_version: 8
---

# 验证

## 当前结果

- 结果: **已归档**
- 验证情况: **已完成检查，验证结果已确认**
- 目标周期: 1
- 迭代: 1
- 验证器尝试次数: 1
- 完成时间: 2026-11-06T02:48:07.358Z
- 摘要: 候选实现将结果面板字段清单从“按当前结果行裁剪”改为固定的 RESULT_FIELDS 全量 + 自定义字段：renderResultFieldOptions、getAllResultFieldOrder、getAvailableResultFieldKeys 全部改用 common.js 中无 DOM 依赖的 buildResultFieldChoices/resultFieldChoiceKeys。以实际源码执行确认无结果行时渲染 26 个内置字段 + 自定义字段，有/无结果行清单完全一致，TXT 默认勾选不变；全选/仅 IP:PORT/默认选择语义正确；导出拼接逻辑未改动。Runtime 三项检查对应当前候选且均通过，独立重跑 Node 测试 10/10 通过。整体判定 pass。

## 验收

| 编号 | 结果 | 来源 | 验收项 | 原因 |
| --- | --- | --- | --- | --- |
| A1 | passed | brief.md | A1：在前端没有任何结果行时（刷新页面或重启程序后未运行任务）打开「结果操作」面板，字段选择列出全部内置字段（26 个，含 `IP地址`、`端口号`、`延迟`、`数据中心`、`落地区域` 等）与已定义的自定义字段，不再只有 `IP:PORT`。 | renderResultFieldOptions 改为 buildResultFieldChoices(RESULT_FIELDS, resultCustomFields)，不再调用 getCurrentResultRows。以实际函数源码在 Node 中执行、getCurrentResultRows 返回 [] 时渲染 27 个 checkbox（26 个内置 + 1 自定义），包含 IP地址/端口号/网络延迟/数据中心/落地区域，自定义字段位于末尾。无结果行时通过「结果操作」标签进入：enhance.js nzShowConfigPanel('results') -> syncResultSourceOptions() -> renderResultFieldOptions()，该路径不依赖结果行。 |
| A2 | passed | brief.md | A2：在已有扫描结果时打开面板，字段选择列出与无结果行时相同的完整字段清单；默认勾选仍为 `IP:PORT`、`数据中心`、`落地区域`（TXT），输出顺序列表与预览不受影响。 | 同一函数在 rows=[] 与 rows=[{ip,port,dc}] 两种输入下产生的 picker HTML 完全一致（均 27 个 checkbox）。TXT 默认勾选实测为 ['ipport','dc','dcCountry']。git diff 显示 app.js 仅改动 getAllResultFieldOrder、renderResultFieldOptions、getAvailableResultFieldKeys 三处，getDefaultResultFieldOrderFor、renderResultOrderList、预览路径未改动。 |
| A3 | passed | brief.md | A3：「全部选」在无结果行时选中全部字段，如齐全时按钮显示「全部选」；「仅 IP:PORT」把输出顺序收敛为 `IP:PORT`；「默认选择」恢复当前格式的默认勾选。 | getAllResultFieldOrder() 实跑返回 26 个唯一 key（ipport 首、timestamp 末）；toggleAllResultFields 后 26 项全部选中；按钮文案未全选时为「全选」，全选后为「全部选」且 title 为「恢复默认选择」；selectIPPortOnly() 实测 resultFieldOrder=['ipport']；selectDefaultResultFields() 实测为 ['ipport','dc','dcCountry']。这些函数均不再读取结果行。 |
| A4 | passed | brief.md | A4：勾选当前结果中没有数据的字段后导出 TXT/CSV，对应列输出空值，拼接规则与现有导出一致；未勾选字段仍不出现。 | buildResultContent 未改动。用实际源码执行：CSV 勾选当前行无数据的 latency 时输出表头 'IP:PORT,网络延迟,数据中心' 与行 ',,SJC'（对应列空值），未勾选的 dc 不出现；TXT 中勾选但无数据的字段按现有规则过滤，未勾选字段不出现，分隔符/IPv6/起止行逻辑未触及。 |
| A5 | passed | brief.md | A5：`combined_refactor` 下 `go build ./...` 与 `go test ./...` 通过，前端 Node 测试通过，现有结果面板与导出测试不回归。 | Runtime 检查记录 operationId 2981890a-767f-4234-97eb-d672d8bd4370、candidateId 与 workspace 均匹配当前候选，时间晚于候选文件 mtime；日志显示 go build exit 0、go test 'ok cfdata 4.392s'、node --test 10/10 通过。独立重跑 node --test web/test/*.test.js 仍为 10 pass / 0 fail。按指令未重跑匹配的 Go 检查。 |

## 检查

| 检查 | 命令 | 工作目录 | 状态 | 退出码 | 耗时 |
| --- | --- | --- | --- | ---: | ---: |
| go build ./... | build ./... | combined_refactor | passed | 0 | 401 ms |
| go test ./... | test ./... | combined_refactor | passed | 0 | 5411 ms |
| node --test web/test/*.test.js | -c node --test web/test/*.test.js | combined_refactor | passed | 0 | 130 ms |

### Builder 报告的证据

以下为 Builder 报告，不等同于 Runtime 检查凭据或独立验收结果。

- go build ./... (combined_refactor): passed — 开发期运行，退出码 0
- go test ./... (combined_refactor): passed — 开发期运行：ok cfdata
- node --test web/test/common.test.js web/test/result-fields.test.js: passed — 开发期运行：10 项通过
- 浏览器实测（localhost 新构建）: passed — 字段清单 26 项、默认勾选与三个快捷按钮行为符合验收
- 已知限制: 复制弹窗（一键复制）的字段选择仍按当前结果行过滤，本次按非目标未改动
- 已知限制: 重启后历史记录仍只恢复统计摘要，不提供全量结果行导出
- 已知限制: TXT 导出对勾选但无数据的字段仍按现有规则跳过，不输出占位空值

## 阻塞项

_无。_

## 风险与跳过的工作

- Verifier 未做真实浏览器端到端实测，结论来自源码执行、DOM 桩与代码路径核对；建议归档前按 brief 验证预期手工点一遍。
- combined_refactor/web/test/result-fields.test.js 目前为未跟踪文件，提交时需纳入。
- Node 单测使用合成字段数组，未锁定真实 RESULT_FIELDS 的 26 项数量与 app.js 接线。
- 「全部选」现在按 RESULT_FIELDS 全量顺序包含 ipport，与此前官方/非标分支不同，符合 D3/A3 意图但属可见行为变化。
- getDefaultResultFieldOrderFor 仍保留按结果行裁剪的兜底分支，仅对 txt/csv/substore 以外格式可达，当前 UI 无此类格式。

## 之前的迭代

| 目标周期 | 迭代 | 尝试 | 结果 | 未解决项 | 摘要 | 完成时间 |
| ---: | ---: | ---: | --- | --- | --- | --- |
| 1 | 1 | 1 | pass | — | 候选实现将结果面板字段清单从“按当前结果行裁剪”改为固定的 RESULT_FIELDS 全量 + 自定义字段：renderResultFieldOptions、getAllResultFieldOrder、getAvailableResultFieldKeys 全部改用 common.js 中无 DOM 依赖的 buildResultFieldChoices/resultFieldChoiceKeys。以实际源码执行确认无结果行时渲染 26 个内置字段 + 自定义字段，有/无结果行清单完全一致，TXT 默认勾选不变；全选/仅 IP:PORT/默认选择语义正确；导出拼接逻辑未改动。Runtime 三项检查对应当前候选且均通过，独立重跑 Node 测试 10/10 通过。整体判定 pass。 | 2026-11-06T02:48:07.358Z |



## 结论

候选实现将结果面板字段清单从“按当前结果行裁剪”改为固定的 RESULT_FIELDS 全量 + 自定义字段：renderResultFieldOptions、getAllResultFieldOrder、getAvailableResultFieldKeys 全部改用 common.js 中无 DOM 依赖的 buildResultFieldChoices/resultFieldChoiceKeys。以实际源码执行确认无结果行时渲染 26 个内置字段 + 自定义字段，有/无结果行清单完全一致，TXT 默认勾选不变；全选/仅 IP:PORT/默认选择语义正确；导出拼接逻辑未改动。Runtime 三项检查对应当前候选且均通过，独立重跑 Node 测试 10/10 通过。整体判定 pass。
