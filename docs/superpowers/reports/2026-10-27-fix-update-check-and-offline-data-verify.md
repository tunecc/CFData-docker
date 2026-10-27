# 验证报告：fix-update-check-and-offline-data

- 日期：2026-10-27
- 阶段：verify
- 验证模式：light（规模评估建议 full，但仅任务拆分数量超过阈值；无 delta spec，改动集中在更新检测与数据兜底，最终按实际风险选择 light）
- 代码审查：`review_mode: off`，跳过自动代码审查；理由为局部 hotfix，已通过单元测试、构建与两种运行场景验证

## 7 项轻量验证

| # | 检查项 | 结果 | 证据 |
| --- | --- | --- | --- |
| 1 | tasks.md 全部完成 | PASS | 8/8 已完成 |
| 2 | 改动文件与任务一致 | PASS | `version.go`、`main.go`、`server.go`、`storage.go`、`fixes_test.go`、`fallback/` 三份快照 |
| 3 | 编译通过 | PASS | `go vet ./...`、`go build -a -o /tmp/cfdata-hotfix-build .`、Build Guard 执行的 `go -C combined_refactor test ./...` 退出码 0 |
| 4 | 相关测试通过 | PASS | `go test -count=1 ./...` → `ok cfdata` |
| 5 | 无明显安全问题 | PASS | 无硬编码密钥；内嵌快照为静态数据；版本请求已带超时；未引入 unsafe |
| 6 | 集成代码审查 | PASS（按配置跳过） | `review_mode: off`，跳过原因见上 |
| 7 | 核心/失败/边界场景 | PASS | 见下方场景验证 |

## 场景验证

### RED（修复前，已复现）

- 构建 `appVersion=edge-c43b159` 启动 Web：输出 `发现新版本 v1.8.5，下载: https://github.com/PoemMisty/CFData-WEB/releases/latest`。
- 空目录 + `-dns 127.0.0.1`：locations 下载失败导致 `locationMap` 为空；官方模式在 `ips-v4` 下载失败后报 `官方模式未发现有效 IP`。
- 更新检测失败时 Web 与 CLI 均输出“更新检测失败”。

### GREEN（修复后）

- `edge-c43b159` 版本启动 Web：不再出现任何“发现新版本”提示；`versionIsOlder` 对 `edge`/`edge-*`/`dev` 返回 false。
- 空目录 + 不可用 DNS：日志仅出现一条 `已使用内置位置数据（离线快照）`，加载 301 个数据中心；官方扫描出现一条 `已使用内置 IP 列表（离线快照）`，随后 `开始扫描：6534 条记录` 并正常产出扫描结果。
- 更新检测失败时：CLI 与控制台静默（仅写调试日志），Web `version_info` 不再携带 `error` 字段。
- 本地缓存优先：预置 `ips-v4.txt`（1 条记录）与 `locations.json` 后，日志显示 `读取本地 locations.json 文件...` 与 `开始扫描：1 条记录`，未触发内置兜底。
- 单元测试：版本比较边界（`edge-*`/`dev`/相等/更高版本）、稳定标签选择（排除预发布与非 `v*`）、三份内嵌数据可解析且非空。

## 实现说明与偏差

- 三份快照实际文件名为 `fallback/locations.snapshot.json`、`ips-v4.snapshot.txt`、`ips-v6.snapshot.txt`：`.gitignore` 会忽略同名运行时缓存文件，改用 `.snapshot` 后缀确保快照进入版本库；`fallbackData` 仍按逻辑文件名 `locations.json`/`ips-v4.txt`/`ips-v6.txt` 映射。
- 更新检测请求本仓库 `tunecc/CFData-docker`：优先 `releases/latest`，404 时回退 `tags?per_page=100` 并选择最高稳定 `v*` 标签；当前仓库尚无 Release/Tag，检测保持静默。

## 风险与后续

- 内置快照会随 Cloudflare 数据变化而过期，仅作为远程不可达时的兜底；联网时仍优先本地缓存/远程数据。
- 推送首个 `v*` 标签后，更新检测才会产生“发现新版本”提示。
- 二进制体积增加约 146KB。

## 结论

PASS。核心成功场景、失败场景与边界场景均已验证；无 CRITICAL/IMPORTANT 问题。
