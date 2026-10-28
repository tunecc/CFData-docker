---
change: scheduled-scan-r2-substore
design-doc: docs/openspec/changes/scheduled-scan-r2-substore/design.md
base-ref: b312c209edf1ca903c53ec0d959118c2c11d3f9b
archived-with: 2026-10-28-scheduled-scan-r2-substore
---

# 实施计划：进程内调度、R2 上传与 Sub-Store 目标列表

<!-- comet-task-authority: docs/openspec/changes/scheduled-scan-r2-substore/tasks.md -->

依据：`docs/openspec/changes/scheduled-scan-r2-substore/design.md` 与 `specs/scan-scheduling`、`specs/r2-worker-upload`、`specs/substore-integration`。所有任务在 `comet/scheduled-scan-r2-substore` 分支执行；Go 命令使用 `go -C combined_refactor ...` 从仓库根运行，便于统一录制检查证据。按已确认的 tdd 模式，每个实现任务先写失败测试并记录 RED，再实现至 GREEN。

## 1. 依赖与调度基础

### 1.1 引入 robfig/cron/v3

<!-- comet-task-ref:7da68423-d294-4fd9-be8d-aaa31a54cb66 -->

- 范围：`combined_refactor/go.mod`、`go.sum`；仅增加 `github.com/robfig/cron/v3 v3.0.1`。
- 依赖：无。
- 约束：模块缓存已存在该版本；不升级其他依赖。
- 验收：`go -C combined_refactor build ./...` 通过；`go -C combined_refactor list -m github.com/robfig/cron/v3` 显示 v3.0.1。

### 1.2 调度配置解析与归一化

<!-- comet-task-ref:adeefb5d-4abd-4532-b637-7628711c03f3 -->

- 范围：`combined_refactor/scheduler.go`（配置读取 + `normalizeSchedulePlan`）、`scheduler_test.go`、`cli.go` 中 `cliFileConfig` 新字段。
- 依赖：1.1。
- 约束：`schedule-enabled` 默认 false；cron 优先；非法 cron 回退间隔并记录；间隔 `<=0→2880`、`<60→60`；配置字段位于嵌套 `config` 对象。
- 验收：表驱动测试覆盖关闭/启用 cron/非法 cron/间隔钳制；`go -C combined_refactor test ./...` 通过。

### 1.3 进程内调度器

<!-- comet-task-ref:a078f515-0102-4bda-8514-ab6fd180c9d7 -->

- 范围：`scheduler.go`（`scanScheduler`、Start/Stop/Reload/Status、mtime 轮询、触发入口）、`scheduler_test.go`。
- 依赖：1.2。
- 约束：cron 启动不立即触发；间隔模式仅进程启动立即一次；Reload 不触发、不中断扫描；`anyTaskRunning()` + 原子 running 标志；测试使用可注入 ticker/时钟。
- 验收：测试验证间隔触发一次、cron 不立即触发、Reload 后旧计划失效且不立即扫描、忙碌时跳过；`go -C combined_refactor test ./...` 通过。

## 2. 移除 systemd 与调度入口

### 2.1 CLI 自动调度菜单

<!-- comet-task-ref:5a8efd30-703b-4fc8-a3f7-94aaf5a59c61 -->

- 范围：删除 `combined_refactor/schedule.go`；新增 `schedule_menu.go`；更新 `menu.go` 菜单项与帮助文案。
- 依赖：1.3。
- 约束：不自动删除已有 systemd unit；菜单提供启用/停用、Cron、间隔、保存并常驻；输出无 systemd 字样。
- 验收：`go -C combined_refactor build ./...` 通过；`go -C combined_refactor vet ./...` 通过；人工检查菜单输出。

### 2.2 Web 与 CLI 常驻启动

<!-- comet-task-ref:19e17169-fa18-4d0e-9471-345fb05e6804 -->

