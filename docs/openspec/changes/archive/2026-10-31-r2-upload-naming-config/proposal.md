# Proposal

## Why

R2 上传内容里每一行的节点别名目前由 `substoreAlias()` 硬编码为「数据中心-城市」（`combined_refactor/cli.go`），没有任何配置可以改变它；而界面上的 R2 文件名（`r2filename`）只影响 R2 对象键、不改变内容，且修改后没有任何反馈，用户在现有编辑处修改后看不到任何变化，误以为"编辑不生效"。用户需要的是上传内容命名可控、编辑效果可见。

## What Changes

- 新增 R2 上传别名模板配置：支持占位符（如 `{dc}`、`{city}`、`{region}`、`{ip}`、`{port}` 等），调度上传与手动上传路径均按模板生成节点别名；模板为空或未配置时回落现有「数据中心-城市」默认行为
- Web 自动化配置区新增别名模板编辑字段，随调度配置一起保存并持久化到 `cfdata-config.json`
- R2 文件名保存后，界面显示最终上传地址（`worker-url` + 对象键名），并说明旧对象不会被删除、订阅地址需同步更换
- 模板中无法解析的占位符按字面输出，不影响上传流程

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `r2-worker-upload`: 新增「上传命名模板」与「上传地址反馈」要求——上传内容的节点别名按可配置模板生成（空模板回落默认），对象键名与最终生效地址在 Web 界面可见。

## Impact

- `combined_refactor/cli.go`：`substoreAlias`/`formatSubstoreResults` 别名生成逻辑模板化
- `combined_refactor/r2_manual.go` 与 `scheduler.go`：手动/调度两条上传路径应用模板；配置字段读写与校验（`r2aliasformat` 等新键）
- `combined_refactor/server.go`：`set_schedule_config` 解析并保存新字段
- `combined_refactor/index.html`：自动化配置区新增模板字段、最终上传地址反馈
- 配置文件 `cfdata-config.json` 新增键，向后兼容：旧配置缺省时回落默认别名行为
