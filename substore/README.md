# Cloudflare 目标列表 operator

`cloudflare-targets.js` 是一个 Sub-Store operator 脚本：把本项目的扫描结果（Sub-Store 目标列表）、静态目标和 WeTest 优选表格转换为可用的 Cloudflare 节点，追加到已有的代理列表。

## 快速开始

1. 在 Sub-Store 中新建或编辑一个 operator，粘贴 `cloudflare-targets.js` 的全部内容。
2. 按需修改脚本顶部的 `CONFIG`。
3. 在订阅的 operator 链中启用该脚本，节点名称会以 `原节点名-别名` 的形式生成。

脚本会为每个原始代理生成每个目标对应的节点；`keepOriginal: true` 时额外保留原节点。

## 配置项

| 键 | 说明 |
| --- | --- |
| `keepOriginal` | 是否保留原始代理节点，默认 `false` |
| `targets` | 静态目标列表，每项一行 `host[:port]#alias`；JS 中用 `//` 注释，以 `#` 开头的字符串条目会被忽略 |
| `urls` | 远程目标列表或 WeTest 页面地址，并发获取；单个地址失败不影响其他来源 |
| `urlTimeout` | `fetch` 回退路径的请求超时（毫秒），默认 `5000` |
| `wetestOperators` | 从 WeTest 表格中保留的运营商关键字，默认 `['电信', '联通', '移动']` |

示例：

```js
const CONFIG = {
  keepOriginal: false,
  targets: [
    '172.67.66.139#139',
    '162.159.149.63#63',
  ],
  urls: [
    'https://cf.090227.xyz/cu',
    'https://www.wetest.vip/page/cloudflare/address_v4.html',
  ],
  urlTimeout: 5000,
  wetestOperators: ['电信', '联通', '移动'],
};
```

## 接入本项目 R2 输出

先在 CFData-WEB 中启用 R2 上传（Web 设置菜单「自动调度 / R2 上传」或配置文件 `r2enabled` 等键）。调度或手动导出完成后，R2 上的文件即为目标列表：

```text
1.2.3.4:443#LAX-洛杉矶
[2606:4700::1]:443#HKG-香港
```

确认 Worker 对该文件名提供公开 GET 后，把地址加入 `urls`：

```js
urls: [
  'https://<你的-R2-公网域名>/cfdata-targets.txt',
],
```

`cfdata-targets.txt` 需与配置中的 `r2filename` 一致。该文件使用纯文本格式，无需登录，脚本会按行解析。

## 输入格式

- 纯文本：每行 `host[:port]#alias`；`#` 后为别名，别名中的 `#` 会被清洗。
- IPv6：推荐 `[v6]:port#alias`；也接受不带方括号且不带端口的 `v6#alias`。
- WeTest 表格：页面包含 `data-label="线路名称"` / `"优选地址"` / `"数据中心"` 时自动识别，别名形如 `CF优选-电信-LAX`。
- 无效行（缺少 `#`、端口非法、别名为空）会被跳过，并在日志中输出 `[targets] skip ...`，不影响其余目标。

## 行为说明

- 相同 `host + port + alias` 的目标会去重；别名重复时追加 `⁰`、`¹` 等后缀，避免节点重名。
- 某个 URL 超时或返回非 2xx 时记录 `[targets] fetch failed`，继续使用其他 URL 与静态目标。
- 运行环境既没有 `$substore.http.get` 也没有 `fetch`、且配置了 `urls` 时，脚本会明确报错，不会静默返回空结果。
- `$substore.http.get` 路径使用 Sub-Store 自身的 `timeout` 参数；`fetch` 回退路径使用 `AbortController` + `urlTimeout`。

## 本地验证

脚本底部仅在存在 `module` 时导出纯函数，Sub-Store 运行环境不受影响。可用 Node 运行仓库内测试：

```bash
node substore/cloudflare-targets.test.js
```

测试覆盖无效行跳过、别名清洗、IPv6、WeTest 表格、去重与上标、`fetch` 超时、缺失 HTTP 客户端报错以及节点数量与名称。

## 常见问题

- **没有任何新节点**：确认 `targets` 或 `urls` 至少一项非空，且 operator 已加入订阅链；查看 Sub-Store 日志中的 `[targets]` 警告。
- **节点名重复**：脚本会自动追加上标；若仍重复，检查 `alias` 是否包含特殊字符，可先清洗别名。
- **R2 地址返回 404**：确认 `r2filename` 与 `urls` 中的文件名一致，且 Worker 已配置公开读取。
- **拉取超时**：调大 `urlTimeout`，或改用 Sub-Store 的 `$substore.http.get` 环境（默认优先使用）。
