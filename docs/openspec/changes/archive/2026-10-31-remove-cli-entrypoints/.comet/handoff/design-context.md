# Comet Design Handoff

- Change: remove-cli-entrypoints
- Phase: design
- Mode: compact
- Context hash: 6f3143661c72884493b668dd322c2f0b83882659984124c6b61c5457539476ea

Generated-by: comet-handoff.sh
Task hash policy: task-content-v1. Read tasks.md for live completion; excerpts are design-time context.

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## docs/openspec/changes/remove-cli-entrypoints/proposal.md

- Source: docs/openspec/changes/remove-cli-entrypoints/proposal.md
- Lines: 1-30
- SHA256: 1d8f59490f91c3cea70149087b8cacdfeea199450fae930d843cd984d9306d33

```md
# Proposal

## Why

产品定位已确定为纯 Web 控制 + Docker 部署：CLI 交互入口（`-cli` 菜单、`-qs` 快速启动、交互式设置向导）不再使用也无人维护。这些代码与 Web/调度共用的管线、配置模型混居在 cli.go 中，长期保留会让共享代码改动时 CLI 路径静默腐坏，并让 Docker 用户误入与产品定位不符的老菜单。趁界面大改完成后上下文清晰，剥离成本最低。

## What Changes

- **删除**：`menu.go`（交互菜单 923 行）、`schedule_menu.go`（CLI 调度菜单 219 行）、`cli.go` 中 CLI 专属部分（`registerCLIFlags`、`runCLI`、`runCLIQuickGitHubUpload`、`handleCLIMessage`、`runCLIQuickStart` 等）、`main.go` 中 `-cli` 分支与 `rewriteBoolFlagArgs` 调用
- **保留**：Web 与调度共用的全部能力——`cliFileConfig`/`cliExportConfig` 配置模型与 JSON 读写、`runOfficialPipeline`/`runNSBPipeline` 任务管线、`writeCLIExportAndMaybeUpload` 导出上传、`loadScheduledCLIConfig`、Sub-Store 格式化与 R2/GitHub/edgetunnel 上传逻辑
- **行为**：此后传入 `-cli`/`-qs` 由 Go flag 包返回"flag provided but not defined"并打印用法（明确报错优于静默忽略）；README 与 docs 中 CLI 用法说明同步移除
- 不改动：Web 行为、调度行为、配置文件格式（`cfdata-config.json`）、Docker 镜像与入口脚本

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

（无——CLI 入口从未被任何规格覆盖，Web 与调度行为规格不变；已按 `.openspec.yaml` 设置 `skip_specs: true`）

## Impact

- `combined_refactor/menu.go`、`schedule_menu.go`：整文件删除
- `combined_refactor/cli.go`：删除 CLI 专属函数（实施时以调用图核对边界），保留共享部分
- `combined_refactor/main.go`：删除 `-cli` 分支与相关调用
- `README.md`/`docs/*.md`：移除 CLI 用法说明（如有）
- 风险控制：`go build`/`go vet`/`go test` 全绿 + Web 启动冒烟 + 调度管线现有测试作为回归防线

```

## docs/openspec/changes/remove-cli-entrypoints/design.md

- Source: docs/openspec/changes/remove-cli-entrypoints/design.md
- Lines: 1-54
- SHA256: f94e01ef8d70ec19aa174e6503ad5121782d072a1844768750ee3a3e069fbb31

```md
---
comet_change: remove-cli-entrypoints
role: technical-design
canonical_spec: openspec
---

# Design

## Context

- `cli.go`（2498 行）混合三类代码：**配置模型与读写**（`cliFileConfig`/`cliExportConfig`、`readScheduleConfigFile`、`saveScheduleConfigFields` 调用面）、**任务管线**（`runOfficialPipeline`/`runNSBPipeline`、`writeCLIExportAndMaybeUpload`、Sub-Store/导出格式化）——Web 服务器与调度器正在使用；**CLI 专属**（`registerCLIFlags`、`runCLI`、`runCLIQuickGitHubUpload`、`handleCLIMessage`、`runCLIQuickStart` 等）——仅 `-cli`/`-qs` 路径使用
- `menu.go`（923 行）、`schedule_menu.go`（219 行）为纯 CLI UI；`main.go` 中 `-cli` 分支 + `rewriteBoolFlagArgs` + CLI flag 注册调用为入口
- 判定边界的方法：从待删函数出发做调用图核对——被 `scheduler.go`/`server.go`/`tasks_*.go` 引用的一律保留；仅被 menu.go/待删函数引用的删除

## Goals / Non-Goals

**Goals:**

- 删除全部 CLI 专属入口与交互代码（约 1800–2100 行）
- 共享代码零行为变化（管线/配置/导出/上传原样保留）
- `-cli`/`-qs` 传入时得到明确的"未定义 flag"错误

**Non-Goals:**

- 不重命名 cli.go、不迁移共享代码到新文件（避免无谓的 diff 扩大）
- 不改配置文件格式与 Docker 结构
- 不处理仓库根目录的遗留旧版文件（cfdata.go 等，与 combined_refactor 无关）

## Decisions

1. **边界以调用图为准，不按文件整删**：`cli.go` 保留共享部分，仅删 CLI 专属函数；删除后以 `go build` 报告的未引用符号为线索二次清扫（Go 对未使用函数不报错，需 `grep` 调用方核对后删除，宁可保守分批）。
2. **`-cli`/`-qs` 标志直接删除**：不保留"打印提示后退出"的兼容层——Go flag 包的 "flag provided but not defined" 即明确反馈，且避免残留死代码。
3. **`handleCLIMessage` 删除后 `appSession.emit` 的消费面核查**：emit 回调仍被 CLI 之外使用（Web 会话sendWSMessageDirect 路径），仅删除 CLI 侧调用者；`cliConfig` 上仅为 CLI 服务的字段（如有）随函数一并清理，但共享字段不动。
4. **文档同步**：README/docs 中 `-cli`、`-qs`、菜单用法段落删除或改写为纯 Web 说明；`docker-compose.yml` 注释示例不含 CLI，无需改动（实施时核对）。

## Risks / Trade-offs

- [误删共享函数导致 Web/调度回归] → 删除前逐函数 grep 调用方；`go build` + `go test ./...`（含调度管线测试）+ Web 启动冒烟作为防线；单 commit 可回滚
- [残留无引用的死代码] → 第二遍以 grep 清扫明显孤儿（CLI 专属辅助函数）；不追求极致，遗留无害函数不阻塞
- [-cli 用户（若有）升级后报错] → 用户已确认纯 Web 定位，报错信息明确可接受

## Migration Plan

单 commit 交付；Web/调度行为不变，无配置迁移。回滚 = `git revert`。

## Implementation Deviation

- `rewriteBoolFlagArgs` 保留未删（proposal/tasks 1.2 原计划删除）：`boolFlagNames` 中剩余的共享布尔 flag `-skipgeo` 仍需支持 `-skipgeo false` 空格写法，函数经裁剪名单后继续为该能力服务；`boolFlagNames` 已从 10 项裁剪为 1 项。
- `-nsbspeedtest` 未随 registerCLIFlags 删除而是升为公共 flag：其值经 `speedTestWorkers` 供 Web NSB 任务默认并发使用（server.go:267），删除会改变 Web 行为；默认值设为 0 以保持与原有效默认链（cliCfg.speedTest 默认 0）一致。
- `newCLISession`/`handleCLIMessage` 保留：经调用图核实为调度器终端输出渲染器（scheduler.go:688），属共享代码。

## Open Questions

无。

```

## docs/openspec/changes/remove-cli-entrypoints/tasks.md

- Source: docs/openspec/changes/remove-cli-entrypoints/tasks.md
- Lines: 1-18
- SHA256: 3946bb11cdd4bd2e67d8dd4e8365c5c8431d9650c2bb5217299626abce4ed072

```md
# Tasks

## 1. 剥离 CLI 专属代码

- [x] 1.1 删除 `menu.go` 与 `schedule_menu.go` 整文件，`go build` 列出悬空引用作为删除清单起点；验证：构建错误清单完整记录 <!-- comet-task:a34cecf4-b4fb-47a8-b63f-02f84d84caa6 -->
- [x] 1.2 删除 `main.go` 的 `-cli`/`-qs` 分支与 `rewriteBoolFlagArgs`、`registerCLIFlags` 调用；验证：`go build ./...` 报告的悬空符号全部归因于 CLI 专属函数 <!-- comet-task:9fe02ef8-225b-4987-b9d7-85a2d53a57ea -->
- [x] 1.3 删除 `cli.go` 中 CLI 专属函数（`registerCLIFlags`、`runCLI`、`runCLIQuickGitHubUpload`、`handleCLIMessage`、`runCLIQuickStart` 及其私有辅助），逐函数 grep 确认无 scheduler/server/tasks 引用后再删；验证：`go build ./...` 与 `go vet .` 通过 <!-- comet-task:dd4a8547-1c0c-4781-bda7-2fa1442c5c4f -->
- [x] 1.4 清扫孤儿辅助函数与对应测试（仅删 CLI 专属测试；共享函数的测试保留）；验证：`grep -rn "runCLIMenu\|runSettingsWizard\|runScheduleMenu\|handleCLIMessage\|registerCLIFlags" combined_refactor/` 无命中 <!-- comet-task:aaba315c-1ea7-48c9-a801-19f96157884a -->

## 2. 文档同步与验证

- [x] 2.1 README 与 docs 中 CLI 用法说明删除或改写为纯 Web 说明；核对 `docker-compose.yml` 注释示例无 CLI 残留；验证：`grep -rn -- "-cli" README.md docs/ docker/` 无 CLI 用法命中 <!-- comet-task:dc5db5f0-871e-45f0-a7a4-bba1903b74f4 -->
- [x] 2.2 全量构建与测试：`go build ./...`、`go vet .`、`go test ./...` 全绿；按 AGENTS.md 产出 `release_assets/cfdata-test`；验证：命令退出码 0 <!-- comet-task:123e1c77-e819-4291-8720-2c35615030cd -->
- [x] 2.3 运行冒烟：启动测试版二进制，Web 登录→页面加载→WS 连接正常；`-cli` 参数得到 flag 未定义错误；验证：curl 首页 200、CLI 参数报错输出明确 <!-- comet-task:365010b4-60be-4c5a-be18-407639a68be6 -->

## Workflow follow-up

- 审查要求满足后归档本 change。

```

## docs/openspec/changes/remove-cli-entrypoints/.openspec.yaml

- Source: docs/openspec/changes/remove-cli-entrypoints/.openspec.yaml
- Lines: 1-3
- SHA256: a2388f7f2e8033f3e9cc7c648738f7ff3d462b6c8eea8267f8ff598cd1149831

```md
schema: spec-driven
created: 2026-10-31
skip_specs: true

```
