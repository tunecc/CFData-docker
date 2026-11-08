# Outcome

落地 subs-check-pro 测速/测延迟并发的三项学习增量（见 `docs/analyses/subs-check-pro-learnings.md` 建议清单及 2026-11-08 调查结论）：

1. **同网段最小间距乱序（① SmartShuffleByServer 思路）**：扫描类阶段（官方/非标扫描候选、官方详细测试输入）与测速批次输入，按 IPv4 /24 分组打散、保持最小间距后乱序，降低相邻并发测同网段被上游限速/测死的概率。对标 subs-check-pro `proxy/shuffle.go` `SmartShuffleByServer` 与 `check/check.go:300-327`。用户已确认（2026-11-08）：扫描+测速都乱序。
2. **拥塞期复核 + 抖动（②）**：网络健康探针（`nethealth.go`）判定拥塞期间失败的测试（扫描/详细测试/测速）用有限预算加 300–800ms 抖动复核一次，避免把本可成功的节点误判为失败。对标 subs-check-pro `check/check.go:756-782`。用户已确认（2026-11-08）：探针全程覆盖（扫描+详细测试+测速）。
3. **总测速带宽限速 + L/r 测速并发推导（③）**：新增"总测速带宽上限"配置（MB/s，0=关闭），所有并发测速下载共享一个 token bucket；启用后测速阶段并发 = min(模式并发配置, max(1, 总带宽 ÷ 单线程达标阈值))。对标 subs-check-pro `check/check.go:163-176` `calcSpeedConcurrency` 与 `check/check.go:350-356` token bucket。

最终效果：同网段节点不再扎堆被测；网络拥塞期的失败有一道复核兜底；配置了总带宽后并发测速不会把出口带宽摊薄到测不准，也不会把本机网络打满。

# Scope

## 目录结构

### 新建

- `combined_refactor/shuffle.go` — 同网段最小间距乱序纯函数：IPv4 /24 分组（域名候选按主域，若存在）、MinSpacing 保持、N 轮改善，带单测（对标 `proxy/shuffle.go`）。
- `combined_refactor/shuffle_test.go` — 乱序单元测试。
- `combined_refactor/bandwidth.go` — 简单 token bucket（纯 Go，不引新依赖），带单测。
- `combined_refactor/bandwidth_test.go` — token bucket 与 L/r 推导单元测试。

### 修改

- `combined_refactor/tasks_nsb.go` — `runNSBTask` 扫描候选展开后先同网段乱序（约 740 行附近）；测速段（约 859-895 行）接入 token bucket 与 L/r 并发推导；`runNSBSpeedBatch`（1156，Web 手动测速路径）同样接入 bucket、L/r 推导与测速输入乱序；测速失败接入拥塞复核；探针生命周期从"扫描结束"（811）延后到测速结束。
- `combined_refactor/tasks_official.go` — `runOfficialTaskWithPool` 扫描候选乱序（369-376 附近）；详细测试输入（545-563，`testIPList` 构建）乱序；扫描/详细测试失败接入拥塞复核；探针起点从 `runOfficialTaskWithPool` 上移（见 `pipelines.go`）。
- `combined_refactor/tasks_speed.go` — `runOfficialSpeedBatch`/`runOfficialSpeedTestsCore` 接入 token bucket（官方测速保持串行节奏不变）；测速输入乱序；测速失败接入拥塞复核。
- `combined_refactor/pipelines.go` — `runOfficialTaskFlow` 级别启动/停止探针（覆盖扫描→详细测试→批量测速全程）。
- `combined_refactor/nethealth.go` — 增加"当前是否拥塞"查询与复核预算（`max(20, 总数/20)`，挂 watcher 上，任务级共享）+ 抖动等待（300–800ms）API；探针起止生命周期调整配合。
- `combined_refactor/taskconfig.go`、`combined_refactor/types.go` — 新增 `speedbandwidth`（总测速带宽上限 MB/s，0=关闭，默认 0）字段、校验、模板与迁移。
- `combined_refactor/web/app.js`（及配置页对应静态文件）— 配置页新增"总测速带宽上限 (MB/s)"表单项与读写。

### 明确不建

- 不引入 `golang.org/x/time/rate`、`juju/ratelimit` 等新依赖 — 保持依赖极简（现有 go.mod 仅 4 个直接依赖），自实现 token bucket。
- 不实现 5 种衰减核/每阶段独立自动并发/内存工程 — 量级不匹配（数千级候选 vs 百万级节点），现有 `resolveScanThreads` 一个公式够用。
- 不做阶段重叠流水线 + 反压（原建议⑩）— L 级重构，不在本次范围。
- 不新增 MinSpacing 等乱序相关配置项 — 按公式自动推导，保持配置面小。

# Non-goals

