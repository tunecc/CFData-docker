---
change: r2-upload-naming-config
design-doc: docs/openspec/changes/r2-upload-naming-config/design.md
base-ref: 161fa3a42723d2e34114d4aeb09817ed314e3217
archived-with: 2026-10-31-r2-upload-naming-config
---

<!-- comet-task-authority: docs/openspec/changes/r2-upload-naming-config/tasks.md -->

# Plan: R2 上传别名模板配置

设计要点（详见 design.md）：新增 `r2aliasformat` 配置，`strings.NewReplacer` 替换 7 个占位符，仅 R2 两条上传路径应用模板，空别名兜底回落默认，界面显示最终上传地址。TDD：后端任务先 RED 后 GREEN。

## 1. 别名模板核心（后端，combined_refactor/）

<!-- comet-task-ref:7091576d-7ece-409a-af8d-fcc684ff59b1 -->
- **1.1** `cli.go` `cliFileConfig` 增加 `R2AliasFormat string` 字段（JSON `r2aliasformat`，默认空）。
  依赖：无。验收：RED 先行——在 `r2_test.go` 增加断言旧配置 JSON（无 `r2aliasformat` 键）解析不报错且字段为零值、含键时正确读入；`go test ./...` 通过。

<!-- comet-task-ref:5d5e1b06-3b0a-4b4e-b249-025d6fde388c -->
- **1.2** 新增 `applyR2AliasTemplate(template string, row cliResultRow) string`：`strings.NewReplacer` 替换 `{dc} {city} {region} {dcCountry} {ip} {port} {ipport}`，替换后经 `sanitizeSubstoreAliasPart` 清洗。
  依赖：无。验收：RED 先行——表驱动测试覆盖：各占位符、非法占位符字面保留、空字段替换为空串、清洗去 `#` 与空白折叠；`go test ./...` 通过。

<!-- comet-task-ref:c175b961-cdc7-4191-8178-06c1bf42b863 -->
- **1.3** `formatSubstoreResults` 增加模板参数（或新增带模板变体并保留旧签名适配本地导出）：模板非空用 `applyR2AliasTemplate`，替换结果为空或模板为空回落 `substoreAlias`。
  依赖：1.2。验收：RED 先行——测试断言「空模板=现行为」「模板生效」「空别名兜底」；`go test ./...` 通过。

<!-- comet-task-ref:23e26f50-6674-43ed-afc6-1cce8d952d96 -->
- **1.4** 调度路径 `writeCLIExportAndMaybeUpload`（cli.go）与手动路径 `uploadManualRowsToR2`（r2_manual.go）传入 `cfg.R2AliasFormat`；本地导出弹窗路径维持空模板。
  依赖：1.3。验收：扩展 `r2_test.go`（调度路径）与 `r2_manual_test.go`（手动路径）断言上传内容别名按模板生成；`go test ./...` 通过。

## 2. 配置读写与 Web 界面

<!-- comet-task-ref:6acc4fa6-4a7d-4148-ade4-5f2bfc404183 -->
- **2.1** `server.go` `set_schedule_config` 解析 `r2AliasFormat` 透传；`scheduler.go` `saveScheduleConfigFields` 持久化 `r2aliasformat`、`validateScheduleConfigUpdate` 增加 ≤200 字符与拒绝 `#`、`scheduleConfigPayload` 回显。
  依赖：1.1。验收：RED 先行——`scheduler_test.go` 往返测试（保存含模板→读取回显；超长/含 `#` 被拒）；`go test ./...` 通过。

<!-- comet-task-ref:8da5dd33-ab8a-4434-849e-48426f8bcef9 -->
- **2.2** `index.html` 自动化配置区新增「别名模板」输入框（`id=r2AliasFormat`，hint 列出占位符），随 `saveScheduleConfig`/`applyScheduleConfig` 收发。
  依赖：2.1。验收：构建通过；页面加载后字段存在且随配置回显（人工在 3.2 验收）。

<!-- comet-task-ref:8060cad9-8325-45f0-81b1-95277a5220d6 -->
- **2.3** `index.html` 保存 R2 配置成功后显示最终上传地址（`workerUrl + '/' + (r2Filename || 'cfdata-targets.txt')`）与固定提示「旧对象不会被自动删除，订阅地址需同步更换」。
  依赖：2.2。验收：构建通过；保存后反馈区块可见（人工在 3.2 验收）。

## 3. 集成验证

<!-- comet-task-ref:03b09e78-e519-4eb4-86db-50870178aef7 -->
- **3.1** 全量构建与测试：`combined_refactor/` 下 `go build ./...` 与 `go test ./...` 全绿；按 AGENTS.md 产出 `release_assets/cfdata-test` 测试版。
  依赖：1–2 全部。验收：命令退出码 0，产物文件更新。

<!-- comet-task-ref:dc9427db-b3a2-402b-8978-8c6398cb9cce -->
- **3.2** 手动验收：启动测试版，配置模板 `{region}-{dc}` 触发上传检查内容别名；清空模板确认回落默认；修改文件名确认界面显示新地址。
  依赖：3.1。验收：验收记录写入检查点（用户环境无 Worker 时以本地单测+界面操作代替实际网络上传，偏差记录）。

## 回退

单 commit 系列，`git revert` 即可整体回退；配置键向后兼容，无数据迁移。