- 范围：`main.go` Web 启动段；`schedule_menu.go` 常驻循环与信号处理。
- 依赖：2.1。
- 约束：调度器在 `initLocations()` 后、HTTP 监听前启动；启动日志包含模式、时区与下次触发；Ctrl+C 优雅停止。
- 验收：本地启动 Web 与 CLI 常驻，日志显示下次触发；Ctrl+C 退出无 panic。

### 2.3 WebSocket 调度配置

<!-- comet-task-ref:b5aad173-e547-4589-bd80-3192546221b4 -->

- 范围：`server.go` 新增 `get_schedule_config`/`set_schedule_config`；配置读改写与原子写入。
- 依赖：2.2。
- 约束：只修改嵌套 `config` 中的调度/R2 字段，保留未知字段；保存后立即 `Reload`；返回最新状态。
- 验收：临时目录手工调用读写，验证 JSON 完整、无半截写入、未知字段保留。

## 3. 调度执行流水线

### 3.1 runScheduledScan

<!-- comet-task-ref:f83f273a-90da-438c-8f5f-6e13927b6927 -->

- 范围：`scheduler.go` 的 `runScheduledScan`；`cli.go` 抽取 `runOfficialPipeline`/`runNSBPipeline` 与配置解析复用。
- 依赖：1.3、2.2。
- 约束：整个流水线一次 `runTaskSync`；已有任务时跳过；参数只取配置文件；headless session。
- 验收：桩测试验证忙碌跳过；官方/非标流水线抽取后现有 CLI 行为不变；`go -C combined_refactor test ./...` 通过。

### 3.2 调度导出与 R2 衔接

<!-- comet-task-ref:e92bb1db-c5eb-481d-89ef-76073c613e14 -->

- 范围：`runScheduledScan` 末尾按 `substore` 导出并调用 R2 上传；`writeCLIExportAndMaybeUpload` 集成。
- 依赖：3.1、4.1、5.1。
- 约束：空结果跳过上传；上传失败不影响调度循环；上传内容恒为 substore 文本。
- 验收：测试覆盖空结果与非空结果；集成测试验证上传内容格式。

### 3.3 调度文档

<!-- comet-task-ref:c73e65c2-258c-4cf3-b43e-de371d1a1583 -->

- 范围：`README.md`、`docs/docker.md`。
- 依赖：2.3、3.2。
- 约束：说明启用开关、Cron/间隔、常驻运行、单实例、systemd 迁移与清理。
- 验收：按文档命令手工执行验证；示例与实现一致。

## 4. Sub-Store 目标列表格式

### 4.1 substore 行构造

<!-- comet-task-ref:f0745bad-32d5-486f-abdc-524b60de8d3f -->

- 范围：`cli.go` 的 `formatCLIResults`、导出配置校验、`formatCLIResults` 测试。
- 依赖：1.2（配置字段可并行）。
- 约束：`host[:port]#alias`；IPv6 强制括号；alias 回退 dc→loc→ip 并清洗；端口 0/空省略；沿用筛选与行数限制。
- 验收：单元测试覆盖 IPv4、IPv6、aliases 缺失/含 `#`/换行、端口省略、行数上限。

### 4.2 CLI/Web substore 选项

<!-- comet-task-ref:76411268-e1fb-457b-a2c4-7694c3ab5d56 -->

- 范围：`cli.go` flags 与帮助；`index.html` 格式下拉、`buildSubstoreContent`、复制按钮。
- 依赖：4.1。
- 约束：前端规则与服务端一致；文件扩展名 `.txt`。
- 验收：CLI `-outformat substore` 输出每行含 `#` 且 IPv6 带括号；Web 下载/复制结果一致。

### 4.3 目标列表文档

<!-- comet-task-ref:3e793ebf-4250-464d-8969-59cc1c0092a0 -->

- 范围：`README.md`。
- 依赖：4.2。
- 约束：提供 Untitled.js/operator `urls` 接入示例；示例与实现输出一致。
- 验收：按文档示例运行 operator 可解析生成节点。

## 5. R2 上传

