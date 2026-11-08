---
generated_from_state_version: 11
---

# 验证

## 当前结果

- 结果: **已归档**
- 验证情况: **已在实现会话中完成验收**
- 目标周期: 2
- 迭代: 1
- 验证器尝试次数: 1
- 完成时间: 2026-11-08T07:50:12.055Z
- 摘要: 对照修正后 brief/spec 独立核对全部 6 项验收：三项能力（同网段乱序/拥塞复核/总带宽限速+L/r）实现完整、证据充分，配置链路端到端贯通，回归全绿。范围核对：git 变更 16 改 4 增，与 brief 目录结构一致（r2_manual_test.go/nethealth_test.go 为签名适配与新增测试，属实现常规伴随之改动），无未经授权的范围缩减或扩张；brief 四核心章节与附加章节齐备，spec 位于 specs/speed-concurrency-learnings/spec.md。与原请求比对：原始请求『把123都去做了』的三项全部落地，A1 参数修正已经用户 2026-11-08 明确授权并走完 revise-requirements 重新确认。

## 验收

| 编号 | 结果 | 来源 | 验收项 | 原因 |
| --- | --- | --- | --- | --- |
| A1 | passed | brief.md | **A1 同网段乱序**（2026-11-08 用户确认修正参数：原参数数学上不可满足——100 条中 30 条同 /24、间距 20 时同组元素需两两相距 ≥21、最多容纳 5 条）：构造 100 条候选，其中 10 条同属 1.2.3.0/24，MinSpacing=10；乱序后同组元素两两位置距离 > 10（等价地任意相邻 10 条窗口内同组 ≤ 2），总数与多重集合不变、顺序确实被打乱（非恒等排列）。单测覆盖：/24 分组、间距保持、小数组（<2）不处理、全部同 /24 与参数不可满足时算法终止且多重集合不变。 | TestSmartShuffleBySubnetSpacingSatisfied：100 条候选中 10 条同属 1.2.3.0/24、MinSpacing=10，20 个固定种子下同组两两位置距离均 >10（等价于任意 10 窗口 ≤2），总数与多重集合守恒、非恒等排列；TestSmartShuffleBySubnetDegenerateSmallInput（<2 与 spacing=0 不处理）、TestSmartShuffleBySubnetDegenerateAllSameSubnet 与 TestSmartShuffleBySubnetUnsatisfiableTerminates（不可满足时终止且多重集合不变）、TestSmartShuffleBySubnetUnsatisfiableStillSpreads（不可满足时按组收敛间距最大化分散）、TestSubnetGroupKey（/24 归组、端口剥离、主域、IPv6/无点不参与）全部通过。接入点核对：官方扫描（tasks_official.go runOfficialTaskWithPool）、官方详细测试（testIPList 构建后）、非标扫描（runNSBTask expandedEntries）、官方测速（runOfficialSpeedBatch results）、非标测速（runNSBTask speed 段 / runNSBSpeedBatch / runNSBReallinkTask）六处均调用 smartShuffleBySubnet，结果展示排序未改（sortNSBResults/sortOfficialTestResults 保留）。 |
| A2 | passed | brief.md | **A2 拥塞复核**：注入假探针返回拥塞 + 假 worker 第一次失败、第二次成功；断言该节点最终记为成功，且复核次数不超过预算 max(20, 总数/20)；预算耗尽后不再复核；抖动在 300–800ms 内（注入时钟）。 | TestCongestionRecheckBudgetAndJitter：initRecheckBudget(30)→预算 20（=max(20,30/20)）逐次消耗、耗尽后 congestionRecheck 返回 false；未判定拥塞（失败 2 轮）时不触发且不耗预算；默认抖动 64 次采样全部落在 300–800ms（注入固定抖动消除随机性后验证预算逻辑）；TestSpeedTestWithCongestionRecheck：拥塞下首次失败复核成功采用第二次结果、复核仍失败以失败收场且无第三轮；TestEnsureNetHealthBorrowsActiveProbe 验证借用/自起自停语义。接入点核对：官方扫描 worker、官方详细测试 worker、官方测速核心（非 429 错误）、非标扫描 worker、非标测速闭包、链路复验测速闭包均接 speedTestWithCongestionRecheck/congestionRecheck；探针生命周期核对：runOfficialTaskFlow 级 startScanNetHealth+defer、runNSBTask startTaskNetHealth+defer（原 811 行中途停止已移除）、独立测速路径 beginSpeedStage→ensureNetHealth。 |
| A3 | passed | brief.md | **A3 总限速**：配置 `speedbandwidth=10`（MB/s）时，多 worker 共同下载的聚合速率 ≤ 约 10MB/s + 桶容量余量（单测注入假下载源统计字节数/时间窗）；`speedbandwidth=0` 时行为与现状完全一致。 | TestTokenBucketSharedRate：10MB/s 桶、8 worker×8 轮×128KB=8MB 聚合消耗落在 (8MB-1MB 容量)/10MB/s 下界的 0.9×–3× 区间内（限速生效且无过度阻塞）；TestTokenBucketWaitNContextCancel 验证 ctx 取消；TestSpeedTestBucketWaitNNoopAndLifecycle 验证带宽 0 不建桶、waitN 空操作、stop 清理。接入点核对：runNSBDownloadSpeed 与 runWindowedSpeedTest 的读取循环均按实际读取字节数 speedTestBucketWaitN（令牌不足暂停读取形成 TCP 背压）；beginSpeedStage 仅在 bandwidth>0 时建桶（setSpeedTestBandwidthMB(0) 存 nil）。 |
| A4 | passed | brief.md | **A4 L/r 推导**：`speedbandwidth=10`、`speedMin=1`（MB/s）→ 测速并发 = min(模式并发, 10)；`speedMin=2.5` → min(模式并发, 4)；`speedbandwidth=0` → 并发 = 模式并发配置。单测直接验证推导函数。 | TestResolveSpeedThreads 9 用例：threads=50/speedMin=1/bandwidth=10→10；speedMin=2.5→4；threads=3→3（并发截断）；speedMin=20→1（推导不足 1 按 1）；speedMin=0→50（r 按 0.1 下限、c=100 被并发截断）；bandwidth=0→原样；threads<=0→原样。接入核对：beginSpeedStage deriveConcurrency=true 仅用于非标测速三处（runNSBTask speed 段/runNSBSpeedBatch/runNSBReallinkTask），官方 runOfficialSpeedBatch 传 false 保持串行（D7），收敛时输出日志。 |
| A5 | passed | brief.md | **A5 配置页**：Web 配置页出现"总测速带宽上限 (MB/s)"输入项，0 或留空=关闭；保存持久化、重启读取生效；旧配置文件无此键时静默迁移为默认 0，现有键不受影响。 | index.html:321 新增 id=speedBandwidth 输入（placeholder『0 不限速』，title 说明共享与 L/r 收敛）；web/validators.js 注册 countValidator 规则；web/app.js：mapCLIConfigToForm 读取 speedbandwidth 键、applyConfigValues 回填、readFormValues 采集、save payload.speedBandwidth、getOfficialSpeedConfig/getSpeedBandwidthValue 及官方批量/非标批量/非标任务/链路复验四处发送均带 bandwidth。后端链路核对：cli.go（cliFileConfig.speedbandwidth、默认模板、buildCLIConfigHelp 新增条目、setInt 无条件应用、两个 prepare 负值归零）、taskconfig.go（SpeedBandwidth 校验 0-100000 + set）、protocol.go（taskConfigUpdateRequest/toTaskConfigUpdate + 四个请求结构 bandwidth 字段）、server.go（四处 handler 负值归零并线程）、scheduler.go（两条调度流读 cfg.speedBandwidth）。迁移机制核对：migrateConfigFileToLatest 对缺键旧配置走 missing 分支——备份 .bak→旧值并入新模板（缺键取默认 0）→写回，现有键值经 json.Unmarshal 保留；cliOfficialFlags 为 CLI 旗标表无新增（符合非目标『不加 CLI 旗标』）。 |
| A6 | passed | brief.md | **A6 回归**：`combined_refactor` 下 `go build ./...`、`go vet ./...`、`go test ./...` 全部通过；web 侧 node:test 现有用例通过；编译测试版到 `release_assets/cfdata-test` 成功。 | combined_refactor 下 go build ./... 成功；go vet ./... 无告警；go test ./... -count=1 全量通过（23.4s，含新增 shuffle 7 个、bandwidth 4 个、nethealth 复核 3 个测试函数）；web/test 下 node --test 37/37 通过；release_assets/cfdata-test 为 linux/amd64 静态 ELF（14913696 字节，AGENTS.md 构建参数 -trimpath -ldflags '-s -w'，CGO_ENABLED=0）；go.mod 无新增第三方依赖。 |

