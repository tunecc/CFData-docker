# Comet Design Handoff

- Change: scheduled-scan-r2-substore
- Phase: design
- Mode: compact
- Context hash: 034d13f6a4e572d5239d0f2c7c6f1dc338cb2d2beb47a3c30415152256a122a6

Generated-by: comet-handoff.sh
Task hash policy: task-content-v1. Read tasks.md for live completion; excerpts are design-time context.

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## docs/openspec/changes/scheduled-scan-r2-substore/proposal.md

- Source: docs/openspec/changes/scheduled-scan-r2-substore/proposal.md
- Lines: 1-44
- SHA256: 4befcf62941615174d997d6d47c15f6a8d79b3f64fa0b5782a495683638f2bc1

```md
# 提案

## 为什么（Why）

当前项目只有 systemd 定时方案（仅 Linux CLI + root，Docker 内不可用），扫描结果导出后需要手工上传，无法形成「定时扫描 → 产出目标列表 → Sub-Store 消费」的自动闭环。参考 subs-check-pro 的 Cron/间隔双调度与 R2 Worker 上传重试后，本项目需要内置调度与 R2 上传能力，并产出 Untitled.js 可直接消费的目标列表。

## 变更内容（What Changes）

- **BREAKING**：移除 systemd 定时方案（`combined_refactor/schedule.go` 的 unit 生成、启停菜单与相关 CLI 入口），改为进程内调度：
  - `cron-expression`：标准 5 段 Cron 表达式，配置有效时优先；
  - `check-interval`：检测间隔（分钟），Cron 为空时生效；`≤0` 按 2880、`<60` 按 60 处理；
  - Web、CLI、Docker 通用；间隔模式启动即执行一次后按间隔循环，Cron 模式按表达式等待；
  - 配置文件变更后热更新定时计划。
- 新增 R2 上传：兼容 subs-check-pro 的 Worker 协议（`POST {worker-url}/storage?token={worker-token}`，JSON `{filename,value}`），默认最多 3 次尝试、固定 2 秒间隔、30 秒超时，走管理面代理；Web 设置与 CLI 配置提供 worker-url、worker-token、上传文件名与开关。
- 新增 Sub-Store 目标列表导出格式：每行 `host[:port]#alias`，IPv6 带端口强制 `[v6]:port`，别名默认 `DC-城市`，保证每行都有别名，导出/上传行数上限可配置。
- 调度任务完成后自动执行「扫描 → 目标列表导出 → R2 上传」；空结果或上传失败不覆盖 R2 上的旧文件，失败记录日志且不影响服务运行。
- 优化并纳入 Untitled.js（Sub-Store operator）：超时与失败回退、别名去重、IPv6/带端口格式兼容；放入仓库 `substore/` 目录统一维护。
- 更新 README 与 Docker 文档；旧 systemd 用户迁移说明（程序不再创建 unit，已安装的 unit 由用户手动关闭）。

不拆分原因：三部分围绕同一条自动流水线（调度触发 → 导出 → 上传 → operator 消费），拆分会造成跨 change 的验收依赖；单 change 内仍按能力独立验收。

## 能力（Capabilities）

### 新增能力

- `scan-scheduling`：进程内扫描调度，覆盖 Cron 表达式与间隔分钟两套计划、启动行为、配置热更新与调度触发范围。
- `r2-worker-upload`：通过 Cloudflare Worker 协议上传结果到 R2，覆盖配置校验、重试与超时、代理、空结果保护与错误处理。
- `substore-integration`：Sub-Store 目标列表导出格式（别名、IPv6、行数限制、上传衔接）与配套 operator 脚本行为。

### 修改的能力

- 无（现有 `container-image`、`image-publishing` 的行为要求不变）。

## 范围与非目标

- 范围：`combined_refactor/` 后端与 Web UI、CLI 配置、导出与上传链路、仓库新增 `substore/` operator 脚本、README 与 Docker 文档。
- 非目标：R2 的 S3 直传、WeTest HTML 生成、扫描/测速核心算法变更、Cloudflare Worker 与 R2 桶的部署本身、自动删除系统级 systemd unit。

## 影响（Impact）

