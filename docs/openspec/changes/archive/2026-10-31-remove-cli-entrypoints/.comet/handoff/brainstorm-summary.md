# Brainstorm Summary

- Change: remove-cli-entrypoints
- Date: 2026-10-31

## 确认的技术方案

与用户本轮通过 AskUserQuestion 确认的 B 方案（删入口保管线）一致，Step 1c 确认提问未获答复，以 B 方案批准为准：

- 删除 menu.go、schedule_menu.go 整文件；main.go 的 -cli/-qs 分支与 rewriteBoolFlagArgs/registerCLIFlags 调用
- 删除 cli.go 中 CLI 专属函数（registerCLIFlags/runCLI/runCLIQuickGitHubUpload/handleCLIMessage/runCLIQuickStart 及私有辅助），边界以调用图为准：被 scheduler.go/server.go/tasks_*.go 引用的一律保留
- 共享部分（cliFileConfig/cliExportConfig 模型、runOfficialPipeline/runNSBPipeline、writeCLIExportAndMaybeUpload、loadScheduledCLIConfig、导出上传）零行为变化
- -cli/-qs 传入由 Go flag 包报 "flag provided but not defined"，不留兼容层
- README/docs CLI 用法同步；不重命名 cli.go；单 commit 可回滚

## 关键取舍与风险

- 误删共享函数 → 逐函数 grep 调用方 + go build/test（含调度管线测试）+ Web 冒烟防线
- 残留孤儿函数 → 二次 grep 清扫，不追求极致

## 测试策略

- go build/vet/test 全绿（含调度管线测试）；Web 启动冒烟（登录/首页/WS）；-cli 报错验证

## Spec Patch

无——skip_specs: true，CLI 从未被规格覆盖。
