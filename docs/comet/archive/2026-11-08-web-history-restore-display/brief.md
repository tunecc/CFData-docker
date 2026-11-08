# 目标

消除 CFData-WEB 每次打开网页只显示「[系统] 等待操作...」+「已恢复最近一次任务分析（来自历史记录）」的现象：学习 subs-check-pro「文件持久化 + 打开时拉取」的模式，把上次任务的完整运行视图（运行日志 + 结果表）落盘，进程重启后打开网页即完整恢复，如同任务刚结束。

## 调查结论（已查明事实）

### CFData-WEB 现状机制

1. **占位符永不清除**：`combined_refactor/index.html:81` 静态写入 `<div id="logBox"><div>[系统] 等待操作...</div></div>`；前端 `log()`（`web/app.js:1251`）只追加不清除占位行，因此任何一次打开都会先看到「[系统] 等待操作...」。
2. **WS 连接时双路径恢复**（`server.go:119-128` `sendTaskRestore`）：
   - 内存回放路径：`taskSession.buildTaskRestore()` 下发 `task_restored`/`task_restore_chunk`/`task_restore_done`，可完整恢复运行日志（上限 500 行）与结果表（scan/test/speed 上限 20000/2000 行，`session.go:22-27`）。但回放缓冲**只存在于进程内存**，进程重启后为空；新任务开始时 `resetReplayState()` 也会清空。
   - 历史记录路径：仅当 `!anyTaskRunning()` 时下发 `history_analysis`（`fromHistory: true`），数据来自磁盘 `history/latest.json`。该记录**刻意只存统计摘要 + Top 20 结果**（`history.go:17-18` 注释："记录是统计摘要加 Top 结果，不是全量结果表"），不含运行日志。
3. **前端表现**（`web/app.js:4883-4890`）：收到 `history_analysis(fromHistory)` 后仅把摘要写入分析报告视图，并在运行日志追加一行「已恢复最近一次任务分析（来自历史记录）」。结果表格与运行日志在进程重启后不可恢复——这就是用户看到的两行现象的全部成因。

### subs-check-pro 的实现（学习对象）

- **落盘**：每次检测结束 `check.go:623` 调 `GenerateAnalysisReport()`（`check/analysis_stats.go:74`）把完整分析统计写入 `output/stats/subs-analysis.yaml`；运行日志持续追加到磁盘日志文件。
- **拉取**：前端页面加载时直接 HTTP 拉取——分析页 `analysis.js:962/989` `sfetch('/api/analysis-report')`，服务端 `getAnalysisReport`（`app/server.go:733`）直接读磁盘 YAML 返回；日志接口 `/api/logs` 从磁盘日志文件读最后 N 行。
- **核心差异**：subs-check-pro 的历史内容是「文件持久化 + 每次打开时主动拉取」，与进程内存状态无关；CFData-WEB 的完整内容只在内存回放缓冲里，持久化的只有统计摘要。

# 范围

## 目录结构

### 新建

- combined_refactor/taskstate.go — 磁盘任务快照的持久化与读取：快照结构定义（终态元信息 + 运行日志 + 各类结果原始行）、任务结束原子落盘、按内存回放同构方式加载重建。
- combined_refactor/taskstate_test.go — 快照落盘/加载往返与恢复分支测试。

### 修改

- combined_refactor/session.go — 任务收尾 defer 中调用快照落盘（`label != ""` 的任务，含被用户停止）；`taskRestorePayload` 增加 `fromDisk` 标记字段。
- combined_refactor/server.go — `sendTaskRestore` 增加磁盘恢复分支：内存回放无任务且无任务运行时，读取磁盘快照并以相同 `task_restored`/`task_restore_chunk`/`task_restore_done` 序列下发。
- combined_refactor/web/app.js — `log()` 首次输出时移除 logBox 静态「[系统] 等待操作...」占位行；`applyTaskRestoredState` 处理 `fromDisk` 标记输出恢复提示。

### 明确不建

- 不新建 HTTP 拉取接口 — 现有 WS 连接恢复序列（`sendTaskRestore`）已具备完整的恢复链路与前端处理，磁盘快照复用同构载荷即可，无需引入 subs-check-pro 式的 REST 拉取端点。
- 不修改 `history.go` 的统计摘要记录 — 其服务分析报告视图，与本需求的运行视图恢复互补。

# 非目标

- 不改变任务运行中的实时推送协议与广播行为（WS 增量消息流保持现状）。
- 不改变历史记录列表（200 条）与导出 CSV/R2 等既有功能。
- 不改动统计摘要记录（`taskHistoryRecord`）的字段与用途；分析报告视图的恢复行为保持现状。
- 不承诺任务运行中进程崩溃后的运行中状态恢复（内存回放丢失场景恢复的是上一个已完成任务）。
- 不改动 subs-check-pro 项目本身。
- 不改变「已恢复最近一次任务分析（来自历史记录）」提示行本身（分析报告视图仍需要它）。

# Decisions

- D1 落盘时机：任务收尾 defer 中、`endTask()` 之后落盘；仅 `label != ""` 的任务落盘（与 persistTaskHistory 的空标签跳过口径一致）；成功、失败、被用户停止的任务都落盘（快照记录的是「上次任务」的真实终态）。
- D2 快照生命周期：`beginTask` 不删除磁盘快照；进程在任务运行中重启后，打开网页显示上一个已完成任务的快照。新任务结束时覆盖。
- D3 恢复优先级：内存回放可用（HasTask）时完全沿用现状，磁盘快照不参与；仅内存为空且 `!anyTaskRunning()` 时从磁盘恢复；快照缺失或损坏时回退现状（仅历史摘要恢复）。
- D4 载荷同构：磁盘快照持久化原始回放数据（logs + 各类结果行 + 终态快照元信息），恢复时复用 `appendTaskRestoreChunks` 重建与内存路径完全同构的 `task_restored` 载荷，前端结果表/进度区恢复逻辑零改动。

# 验收示例

- A1 重启后恢复完整视图：任务完成后重启进程（内存回放清空、无任务运行），打开网页 → 运行日志呈现上次任务的日志内容（而非仅一行恢复提示），结果表格数据与上次任务结束时的内存回放一致（scan/test/speed/nsb 各按现有上限截断），进度区显示「完成」终态。
- A2 快照落盘内容与生命周期：任务结束后 `<historyDir>/task-state.json` 存在，包含运行日志与各类结果行（含被用户停止的任务）；下一次任务开始（beginTask）不删除该文件；新任务结束后文件内容被覆盖为新任务。
- A3 运行中不恢复快照：有任务运行时新连接不读取、不下发磁盘快照内容，恢复路径与现状一致（内存回放优先）。
- A4 无快照回退：`task-state.json` 缺失或损坏时打开网页不报错，行为与现状一致（历史摘要恢复可用）。
- A5 占位符清除：任何日志或恢复内容到达后，logBox 中不再残留静态「[系统] 等待操作...」行。
- A6 内存优先不回归：进程未重启（内存回放可用）时打开网页，恢复视图与现状完全一致，磁盘快照不参与、不产生重复行。