- 代码：`combined_refactor/schedule.go`（替换）、`cli.go`（配置与上传）、`server.go` 与 `index.html`（Web 设置与上传）、`tasks_official.go`/`tasks_nsb.go`（调度触发衔接）、新增 R2 上传与调度模块、新增 `substore/` 脚本。
- 依赖：引入 `robfig/cron/v3`（与 subs-check-pro 同款）。
- 配置：CLI JSON 配置新增 `cron-expression`、`check-interval` 与 R2 字段；Web 设置新增对应项。
- 文档：README、`docs/docker.md`。

```

## docs/openspec/changes/scheduled-scan-r2-substore/design.md

- Source: docs/openspec/changes/scheduled-scan-r2-substore/design.md
- Lines: 1-194
- SHA256: 10a2283bb5da72296c5b850341449e79a91ea4e11612b0f42c70623b99ff10c5

[TRUNCATED]

```md
---
comet_change: scheduled-scan-r2-substore
role: technical-design
canonical_spec: openspec
---

# 设计

## Context

当前 `combined_refactor/schedule.go` 通过生成 systemd unit 实现按天/周/月定时，仅 Linux CLI + root 可用；Docker 内 Web 服务常驻但没有调度。扫描任务由 Web 的 `startTaskNamed` 或 CLI 的 `newCLISession` 驱动，结果行统一为 `cliResultRow`，导出统一走 `formatCLIResults`；管理面请求统一使用 `managementHTTPClient` 与代理配置。Untitled.js 从 URL 拉取 `host[:port]#alias` 文本并克隆节点，现有 TXT 默认输出已接近该格式，但 IPv6 括号默认关闭、别名不稳定，且缺少专用的目标列表格式与上传闭环。

已确认需求以 `proposal.md` 与 `specs/**/spec.md` 为准；本设计只记录技术决策。

## Goals / Non-Goals

**Goals:**

- 单一进程内调度器，Cron + 间隔双计划，Web/CLI/Docker 一致。
- R2 Worker 上传作为可复用上传后端，带固定重试、超时与空结果保护。
- 调度 → 扫描 → 目标列表导出 → R2 上传形成自动闭环。
- 仓库内维护的 operator 脚本与本项目输出格式互相兼容。

**Non-Goals:**

- 不引入 S3 直传、Worker 部署脚本或 WeTest HTML 生成。
- 不重构扫描/测速算法与现有 GitHub/edgetunnel 上传。
- 不为已安装的 systemd unit 提供自动删除。
- 不做多实例分布式调度与错过任务的补跑。

## 技术方案

### 数据流

```text
配置来源: cfdata-config.json (config 嵌套对象)
        |
        v
scanScheduler (单例, mutex)
  - 归一化计划: enabled -> cron | interval | disabled
  - cron: robfig/cron/v3, 5 段标准表达式
  - interval: time.Ticker + 进程启动立即一次
        |
        v  触发 (检查 anyTaskRunning + running 原子标志)
runScheduledScan
  - 读取配置 -> resolveCLIConfigFromFile -> 校验导出配置
  - newCLISession(cfg) 无头 session
  - runTaskSync(整个流水线, 持有全局任务计数)
        |
        v
runOfficialPipeline / runNSBPipeline (从 runOfficialCLI / runNSBCLI 抽取)
        |
        v
writeCLIExportAndMaybeUpload
  - substore 格式化 -> 本地文件
  - GitHub / edgetunnel (现有逻辑不变)
  - R2 上传 (启用时; 空内容跳过; 失败仅记录)
```

### 1. 调度器：`robfig/cron/v3` + Ticker

新增 `combined_refactor/scheduler.go`，单例 `scanScheduler`：

- 计划归一化（纯函数 `normalizeSchedulePlan`，表驱动可测）：
  - `schedule-enabled` 为 false → `disabled`。
  - `cron-expression` 非空且可解析 → cron 模式，忽略检测间隔；解析失败记录错误并回退间隔。
  - 间隔：`<=0 → 2880` 分钟，`<60 → 60` 分钟。
