# 设计

## ASN 离线优先加载

- 新增 `combined_refactor/fallback/geolite2-asn.snapshot.mmdb.gz`（约 5.1 MB），用 `//go:embed` 编入二进制；运行时用 `compress/gzip` 解压后交给 `maxminddb.FromBytes`，不落临时文件。
- `getASNReader` 加载顺序：
  1. 本地 `GeoLite2-ASN.mmdb` 存在且通过 `maxminddb.Open` + 元数据校验 → 直接使用；若数据库构建时间超过 30 天，后台尝试更新缓存。
  2. 本地缺失或损坏 → 解压内嵌快照并校验后立即返回，不阻塞扫描。
  3. 后台刷新：依次尝试 jsDelivr、fastly、gcore、raw GitHub、jsd 镜像；下载后校验数据库类型，原子写入本地缓存，下次启动生效。
- 使用 generation 计数器防止“用户清理缓存后后台又把文件写回”。
- 本地缓存损坏时不删除用户文件；后台刷新成功后原子覆盖。
- ASN 快照来源与许可在 `asn.go` 顶部注明（MaxMind GeoLite2-ASN，CC BY-SA 4.0）。

## locations / IP 列表加固

- 新增统一校验函数：
  - `parseLocationsContent`：JSON 可解析且至少一个有效 IATA。
  - `validateIPListContent`：复用 `parseIPList`，要求至少一个有效 IPv4/IPv6 条目，并匹配文件名对应地址族。
- 读取顺序统一为：本地缓存（校验通过）→ 远端（8 秒超时，校验通过后写盘）→ 内置快照（校验通过后写盘并返回）。
- 本地缓存损坏时直接尝试远端/内置快照，不再反复返回损坏内容。
- 回退内容写入本地缓存，后续启动/扫描直接命中；用户可通过“清理本地缓存”重新在线更新。
- 回退提示保留终端日志；扫描任务不再收到 `error` 消息。

## 关键取舍

- 内嵌 ASN 让二进制和仓库增加约 5.1 MB，换取首次扫描无需等待远端即可显示 ASN。
- 后台刷新只在缓存缺失、损坏或超过 30 天时触发；失败只写调试日志，不打扰用户。
- 回退数据写缓存沿用现有“缓存一次、清理后重新下载”的语义，不引入额外的 TTL 状态文件。

## 测试策略

- `TestEmbeddedASNSnapshot`：内嵌快照可 gunzip、`maxminddb.FromBytes` 成功、数据库类型为 `GeoLite2-ASN`、构建时间非零。
- `TestValidateIPListContent`：合法 v4/v6、空内容、乱码、地址族不匹配。
- `TestParseLocationsContent`：合法、空数组、损坏 JSON。
- `TestGetIPListContentFallsBackOnCorruptCache`：损坏的本地缓存应回退到有效内置列表，而不是原样返回。
- `go test -count=1 ./...`、`go vet ./...`、`go build`。
- 手工验证：把工作目录中的缓存文件移走/写坏，断网启动，确认 locations/IP/ASN 均可用且无 20 秒等待。

## 风险

- 内嵌快照会随发布时间变旧；后台刷新和“清理缓存后重试”提供更新路径。
- 首次 ASN 后台更新可能占用带宽，但不阻塞请求。
- 远端镜像返回的 MMDB 必须通过类型校验才会写盘，避免缓存错误内容。
