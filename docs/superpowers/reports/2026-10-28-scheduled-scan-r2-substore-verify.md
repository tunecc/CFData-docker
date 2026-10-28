# 验证报告：scheduled-scan-r2-substore

- 日期：2026-10-28
- 分支：comet/scheduled-scan-r2-substore
- 基线：b312c209edf1ca903c53ec0d959118c2c11d3f9b
- 验证模式：full（`comet state scale`：21 个任务 / 3 个 delta spec 能力，建议 full）
- 审查模式：standard（最终集成审查由独立审查者完成）
- 说明：受项目 AGENTS.md 约束，本次改动尚未提交，验证覆盖当前工作区内容；归档与提交需用户另行授权。本报告是 verify 阶段唯一写入的项目文档。

## 摘要

| 维度 | 结果 |
| --- | --- |
| 完整性 | 21/21 任务完成；3 个能力共 14 条需求全部实现 |
| 正确性 | 全部验收场景有实现与测试或运行时证据；无 CRITICAL/IMPORTANT 问题 |
| 一致性 | 实现遵循 design.md 的调度、R2、Sub-Store 与 operator 决策；偏差与已接受建议见文末 |

## 检查项

| # | 检查 | 结果 | 证据 |
| --- | --- | --- | --- |
| 1 | tasks.md 全部完成 | PASS | `comet state` 任务状态：21/21，下一任务为空，任务映射无缺失 |
| 2 | 改动文件与任务一致 | PASS | 修改：`cli.go`、`scheduler.go`、`schedule_menu.go`、`r2.go`、`server.go`、`main.go`、`menu.go`、`proxy.go`、`index.html`、`go.mod`/`go.sum`、`README.md`、`docs/docker.md`；删除 `schedule.go`；新增 `substore/` 与三个测试文件。未改动 `.github/` 与顶层“原版源码”文件 |
| 3 | 编译通过 | PASS | build check `go -C combined_refactor test -count=1 ./...` exit 0（日志 `.comet/runtime/classic-sdk-checks/39661ef437be04ef1e70941c1a720e06236874949cdc3a7843870c4bbc206f23/02b61eb9-b5eb-4447-9e4a-be27465448b6.log`）；`go vet ./...` 无输出 |
| 4 | 相关测试通过 | PASS | 全量 `go test ./...` 通过（verify guard 以同一命令复跑）；`-count=3 -race` 覆盖调度器/R2 配置写入 4.6s 通过；`node --test substore/cloudflare-targets.test.js` 通过；重建二进制后 E2E 再次通过（见“场景验证”） |
| 5 | 无明显安全问题 | PASS | 无硬编码密钥；`get_config` 经 `buildConfigDataPayload` 统一置空根级与嵌套 `r2workertoken`（`server.go:526`、`server.go:889`），测试 `TestConfigDataPayloadStripsR2Token` 覆盖两条路径；调度配置响应仅返回 `r2WorkerTokenSet` 布尔值（`scheduler.go:278`）；R2 日志/错误脱敏测试 `TestR2UploadErrorDoesNotLeakToken`、`TestR2ErrorRedactsTokenEchoedInResponseBody`；新增 UI 使用 `value`/`textContent`，未用 `innerHTML` 拼接不可信数据 |
| 6 | 最终集成代码审查 | PASS | 独立审查者共三轮（全量 diff、修复后 diff、最终增量），均无 CRITICAL/IMPORTANT；明细见“集成审查” |
| 7 | 核心成功/失败/边界场景 | PASS | E2E：cron 触发→扫描→导出 10 行 substore→R2 上传成功（317/322 字节两次）；停掉 mock 后三次重试失败仍继续调度并保留旧内容；单元测试覆盖空结果、鉴权失败、非法配置、运行中跳过、并发配置写入等边界 |

## 需求覆盖

### scan-scheduling（5 条需求）