- 启动行为：cron 模式不立即执行；间隔模式进程启动后立即执行一次，然后按 ticker。间隔模式的立即执行只在进程启动时发生，`Reload` 不触发。
- 热更新：后台 goroutine 每 10 秒对比配置文件 mtime；Web 保存后直接调用 `Reload`。`Reload` 在互斥区内停止旧计划、按新配置重建，不等待也不中断进行中的扫描。
- 调度器状态快照（`Status()`）：模式、是否启用、下次触发时间、最近一次触发时间与结果摘要，供 Web 展示；不新增持久化。
- 测试注入：调度器内部允许传入测试用 ticker 间隔与时钟，生产路径使用归一化后的分钟值。

文件布局：

- `scheduler.go`：`normalizeSchedulePlan`、`scanScheduler`、`Start/Stop/Reload/Status`、`runScheduledScan`。
- `schedule_menu.go`：CLI「自动调度」子菜单（启用/停用、Cron、间隔、保存并常驻、Ctrl+C 退出）。
- 删除 `schedule.go`（systemd 生成/启停/状态）；`menu.go` 菜单项改为「自动调度」。

### 2. 调度执行复用 CLI 流水线


```

Full source: docs/openspec/changes/scheduled-scan-r2-substore/design.md

## docs/openspec/changes/scheduled-scan-r2-substore/tasks.md

- Source: docs/openspec/changes/scheduled-scan-r2-substore/tasks.md
- Lines: 1-48
- SHA256: 1193f9776698d378107bb0f2bd098c2bbe4f5487d5fa2cbe88934b73603107ed

```md
# 任务

## 1. 依赖与调度基础

- [ ] 1.1 在 `combined_refactor/go.mod` 增加 `github.com/robfig/cron/v3` 并更新 `go.sum`，在 `combined_refactor` 下运行 `go build ./...` 通过 <!-- comet-task:7da68423-d294-4fd9-be8d-aaa31a54cb66 -->
- [ ] 1.2 实现调度配置解析与归一化（`schedule-enabled` 默认关闭、Cron 优先、非法 Cron 回退间隔、间隔 `<=0→2880`、`<60→60`），单元测试覆盖关闭/启用/非法 Cron/间隔钳制并运行 `go test ./...` 通过 <!-- comet-task:adeefb5d-4abd-4532-b637-7628711c03f3 -->
- [ ] 1.3 实现进程内调度器（Cron/Ticker、启动行为、`anyTaskRunning` 并发保护、配置文件 mtime 热更新），单元测试用秒级测试钩子验证间隔触发、Cron 不立即触发和热更新后旧计划失效 <!-- comet-task:a078f515-0102-4bda-8514-ab6fd180c9d7 -->

## 2. 移除 systemd 与调度入口

- [ ] 2.1 删除 `schedule.go` 中 systemd unit 生成/启停/状态代码，CLI 菜单改为“自动调度”子菜单（查看状态、设置 Cron/间隔、启动常驻调度），`go build ./...` 通过且菜单输出经手工确认无 systemd 字样 <!-- comet-task:5a8efd30-703b-4fc8-a3f7-94aaf5a59c61 -->
- [ ] 2.2 Web 服务启动后异步启动调度器，CLI 常驻模式运行阻塞循环并响应 Ctrl+C；启动日志包含下一次触发时间，手工启动验证 <!-- comet-task:19e17169-fa18-4d0e-9471-345fb05e6804 -->
- [ ] 2.3 新增 `get_schedule_config`/`set_schedule_config` WebSocket 消息，读-改-写 `cfdata-config.json` 且保留未知字段；用临时目录手工调用验证写入内容与原子替换 <!-- comet-task:b5aad173-e547-4589-bd80-3192546221b4 -->

## 3. 调度执行流水线

- [ ] 3.1 实现 `runScheduledScan`：用无头 `appSession` 复用 CLI 配置执行官方/非标扫描并收集结果行，单元测试用桩验证已有任务运行时跳过触发 <!-- comet-task:f83f273a-90da-438c-8f5f-6e13927b6927 -->
- [ ] 3.2 调度完成后按 `substore` 格式导出并触发 R2 上传（启用时），空结果跳过上传；测试覆盖空结果与非空结果两条路径 <!-- comet-task:e92bb1db-c5eb-481d-89ef-76073c613e14 -->
- [ ] 3.3 更新 README 与 `docs/docker.md`：调度配置、常驻运行、systemd 迁移步骤；按文档命令手工执行验证 <!-- comet-task:c73e65c2-258c-4cf3-b43e-de371d1a1583 -->

