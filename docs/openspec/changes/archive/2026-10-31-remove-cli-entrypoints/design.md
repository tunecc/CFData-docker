---
comet_change: remove-cli-entrypoints
role: technical-design
canonical_spec: openspec
archived-with: 2026-10-31-remove-cli-entrypoints
status: final
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
