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
- 完成时间: 2026-11-06T09:14:51.243Z
- 摘要: 实现符合 Spec：无结果时以 192.0.2.x 示例生成 3 行并按当前字段顺序/分隔符/格式渲染并显示「示例」badge；有结果时按规定取行并截断到前 5 行，TXT/CSV/Sub-Store 与导出规则一致；预览框最小约 3 行、超出可滚动；设置变更即时刷新。导出、字段选择与持久化未改动，示例不会混入导出。整体 pass。

## 验收

| 编号 | 结果 | 来源 | 验收项 | 原因 |
| --- | --- | --- | --- | --- |
| A1 | passed | brief.md | A1：没有任何结果行时打开「结果操作」，预览区显示 3 行示例结果（使用当前字段顺序、分隔符与格式），并有「示例」标识，不再是「暂无预览」。 | 无可用行时改用 3 条 TEST-NET 示例（192.0.2.10/.20/.30）并置 resultPreviewShowingSample；index.html 新增「示例」badge，updateResultPreview 仅在示例态显示；右侧结果操作标签进入即刷新，不再停留「暂无预览」。 |
| A2 | passed | brief.md | A2：存在真实结果时预览显示前 5 行（受当前数据来源、筛选与起止行影响），按 TXT 规则每行含 `IP:PORT#字段`；超过 5 行时预览框可滚动。 | buildResultPreview 经 getSelectedResultRows（来源/筛选/起止行）取行后由 takePreviewRows 截断到 5 行；TXT 逐行按 IP:PORT#字段 生成，与导出拼接一致；预览框 min-height 66px、max-height 160px、overflow auto。 |
| A3 | passed | brief.md | A3：切换字段勾选、输出顺序、分隔符或 IPv6 方括号后，示例/真实预览立即按新设置重新生成。 | 字段勾选、顺序、分隔符、IPv6、格式切换均汇聚到 updateResultPreview，示例与真实行共用同一分支，改动后立即重建文本。 |
| A4 | passed | brief.md | A4：CSV 预览显示表头 + 前 5 行，Sub-Store 预览显示前 5 条目标，与导出规则一致。 | CSV 预览为表头 + 前 5 行，复用同一 label/字段顺序/formatCSVCell；Sub-Store 预览直接调用 buildSubstoreContent(rows)，与导出同一函数。 |
| A5 | passed | brief.md | A5：`go build ./...`、`go test ./...` 与前端 Node 测试通过，导出与持久化测试不回归。 | Runtime 记录 candidateId/executionId 对应、时间晚于文件修改且 exitCode 0（go-test ok cfdata 22.517s）；独立重跑 node --test 17/17 通过；导出与持久化代码未被 diff 触及。 |

## 检查

| 检查 | 命令 | 工作目录 | 状态 | 退出码 | 耗时 |
| --- | --- | --- | --- | ---: | ---: |
| go build ./... | build ./... | combined_refactor | passed | 0 | 457 ms |
| go test ./... | test ./... | combined_refactor | passed | 0 | 23534 ms |
| node --test web/test/*.test.js | -c node --test web/test/*.test.js | combined_refactor | passed | 0 | 141 ms |

### Builder 报告的证据

以下为 Builder 报告，不等同于 Runtime 检查凭据或独立验收结果。

- go build ./... (combined_refactor): passed — 退出码 0
- go test ./... (combined_refactor): passed — ok cfdata 22.4s
- node --test web/test/*.test.js: passed — 17 项通过
- 浏览器实测（本地新构建 52640）: passed — 示例 3 行/真实 5 行/CSV/分隔符/Sub-Store 均符合
- 已知限制: 预览最多 5 行；更多结果不展示，导出仍按起止行与筛选生成完整内容
- 已知限制: 示例预览使用文档保留地址 192.0.2.x，不代表真实扫描结果
- 已知限制: 未覆盖 RTL/超长字段的排版；预览沿用 word-break: break-all
- 已知限制: 字段选择持久化与服务器同步逻辑未改动

## 阻塞项

_无。_

## 风险与跳过的工作

- Node 单测只覆盖 takePreviewRows，未覆盖 buildResultPreview 的示例生成、多行 TXT、CSV 表头+5 行、Sub-Store 分支。
- 示例行只提供部分字段，NSB 默认顺序里的 loc、自定义字段不会出现在示例中。
- 示例 IP 全为 IPv4，切换「IPv6 方括号」时示例预览不变，可能被误以为开关无效。
- .result-preview 的 min-height 同时作用于复制弹窗预览，复制预览高度也变为约 3 行。
- 预览的 TXT/CSV 拼接与 buildResultContent 是重复实现，未来改导出规则可能漂移。
- 记录中的 evidenceDigest 与日志文件明文 sha256 不一致，检查记录按命令、cwd、时间与日志内容核对。

## 之前的迭代

| 目标周期 | 迭代 | 尝试 | 结果 | 未解决项 | 摘要 | 完成时间 |
| ---: | ---: | ---: | --- | --- | --- | --- |
| 1 | 1 | 1 | pass | — | 实现符合 Spec：无结果时以 192.0.2.x 示例生成 3 行并按当前字段顺序/分隔符/格式渲染并显示「示例」badge；有结果时按规定取行并截断到前 5 行，TXT/CSV/Sub-Store 与导出规则一致；预览框最小约 3 行、超出可滚动；设置变更即时刷新。导出、字段选择与持久化未改动，示例不会混入导出。整体 pass。 | 2026-11-06T09:14:51.243Z |



## 结论

实现符合 Spec：无结果时以 192.0.2.x 示例生成 3 行并按当前字段顺序/分隔符/格式渲染并显示「示例」badge；有结果时按规定取行并截断到前 5 行，TXT/CSV/Sub-Store 与导出规则一致；预览框最小约 3 行、超出可滚动；设置变更即时刷新。导出、字段选择与持久化未改动，示例不会混入导出。整体 pass。
