# 离线资源回退加固

## 问题描述

CFData-WEB 在国内无代理环境运行时，多个远端资源不可达或响应缓慢。当前只有 `locations.json`、`ips-v4.txt`、`ips-v6.txt` 有内嵌离线快照，ASN 数据库 `GeoLite2-ASN.mmdb` 仍完全依赖远端下载；其默认镜像 `https://jsd.onmicrosoft.cn/gh/seketiti/GeoLiet2@release/GeoLite2-ASN.mmdb` 当前返回 HTTP 522。用户实际遇到的错误为：

- `Get "https://www.baipiao.eu.org/cloudflare/ips-v4": context deadline exceeded`
- `Get "https://www.baipiao.eu.org/cloudflare/ips-v4": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)`

现有回退实现还存在以下缺陷：

1. 本地缓存只判断“文件存在”，不校验内容。缓存为空、截断或格式错误时，`locations` 解析失败后不会改用内嵌快照；IP 列表则会把损坏内容直接交给扫描任务。
2. 远端响应在写盘前没有校验，可能把错误页或空内容缓存下来。
3. 回退内容不写入本地缓存，每次启动/扫描都会重新等待 20 秒超时，用户感知为“卡住后报错”。
4. 回退提示只写终端，Web 端看不到数据来源。
5. ASN 加载失败时静默返回空值，ASN 列显示 `-`，且没有可用的离线兜底。

## 根因

- ASN 加载路径只有“本地文件 → 单一远端 URL”，没有内嵌快照，也没有多镜像。
- 数据缓存读取路径缺少统一的“可解析、非空、地址族匹配”校验，回退只在网络请求返回错误时触发。
- 回退分支直接返回内存快照，没有复用缓存写入逻辑。

## 修复目标

- 将 ASN 数据库压缩后内嵌到二进制，作为永久可用的离线兜底。
- ASN 采用“本地缓存优先 → 内置快照 → 后台更新”的离线优先策略，并支持多个镜像。
- 对 `locations`、`ips-v4`、`ips-v6` 的本地缓存、远端响应和内置快照统一校验；无效内容自动回退。
- 回退数据写入本地缓存，避免每次启动/扫描重复等待超时。
- 将数据抓取超时缩短到 8 秒，降低离线环境的等待感。
- 增加回归测试，覆盖内嵌 ASN 可解析、IP 列表校验、损坏缓存回退。

## 非目标

- 不内嵌需要实时性的资源：GitHub 版本检测、Cloudflare `cdn-cgi/trace`、ISP 探测、测速端点、用户订阅 URL。
- 不改变 Web 前端资源（已完全本地化，无 CDN 依赖）。
- 不修改 `.github/` 下除 `docker.yml` 之外的文件（本次无需修改）。