## 4. Sub-Store 目标列表格式

- [ ] 4.1 实现 `outformat: substore` 行构造（别名回退与清洗、IPv6 强制括号、筛选与行数上限），单元测试覆盖 IPv4、IPv6、别名缺失、行数上限 <!-- comet-task:f0745bad-32d5-486f-abdc-524b60de8d3f -->
- [ ] 4.2 CLI 与 Web 导出/复制增加 `substore` 选项，运行 CLI `-outformat substore` 验证输出每行含 `#` 且 IPv6 带括号 <!-- comet-task:76411268-e1fb-457b-a2c4-7694c3ab5d56 -->
- [ ] 4.3 README 增加目标列表格式说明与 Untitled.js `urls` 接入示例，检查示例与实现输出一致 <!-- comet-task:3e793ebf-4250-464d-8969-59cc1c0092a0 -->

## 5. R2 上传

- [ ] 5.1 实现 `r2.go`：Worker 协议、3 次尝试间隔 2 秒、30 秒超时、401/403 不重试、管理面代理、配置校验与空内容跳过、token 不落日志；`httptest` 单元测试覆盖成功、前两次失败后成功、三次失败、鉴权失败、空内容 <!-- comet-task:c51c608e-7327-40b5-b387-ad7dc9347024 -->
- [ ] 5.2 在 CLI JSON 配置与 Web 设置中接入 `r2enabled`、`r2workerurl`、`r2workertoken`、`r2filename`，验证保存后可读回且默认关闭 <!-- comet-task:38f42b80-2b8b-4cc2-bc72-cf9fd51248b2 -->
- [ ] 5.3 将 R2 上传接入调度与导出链路，集成测试验证上传内容为 substore 文本、失败不覆盖旧文件 <!-- comet-task:dd02c5be-07ba-462b-bf49-91fff30a538b -->
- [ ] 5.4 文档补充 Worker 协议、R2 配置示例与故障排查 <!-- comet-task:af10315a-0c2a-4012-b25f-ba243561a2fa -->

## 6. Untitled.js 入库与优化

- [ ] 6.1 新增 `substore/cloudflare-targets.js`：别名清洗、重复目标去重、无效行跳过、URL 失败回退、无 HTTP 客户端报错；用 Node 以样例文本与模拟 fetch 验证输出节点数量与名称 <!-- comet-task:78d044bd-49a5-4158-be0f-2a62e20c760e -->
- [ ] 6.2 新增 `substore/README.md` 说明脚本用法、配置项与本项目 R2 输出的对接方式，按文档示例运行脚本验证 <!-- comet-task:a243891e-7700-4cd3-bd23-b606dd7c6914 -->

## 7. 集成验证

- [ ] 7.1 在 `combined_refactor` 下运行 `go vet ./...` 与 `go test ./...` 全部通过 <!-- comet-task:6d9d84d4-864d-46c6-ae08-44831c275415 -->
- [ ] 7.2 本地构建 linux/amd64 测试二进制（`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w"`）成功 <!-- comet-task:55d9feec-c640-4d73-b980-ef81250a8253 -->
- [ ] 7.3 本地启动 Web 服务，配置 `cron-expression: "* * * * *"` 与 R2 测试端点，观察一次完整“扫描 → 导出 → 上传”日志并核对导出文件内容 <!-- comet-task:a0deb5b8-eab2-46ec-a8ec-178f1e444dc1 -->

## Workflow follow-up

- 完成 Build 后进入 Verify，执行验收与最终集成审查。
- 验收通过后归档并按用户选择交付。

```

## docs/openspec/changes/scheduled-scan-r2-substore/.openspec.yaml

- Source: docs/openspec/changes/scheduled-scan-r2-substore/.openspec.yaml
- Lines: 1-2
- SHA256: 9e0a467e3934b14a2d9c9175305310e9359d34e2484a458af3d182c2956b9e2d

```md
schema: spec-driven
created: 2026-10-28

```

## docs/openspec/changes/scheduled-scan-r2-substore/specs/r2-worker-upload/spec.md

- Source: docs/openspec/changes/scheduled-scan-r2-substore/specs/r2-worker-upload/spec.md
- Lines: 1-73
- SHA256: db76ec533da6f2c7758a5d95a7f44061acb3e24a5e9fcf45071188db2ad1d259

```md
# 规格增量