| 需求 | 实现 | 测试/证据 |
| --- | --- | --- |
| 调度开关 | `normalizeSchedulePlan` 在 `schedule-enabled=false` 时返回 disabled（`scheduler.go:40`） | `TestSchedulerDisabledDoesNotTrigger`、`TestNormalizeSchedulePlanDisabled` |
| 双调度计划 | cron 优先（表达式先 `TrimSpace`）、非法 cron 回退、间隔钳制（`scheduler.go:40-72`） | `TestNormalizeSchedulePlanCronPrecedence`、`...InvalidCronFallsBack...`、`...IntervalClamp`、`...TrimsCronExpression` |
| 启动与并发行为 | 间隔模式启动立即一次、cron 不立即、`running` 原子标志 + `anyTaskRunning` | `TestSchedulerIntervalTriggersImmediately`、`TestSchedulerCronDoesNotTriggerImmediately`、`TestSchedulerBusySkipsTrigger`、`TestRunScheduledScanSkipsWhenTaskRunning`、`TestSchedulerStartStopImmediately` |
| 配置热更新 | 10s mtime 轮询 + Web 保存同步 `Reload`，不立即触发、不中断扫描（`scheduler.go:400`） | `TestSchedulerReloadAppliesNewPlanWithoutTriggering`、`TestSchedulerMtimeChangeReloadsPlan`、`TestSchedulerReloadUpdatesStatusSynchronously`、`TestSchedulerRecoversAfterInvalidConfigViaPolling` |
| 调度触发完整流水线 | `runScheduledScan` 一次性 `runTaskSync` 包裹完整流水线，失败记录后返回（`scheduler.go:630`） | E2E 成功与失败路径；`TestScheduledExportUploadsSubstoreContent`、`TestSchedulerRecoversFromPanic` |
| 跨环境可用 | 纯进程内实现；`schedule.go` 删除，CLI 菜单改为“自动调度”（`menu.go:217`、`schedule_menu.go`） | `grep systemd` 仅文档迁移说明；E2E 在无 systemd 的本地进程运行 |

### r2-worker-upload（4 条需求）

| 需求 | 实现 | 测试/证据 |
| --- | --- | --- |
| Worker 上传协议 | `buildR2Endpoint` 校验 http/https、拼 `/storage`、`url.Values` 编码 token；`{filename,value}` JSON（`r2.go:45`、`r2.go:82`） | `TestR2UploadSuccess`、`TestR2UploadRejectsInvalidConfig` |
| 重试与超时 | 3 次尝试、2s 间隔、30s 超时、401/403 不重试（`r2.go:16-79`） | `TestR2UploadRetriesThenSucceeds`、`TestR2UploadFailsAfterThreeAttempts`、`TestR2UploadAuthFailureNoRetry` |
| 上传内容与空结果保护 | 空内容直接跳过；上传失败不写入；内容始终为 substore（`r2.go:45-50`、`cli.go:1817`） | `TestR2UploadSkipsEmptyContent`、`TestWriteExportR2FailureKeepsOldContent`、`TestWriteExportUploadsSubstoreRegardlessOfLocalFormat`、`TestScheduledExportEmptyResultsSkipsR2` |
| 管理面代理 | R2 客户端使用 `managementTransport()`，不包 debug 传输（`r2.go:24-26`、`proxy.go:121`） | 代码路径复核；E2E 直连本地上传成功，代理路径复用既有管理面实现 |

### substore-integration（5 条需求）

| 需求 | 实现 | 测试/证据 |
| --- | --- | --- |
| 目标列表导出格式 | `formatSubstoreResults`、alias 回退与清洗、IPv6 强制括号（`cli.go:1900`） | `TestFormatSubstoreResultsIPv4`、`...IPv6AlwaysBracketed`、`...AliasFallback`、`...AliasSanitized`、`...IPv6AlwaysBracketedWithV6BracketDisabled`、`TestFormatSubstoreResultsAppliesCustomFields` |
| 导出内容受筛选与行数约束 | 复用 `applyCLIExportFilters` 与起止行（`cli.go:2340`） | `TestFormatSubstoreResultsHonorsRowLimit`；E2E 输出 10 行受配置上限约束 |
| 上传与调度衔接 | 调度固定 substore；启用 R2 时上传（`scheduler.go:602-628`、`cli.go:1763`） | `TestLoadScheduledCLIConfigForcesSubstoreExport`、`TestScheduledExportUploadsSubstoreContent`、E2E |
| operator 解析兼容 | 静态/远程逐行解析、非法行跳过、WeTest HTML、去重与别名后缀（`substore/cloudflare-targets.js`） | `node --test substore/cloudflare-targets.test.js` 通过；非法端口与坏行日志确认 |
| operator 网络与运行环境 | `$substore.http.get` / fetch + AbortController 超时（`cloudflare-targets.js:304`）；无客户端抛错；守卫式导出（`:398`） | Node 测试覆盖 fetch 回退与无客户端错误分支 |

## 场景验证

