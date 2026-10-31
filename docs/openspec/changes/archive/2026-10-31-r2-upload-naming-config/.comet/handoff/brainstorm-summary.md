# Brainstorm Summary

- Change: r2-upload-naming-config
- Date: 2026-10-31

## 确认的技术方案

- 配置：`cliFileConfig` 新增 `R2AliasFormat`（JSON 键 `r2aliasformat`），空值回落现有「数据中心-城市」默认别名，旧配置零迁移
- 模板引擎：`strings.NewReplacer` 简单替换 7 个占位符 `{dc} {city} {region} {dcCountry} {ip} {port} {ipport}`，未识别占位符按字面保留；不用 `text/template`
- 应用点：`formatSubstoreResults` 参数化别名生成；R2 调度路径（`writeCLIExportAndMaybeUpload`）与手动路径（`uploadManualRowsToR2`）传入模板，本地导出弹窗传空维持现行为
- 兜底：替换并清洗后别名为空时回落默认 `substoreAlias` 逻辑，不产出空别名行
- 界面：自动化配置区新增「别名模板」输入框（占位符 hint）；保存 R2 配置后显示最终上传地址（`worker-url/filename`）与「旧对象不删除」提示
- 校验：`validateScheduleConfigUpdate` 长度 ≤200、拒绝 `#` 字符

## 关键取舍与风险

- R2 旧对象不自动删除（破坏性操作不做）→ UI 文案提示替代
- region 等字段个别节点为空 → 空别名兜底回落默认
- 已核实：官方与非标（NSB）结果行均具备全部占位符字段，两条路径行为一致

## 测试策略

- 表驱动单元测试覆盖模板渲染（各占位符、非法占位符字面输出、空字段、空模板回落、空别名兜底）
- 扩展 `r2_test.go`/`r2_manual_test.go` 断言两条上传路径内容按模板命名
- `scheduler_test.go` 覆盖保存/校验/回显往返
- 手动验收：配置模板后触发上传检查内容，清空模板确认回落

## Spec Patch

无——Open 阶段 delta spec 已完整覆盖，无需回写。
