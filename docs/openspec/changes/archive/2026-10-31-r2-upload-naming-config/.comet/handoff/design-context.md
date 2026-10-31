# Comet Design Handoff

- Change: r2-upload-naming-config
- Phase: design
- Mode: compact
- Context hash: 3c3147653da3ae5d424de149a117ab13d01e1c3d8dbb3e33a31279e925277dd4

Generated-by: comet-handoff.sh
Task hash policy: task-content-v1. Read tasks.md for live completion; excerpts are design-time context.

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## docs/openspec/changes/r2-upload-naming-config/proposal.md

- Source: docs/openspec/changes/r2-upload-naming-config/proposal.md
- Lines: 1-30
- SHA256: bba95c1d756e7e0c616a84a52b20ec9675b81a423ac706b5d0a0a7f13d244077

```md
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

```

## docs/openspec/changes/r2-upload-naming-config/design.md

- Source: docs/openspec/changes/r2-upload-naming-config/design.md
- Lines: 1-53
- SHA256: fb30812f8de7be1654a3a4aed6f112be0fd634ce4fb7bb5538d9e8f4c79a86b2

```md
---
comet_change: r2-upload-naming-config
role: technical-design
canonical_spec: openspec
---

# Design

## Context

- 上传对象键名已由 `r2filename` 配置驱动（`cli.go` → `uploadR2Content`），本设计不动上传协议，只改内容别名生成与界面反馈
- 内容别名由 `substoreAlias(row)`（`combined_refactor/cli.go:2011`）硬编码，调用方为 `formatSubstoreResults`；R2 上传有两条路径：调度路径（`cli.go` `writeCLIExportAndMaybeUpload`）与手动路径（`r2_manual.go` `uploadManualRowsToR2`，每次上传前重读配置文件）
- 结果行字段可用：`ip/port/ipport/dc/dcCountry/region/city`（官方与非标任务均有），清洗函数 `sanitizeSubstoreAliasPart` 已存在
- 配置持久化：`cliFileConfig`（`cli.go`，JSON 键全小写）→ `cfdata-config.json`；Web 保存链路 `set_schedule_config`（`server.go`）→ `saveScheduleConfigFields`（`scheduler.go`，含校验）→ 回显 `scheduleConfigPayload` → 前端 `applyScheduleConfig`

## Goals / Non-Goals

**Goals:**

- 别名模板：占位符替换生成别名，两条上传路径一致生效
- 新配置键向后兼容，旧配置零迁移
- 界面反馈最终上传地址与旧对象说明

**Non-Goals:**

- 不改 R2 Worker 端协议与重试逻辑
- 不改上传行的筛选规则（达标/延迟筛选维持现状）
- 不改动本地结果操作弹窗的导出格式（其别名维持默认，避免扩大范围）
- 不做 Web 界面大改（由 webui-scp-style-redesign 承担）

## Decisions

1. **模板引擎用 `strings.NewReplacer` 简单替换，不用 `text/template`**：占位符集合小且固定，简单替换对用户可预期、无注入面、无模板语法错误；`text/template` 的语法报错处理成本高于收益。
2. **占位符集合**：`{dc} {city} {region} {dcCountry} {ip} {port} {ipport}`，与现有结果行字段一一对应，不做额外映射查询。未识别占位符按字面保留。
3. **新配置键 `r2aliasformat`**（`cliFileConfig.R2AliasFormat`）：JSON 键遵循现有全小写约定；缺省零值为空，空值回落 `substoreAlias` 现有逻辑，旧配置无需迁移。
4. **模板应用点**：`formatSubstoreResults` 增加模板参数（或新增变体函数），仅 R2 上传两条路径传入 `cfg.R2AliasFormat`；本地导出弹窗传空保持默认。理由：与既有规格「R2 内容不受导出弹窗影响」的边界一致。
5. **空别名保护**：模板替换结果清洗后为空时，回落默认 `substoreAlias` 逻辑，保证不产出 `#`（空别名）行。
6. **界面反馈在前端拼接**：保存成功后前端以 `worker-url + '/' + filename`（filename 为空取默认 `cfdata-targets.txt`）直接展示，不发探测请求；同时展示固定提示「旧对象不会被删除，订阅地址需同步更换」。
7. **校验**：`validateScheduleConfigUpdate` 对模板增加长度上限（200 字符）与 `#` 字符拒绝（别名分隔符），其余不校验白名单（未识别占位符按字面输出的规格行为）。

