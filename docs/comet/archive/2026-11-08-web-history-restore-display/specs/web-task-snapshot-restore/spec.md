# Capability: web-task-snapshot-restore

本 capability 定义 CFData-WEB 打开网页时对上次任务完整运行视图的磁盘级恢复：任务结束时把全局任务会话的回放内容（运行日志 + 各类结果行）与终态快照落盘，进程重启后新连接仍能完整恢复上次任务视图，对齐 subs-check-pro「文件持久化 + 打开即见」的体验。

## 行为规格

### 磁盘任务快照（task-state.json）

- 快照文件位于 `historyDir()` 目录（与 `latest.json` 同级），文件名 `task-state.json`，原子写入（复用 `atomicWriteFile`），格式为 JSON。
- 快照内容为全局任务会话回放缓冲的完整序列化：
  - 元信息：mode、label、终态 `backgroundTaskSnapshot`（含 phase/message/进度统计/params）。
  - 运行日志：`taskLogs` 全量（沿用 `taskLogReplayLimit` 上限 500 行）。
  - 结果行：scan（≤20000）、test（≤20000）、speed（≤2000）、nsbLive（≤20000）、dc、nsbFinal（headers+rows）——各沿用现有回放上限，与内存缓冲内容逐条一致。
- 落盘时机：`startTaskNamed` 收尾 defer 中、`endTask()` 之后调用；仅 `label != ""` 的任务落盘；任务成功、失败（管线错误）、被用户停止三种终态都落盘。落盘失败仅记日志，不影响任务终态与既有流程。
- `beginTask`（新任务开始清空内存回放）不删除磁盘快照；快照只在新任务结束时被覆盖。

### 新连接恢复（WS）

- `sendTaskRestore` 恢复优先级：
  1. 内存回放 `HasTask == true`：完全沿用现状（`task_restored`/`task_restore_chunk`/`task_restore_done`），磁盘快照不参与。
  2. 内存回放无任务且 `!anyTaskRunning()`：读取 `task-state.json`，以与内存路径完全同构的序列下发——`task_restored` 载荷含快照元信息、日志与 `fromDisk: true` 标记（`HasTask: true`、`Running: false`），结果行经 `appendTaskRestoreChunks` 分片为 `task_restore_chunk`，`task_restore_done` 的 `lastSeq` 取当前会话序号，后续增量消息按现有 seq 过滤衔接。
  3. 快照缺失、损坏或校验失败：跳过磁盘恢复，回退现状（仅历史摘要路径）。
- 有任务运行时（`anyTaskRunning()`）不读取、不下发磁盘快照。
- `sendLatestHistoryToSession`（历史统计摘要下发）行为不变，与磁盘快照恢复叠加生效（分析报告视图仍由历史摘要驱动）。
- 快照文件名固定、不可被前端指定；读取仅限定在 history 目录内，不做路径拼接输入。

### 前端行为（web/app.js）

- `log()` 首次输出前移除 `logBox` 内静态占位行「[系统] 等待操作...」（`index.html:81` 保留静态占位作为无任何日志时的初始态，静态测试基线不变）。
- `applyTaskRestoredState` 识别载荷 `fromDisk` 标记：先清除占位行，按现有逻辑恢复进度区/模式/参数，再回放日志，并输出一条「已从磁盘恢复上次任务结果」类提示（具体文案由实现确定，须表明内容来自上次任务）。
- 结果表格、徽标计数、NSB 实时行等恢复由现有 `task_restore_done` 处理逻辑承担，不新增前端恢复代码路径。

## 无产品行为变更声明

本 capability 为新增能力：内存回放可用时的新连接恢复行为逐字节不变（A6）；统计摘要记录与历史管理行为不变；任务运行时协议与广播不变。新增的只有任务收尾时的一次落盘与内存为空时的磁盘恢复分支；落盘失败不影响任何既有流程。

## 场景（与 brief 验收示例对应）

A1–A6 的完整场景定义见 `docs/comet/changes/web-history-restore-display/brief.md` 的 `# 验收示例`；本 Spec 不重复罗列，以 brief 为验收项唯一来源。
