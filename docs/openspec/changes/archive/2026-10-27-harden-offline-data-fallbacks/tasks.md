# 任务

- [x] 1. 下载并压缩 ASN 快照到 `combined_refactor/fallback/geolite2-asn.snapshot.mmdb.gz`，校验 sha256 与数据库元数据。 <!-- comet-task:aafa059b-12b4-4dd6-a0ea-bbf37dd28457 -->
- [x] 2. 改造 `asn.go`：内嵌快照、缓存校验、多镜像后台刷新、generation 防护。 <!-- comet-task:e5f838da-dcca-479c-995b-271881b60e57 -->
- [x] 3. 改造 `storage.go`：locations/IP 列表统一校验、回退写缓存、8 秒超时。 <!-- comet-task:bcc02cd9-7642-4e31-bd54-753cf6941de8 -->
- [x] 4. 在 `fixes_test.go` 增加 ASN 与数据校验回归测试，并先运行损坏缓存用例确认 RED。 <!-- comet-task:726a8305-1a89-463a-873d-b9fcdcce8d9a -->
- [x] 5. 运行 gofmt、go vet、go test、go build，并做离线手工验证。 <!-- comet-task:804472ea-3112-4367-b58e-bb3ab75826ea -->