## Risks / Trade-offs

- [改文件名后 R2 上旧对象与新对象并存] → 界面固定提示 + 反馈地址展示，不做自动删除（删除属于破坏性操作，超出本 change）
- [模板导致大量节点别名重复] → 不去重，别名仅是展示名；Sub-Store 下游以 host:port 区分节点
- [region 等字段部分节点为空] → 空占位符替换为空串，整别名兜底回落默认

## Migration Plan

无破坏性变更：部署后旧配置 `r2aliasformat` 缺省为空，行为与现状一致；用户按需在 Web 配置模板。

## Open Questions

无。

```

## docs/openspec/changes/r2-upload-naming-config/tasks.md

- Source: docs/openspec/changes/r2-upload-naming-config/tasks.md
- Lines: 1-23
- SHA256: 5a826a671cf4aed13dc34406302d2b4f45d3e3d3a809bab4269556000e4e5736

```md
# Tasks

## 1. 别名模板核心

- [ ] 1.1 在 `cli.go` 为 `cliFileConfig` 增加 `R2AliasFormat` 字段（JSON 键 `r2aliasformat`）与默认值（空），确认旧配置 JSON 解析不报错；验证：`go build ./...` 通过且现有配置文件加载测试通过 <!-- comet-task:7091576d-7ece-409a-af8d-fcc684ff59b1 -->
- [ ] 1.2 实现模板替换函数（`strings.NewReplacer`，占位符 `{dc} {city} {region} {dcCountry} {ip} {port} {ipport}`，未识别占位符按字面保留），替换后经 `sanitizeSubstoreAliasPart` 清洗；验证：单元测试覆盖各占位符、非法占位符字面输出、空字段为空串 <!-- comet-task:5d5e1b06-3b0a-4b4e-b249-025d6fde388c -->
- [ ] 1.3 `formatSubstoreResults` 参数化别名生成，空模板回落现有 `substoreAlias` 逻辑；生成别名为空时回落默认别名，保证无空别名行；验证：单元测试覆盖「空模板=现行为」「空别名回落」两个场景 <!-- comet-task:c175b961-cdc7-4191-8178-06c1bf42b863 -->
- [ ] 1.4 调度路径（`writeCLIExportAndMaybeUpload`）与手动路径（`r2_manual.go` `uploadManualRowsToR2`）传入 `cfg.R2AliasFormat`，本地导出弹窗路径传空；验证：扩展现有 `r2_test.go`/`r2_manual_test.go` 断言上传内容别名按模板生成 <!-- comet-task:23e26f50-6674-43ed-afc6-1cce8d952d96 -->

## 2. 配置读写与 Web 界面

- [ ] 2.1 `server.go` `set_schedule_config` 解析并透传 `r2AliasFormat`，`scheduler.go` `saveScheduleConfigFields` 持久化新键、`validateScheduleConfigUpdate` 增加长度 ≤200 与拒绝 `#` 校验、`scheduleConfigPayload` 回显新字段；验证：scheduler 测试覆盖保存/校验/回显往返 <!-- comet-task:6acc4fa6-4a7d-4148-ade4-5f2bfc404183 -->
- [ ] 2.2 `index.html` 自动化配置区新增「别名模板」输入框（含占位符说明），随 `saveScheduleConfig`/`applyScheduleConfig` 收发；验证：保存后重载页面回显一致 <!-- comet-task:8da5dd33-ab8a-4434-849e-48426f8bcef9 -->
- [ ] 2.3 `index.html` 保存 R2 配置后展示最终上传地址（`worker-url + '/' + filename`，filename 空取默认 `cfdata-targets.txt`）与「旧对象不删除、订阅地址需同步更换」提示；验证：界面操作可见该反馈 <!-- comet-task:8060cad9-8325-45f0-81b1-95277a5220d6 -->

