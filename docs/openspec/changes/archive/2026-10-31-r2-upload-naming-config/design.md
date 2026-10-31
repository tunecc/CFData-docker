---
comet_change: r2-upload-naming-config
role: technical-design
canonical_spec: openspec
archived-with: 2026-10-31-r2-upload-naming-config
status: final
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
