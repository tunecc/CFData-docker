# Brainstorm Summary

- Change: scheduled-scan-r2-substore
- Date: 2026-10-28

## 确认的技术方案

状态：已确认（2026-10-28）

### 1. 调度器（进程内，替换 systemd）

- 新增 `scheduler.go`：`robfig/cron/v3` + `time.Ticker`，由全局单例 `scanScheduler` 管理。
- 配置归一化（`normalizeSchedulePlan`，纯函数可测）：
  - `schedule-enabled=false` → 不调度（新增显式开关，默认 false）。
  - `cron-expression` 非空且可解析 → cron 模式，优先于间隔。
  - cron 非法 → 记录错误并回退间隔模式。
  - `check-interval <= 0 → 2880`，`< 60 → 60`。
- 启动行为：cron 模式不立即执行（按表达式下一次触发）；间隔模式进程启动即执行一次，然后按 ticker。
- 热更新：读取配置文件 mtime 轮询（10s）+ Web 保存后立即 `Reload`。Reload 只替换计划，不立即扫描、不打断进行中的扫描。
- 并发保护：扫描入口先 `anyTaskRunning()` 跳过（记录日志）；实际执行用 `runTaskSync` 持有全局任务计数，与手动 Web/CLI 任务互斥。另加原子 `running` 标志防止 ticker 重入。
- 流水线复用：从 `runOfficialCLI` / `runNSBCLI` 抽取 `runOfficialPipeline(cfg, session)` / `runNSBPipeline(cfg, session)`；官方流水线在同一个 `runTaskSync` 内完成扫描→DetailedIPs 检测→测速→导出，消除现有两段任务之间的并发空窗。
- 定时任务使用 headless session（`newCLISession`）复用 CLI 输出通道，由调度器包装为静默日志。
- Web：启动时（`initLocations()` 之后、HTTP 监听之前）启动调度器；新增 WS `get_schedule_config` / `set_schedule_config`，读改写配置文件嵌套 `config` 对象，保留未知字段，使用 `storage.go` 的 `atomicWriteFile` 原子写入，写后立即 `Reload`。
- CLI：删除 `schedule.go` 的 systemd 实现，替换为 `schedule_menu.go`（启用/停用、cron 表达式、检测间隔、保存并常驻运行、Ctrl+C 退出）。`menu.go` 菜单项 4 文案改为「自动调度」。
- 时区：cron 使用进程本地时区；启动日志打印当前时区与计划描述。

### 2. R2 上传（CF Worker 兼容协议）

- 新增 `r2.go`：
  - URL 校验：仅 http/https，base 拼接 `/storage`；token 通过 `url.Values` 编码进查询串。
  - 请求体：JSON `{"filename": ..., "value": ...}`，`Content-Type: application/json`。
  - 成功判定：HTTP 2xx。
  - 重试：最多 3 次，固定间隔 2s（间隔与客户端可注入以便测试）；4xx 中 401/403 视为配置错误直接失败，其余按重试处理。
  - 超时：30s。
  - 空内容保护：内容为空或导出 0 行时跳过上传并记录错误，绝不覆盖 R2 旧文件。
  - 代理：复用管理面代理传输（`applyManagementProxy`），但**不套 debug 包装**，避免 `?token=` 写入 debug 日志；错误日志只打印脱敏 URL（去掉 query）。
  - 上传集成进 `writeCLIExportAndMaybeUpload`：手动 CLI、Web 导出、定时任务统一生效；上传失败只记录，不阻断导出与后续流程。
- 配置字段（嵌套 `config`）：`r2enabled`、`r2workerurl`、`r2workertoken`、`r2filename`（默认 `cfdata-targets.txt`）。
- 上传内容：始终为 Sub-Store 目标列表格式（不论本地导出格式）。

### 3. Sub-Store 目标列表导出

