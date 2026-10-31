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
