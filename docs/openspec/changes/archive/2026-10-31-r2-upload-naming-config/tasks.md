# Tasks

## 1. 别名模板核心

- [x] 1.1 在 `cli.go` 为 `cliFileConfig` 增加 `R2AliasFormat` 字段（JSON 键 `r2aliasformat`）与默认值（空），确认旧配置 JSON 解析不报错；验证：`go build ./...` 通过且现有配置文件加载测试通过 <!-- comet-task:7091576d-7ece-409a-af8d-fcc684ff59b1 -->
- [x] 1.2 实现模板替换函数（`strings.NewReplacer`，占位符 `{dc} {city} {region} {dcCountry} {ip} {port} {ipport}`，未识别占位符按字面保留），替换后经 `sanitizeSubstoreAliasPart` 清洗；验证：单元测试覆盖各占位符、非法占位符字面输出、空字段为空串 <!-- comet-task:5d5e1b06-3b0a-4b4e-b249-025d6fde388c -->
- [x] 1.3 `formatSubstoreResults` 参数化别名生成，空模板回落现有 `substoreAlias` 逻辑；生成别名为空时回落默认别名，保证无空别名行；验证：单元测试覆盖「空模板=现行为」「空别名回落」两个场景 <!-- comet-task:c175b961-cdc7-4191-8178-06c1bf42b863 -->
- [x] 1.4 调度路径（`writeCLIExportAndMaybeUpload`）与手动路径（`r2_manual.go` `uploadManualRowsToR2`）传入 `cfg.R2AliasFormat`，本地导出弹窗路径传空；验证：扩展现有 `r2_test.go`/`r2_manual_test.go` 断言上传内容别名按模板生成 <!-- comet-task:23e26f50-6674-43ed-afc6-1cce8d952d96 -->

## 2. 配置读写与 Web 界面

- [x] 2.1 `server.go` `set_schedule_config` 解析并透传 `r2AliasFormat`，`scheduler.go` `saveScheduleConfigFields` 持久化新键、`validateScheduleConfigUpdate` 增加长度 ≤200 与拒绝 `#` 校验、`scheduleConfigPayload` 回显新字段；验证：scheduler 测试覆盖保存/校验/回显往返 <!-- comet-task:6acc4fa6-4a7d-4148-ade4-5f2bfc404183 -->
- [x] 2.2 `index.html` 自动化配置区新增「别名模板」输入框（含占位符说明），随 `saveScheduleConfig`/`applyScheduleConfig` 收发；验证：保存后重载页面回显一致 <!-- comet-task:8da5dd33-ab8a-4434-849e-48426f8bcef9 -->
- [x] 2.3 `index.html` 保存 R2 配置后展示最终上传地址（`worker-url + '/' + filename`，filename 空取默认 `cfdata-targets.txt`）与「旧对象不删除、订阅地址需同步更换」提示；验证：界面操作可见该反馈 <!-- comet-task:8060cad9-8325-45f0-81b1-95277a5220d6 -->

## 3. 集成验证

- [x] 3.1 全量构建与测试：在 `combined_refactor/` 执行 `go build ./...` 与 `go test ./...` 全部通过，并按 AGENTS.md 产出 `release_assets/cfdata-test` 测试版 <!-- comet-task:03b09e78-e519-4eb4-86db-50870178aef7 -->
- [x] 3.2 手动验收：配置模板 `{region}-{dc}` 后触发手动上传，检查上传内容别名符合模板；清空模板确认回落默认；修改文件名确认界面显示新地址 <!-- comet-task:dc9427db-b3a2-402b-8978-8c6398cb9cce -->

## Workflow follow-up

- 审查要求满足后归档本 change。