- 不改变现有结果排序/展示逻辑：扫描结果仍按延迟/丢包排序展示，测速结果仍按达标/速度排序展示；乱序只作用于"被测顺序"，不作用于"结果顺序"。
- 不改变"达标数量上限"（`offspeedlimit`/`nsbspeedlimit`，收集合格条数）的既有语义；新增 `speedbandwidth` 是独立的带宽维度。
- 不改变官方批量测速的串行节奏与 429 退避/换源机制（`56c8ae6` 刚落地，并行化属高风险重构）——bucket 只限制其下载带宽。
- 不实现 NetGuard 式分阶段并发闸门（Acquire/Release 收紧在途数）——现有 nethealth 节流（拥塞取任务前 2s 延迟）保持现状，仅新增复核。
- 不为 CLI 补充新旗标（CLI 交互入口已移除，仅 Web 配置页 + 配置文件）。

# Acceptance examples

- **A1 同网段乱序**（2026-11-08 用户确认修正参数：原参数数学上不可满足——100 条中 30 条同 /24、间距 20 时同组元素需两两相距 ≥21、最多容纳 5 条）：构造 100 条候选，其中 10 条同属 1.2.3.0/24，MinSpacing=10；乱序后同组元素两两位置距离 > 10（等价地任意相邻 10 条窗口内同组 ≤ 2），总数与多重集合不变、顺序确实被打乱（非恒等排列）。单测覆盖：/24 分组、间距保持、小数组（<2）不处理、全部同 /24 与参数不可满足时算法终止且多重集合不变。
- **A2 拥塞复核**：注入假探针返回拥塞 + 假 worker 第一次失败、第二次成功；断言该节点最终记为成功，且复核次数不超过预算 max(20, 总数/20)；预算耗尽后不再复核；抖动在 300–800ms 内（注入时钟）。
- **A3 总限速**：配置 `speedbandwidth=10`（MB/s）时，多 worker 共同下载的聚合速率 ≤ 约 10MB/s + 桶容量余量（单测注入假下载源统计字节数/时间窗）；`speedbandwidth=0` 时行为与现状完全一致。
- **A4 L/r 推导**：`speedbandwidth=10`、`speedMin=1`（MB/s）→ 测速并发 = min(模式并发, 10)；`speedMin=2.5` → min(模式并发, 4)；`speedbandwidth=0` → 并发 = 模式并发配置。单测直接验证推导函数。
- **A5 配置页**：Web 配置页出现"总测速带宽上限 (MB/s)"输入项，0 或留空=关闭；保存持久化、重启读取生效；旧配置文件无此键时静默迁移为默认 0，现有键不受影响。
- **A6 回归**：`combined_refactor` 下 `go build ./...`、`go vet ./...`、`go test ./...` 全部通过；web 侧 node:test 现有用例通过；编译测试版到 `release_assets/cfdata-test` 成功。

# Constraints and invariants

- 构建约束：`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`（AGENTS.md），不得引入需要 CGO 的依赖。
- 每次修改完成后主动编译测试版到 `release_assets/cfdata-test`（AGENTS.md 要求）；Go 模块在 `combined_refactor/` 下独立，须在此目录下构建。
- 依赖极简：不新增第三方依赖。
- 配置新增键沿用现有"模板基准 + 差集迁移"机制（`taskconfig.go`），旧配置文件无痛升级。
- 前端为 `web/` embed 源文件 + node 测试基线（`7d3c120` 引入的形态），配置页改动需同步对应静态文件。
- subs-check-pro 参考路径：`/Users/tune/Downloads/subs-check-pro`（checkout 68394cb），仅作实现参考，不是需求来源文档。

# Decisions

- **D1 总限速默认关闭（0）**：不改变现有行为，用户显式配置后才生效。
- **D2 乱序为纯函数 + 参数自动推导**：MinSpacing 沿用 subs-check-pro 公式 `max(threads*5, len/15)` 自动推导，不新增配置项；使用 math/rand/v2 随机源。
- **D3 token bucket 自实现**：容量 = rate/10（对标 `capacity := int64(rate / 10)`），线程安全，测速 worker 每次写前 Wait。
- **D4 L/r 推导仅在 bandwidth>0 时生效**：测速并发 = min(模式并发配置, max(1, int(L/r)))，L 单位 MB/s、r = speedMin（MB/s，含 0.1 默认下限）。
- **D5 乱序作用范围（用户确认 2026-11-08：扫描+测速都乱序）**：扫描候选（官方/非标）、官方详细测试输入（同为扫描类阶段，输入为全量幸存者，乱序不影响选择集）、官方/非标测速批次输入；结果展示排序不变。
- **D6 复核覆盖范围（用户确认 2026-11-08：全程覆盖）**：探针生命周期提升到任务级——官方从 `runOfficialTaskFlow` 开始到测速收尾；非标从 `runNSBTask` 扫描前到测速结束；Web 手动测速路径（`server.go` 直接调 `runNSBSpeedBatch`）自起自停。
- **D7 官方批量测速保持串行**：仅接入 bucket 限速；L/r 并发推导作用于非标测速（调度路径 `speedTest` 线程数与 Web 手动路径 `maxWorkers`），官方测速不并行化（429 限流敏感，见 Non-goals）。
- **D8 复核预算挂 watcher**：预算 `max(20, 总数/20)` 在任务启动探针时初始化，全任务共享；拥塞恢复后剩余预算不再补发复核（仅对拥塞期间发生的失败复核）。

# Open questions

（无——Q1 乱序范围、Q2 复核覆盖阶段已于 2026-11-08 由用户确认，见 D5/D6。）