## 检查

| 检查 | 命令 | 工作目录 | 状态 | 退出码 | 耗时 |
| --- | --- | --- | --- | ---: | ---: |
| go test ./... | test ./... -count=1 | combined_refactor | passed | 0 | 24198 ms |
| web node:test 基线 | node --test | combined_refactor/web/test | passed | 0 | 153 ms |

### Builder 报告的证据

以下为 Builder 报告，不等同于 Runtime 检查凭据或独立验收结果。

- go vet ./...: passed — 无告警
- go test ./... -count=1: passed — 全量通过（含新增 shuffle/bandwidth/nethealth 复核 10 个测试函数）；65s
- node --test (web/test): passed — 37/37 通过
- 编译测试版: passed — CGO_ENABLED=0 GOOS=linux GOARCH=amd64 -trimpath -ldflags '-s -w' → release_assets/cfdata-test（ELF x86-64 静态，14913696 字节）
- 依赖核查: passed — go.mod 无新增第三方依赖（token bucket 自实现）
- 已知限制: 官方批量测速保持串行节奏（429 限流敏感，brief 决定 D7），仅接入带宽桶与乱序，不参与 L/r 并发推导
- 已知限制: 独立「单 IP 测速」（runSpeedTest）不在批量测速治理范围，不受带宽桶约束（spec 未要求）
- 已知限制: 拥塞复核仅在探针活跃时生效（任务编排全程/独立测速自起自停）；独立「详细测试」与「单 IP 测速」无探针，维持现状
- 已知限制: 乱序为最优努力：组可放下时间距严格成立，放不下时收敛到本组最大可行间距最大化分散（含单测）

