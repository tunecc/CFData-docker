# Capability: speed-concurrency-learnings

## Purpose

CFData-WEB 落地 subs-check-pro 测速/测延迟并发的三项学习增量：同网段最小间距乱序、拥塞期复核 + 抖动、总测速带宽限速 + L/r 测速并发推导。目标是在不改变结果展示语义的前提下，降低同网段扎堆被测导致的上游限速/误判，给拥塞期的失败提供一次复核兜底，并让配置了总带宽的部署不再把出口带宽摊薄到测不准。完整动机与取舍见 `docs/comet/changes/speed-concurrency-learnings/brief.md`。

## Requirements

### Requirement: 同网段最小间距乱序

系统 SHALL 在"被测顺序"上对以下输入应用同网段最小间距乱序（结果展示排序不受影响）：

- 官方优选扫描候选（`runOfficialTaskWithPool`，扫描前）；
- 官方详细测试输入（`testIPList` 构建后）；
- 非标优选扫描候选（`runNSBTask` 展开后、扫描前）；
- 官方批量测速输入（`runOfficialSpeedBatch`）与非标批量测速输入（`runNSBTask` 测速段、Web 手动测速 `runNSBSpeedBatch`）。

乱序实现 SHALL：

- 以 IPv4 /24 为分组键（`ip4:<prefix24>`）；非 IPv4 文本（域名等）以主域为分组键（`dom:<root>`），无法归组者不参与间距约束；
- 保持最小间距 MinSpacing = max(模式并发线程数 × 5, 候选总数 ÷ 15)（向下取整；候选 < 2 时不处理）；
- 在满足间距的前提下随机打散（math/rand/v2），同组相邻冲突向前有限扫描修复，多轮改善后仍无法满足时接受当前结果（不无限重试）；
- 保持元素总数与多重集合不变（只重排，不增删）。

### Requirement: 拥塞期复核 + 抖动

系统 SHALL 在网络健康探针（`nethealth.go`）处于拥塞判定状态时，对以下失败各复核一次：

- 非标扫描（TCPing/HTTPing）失败节点；
- 官方扫描与官方详细测试失败节点；
- 非标测速与官方批量测速中测速失败节点。

复核机制 SHALL：

- 仅当失败发生时探针处于拥塞状态（连续 3 轮直连探测失败）才触发；网络健康时不产生任何额外请求；
- 每任务复核预算 = max(20, 候选总数 ÷ 20)（至少 20），挂载在任务级探针 watcher 上全任务共享；预算耗尽后不再复核；
- 复核前等待 300–800ms 随机抖动（含边界）；
- 复核以最终一次结果为准：复核成功记成功，复核失败仍记失败；不进行第三轮复核；
- 探针生命周期覆盖任务全程：官方任务从 `runOfficialTaskFlow` 开始到测速收尾；非标任务从 `runNSBTask` 扫描前到测速结束；Web 手动测速路径（`server.go` 直调 `runNSBSpeedBatch`）自起自停；
- 现有拥塞节流行为（拥塞时 worker 取任务前 2s 延迟）保持不变。

### Requirement: 总测速带宽限速 + L/r 测速并发推导

系统 SHALL 新增配置键 `speedbandwidth`（总测速带宽上限，单位 MB/s，整数，0=关闭，默认 0），并满足：

- `speedbandwidth > 0` 时，官方批量测速与非标测速（调度路径与 Web 手动路径）的所有下载共享一个任务级 token bucket：速率 = speedbandwidth × 1MB/s，桶容量 = 速率 ÷ 10；worker 在写入下载前先取得令牌；
- `speedbandwidth > 0` 时，非标测速并发 = min(模式并发配置, max(1, int(speedbandwidth ÷ speedMin)))，其中 speedMin 为该模式测速阈值（MB/s，低于 0.1 按 0.1 计）；Web 手动测速路径同理作用于其 `maxWorkers`；
- `speedbandwidth = 0` 时：不创建 bucket、不改变任何并发数，行为与本需求落地前完全一致；
- 官方批量测速保持现有串行节奏（单节点下载 + 1200ms 间歇）与 429 退避/换源机制不变，仅其下载受 bucket 约束；
- 配置页（Web「配置」页）SHALL 出现"总测速带宽上限 (MB/s)"输入项，0 或留空表示关闭，保存后持久化到配置文件，重启读取生效；
- 旧配置文件（无 `speedbandwidth` 键）SHALL 按现有"模板基准 + 差集迁移"机制静默迁移为默认 0，现有键不受影响。

### Requirement: 行为保持

除上述三点外，系统 SHALL 保持既有行为不变：

- 扫描结果仍按延迟/丢包排序展示，测速结果仍按达标/速度排序展示；乱序只改变被测顺序；
- "达标数量上限"（`offspeedlimit`/`nsbspeedlimit`）语义不变；
- 0=自动并发（`resolveScanThreads`）、ETA、历史落盘、通知等既有能力不受影响；
- 不新增第三方依赖；构建保持 `CGO_ENABLED=0`。

## Scenarios

### Scenario: 同网段间距打散生效

（2026-11-08 用户确认修正参数：原 A1 参数数学上不可满足，见 brief。）构造 100 条候选（10 条同属 1.2.3.0/24，MinSpacing=10）执行乱序：同组元素两两位置距离 > 10（等价地任意相邻 10 条窗口内同组 ≤ 2），总数与多重集合不变且顺序被打乱；候选 < 2、全部同 /24、参数不可满足时不报错、正常返回且多重集合不变。
验收：A1

### Scenario: 拥塞复核兜底且预算受限

假探针返回拥塞，假 worker 首次失败、复核成功 → 节点最终记为成功；连续制造超过预算次失败 → 复核次数不超过 max(20, 总数/20)，预算耗尽后直接记失败；探针健康时失败不触发任何复核请求；抖动等待落在 300–800ms。
验收：A2

### Scenario: 总限速生效与关闭等价

`speedbandwidth=10` 时多 worker 聚合下载速率不超过约 10MB/s + 桶容量余量；`speedbandwidth=0` 时并发数、下载行为与落地前一致（不创建 bucket）。
验收：A3

### Scenario: L/r 并发推导

`speedbandwidth=10, speedMin=1` → 非标测速并发 = min(模式并发, 10)；`speedMin=2.5` → min(模式并发, 4)；`speedbandwidth=0` → 并发 = 模式并发配置；官方测速并发行为不受 L/r 影响（保持串行）。
验收：A4

### Scenario: 配置页读写与旧配置迁移

配置页显示并保存"总测速带宽上限 (MB/s)"；重启后读取生效；无 `speedbandwidth` 键的旧配置文件启动后静默迁移为 0，其余键不变。
验收：A5

### Scenario: 构建与回归

`combined_refactor` 下 `go build ./...`、`go vet ./...`、`go test ./...` 全部通过；web 侧 node:test 现有用例通过；测试版编译到 `release_assets/cfdata-test` 成功；`go.mod` 无新增依赖。
验收：A6