### 5.1 r2.go

<!-- comet-task-ref:c51c608e-7327-40b5-b387-ad7dc9347024 -->

- 范围：新增 `combined_refactor/r2.go`、`r2_test.go`；`proxy.go` 抽出 `managementTransport()`。
- 依赖：1.1。
- 约束：Worker 协议；3 次/2 秒/30 秒；401/403 不重试；空内容跳过；URL 校验；独立无 debug 客户端；token 脱敏。
- 验收：httptest 覆盖成功、失败两次后成功、三次失败、鉴权失败、空内容；日志断言不含 token。

### 5.2 R2 配置与 Web 设置

<!-- comet-task-ref:38f42b80-2b8b-4cc2-bc72-cf9fd51248b2 -->

- 范围：`cli.go` 配置字段与模板；`index.html` 设置面板。
- 依赖：5.1、2.3。
- 约束：默认关闭；字段读回一致；Token 不回显明文（或按现有密码框方式处理）。
- 验收：保存后可读回；默认值正确；页面刷新状态一致。

### 5.3 调度链路集成

<!-- comet-task-ref:dd02c5be-07ba-462b-bf49-91fff30a538b -->

- 范围：`writeCLIExportAndMaybeUpload` 与 `runScheduledScan` 的上传调用。
- 依赖：5.1、5.2、3.2。
- 约束：手动导出与调度都上传；失败不阻断；空结果不覆盖旧文件。
- 验收：集成测试验证内容与失败路径。

### 5.4 R2 文档

<!-- comet-task-ref:af10315a-0c2a-4012-b25f-ba243561a2fa -->

- 范围：`README.md`、`docs/docker.md`、`substore/README.md`。
- 依赖：5.3。
- 约束：协议、配置示例、故障排查、Token 安全说明。
- 验收：示例配置可直接使用；故障排查覆盖 401/403/超时。

## 6. Operator 脚本入库

### 6.1 substore/cloudflare-targets.js

<!-- comet-task-ref:78d044bd-49a5-4158-be0f-2a62e20c760e -->

- 范围：新增脚本；Node 测试 harness（临时或脚本内置）。
- 依赖：4.1。
- 约束：静态目标逐行跳过非法行；alias 清洗；`fetch` 超时；完全重复去重；守卫式 `module.exports`；Sub-Store 环境行为不变。
- 验收：Node 样例验证节点数量、名称、非法行跳过、URL 失败回退、无 HTTP 客户端报错。

### 6.2 substore/README.md

<!-- comet-task-ref:a243891e-7700-4cd3-bd23-b606dd7c6914 -->

- 范围：新增 `substore/README.md`。
- 依赖：6.1。
- 约束：说明 targets/urls/urlTimeout、R2 公网地址、WeTest 表格兼容、常见问题。
- 验收：按示例运行脚本生成节点。

## 7. 集成验证

### 7.1 go vet 与全量测试

<!-- comet-task-ref:6d9d84d4-864d-46c6-ae08-44831c275415 -->

- 范围：`combined_refactor`。
- 依赖：全部实现任务。
- 验收：`go -C combined_refactor vet ./...` 与 `go -C combined_refactor test ./...` 通过，并录制 build 证据。

### 7.2 linux/amd64 构建

<!-- comet-task-ref:55d9feec-c640-4d73-b980-ef81250a8253 -->

- 范围：`combined_refactor`。
- 依赖：7.1。
- 验收：`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go -C combined_refactor build -trimpath -ldflags="-s -w" -o /tmp/cfdata-test .` 成功。

### 7.3 本地端到端验证

<!-- comet-task-ref:a0deb5b8-eab2-46ec-a8ec-178f1e444dc1 -->

- 范围：本地 Web + R2 mock。
- 依赖：7.2。
- 约束：`cron-expression: "* * * * *"`、`schedule-enabled: true`、本地 R2 测试端点；不连接真实 R2。
- 验收：观察一次完整“扫描 → 导出 → 上传”日志；核对导出文件每行格式；确认 R2 mock 收到 substore 内容。

