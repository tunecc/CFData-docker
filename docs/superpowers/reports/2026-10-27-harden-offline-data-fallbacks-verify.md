# 验证报告：harden-offline-data-fallbacks

- 日期：2026-10-27
- 分支：hotfix/20261027/harden-offline-data-fallbacks
- 基线：14193184f8ed1a5885beb354005f7b36b4d3e314
- 验证模式：light（规模评估建议 full，但 SDK 预设保持 light；本报告额外覆盖设计一致性与手工离线场景）
- 审查模式：off（跳过自动代码审查；本次由实现者完成逐文件复核与运行时验证）

## 摘要

| 维度 | 结果 |
| --- | --- |
| 任务完成度 | 5/5 |
| 规格覆盖 | 无 delta spec；proposal 目标 6 项全部落实 |
| 设计一致性 | 与 design.md 的加载顺序、校验、刷新策略一致 |
| 核心场景 | 通过（损坏缓存回退、内嵌 ASN、离线启动） |

## 检查项

| # | 检查 | 结果 | 证据 |
| --- | --- | --- | --- |
| 1 | tasks.md 全部完成 | PASS | `comet state tasks`：5/5 |
| 2 | 改动文件与任务一致 | PASS | 4 个交付文件：`asn.go`、`storage.go`、`fixes_test.go`、`fallback/geolite2-asn.snapshot.mmdb.gz` |
| 3 | 编译通过 | PASS | `go build -C combined_refactor -trimpath -ldflags="-s -w" -o /tmp/cfdata-build-check .` |
| 4 | 测试通过 | PASS | `go test -C combined_refactor -count=1 ./...`；`go test -race -count=1 ./...` 此前通过；`go vet ./...` 通过；`gofmt -l .` 无输出 |
| 5 | 无安全问题 | PASS | 无硬编码密钥、无新增 unsafe；内嵌数据来源与 CC BY-SA 4.0 许可已在 `asn.go` 注明 |
| 6 | 集成代码审查 | SKIP | `review_mode: off`，已记录原因；实现者完成逐文件复核 |
| 7 | 核心成功/失败/边界场景 | PASS | 见下方场景验证 |

## 场景验证

### 1. 离线启动回退（运行时）

在空目录中以失效代理模拟远端不可达：

```bash
mkdir -p /tmp/cfdata-offline && cd /tmp/cfdata-offline
HTTPS_PROXY=http://127.0.0.1:1 HTTP_PROXY=http://127.0.0.1:1 /tmp/cfdata-hotfix -port 13336 -host 127.0.0.1
```

实际输出：

```text
本地 locations.json 不存在，正在从服务器下载...
已加载 301 个数据中心位置信息
已使用内置位置数据（离线快照）
服务启动于 http://127.0.0.1:13336
```

结果：远端失败后使用内嵌 `locations` 快照，且生成有效的 `locations.json`（31,438 字节）；不再向扫描任务抛出 `context deadline exceeded` / `Client.Timeout`。

### 2. 损坏 IP 缓存回退

`TestGetIPListContentFallsBackOnCorruptCache`：本地 `ips-v4.txt` 写入 `not-an-ip`，远端指向 `127.0.0.1:1`。返回内容可解析且包含有效 CIDR 条目，不再原样返回损坏缓存。RED 证据：改动前该用例失败于 `returned invalid cached content "not-an-ip\n"`。

### 3. 内嵌 ASN 快照

`TestEmbeddedASNSnapshot`：gunzip 后 `maxminddb.FromBytes` 成功，数据库类型为 `GeoLite2-ASN`，构建时间非零，`1.1.1.1` 查询得到非空 ASN/组织。

快照校验：解压后 sha256 = `e7bdda578f8401f182b76e502e9f76ba8a92eb1ab67fa8eb8aba0d42a7e59743`，元数据 `build=2026-10-26T08:15:28Z`。

### 4. 数据校验边界

- `TestValidateIPListContent`：合法 v4、合法 v6、空内容、乱码、v6 混入 v4 文件、v4 混入 v6 文件均按预期通过/拒绝。
- `TestParseLocationsContent`：合法数组通过；空数组与损坏 JSON 被拒绝。

### 5. 后台刷新与缓存清理竞态

代码复核：`refreshGeneration` 在 `resetASNReader` 与每次启动刷新时递增；`writeASNCacheIfCurrent` 持锁比对 generation，用户清理缓存后旧刷新任务不会把文件写回。该路径为简单计数器逻辑，未单独编写并发单元测试。

## 设计与 proposal 一致性

- ASN 加载顺序：本地缓存 → 内嵌快照 → 后台刷新，与 design.md 一致。
- 多镜像顺序：jsDelivr → fastly → gcore → raw GitHub → jsd，与 design.md 一致。
- 本地缓存损坏不删除用户文件，刷新成功后原子覆盖，与 design.md 一致。
- `locations`/IP 列表统一为“本地校验 → 远端 8 秒超时校验 → 内嵌快照校验并写盘”，与 design.md 一致。
- 回退提示仅保留终端日志，扫描任务不再收到 `error`，与非目标（不内嵌实时资源）一致。

## 遗留说明

- ASN 后台刷新成功后不热替换当前 reader，日志明确提示“下次启动生效”；这是为避免并发关闭 reader 的刻意取舍。
- `resetASNReader` 仍要求无任务运行（沿用原有语义），未扩大改动范围。
- 实现已提交为 `a1b13e9`（用户授权），工作区干净；本次复审基于该提交内容。

## 结论

PASS。无 CRITICAL / IMPORTANT 问题，可进入归档确认。
