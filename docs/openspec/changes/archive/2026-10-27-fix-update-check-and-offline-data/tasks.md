# Tasks

## 1. 内嵌数据快照与兜底读取

- [x] 新增 `combined_refactor/fallback/locations.json`、`ips-v4.txt`、`ips-v6.txt` 快照并校验可解析。 <!-- comet-task:2b7207e9-4aa6-4e0b-a84b-da5095cf4c98 -->
- [x] 在 `storage.go` 使用 `//go:embed` 暴露快照；`initLocations` 与 `getIPListContent` 在远程失败时使用快照，不写盘。 <!-- comet-task:514364ce-c020-4018-868a-5b28c5afeed7 -->

## 2. 更新检测指向本仓库

- [x] `version.go` 更新仓库常量，新增稳定标签筛选/选择函数。 <!-- comet-task:f5592837-e765-4975-8af4-3b7f5d10f6cb -->
- [x] `main.go` 改用本仓库 API；`releases/latest` 404 时回退 `tags`；`edge-*`/`dev` 跳过比较；失败不打印。 <!-- comet-task:b4d0e435-d7b6-41ef-840c-7e9811b3e7dd -->
- [x] `server.go` 不再向 Web 下发更新检测 error；更新链接使用本仓库。 <!-- comet-task:0eb25b16-155d-40c2-aea1-74d87de7d62c -->

## 3. 回归测试

- [x] 新增 `combined_refactor/fixes_test.go`：版本比较边界、稳定标签选择、内嵌数据解析。 <!-- comet-task:8e5cb7fd-0664-40e7-9dcf-e9c3fcda6875 -->

## 4. 构建与手工验证

- [x] 构建 Web 二进制并复现 RED（旧提示/下载失败）。 <!-- comet-task:07c58f7f-a6d6-4433-bc9b-09e0e3cc2143 -->
- [x] 修复后用同一环境验证 GREEN（无旧提示/使用内置数据）。 <!-- comet-task:b266193c-bfa7-4688-8014-14ea576450a9 -->