## 8. Verify 反馈修复（verify-fail 第 2 次）

集成审查（两份独立意见）确认以下问题，均为对既有任务的收尾加固，不改变需求范围：

### 8.1 get_config 不再返回 R2 token

<!-- comet-task-ref:38f42b80-2b8b-4cc2-bc72-cf9fd51248b2 -->

- 范围：`combined_refactor/server.go` 的 `get_config`；新增纯函数剥离嵌套与顶层 `r2workertoken`。
- 约束：`schedule_config` 已脱敏，但 `get_config` 仍原样回传整个配置，认证默认关闭时可被局域网客户端读取；只处理本 change 新增的 R2 token，不改变 `ghtoken` 等既有字段行为。
- 验收：新测试断言序列化后的 `config_data` 不含 token 值；现有测试通过。

### 8.2 配置文件写入共享锁与原子写

<!-- comet-task-ref:b5aad173-e547-4589-bd80-3192546221b4 -->

- 范围：`combined_refactor/scheduler.go` 的 `saveScheduleConfigFields`、`combined_refactor/proxy.go` 的 `updateProxyInConfigFile`。
- 约束：调度保存与代理保存共用同一把配置写锁；代理保存改用 `atomicWriteFile`；未知字段保留。
- 验收：并发读改写测试在 `-race` 下通过，最终 JSON 同时保留 proxy 与调度字段。

### 8.3 启动/重载记录时区与下次触发

<!-- comet-task-ref:a078f515-0102-4bda-8514-ab6fd180c9d7 -->

- 范围：`combined_refactor/scheduler.go` 的日志路径。
- 约束：满足 design §8「启动与每次重载记录当前时区与下一次触发时间」；不改变调度行为。
- 验收：新增描述函数测试；启动与 Reload/mtime 重载输出包含时区与下次触发。

### 8.4 substore 导出应用自定义字段

<!-- comet-task-ref:f0745bad-32d5-486f-abdc-524b60de8d3f -->

- 范围：`combined_refactor/cli.go` 的 `formatCLIResults` substore 分支。
- 约束：与服务端导出一致，R2 内容随 `rows` 复用；客户端 `getSelectedCopyRows` 已合并自定义字段，因此只改服务端。
- 验收：自定义字段覆盖 `dc`/`city` 时 substore 行使用覆盖值；既有格式行为不变。

### 8.5 Cron 表达式去空白

<!-- comet-task-ref:adeefb5d-4abd-4532-b637-7628711c03f3 -->

- 范围：`combined_refactor/scheduler.go` 的 `normalizeSchedulePlan`。
- 约束：手工编辑配置带首尾空白时按 cron 处理，不误回退间隔。
- 验收：新增表驱动用例通过。

### 8.6 接受并记录的偏差

- Web 进程 SIGTERM 无优雅停机：既有生命周期限制（本 change 前手动扫描同样被终止），调度器 `Stop` 仅覆盖正常返回路径；不在本 change 扩展进程管理。
- 仅保存 R2 设置会重置间隔 `nextRun`：保存即重载属既有语义，下一次扫描仍在不超过一个间隔后发生，不改变验收结果。
- `ipport` 回退中端口 0/非法时整行跳过：服务端与客户端行为一致，扫描行本身不会产生此类值；改变丢弃/省略语义属于产品决策。
- `disableANSIColors` 影响进程级颜色：仅为日志观感，调度运行本就要求无色输出。
- 扫描内部错误已通过 `lastResult`（失败字符串）与日志记录；R2 上传失败按设计只记录、不改变扫描结果。

## 交付注意

- 依据 AGENTS.md，未经用户明确许可不执行 `git commit` / `git push`，且不在 `main` 分支操作；本计划只在本 change 分支执行。
- `.gitignore` 忽略 `docs/`；归档阶段如需提交文档产物，使用 `git add -f` 并取得提交许可。