## 阻塞项

_无。_

## 风险与跳过的工作

- 乱序为最优努力：组可放下时严格满足间距，放不下时按组收敛到最大可行间距最大化分散（有单测覆盖），不承诺全局最优
- 官方批量测速保持串行（决定 D7），带宽桶对其单下载生效但 L/r 推导不适用；独立『单 IP 测速』不受带宽桶约束（spec 未要求）
- 拥塞复核仅在探针活跃的任务内生效；独立『详细测试』『单 IP 测速』无探针，维持现状（与 spec 的探针生命周期枚举一致）

## 之前的迭代

| 目标周期 | 迭代 | 尝试 | 结果 | 未解决项 | 摘要 | 完成时间 |
| ---: | ---: | ---: | --- | --- | --- | --- |
| 1 | 1 | 0 | recovery | — | 验收示例 A1 参数修正（原参数数学上不可满足）——2026-11-08 用户已明确授权；brief 与 spec 已同步更新，产品行为与 A2-A6 不变；实现与单测已落地且全部检查通过，工作区完整保留。 | 2026-11-08T07:40:38.168Z |
| 2 | 1 | 1 | pass | — | 对照修正后 brief/spec 独立核对全部 6 项验收：三项能力（同网段乱序/拥塞复核/总带宽限速+L/r）实现完整、证据充分，配置链路端到端贯通，回归全绿。范围核对：git 变更 16 改 4 增，与 brief 目录结构一致（r2_manual_test.go/nethealth_test.go 为签名适配与新增测试，属实现常规伴随之改动），无未经授权的范围缩减或扩张；brief 四核心章节与附加章节齐备，spec 位于 specs/speed-concurrency-learnings/spec.md。与原请求比对：原始请求『把123都去做了』的三项全部落地，A1 参数修正已经用户 2026-11-08 明确授权并走完 revise-requirements 重新确认。 | 2026-11-08T07:50:12.055Z |



## 结论

对照修正后 brief/spec 独立核对全部 6 项验收：三项能力（同网段乱序/拥塞复核/总带宽限速+L/r）实现完整、证据充分，配置链路端到端贯通，回归全绿。范围核对：git 变更 16 改 4 增，与 brief 目录结构一致（r2_manual_test.go/nethealth_test.go 为签名适配与新增测试，属实现常规伴随之改动），无未经授权的范围缩减或扩张；brief 四核心章节与附加章节齐备，spec 位于 specs/speed-concurrency-learnings/spec.md。与原请求比对：原始请求『把123都去做了』的三项全部落地，A1 参数修正已经用户 2026-11-08 明确授权并走完 revise-requirements 重新确认。
