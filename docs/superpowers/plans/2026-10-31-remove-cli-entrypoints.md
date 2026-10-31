---
change: remove-cli-entrypoints
design-doc: docs/openspec/changes/remove-cli-entrypoints/design.md
base-ref: 6949ae364a3c0251dcaf5ddce78be8d4710e1476
archived-with: 2026-10-31-remove-cli-entrypoints
---

<!-- comet-task-authority: docs/openspec/changes/remove-cli-entrypoints/tasks.md -->

# Plan: 删除 CLI 入口与交互代码

实现策略：以调用图为准的删除。任务 1.1 的构建错误清单即为后续删除的核对清单；每个删除函数先 grep 调用方确认无 scheduler/server/tasks 引用。删除类任务无 RED/GREEN（无新增行为），验证形式为构建/测试/冒烟（已在 tasks.md 各项注明）。

## 1. 剥离 CLI 专属代码

<!-- comet-task-ref:a34cecf4-b4fb-47a8-b63f-02f84d84caa6 -->
- **1.1** 删除 `menu.go`、`schedule_menu.go` 整文件。
  验收：`go build ./...` 的错误清单记录并全部归因于 CLI 专属符号。

<!-- comet-task-ref:9fe02ef8-225b-4987-b9d7-85a2d53a57ea -->
- **1.2** 删除 `main.go` 的 `-cli`/`-qs` 分支、`rewriteBoolFlagArgs`、`registerCLIFlags` 调用。
  验收：构建错误清单全部归因于 CLI 专属函数。

<!-- comet-task-ref:dd4a8547-1c0c-4781-bda7-2fa1442c5c4f -->
- **1.3** 删除 `cli.go` 中 CLI 专属函数（`registerCLIFlags`、`runCLI`、`runCLIQuickGitHubUpload`、`handleCLIMessage`、`runCLIQuickStart` 及其私有辅助），逐函数 grep 确认无 scheduler/server/tasks 引用。
  验收：`go build ./...`、`go vet .` 通过。

<!-- comet-task-ref:aaba315c-1ea7-48c9-a801-19f96157884a -->
- **1.4** 清扫孤儿辅助函数与 CLI 专属测试；共享函数测试保留。
  验收：CLI 专属符号 grep 无命中；`go test ./...` 全绿。

## 2. 文档同步与验证

<!-- comet-task-ref:dc5db5f0-871e-45f0-a7a4-bba1903b74f4 -->
- **2.1** README/docs 删除或改写 CLI 用法；核对 docker-compose.yml 注释。
  验收：`grep -rn -- "-cli" README.md docs/ docker/` 无 CLI 用法命中。

<!-- comet-task-ref:123e1c77-e819-4291-8720-2c35615030cd -->
- **2.2** 全量构建测试 + AGENTS.md 测试版产出。
  验收：命令退出码 0，产物更新。

<!-- comet-task-ref:365010b4-60be-4c5a-be18-407639a68be6 -->
- **2.3** 运行冒烟：Web 登录→首页→WS 正常；`-cli` 得到明确报错。
  验收：curl 首页 200；CLI 参数报错输出明确。

## 回退

单 commit，`git revert` 即可。