- 新导出格式 `substore`（TXT 扩展名）：
  - 每行 `host[:port]#alias`；IPv6 始终输出 `[v6]:port#alias`（不受 `outv6bracket=false` 影响）。
  - alias 默认 `dc-city`，回退顺序 dc → loc → ip；清洗 `#`、换行、多余空白，空 alias 回退到地址本身。
  - 复用现有 IP 类型/合格筛选、起止行限制。
  - 端口为空或 0 时不追加 `:port`。
- CLI/Web 增加 substore 格式选项；Web 端在 `index.html` 中镜像实现 `buildSubstoreContent`（服务端 `formatCLIResults` 与前端各自维护格式），并提供「复制 Sub-Store 列表」按钮。
- GitHub/edgetunnel 上传配置默认文件名相应为 `results/ip.txt` 场景，不强制。

### 4. Operator 脚本入库

- 新增 `substore/cloudflare-targets.js`：基于 Untitled.js 加固：
  - 静态目标逐行解析，非法行跳过并告警，不再整脚本抛错。
  - alias 清洗（`#`、换行、空白）。
  - `fetch` 回退路径增加 `AbortController` 超时（与 `urlTimeout` 一致）。
  - 完全重复目标去重；保留 `dedupAliases` 上标逻辑。
  - 守卫式 `module.exports`（存在 `module` 时导出纯函数）便于 Node 测试，Sub-Store 环境不受影响。
- 新增 `substore/README.md`：输入格式、urls 示例、R2 公网地址接入、常见问题。

## 关键取舍与风险

- 不引入状态持久化：定时任务重启后不补跑；间隔模式启动即跑一次，cron 模式等下一次触发。
- 单实例假设：进程内调度无法跨副本协调；文档明确同一配置目录只运行一个实例。
- 配置迁移：新增字段会被 `migrateConfigFileToLatest` 识别为缺失并触发一次性重写（生成 `.bak`）；文档说明。
- Web 导出为客户端实现，与服务端格式化存在两份代码；通过相同用例的单元/手动验证降低漂移风险。
- 移除 systemd 后不自动删除既有 unit 文件，文档给出迁移与清理命令。
- R2 令牌保护：不用 debug 传输；错误日志脱敏。
- 4xx 非鉴权错误重试意义有限，但保持与参考实现一致（3 次），避免复杂分类。

## 测试策略

- Go 单元测试：
  - `normalizeSchedulePlan` 表驱动（关闭、cron 优先、非法 cron 回退、间隔钳制）。
  - 调度器：间隔模式立即触发一次；cron 模式启动不立即触发；Reload 后不立即触发且新计划生效；运行中跳过。
  - R2：httptest 成功、失败两次后成功、全部失败、空内容跳过、token 不出现在日志。
  - Sub-Store 格式：IPv4/IPv6、端口省略、alias 回退/清洗、筛选与行数限制。
- Node 脚本测试：`node --test` 或临时 harness 校验解析、去重、非法行跳过。
- 集成验证：`go vet ./...`、`go test ./...`、linux/amd64 构建；本地 Web 用 `cron-expression: "* * * * *"` + 本地 R2 mock 端点跑通一次定时流水线。
- 手动：CLI 菜单启用/保存/常驻；Web 设置保存后刷新页面状态一致；禁用调度后不再触发。

## Spec Patch

将回写 delta spec（不重写需求结构，只补边界与场景）：

1. `specs/scan-scheduling/spec.md`
   - 新增「调度开关」需求：`schedule-enabled` 默认 false；场景「未启用时不调度」「启用为 cron 模式」「启用为间隔模式」。
   - 启动行为场景限定为「已启用且为间隔模式」；补充「cron 模式启动不立即扫描」。
   - 热更新场景补充：保存/重载不立即触发扫描、不中断进行中的扫描。
2. `specs/substore-integration/spec.md`
   - 新增场景：alias 清洗（`#`、换行、空白）与空 alias 回退。
   - 新增场景：`outv6bracket=false` 时 substore 格式仍输出 `[v6]:port`。
3. `specs/r2-worker-upload/spec.md`
   - 补充场景：令牌不得出现在日志/debug 输出；URL 非法时跳过并报错。