- **E2E 调度闭环（修复后复跑）**：以 `/tmp/cfdata-e2e-final` 的 cron `* * * * *` 与本地 R2 mock，用当前工作区重建二进制启动 Web。启动日志为 `[schedule] 调度已启动：时区 UTC+08:00，已启用（Cron "* * * * *"），下一次触发 2026-10-28 13:30:00`；随后 `[official-output] cfdata-results.txt (10 rows, substore)`、`[r2] 上传成功: http://127.0.0.1:18925/storage (322 字节)`、`[schedule] 调度扫描完成`。Mock 收到 `POST /storage?token=e2e-token`，JSON `filename=cfdata-targets.txt`、`value` 为 10 行 `IP:443#HKG-Hong Kong`（`/tmp/cfdata-e2e-final/r2-requests.jsonl`）。
- **E2E 首次运行（修复前基线）**：同样闭环成功（317 字节），证明修复未改变成功路径的对外行为。
- **R2 失败保留旧内容**：停止 mock 后，调度继续每轮触发，日志显示 3 次重试后 `[r2] 上传失败，保留 R2 上的旧内容`，Web 服务与后续调度未中断。
- **边界**：空结果跳过上传（测试）；401/403 不重试（测试）；非法 worker-url 保存时拒绝（测试）；调度运行中跳过（测试）；配置损坏时保留旧计划并轮询恢复（测试）；并发写入配置时读者从未观察到半写 JSON 且 128KB 哨兵字段保留（`TestConfigWritersPreserveConcurrentUpdates`）；进程退出 `Stop()` 等待进行中扫描最多 5s。
- **前端**：`index.html` 两个内联脚本经 `node --check` 通过；substore 客户端格式化与服务端规则一致；R2 token 输入框不回显已保存值（以 `r2WorkerTokenSet` 显示“已保存，留空保持不变”）。

## 集成审查

- 审查范围：最终工作区 diff（含未跟踪新文件）对照 proposal、3 份 delta spec、design.md 与测试结果。
- 审查者：独立子代理（未参与实现），共三轮；修复后的增量单独复查，未用实现者自评代替。

| 轮次 | 范围 | 结论 |
| --- | --- | --- |
| 1 | 初始全量 diff | 无 CRITICAL/IMPORTANT；2 条 WARNING、3 条 SUGGESTION，已全部修复（见“验证反馈修复”） |
| 2 | 修复后 diff | 无 CRITICAL/IMPORTANT；2 条 WARNING、4 条 SUGGESTION，除呼叫点日志直测外全部落实 |
| 3 | 最终增量（时区标签、payload builder、原子模板写入、测试强化） | PASS，四项改动可纳入最终 diff；3 条 SUGGESTION，均为非阻塞（见“已接受建议”） |

补充：最后一处测试强化（初始 `check-interval` 60 → 59）在 Verify 阶段触发 Comet edit-review；由独立复核会话确认其为 test-only、非行为性并通过测量复现了旧断言的盲区，receipt `proposalHash=f6bfbf1b…`、`decision=non-behavioral`。应用后重跑全量测试与 race 测试通过。

### 第 1 轮发现与处理

| 级别 | 问题 | 处理 |
| --- | --- | --- |
| WARNING（按安全问题上调处理） | `get_config` 未鉴权即可返回配置文件原文，含 `r2workertoken` | 新增 `buildConfigDataPayload` 置空根级与嵌套 token；补测试 |
| WARNING | 配置读改写无锁、代理写入非原子，可能丢更新或产生半写文件 | 共享 `configFileWriteMutex` + `atomicWriteFile`；补并发测试 |
| SUGGESTION | 启动/重载未打印时区与下一次触发，design §8 要求 | 新增 `describeScheduleApplied` 并在 Start/Reload/mtime 重载输出；补测试 |
| SUGGESTION | cron 表达式未 trim，前导空格导致回退间隔模式 | `normalizeSchedulePlan` 先 `TrimSpace`；补测试 |
| SUGGESTION | 服务端 substore 导出未应用自定义字段，与前端复制结果不一致 | `formatCLIResults` substore 分支应用 `applyCLICustomFields`；补测试 |

### 第 2 轮发现与处理

| 级别 | 问题 | 处理 |
| --- | --- | --- |
| WARNING | 并发测试未真正检验锁、忽略写入错误 | 记录全部写入错误并断言为空；断言最终值来自成功的并发写入；断言 128KB 哨兵字段保留；初始 `check-interval` 改为 59（落在断言区间 60..99 之外，使丢失写无法蒙混通过） |
| WARNING | `time.Local.String()` 可能输出字面量 `Local`，日志无信息量 | 改为 `scheduleTimezoneLabel()`：时区缩写 + UTC 偏移（如 `CST (UTC+08:00)` / `UTC+08:00`） |
| SUGGESTION | 只测 `sanitizeConfigDataForWeb`，未覆盖 `get_config` 接线 | 抽出 `buildConfigDataPayload(path)` 并由 handler 使用；测试改为写临时配置文件走完整路径 |
| SUGGESTION | `writeCLIConfigTemplate` 仍用非原子 `os.WriteFile` | 改为 `atomicWriteFile`；不额外加锁以避免与 `updateProxyInConfigFile` 的持锁调用死锁 |
| SUGGESTION | 未直接测试日志的呼叫点 | 由 E2E 启动日志与 `TestDescribeScheduleApplied...` 间接覆盖，不再单独加呼叫点测试 |
| SUGGESTION | 禁用时启动日志与 Web 提示措辞可能矛盾 | 启动统一输出“调度已启动：…”状态行，Web 仅在不启用时提示开启方式 |