## Purpose

通过 Cloudflare Worker 兼容接口把扫描结果写入 R2 存储，供 Sub-Store 等下游按 URL 读取，并在网络波动时自动重试、失败不破坏已有数据。

## ADDED Requirements

### Requirement: Worker 上传协议

配置 `worker-url` 与 `worker-token` 后，系统 SHALL 以 `POST {worker-url}/storage?token={worker-token}`、`Content-Type: application/json`、请求体 `{"filename":..., "value":...}` 的形式上传内容。`worker-url` MUST 为 http 或 https 地址。配置缺失或 URL 非法时系统 SHALL 跳过上传并记录明确错误，不阻塞扫描与导出流程。

#### Scenario: 成功上传

- **WHEN** 已配置有效的 worker-url、worker-token、文件名与内容
- **THEN** 系统向 Worker 的 `/storage` 端点发送 POST 请求，服务端返回 2xx 后记录上传成功

#### Scenario: 配置缺失时跳过

- **WHEN** worker-url 或 worker-token 为空且启用了 R2 上传
- **THEN** 系统记录配置不完整错误并跳过上传，扫描与本地导出仍正常完成

#### Scenario: URL 非法时跳过

- **WHEN** worker-url 不是有效的 http/https 地址
- **THEN** 系统记录配置错误并跳过上传，不发起网络请求

#### Scenario: 令牌不出现在日志

- **WHEN** 上传成功或失败
- **THEN** 日志与调试输出不包含 worker-token 明文

### Requirement: 重试与超时

单次上传失败时系统 SHALL 最多尝试 3 次，每次间隔 2 秒；单次请求 SHALL 使用 30 秒超时。鉴权失败（HTTP 401/403）SHALL NOT 重试。全部尝试失败后系统 SHALL 返回错误并记录最后一次失败原因。

#### Scenario: 前两次失败后成功

- **WHEN** 第 1、2 次上传失败而第 3 次成功
- **THEN** 系统记录最终上传成功，不再继续重试

#### Scenario: 三次均失败

- **WHEN** 连续 3 次上传均失败
- **THEN** 系统返回失败结果，记录最后一次错误，并保留 R2 上的旧内容

#### Scenario: 鉴权失败不重试

- **WHEN** Worker 返回 HTTP 401 或 403
- **THEN** 系统不再重试，直接记录配置或鉴权错误

### Requirement: 上传内容与空结果保护

上传内容 SHALL 来自当前导出结果（默认为 Sub-Store 目标列表）。当导出结果为空或上传最终失败时，系统 SHALL NOT 覆盖 R2 上的既有文件。

#### Scenario: 空结果不上传

- **WHEN** 本次扫描没有产生可导出的结果
- **THEN** 系统跳过 R2 上传，R2 上的旧文件保持不变

#### Scenario: 上传失败保留旧文件

- **WHEN** 本次上传在重试后仍失败
- **THEN** R2 上的旧文件保持原样，系统记录失败日志

### Requirement: 管理面代理

R2 上传请求 SHALL 遵循现有管理面代理配置；未配置代理时使用系统环境变量代理。扫描与测速流量 SHALL NOT 受该代理影响。

#### Scenario: 配置代理后走代理

- **WHEN** 管理面代理配置为 socks5/http 地址并发起 R2 上传
- **THEN** 上传请求经该代理发出，扫描与测速仍保持直连

```

## docs/openspec/changes/scheduled-scan-r2-substore/specs/scan-scheduling/spec.md

- Source: docs/openspec/changes/scheduled-scan-r2-substore/specs/scan-scheduling/spec.md
- Lines: 1-107
- SHA256: 49a629318871582368a53c79314b398de8113f0a7991e298ec8b9d297cc80aeb

[TRUNCATED]

```md
# 规格增量

## Purpose

让扫描任务按 Cron 表达式或固定间隔自动触发，在 Web、CLI 与 Docker 部署中无需依赖 systemd 等外部调度器即可持续产出扫描结果。

## ADDED Requirements

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