## 3. 集成验证

- [ ] 3.1 全量构建与测试：在 `combined_refactor/` 执行 `go build ./...` 与 `go test ./...` 全部通过，并按 AGENTS.md 产出 `release_assets/cfdata-test` 测试版 <!-- comet-task:03b09e78-e519-4eb4-86db-50870178aef7 -->
- [ ] 3.2 手动验收：配置模板 `{region}-{dc}` 后触发手动上传，检查上传内容别名符合模板；清空模板确认回落默认；修改文件名确认界面显示新地址 <!-- comet-task:dc9427db-b3a2-402b-8978-8c6398cb9cce -->

## Workflow follow-up

- 审查要求满足后归档本 change。

```

## docs/openspec/changes/r2-upload-naming-config/.openspec.yaml

- Source: docs/openspec/changes/r2-upload-naming-config/.openspec.yaml
- Lines: 1-2
- SHA256: c6ab3a95d8d0fd469ca8750ef79234025bf4a8356d5d57856e97d7e47fc8b6c2

```md
schema: spec-driven
created: 2026-10-31

```

## docs/openspec/changes/r2-upload-naming-config/specs/r2-worker-upload/spec.md

- Source: docs/openspec/changes/r2-upload-naming-config/specs/r2-worker-upload/spec.md
- Lines: 1-46
- SHA256: a680425cca70360d572489a4d86eea6465e0e51884ca8e12dbd006fcf2e1068c

```md
# Spec Delta

## ADDED Requirements

### Requirement: 上传内容命名模板

启用 R2 上传时，上传内容中每个节点的别名 SHALL 按可配置的命名模板生成。模板 SHALL 支持占位符 `{dc}`、`{city}`、`{region}`、`{dcCountry}`、`{ip}`、`{port}`、`{ipport}`，占位符替换为该节点对应字段的净化值。模板未配置或为空时，系统 SHALL 回落现有默认别名行为（数据中心-城市，缺省时依次回落数据中心、城市、位置、IP）。调度上传与手动上传两条路径 SHALL 应用同一模板配置。

#### Scenario: 配置模板后调度上传按模板命名

- **WHEN** 别名模板配置为 `{region}-{dc}` 且调度扫描完成并启用 R2 上传
- **THEN** 上传内容中节点别名为该节点的地区与数据中心组合，而非默认的「数据中心-城市」

#### Scenario: 手动上传应用同一模板

- **WHEN** 别名模板已配置且用户在结果页触发 R2 自动上传
- **THEN** 上传内容中节点别名按同一模板生成

#### Scenario: 模板为空回落默认

- **WHEN** 别名模板未配置或保存为空
- **THEN** 上传内容节点别名与现有默认行为完全一致

#### Scenario: 非法占位符按字面输出

- **WHEN** 别名模板包含无法识别的占位符（如 `{foo}`）
- **THEN** 该占位符按字面保留在别名中，上传流程正常完成

#### Scenario: 占位符字段为空时别名回落

- **WHEN** 模板引用的字段在节点上为空（如无城市信息）
- **THEN** 生成的别名为空时回落默认别名，不产出空别名节点

### Requirement: 上传地址反馈

Web 界面保存 R2 上传配置后，系统 SHALL 展示最终生效的上传地址（Worker 地址与对象键名组合）；对象键名为空时按默认键名展示。界面 SHALL 提示旧对象不会被自动删除、下游订阅地址需同步更换。

#### Scenario: 保存后显示最终地址

- **WHEN** 用户在 Web 自动化配置区保存 R2 文件名
- **THEN** 界面显示由 Worker 地址与该文件名组合的完整上传地址

#### Scenario: 文件名为空显示默认键名

- **WHEN** 用户清空 R2 文件名后保存
- **THEN** 界面按默认键名展示最终上传地址

```
