---
generated_from_state_version: 12
---

# 验证

## 当前结果

- 结果: **已归档**
- 验证情况: **已完成检查，验证结果已确认**
- 目标周期: 1
- 迭代: 2
- 验证器尝试次数: 1
- 完成时间: 2026-11-06T07:49:40.765Z
- 摘要: 第二轮候选已修复上一轮两处 Spec MUST 偏差：损坏/不可读设置文件按空配置处理并由本地设置自愈；自动同步连接未就绪时一次性可见提示且成功同步重置；suppressScheduleSavedNotice 增加 5 秒超时兜底。429 退避、自动换源与连续 6 个终止均未回归。A1–A8 全部 passed，独立重跑构建与前后端测试通过，未对仓库产生改动。

## 验收

| 编号 | 结果 | 来源 | 验收项 | 原因 |
| --- | --- | --- | --- | --- |
| A1 | passed | brief.md | A1：结果操作页改动字段勾选/顺序/自定义字段/格式/来源/行范围/筛选/分隔符/IPv6 后，刷新页面设置全部保留；右上角「保存」同样落盘并提示成功。 | 字段勾选/顺序/自定义字段/格式/来源/行范围/筛选/分隔符/IPv6 均汇入 updateResultPreview 或 renderResultFieldOptions→saveResultPanelSettings（写 localStorage + 500ms 防抖同步）；右上角保存 flush 后由 result_panel_config_saved 显示「结果操作配置已保存」。 |
| A2 | passed | brief.md | A2：换浏览器，或改用另一个访问地址（如 `http://localhost:端口` 与 `http://局域网IP:端口`）打开页面，仍恢复服务器上最后一次保存的结果面板设置。 | server.go 提供 get/save_result_panel_config，写入与 cfdata-config.json 同目录的 result-panel.json（容器 /data）；init_config 后前端请求并 applyServerResultPanelSettings 以服务器值覆盖本地并回写缓存。 |
| A3 | passed | brief.md | A3：服务器没有设置记录、但浏览器本地已有旧设置时，首次加载沿用本地设置并自动写入服务器（迁移），刷新与换浏览器后不再丢失。 | readResultPanelSettings 对缺失、不可读与 JSON 损坏均返回 (nil,nil) 并打印日志；前端收到空设置时调用 migrateLocalResultPanelSettings 上送本地旧设置，实现自愈；对应测试通过。 |
| A4 | passed | brief.md | A4：服务器写入失败（如 `/data` 不可写）时界面给出可见提示，不显示保存成功；本地 localStorage 仍保留当前设置。 | wsSend 失败时手动保存每次提示、自动同步首次提示并置标记、成功同步清零；服务器写入失败返回 saved:false+error 并显示错误不显示成功；suppressScheduleSavedNotice 有 5 秒超时兜底。上一轮两处 MUST 偏差已修复。 |
| A5 | passed | brief.md | A5：批量测速中单个 IP 返回 429 时，先等待 3 秒重试、仍 429 再等 6 秒重试；重试成功则正常记录速度并按阈值判定达标。 | speedRateLimitBackoff={3s,6s}，最多重试 2 次，sleepCtx 响应取消；重试成功进入正常记录与达标判定；三个退避测试均通过。 |
| A6 | passed | brief.md | A6：重试后仍为 429 时自动切换到另一个可用测速源继续测速，日志说明「已切换测速源」。 | 仍 429 时从 speedURLCandidates 选与当前不同、未尝试且非不可用的源并更新 currentURL；onSourceSwitch 输出「测速源触发限流，已切换测速源：A → B」，不改用户配置。 |
| A7 | passed | brief.md | A7：连续多个 IP（6 个）在换源后仍全部 429 时才停止批量测速，并提示已尝试更换测速源，达标统计与已测结果保持一致。 | 阈值为连续 6 个 429，非 429 或成功即清零；终止日志在换源时追加「（已尝试更换测速源）」；结果按 IP 合并，qualified/totalQualified 与 rateLimited 如实反映；两个核心测试通过。 |
| A8 | passed | brief.md | A8：`go build ./...`、`go test ./...` 与前端 Node 测试通过；新增测试覆盖 429 退避/换源/终止与设置文件读写。 | 独立重跑 go build、go test（ok cfdata 22.4s）、node --test（13/13）均通过；新增测试覆盖退避/取消/换源/终止/计数重置与设置文件往返/缺失/损坏/字段清洗。 |

