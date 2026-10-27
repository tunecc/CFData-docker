---
comet_change: fix-update-check-and-offline-data
role: technical-design
canonical_spec: openspec
---

# 设计：更新检测与内置数据兜底

## 上下文

- 项目是长期维护的下游 fork，持续同步上游 `PoemMisty/CFData-WEB`。
- 主要交付形态是 Docker 镜像（GHCR）与直接运行二进制；国内环境通常无代理，GitHub API 与 baipiao 可能不可达。
- 已确认：hotfix 分支隔离；继续 hotfix（仅文件数超限）；更新检测指向本仓库；数据采用内嵌快照兜底。

## 更新检测

涉及文件：`combined_refactor/version.go`、`combined_refactor/main.go`、`combined_refactor/server.go`。

- `version.go` 定义 `updateRepo = "tunecc/CFData-docker"`、`updateRepoAPI` 与 `releaseLatestURL`；新增 `pickLatestVersionTag`/`isStableVersionTag`，只接受纯数字点分的 `v*` 标签，排除 `v1.3.0-beta.1` 等预发布标签。
- `getLatestRelease` 先请求 `releases/latest`；404 时请求 `tags?per_page=100`，用 `versionIsOlder` 选出最高稳定标签；两者都没有版本时返回空结果且不报错。
- `versionIsOlder` 对 `dev`、`edge`、`edge-*` 直接返回 false，避免手动构建永远提示更新。
- 控制台检查失败只写调试日志，不打印“更新检测失败”；Web 端 `version_info` 不再下发 `error` 字段，版本行只显示当前版本。更新可用时链接指向本仓库 Release，标签兜底时指向本仓库 releases 页面。
- 不在本次范围内修改 CI 生成 Release；用户推送 `v*` tag 触发镜像构建后，Tags 检查即可工作。

## 数据兜底

涉及文件：`combined_refactor/storage.go`、`combined_refactor/fallback/locations.json`、`combined_refactor/fallback/ips-v4.txt`、`combined_refactor/fallback/ips-v6.txt`。

- 使用 `//go:embed` 将三份快照编入二进制，约 150KB；不改 Dockerfile，镜像与裸二进制同时生效。
- `initLocations`：本地 `locations.json` 不存在时先远程下载；失败则解析内置快照并输出一条提示。内置数据不落盘，下次启动继续尝试远程。
- `getIPListContent`：本地 `ips-v4.txt`/`ips-v6.txt` 不存在时先远程下载；失败则返回内置快照并输出一条提示。官方扫描与精简 IPv4 共用该入口，无需分别修改。
- 本地缓存存在时行为不变，不覆盖用户缓存；清理缓存后仍会重新下载并使用最新数据。

## 风险与权衡

- 快照会随 Cloudflare 数据变化而过期；仅作为不可达时的兜底，正常网络仍使用远程/本地缓存。
- 仓库当前没有 Release/Tag，更新检测会保持静默；等用户推送第一个 `v*` 标签后生效。
- Go 二进制增大约 150KB；可接受。
- 与上游同步时，`main.go`/`server.go`/`storage.go` 是热点文件，后续合并需保留本设计的下行改动。

## 验证策略

- 新增 `combined_refactor/fixes_test.go`：覆盖 `edge-*`/`dev` 跳过、稳定标签选择、三份内嵌数据可解析。
- RED：用当前代码构建 `appVersion=edge-c43b159`，启动 Web，确认仍提示上游 `v1.8.5`；在空目录用不可用 DNS 启动，确认 `ips-v4` 下载失败导致官方模式报错。
- GREEN：修复后同一命令不再提示更新；空目录 + 不可用 DNS 时使用内置数据继续扫描，日志只出现一条兜底提示。
- 构建：`cd combined_refactor && go test ./... && go build ./...`；再按 AGENTS.md 构建测试版。
- 回归：本地有缓存时仍优先使用缓存；端口/健康检查/镜像行为不变。
