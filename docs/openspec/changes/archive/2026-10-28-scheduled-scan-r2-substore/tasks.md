# 任务

## 1. 依赖与调度基础

- [x] 1.1 在 `combined_refactor/go.mod` 增加 `github.com/robfig/cron/v3` 并更新 `go.sum`，在 `combined_refactor` 下运行 `go build ./...` 通过 <!-- comet-task:7da68423-d294-4fd9-be8d-aaa31a54cb66 -->
- [x] 1.2 实现调度配置解析与归一化（`schedule-enabled` 默认关闭、Cron 优先、非法 Cron 回退间隔、间隔 `<=0→2880`、`<60→60`），单元测试覆盖关闭/启用/非法 Cron/间隔钳制并运行 `go test ./...` 通过 <!-- comet-task:adeefb5d-4abd-4532-b637-7628711c03f3 -->
- [x] 1.3 实现进程内调度器（Cron/Ticker、启动行为、`anyTaskRunning` 并发保护、配置文件 mtime 热更新），单元测试用秒级测试钩子验证间隔触发、Cron 不立即触发和热更新后旧计划失效 <!-- comet-task:a078f515-0102-4bda-8514-ab6fd180c9d7 -->

## 2. 移除 systemd 与调度入口

- [x] 2.1 删除 `schedule.go` 中 systemd unit 生成/启停/状态代码，CLI 菜单改为“自动调度”子菜单（查看状态、设置 Cron/间隔、启动常驻调度），`go build ./...` 通过且菜单输出经手工确认无 systemd 字样 <!-- comet-task:5a8efd30-703b-4fc8-a3f7-94aaf5a59c61 -->
- [x] 2.2 Web 服务启动后异步启动调度器，CLI 常驻模式运行阻塞循环并响应 Ctrl+C；启动日志包含下一次触发时间，手工启动验证 <!-- comet-task:19e17169-fa18-4d0e-9471-345fb05e6804 -->
- [x] 2.3 新增 `get_schedule_config`/`set_schedule_config` WebSocket 消息，读-改-写 `cfdata-config.json` 且保留未知字段；用临时目录手工调用验证写入内容与原子替换 <!-- comet-task:b5aad173-e547-4589-bd80-3192546221b4 -->

## 3. 调度执行流水线

- [x] 3.1 实现 `runScheduledScan`：用无头 `appSession` 复用 CLI 配置执行官方/非标扫描并收集结果行，单元测试用桩验证已有任务运行时跳过触发 <!-- comet-task:f83f273a-90da-438c-8f5f-6e13927b6927 -->
- [x] 3.2 调度完成后按 `substore` 格式导出并触发 R2 上传（启用时），空结果跳过上传；测试覆盖空结果与非空结果两条路径 <!-- comet-task:e92bb1db-c5eb-481d-89ef-76073c613e14 -->
- [x] 3.3 更新 README 与 `docs/docker.md`：调度配置、常驻运行、systemd 迁移步骤；按文档命令手工执行验证 <!-- comet-task:c73e65c2-258c-4cf3-b43e-de371d1a1583 -->

## 4. Sub-Store 目标列表格式

- [x] 4.1 实现 `outformat: substore` 行构造（别名回退与清洗、IPv6 强制括号、筛选与行数上限），单元测试覆盖 IPv4、IPv6、别名缺失、行数上限 <!-- comet-task:f0745bad-32d5-486f-abdc-524b60de8d3f -->
- [x] 4.2 CLI 与 Web 导出/复制增加 `substore` 选项，运行 CLI `-outformat substore` 验证输出每行含 `#` 且 IPv6 带括号 <!-- comet-task:76411268-e1fb-457b-a2c4-7694c3ab5d56 -->
- [x] 4.3 README 增加目标列表格式说明与 Untitled.js `urls` 接入示例，检查示例与实现输出一致 <!-- comet-task:3e793ebf-4250-464d-8969-59cc1c0092a0 -->

## 5. R2 上传

- [x] 5.1 实现 `r2.go`：Worker 协议、3 次尝试间隔 2 秒、30 秒超时、401/403 不重试、管理面代理、配置校验与空内容跳过、token 不落日志；`httptest` 单元测试覆盖成功、前两次失败后成功、三次失败、鉴权失败、空内容 <!-- comet-task:c51c608e-7327-40b5-b387-ad7dc9347024 -->
- [x] 5.2 在 CLI JSON 配置与 Web 设置中接入 `r2enabled`、`r2workerurl`、`r2workertoken`、`r2filename`，验证保存后可读回且默认关闭 <!-- comet-task:38f42b80-2b8b-4cc2-bc72-cf9fd51248b2 -->
- [x] 5.3 将 R2 上传接入调度与导出链路，集成测试验证上传内容为 substore 文本、失败不覆盖旧文件 <!-- comet-task:dd02c5be-07ba-462b-bf49-91fff30a538b -->
- [x] 5.4 文档补充 Worker 协议、R2 配置示例与故障排查 <!-- comet-task:af10315a-0c2a-4012-b25f-ba243561a2fa -->

## 6. Untitled.js 入库与优化

- [x] 6.1 新增 `substore/cloudflare-targets.js`：别名清洗、重复目标去重、无效行跳过、URL 失败回退、无 HTTP 客户端报错；用 Node 以样例文本与模拟 fetch 验证输出节点数量与名称 <!-- comet-task:78d044bd-49a5-4158-be0f-2a62e20c760e -->
- [x] 6.2 新增 `substore/README.md` 说明脚本用法、配置项与本项目 R2 输出的对接方式，按文档示例运行脚本验证 <!-- comet-task:a243891e-7700-4cd3-bd23-b606dd7c6914 -->

## 7. 集成验证

- [x] 7.1 在 `combined_refactor` 下运行 `go vet ./...` 与 `go test ./...` 全部通过 <!-- comet-task:6d9d84d4-864d-46c6-ae08-44831c275415 -->
- [x] 7.2 本地构建 linux/amd64 测试二进制（`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w"`）成功 <!-- comet-task:55d9feec-c640-4d73-b980-ef81250a8253 -->
- [x] 7.3 本地启动 Web 服务，配置 `cron-expression: "* * * * *"` 与 R2 测试端点，观察一次完整“扫描 → 导出 → 上传”日志并核对导出文件内容 <!-- comet-task:a0deb5b8-eab2-46ec-a8ec-178f1e444dc1 -->

## Workflow follow-up

- 完成 Build 后进入 Verify，执行验收与最终集成审查。
- 验收通过后归档并按用户选择交付。
