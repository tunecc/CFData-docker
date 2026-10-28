# scan-scheduling Specification

## Purpose
让扫描任务按 Cron 表达式或固定间隔自动触发，在 Web、CLI 与 Docker 部署中无需依赖 systemd 等外部调度器即可持续产出扫描结果。

## Requirements

### Requirement: 调度开关

系统 SHALL 提供显式调度开关 `schedule-enabled`，默认关闭。开关关闭时，即使已配置 Cron 表达式或检测间隔，系统 SHALL NOT 注册定时计划或自动执行扫描。

#### Scenario: 未启用时不调度

- **WHEN** `schedule-enabled` 为 false 且已配置合法的 `cron-expression` 或 `check-interval`
- **THEN** 系统不注册定时计划，不自动执行扫描

#### Scenario: 启用后按计划调度

- **WHEN** `schedule-enabled` 为 true 且配置了合法的 `cron-expression`
- **THEN** 系统注册 Cron 计划并在触发时执行扫描

### Requirement: 双调度计划

系统 SHALL 支持 Cron 表达式与检测间隔（分钟）两种调度计划。Cron 表达式非空且有效时 SHALL 优先使用并忽略检测间隔；检测间隔 `<=0` SHALL 按 2880 分钟、`<60` SHALL 按 60 分钟处理。

#### Scenario: Cron 优先于间隔

- **WHEN** `cron-expression` 为合法的 `0 4 * * *` 且 `check-interval` 为 120
- **THEN** 任务每天 04:00 触发，不按 120 分钟循环触发

#### Scenario: Cron 无效时回退间隔

- **WHEN** `cron-expression` 非空但无法解析
- **THEN** 系统记录解析错误并改用检测间隔计划继续运行

#### Scenario: 间隔值钳制

- **WHEN** `check-interval` 配置为 30
- **THEN** 实际检测间隔按 60 分钟执行
- **AND** 当 `check-interval` 为 0 或负数时按 2880 分钟执行

### Requirement: 启动与并发行为

调度已启用且使用 Cron 模式时，系统 SHALL NOT 在启动时立即扫描，而是等待下一次表达式触发；调度已启用且使用间隔模式时，系统 SHALL 在进程启动后立即执行一次，之后按间隔循环。已有扫描任务运行时系统 SHALL NOT 重复触发新扫描。

#### Scenario: 间隔模式启动即执行

- **WHEN** `schedule-enabled` 为 true、仅配置 `check-interval` 为 120 并启动程序
- **THEN** 启动后立即执行一次扫描，此后每 120 分钟执行

#### Scenario: Cron 模式启动不立即执行

- **WHEN** 配置了合法的 `cron-expression` 并启动程序
- **THEN** 启动后不立即扫描，只在表达式规定的时刻触发

#### Scenario: 运行中不重复触发

- **WHEN** 上一次扫描尚未结束且到达下一次触发时刻
- **THEN** 本次触发被跳过，不产生并发扫描

### Requirement: 配置热更新

运行期间检测到调度配置变化时，系统 SHALL 重新注册定时计划；新计划立即生效，旧计划不再触发，无需重启进程。热更新 SHALL NOT 立即触发一次扫描，也 SHALL NOT 中断进行中的扫描。

#### Scenario: 修改计划后热更新

- **WHEN** 程序运行中把 `cron-expression` 或 `check-interval` 修改为新值
- **THEN** 系统按新配置重新注册计划，旧计划不再触发

#### Scenario: 热更新不立即扫描

- **WHEN** 程序运行中保存新的调度配置
- **THEN** 系统只替换计划，不因保存动作立即执行扫描

#### Scenario: 热更新不打断进行中的扫描

- **WHEN** 扫描进行中检测到调度配置变化
- **THEN** 当前扫描继续执行直至结束，新计划从下一次触发开始生效

### Requirement: 调度触发完整流水线

每次调度触发 SHALL 使用当前生效的扫描配置执行扫描，并在扫描成功后执行导出与已启用的上传。扫描失败或上传失败 SHALL 记录错误并保持服务继续运行。

#### Scenario: 触发后自动导出并上传

- **WHEN** 调度触发且扫描成功、已启用 R2 上传
- **THEN** 系统完成导出并上传结果，然后等待下一次触发

#### Scenario: 扫描失败不影响服务

- **WHEN** 调度触发后扫描失败
- **THEN** 系统记录错误并保持 Web 服务与后续调度可用

### Requirement: 跨环境可用

调度 SHALL 以进程内方式运行，不依赖 systemd、root 权限或 Linux 专有机制，可在 Docker 容器与 Web 服务中持续工作。移除 systemd 方案后，CLI 菜单 SHALL NOT 再提供 systemd 定时设置入口。

#### Scenario: Docker 内按计划运行

- **WHEN** 容器中运行 Web 服务并配置了检测间隔
- **THEN** 服务按计划自动执行扫描，无需容器内 systemd

#### Scenario: CLI 菜单不再提供 systemd 定时入口

- **WHEN** 用户进入 CLI 菜单
- **THEN** 菜单中不再出现 systemd 定时任务设置项
