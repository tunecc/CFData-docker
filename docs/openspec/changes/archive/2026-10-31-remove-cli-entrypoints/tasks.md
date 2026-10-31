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