## 检查

| 检查 | 命令 | 工作目录 | 状态 | 退出码 | 耗时 |
| --- | --- | --- | --- | ---: | ---: |
| go build ./... | build ./... | combined_refactor | passed | 0 | 411 ms |
| go test ./... | test ./... | combined_refactor | passed | 0 | 23786 ms |
| node --test web/test/*.test.js | -c node --test web/test/*.test.js | combined_refactor | passed | 0 | 150 ms |

### Builder 报告的证据

以下为 Builder 报告，不等同于 Runtime 检查凭据或独立验收结果。

- go build ./... (combined_refactor): passed — 退出码 0
- go test ./... (combined_refactor): passed — ok cfdata 22.4s
- node --test web/test/*.test.js: passed — 13 项通过
- 浏览器实测（本地新构建 52631）: passed — 损坏文件自愈、离线自动同步提示一次
- 已知限制: 429 退避真实环境端到端未实测，依赖注入 runner 的单元测试
- 已知限制: 服务器设置对所有浏览器共享，多人同时使用会互相覆盖
- 已知限制: 500ms 防抖窗口内刷新可能被服务器旧值覆盖（符合服务器权威设计）
- 已知限制: applyResultPanelSettingsFields 的 setVal 跳过空字符串，空值不回填（沿用旧恢复语义）
- 已知限制: gofmt 报告的 geo_warning.go 为仓库既有未格式化文件，本次未改动

## 阻塞项

_无。_

## 风险与跳过的工作

- A1–A4 为代码路径 + 单元/前端 Node 测试核验，未做真实浏览器 E2E；建议部署实例手动再走一遍。
- 窄竞态：主动保存与已发出的防抖自动保存共用无请求 ID 的 result_panel_config_saved 响应，服务器响应乱序且写盘失败叠加时可能提前弹出成功提示。
- 测速地址为「自动选择」时，换源日志可能显示「自定义 → Cloudflare」等标签差异，不影响配置与换源。
- brief 计划改 protocol.go 与 speed_url.go，实际改为 server.go 内联处理器与 tasks_speed.go 内联 switchSource，功能等价但与文件级计划有偏差。
- get_result_panel_config 的 error 分支在 readResultPanelSettings 永不返回 error 后成为死代码，无功能影响。

## 之前的迭代

| 目标周期 | 迭代 | 尝试 | 结果 | 未解决项 | 摘要 | 完成时间 |
| ---: | ---: | ---: | --- | --- | --- | --- |
| 1 | 1 | 1 | pass | — | 候选实现达到 A1–A8：结果面板设置落盘服务器并在连接后以服务器值恢复，支持旧本地设置迁移与写入失败提示；429 退避重试与自动换源、连续 6 个才终止均按 Spec 实现并有单元测试。整体判定 pass；存在两处与 Spec MUST 的边角偏差（损坏文件未按空配置处理、自动同步离线静默），建议修复。 | 2026-11-06T07:16:40.864Z |
| 1 | 1 | 1 | recovery | — | 用户选择先修两处 Must 偏差：损坏配置文件按空配置处理并允许本地迁移自愈；自动同步在连接未就绪时给出可见提示；另给 suppressScheduleSavedNotice 加超时清理。修复后重新验收。 | 2026-11-06T07:16:59.587Z |
| 1 | 2 | 1 | pass | — | 第二轮候选已修复上一轮两处 Spec MUST 偏差：损坏/不可读设置文件按空配置处理并由本地设置自愈；自动同步连接未就绪时一次性可见提示且成功同步重置；suppressScheduleSavedNotice 增加 5 秒超时兜底。429 退避、自动换源与连续 6 个终止均未回归。A1–A8 全部 passed，独立重跑构建与前后端测试通过，未对仓库产生改动。 | 2026-11-06T07:49:40.765Z |



## 结论

第二轮候选已修复上一轮两处 Spec MUST 偏差：损坏/不可读设置文件按空配置处理并由本地设置自愈；自动同步连接未就绪时一次性可见提示且成功同步重置；suppressScheduleSavedNotice 增加 5 秒超时兜底。429 退避、自动换源与连续 6 个终止均未回归。A1–A8 全部 passed，独立重跑构建与前后端测试通过，未对仓库产生改动。