```

Full source: docs/openspec/changes/scheduled-scan-r2-substore/specs/scan-scheduling/spec.md

## docs/openspec/changes/scheduled-scan-r2-substore/specs/substore-integration/spec.md

- Source: docs/openspec/changes/scheduled-scan-r2-substore/specs/substore-integration/spec.md
- Lines: 1-93
- SHA256: 6590726f30fca09c4111216bcd9a77444d74e2291da2c1d3b8d89f5749e1d16b

[TRUNCATED]

```md
# 规格增量

## Purpose

让扫描结果以 Sub-Store operator 可直接消费的目标列表形式产出，并保证配套 operator 脚本在真实网络与各种输入格式下稳定解析、去重与回退。

## ADDED Requirements

### Requirement: 目标列表导出格式

系统 SHALL 提供专用的 Sub-Store 目标列表导出/上传格式，每行格式为 `host[:port]#alias`，且每行 MUST 包含非空别名。IPv6 带端口时 MUST 输出 `[v6]:port` 形式。别名默认由数据中心与城市组成，相关字段缺失时 MUST 回退为至少包含数据中心或 IP 的非空值。

#### Scenario: IPv4 目标行

- **WHEN** 导出结果为 IPv4 `1.2.3.4`、端口 443、数据中心 LAX、城市洛杉矶
- **THEN** 输出行形如 `1.2.3.4:443#LAX-洛杉矶`

#### Scenario: IPv6 目标行

- **WHEN** 导出结果为 IPv6 `2606:4700::1`、端口 443
- **THEN** 输出行形如 `[2606:4700::1]:443#HKG-香港`，方括号不可省略

#### Scenario: 别名缺失时回退

- **WHEN** 城市字段为空但数据中心为 NRT
- **THEN** 输出的别名至少包含 `NRT`，不会输出空别名行

#### Scenario: 别名包含破坏分隔的字符

- **WHEN** 别名包含 `#`、换行或多余空白
- **THEN** 输出前清洗这些字符，仍保持单行 `host[:port]#alias` 结构
- **AND** 清洗后为空时回退为数据中心或 IP，不输出空别名

#### Scenario: 关闭 IPv6 括号不影响目标列表

- **WHEN** `outv6bracket` 为 false 且导出 IPv6 目标
- **THEN** 目标列表仍输出 `[v6]:port#alias`，方括号不可省略

### Requirement: 导出内容受筛选与行数约束

目标列表 SHALL 遵循现有导出筛选（合格结果、IP 类型）与行数上限配置；超出上限的行 MUST NOT 被导出或上传。

#### Scenario: 行数上限生效

- **WHEN** 目标列表格式配置行数上限为 20 且合格结果超过 20 行
- **THEN** 导出与上传的文件只包含前 20 行

### Requirement: 上传与调度衔接

启用 R2 上传时，目标列表 SHALL 作为默认上传内容，上传文件名可配置。调度完成后系统 SHALL 自动生成并按配置上传目标列表；未启用上传时仅执行本地导出。

#### Scenario: 调度后自动上传目标列表

- **WHEN** 调度触发完成且启用了 R2 上传
- **THEN** 上传到 R2 的内容为目标列表格式文本

#### Scenario: 未启用上传仅本地导出

- **WHEN** R2 上传未启用且调度触发完成
- **THEN** 系统仅写出本地导出文件，不发起网络上传

### Requirement: operator 解析兼容

仓库内维护的 Sub-Store operator 脚本 SHALL 同时支持纯文本 `host[:port]#alias` 与 WeTest HTML 表格输入。无效行 SHALL 被跳过而不是导致整体失败；某个 URL 获取失败时 SHALL 继续使用其余来源与静态目标。别名重复时 SHALL 通过后缀去重，确保不产生重名节点。

#### Scenario: 无效行被跳过

- **WHEN** 输入文本中包含缺少 `#` 或端口非法的行
- **THEN** 脚本忽略这些行，其余有效目标仍被生成

#### Scenario: URL 获取失败时回退

- **WHEN** 配置的某个远程 URL 超时或返回错误
- **THEN** 脚本记录失败并继续使用其他 URL 与静态目标

#### Scenario: 重复别名去重

- **WHEN** 多个目标使用相同别名
- **THEN** 脚本为重复项追加可区分的后缀，节点名称不重复


```

Full source: docs/openspec/changes/scheduled-scan-r2-substore/specs/substore-integration/spec.md