## 验证反馈修复（verify-fail 第 2 次）

1. **R2 token 暴露**：`get_config` 返回的配置中 `r2workertoken` 置空（根级与嵌套），保留 `r2WorkerTokenSet` 供 UI 提示；新增 `TestConfigDataPayloadStripsR2Token`。
2. **配置并发写入**：`updateProxyInConfigFile` 与 `saveScheduleConfigFields` 共享 `configFileWriteMutex`，统一经 `atomicWriteFile` 落盘；`writeCLIConfigTemplate` 改为原子写；新增 `TestConfigWritersPreserveConcurrentUpdates`（reader 校验无半写、写入错误断言、最终值与哨兵校验）。
3. **调度可观测性**：`Start`/`Reload`/mtime 重载统一输出“时区 + 计划 + 下一次触发”；新增 `TestDescribeScheduleAppliedIncludesTimezoneAndNextRun`。
4. **cron 空白容错**：`normalizeSchedulePlan` trim 表达式；新增 `TestNormalizeSchedulePlanTrimsCronExpression`。
5. **substore 自定义字段一致**：服务端终稿应用与前端复制相同的自定义字段；新增 `TestFormatSubstoreResultsAppliesCustomFields`。
6. 证据：`go vet` 通过；全量 `go test` 通过；`-count=3 -race` 目标测试 4.6s 通过；Node operator 测试通过；重建二进制 E2E 上传 322 字节成功；并发测试初始值强化经独立 edit review 批准后应用并重跑通过。

## 设计一致性

- 调度器使用 `robfig/cron/v3` + ticker，计划归一化为纯函数，测试可注入 ticker/时钟，符合 design.md §1。
- `runOfficialCLI`/`runNSBCLI` 抽取流水线并由 `runTaskSync` 包裹，调度复用同一流水线，符合 §2、§3。
- R2 使用独立 `managementTransport()` 且不包 debug 传输、token 脱敏，符合 §4。
- substore 格式规则、Web 镜像实现与复制按钮符合 §5；operator 入库与加固符合 §6。
- 实施中额外修复：调度运行不再改写进程级 `speedTestURL`/`speedTestWorkers`/debug/DNS/代理等全局设置，改为由 `scheduledSpeedURL(cfg)` 显式传参。该修复不改变对外行为，属于对 design“调度参数完全由配置文件决定”的落实。
- 验证修复（token 置空、配置写入加锁、调度日志）是对 design 安全与可观测性约束的落实，不改变需求语义。

## 已接受偏差与建议

| 项 | 说明 | 理由 / 影响 |
| --- | --- | --- |
| Web SIGTERM 未显式 `scheduler.Stop()` | 进程退出即终止 goroutine | 既有生命周期行为，本次未新增退出路径；不影响调度正确性 |
| R2-only 保存后间隔模式 `nextRun` 重置 | `Reload` 重建计划，不立即扫描 | 与“保存不触发扫描”的需求一致；最坏情况延后一轮间隔 |
| substore 回退 `ipport` 时丢弃 `:0` 行 | 服务端与客户端一致 | 端口 0 不是有效目标；避免生成不可用条目 |
| 进程级 ANSI 颜色 | 调度输出复用 CLI 颜色开关 | 仅影响终端显示，不影响日志内容与功能 |
| 扫描失败与 R2 失败语义 | 扫描失败写入 `lastResult`；R2 失败仅记录并保留旧内容 | 与 design“上传失败不阻断导出/调度”一致 |
| `get_config` 仍返回 `ghtoken`、`etpassword`、代理等配置 | 审查者标注的既有问题，本次仅修复新增的 R2 token | 超出本 change 范围；建议后续单独加固 |
| 时区标签对分钟级数字名称略冗余 | 如 `+0545 (UTC+05:45)` | 纯显示问题，不影响调度 |
| 原子写入重命名替换符号链接配置 | 与所有既有原子写一致 | 配置为普通文件；若用户使用符号链接需改为写目标路径 |
| 并发测试初始值 | 已改为 59（test-only，经 Comet edit review 独立复核批准） | 使“丢失调度写”无法通过断言，测试强度提升，不涉及产品行为 |
| 删除 systemd 后未自动清理既有 unit 文件 | 文档提供迁移与清理说明 | 避免破坏用户现有部署；由用户按文档处理 |

## 结论

全部检查项 PASS，无 CRITICAL/IMPORTANT 问题。改动满足 proposal 目标与 delta spec 验收场景，可进入归档；归档与提交需用户明确授权。
