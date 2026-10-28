# CFData-Web

CFData-Web 是一个基于 Go 的 Cloudflare IP 测试与筛选工具，提供本地 Web 与 CLI 两种使用方式，支持官方 IP 段扫描、非标目标测试、测速、结果筛选、导出和 GitHub 上传。

[在线演示站](https://cfdata-demo.cce.de5.net/) 仅使用浏览器内虚拟数据，用于预览界面与交互；真实使用请下载正式版本。

![image](img/demo.png)

### 更新说明

项目功能已趋于完善，基本达到了作者预期的效果。后续更新将以维护和新功能为主，版本迭代频率会有所降低。如果你在使用过程中有新的需求或建议，欢迎提交 Issue，作者会在评估后酌情纳入后续版本。

近期主要更新：

- 非标链路复验：扫描完成后对候选做真实 TCP+TLS+HTTP 探针验证（Web 支持手动复验按钮），不可用候选自动剔除
- 测速地址可用性预检：启动时并行探测全部内置与自定义测速地址，自动选择时排除不可用项，全部不可用回退 CF 官方地址；Web 下拉框实时展示可用性
- 结果区「筛选排序」合一：原独立筛选下拉与一键排序合并进单个弹窗，新增「延迟低于 xx ms」「丢包低于 xx%」筛选，并与复制/导出内容联动
- 管理面代理：新增 -proxy 参数与配置文件 proxy 键（支持 socks5://、socks5h://、http://、https://），覆盖数据下载、版本检查、GitHub/订阅上传；Web 设置菜单可直接配置，未配置时自动使用系统环境变量代理；扫描与测速保持直连
- CLI 交互菜单：`-cli` 进入菜单（按配置启动 / 自定义参数启动 / 修改配置参数 / 自动调度 / 帮助），`-cli qs` 按已保存配置快速启动，`-cli -参数…` 直接执行；启动时输出 Banner 与「调试等级」
- 参数设置向导：分模块逐项设置，提示格式与可选值，可保存为配置文件
- 自动调度：进程内 Cron/Ticker 调度，支持 Cron 表达式与检测间隔两种计划，Web 设置与 CLI 菜单可直接开关，无需 systemd
- 官方/非标的扫描并发、延迟阈值、测速网址在命令行与配置文件中完全独立，互不影响
- 配置文件自动迁移：键名无变化时静默按新模板迁移；存在旧键名时交互提示（仅迁移同名值），原文件备份为 `.bak`，非交互环境自动跳过
- 启动输出统一为简洁的「当前配置」单块，版本行下方显示调试等级

## 功能

- 官方优选：扫描 Cloudflare IPv4/IPv6，按数据中心继续详细延迟测试。
- 非标优选：上传本地 txt/csv 或填写网络 URL，测试自定义 IP/域名与端口。
- 测速：支持单点测速、批量测速、非标并发测速和测速阈值筛选。
- 链路复验：扫描完成后对候选做真实 TCP+TLS+HTTP 探针验证，自动剔除不可用项；Web 支持手动复验按钮。
- 导出：支持 CSV/TXT/Sub-Store 目标列表、自定义字段、IP 类型筛选、合格结果筛选。
- 上传：支持将导出结果上传到 GitHub 与 R2（Worker 兼容接口）。
- CLI：交互菜单、参数设置向导、自动调度、配置自动迁移。
- APK：支持 Android WebView 壳运行内置后端。

## 快速开始

从 [Releases](https://github.com/PoemMisty/CFData-WEB/releases/latest) 下载对应平台程序后运行。

默认启动 Web 模式：

```text
服务启动于 http://localhost:13335
当前测速网址: auto
```

浏览器打开终端中的地址即可使用。

CLI 模式（进入交互菜单）：

```bash
./cfdata-linux-amd64 -cli
```

菜单提供：按配置启动（菜单 1，等价 `-cli qs`）、自定义参数启动（向导）、修改配置参数、自动调度、帮助。

首次使用 CLI 配置文件时会生成模板并退出，编辑配置后重新运行即可；也可通过菜单「修改配置参数」逐项设置。升级后启动时若配置键名有变化会提示迁移：同名值保留，原文件备份为 `.bak`，非交互环境自动跳过。

简单示例：

```bash
# 交互菜单（不带其他参数时默认进入菜单）
./cfdata-linux-amd64 -cli

# 按已保存配置快速启动（qs 大小写不敏感）
./cfdata-linux-amd64 -cli qs

# 官方模式直接执行：扫描 IPv4，测试 443 端口，测速地址自动选择
./cfdata-linux-amd64 -cli -mode official -offiptype 4 -offport 443 -offurl auto

# 非标模式直接执行：读取本地文件，开启 TLS 和 5 个测速线程
./cfdata-linux-amd64 -cli -mode nsb -nsbfile ip.txt -nsbtls=true -nsbspeedtest 5 -nsburl auto
```

## Docker

镜像发布在 GHCR：`ghcr.io/tunecc/cfdata-web`，支持 linux/amd64 与 linux/arm64。

```bash
docker pull ghcr.io/tunecc/cfdata-web:latest
docker run -d --name cfdata -p 13335:13335 -v cfdata-data:/data ghcr.io/tunecc/cfdata-web:latest
```

浏览器打开 `http://localhost:13335` 即可使用。配置、缓存与日志保存在 `/data` 卷中，容器重建后不会丢失。认证参数、Compose、健康检查、升级与标签策略见 [docs/docker.md](docs/docker.md)。

## Web 使用

界面顶部「扫描方式」选择器支持 TCPing（默认）和 HTTPing。不同扫描模式的延迟数据不可互相比较，仅同模式内的对比才有意义。

### 扫描方式说明

- **TCPing**：测量 TCP 握手延迟，基准值。
- **HTTPing**：测量 HTTP TTFB（Time To First Byte），延迟比 TCPing 高属正常现象。延迟阈值和渲染颜色已按倍率自动缩放，倍率仅为延迟等级参考值，非精确换算：
  - 无 TLS（HTTP 端口）：×1.3
  - 有 TLS（HTTPS 端口）：×4.0

### 官方优选

1. 选择 IPv4 或 IPv6。
2. 设置测试端口、扫描并发、延迟阈值。
3. 点击“开始扫描与测试”。
4. 扫描完成后选择数据中心继续详细测试。
5. 在详细测试结果中可单点测速或批量测速。

### 非标优选

1. 切换到“非标优选”。
2. 上传 txt/csv，或填写网络 URL（二选一）。
3. 设置备用端口、并发、TLS、结果上限、测速线程、测速阈值等参数。
4. 点击“开始扫描与测试”。
5. 在结果表格查看、筛选、导出或上传。

非标输入推荐格式：

```text
1.2.3.4 443
5.6.7.8 8443
2606:4700::1111 443
1.1.1.1
```

未提供端口时会使用备用端口；备用端口默认随 TLS 模式自动选择，关闭 TLS 为 80，开启 TLS 为 443。

### Sub-Store 目标列表

导出弹窗的「文件格式」和一键复制弹窗提供「Sub-Store 目标列表」，CLI 使用 `-outformat substore`。输出为每行 `host[:port]#alias` 的纯文本，文件扩展名固定为 `.txt`：

```text
1.2.3.4:443#LAX-洛杉矶
[2606:4700::1]:443#HKG-香港
```

- IPv6 始终输出方括号，不受 `-outv6bracket` 影响；端口为空或为 0 时省略 `:port`。
- 别名默认为「数据中心-城市」，缺失时依次回退到数据中心、城市、`loc`、IP，并清洗 `#`、换行与多余空白。
- 输出遵循开始行/结束行、IP 类型与合格结果筛选，超出上限的行不会导出或上传。

配合 Sub-Store operator 脚本 [`substore/cloudflare-targets.js`](substore/cloudflare-targets.js) 使用，把导出的文件地址加入脚本的 `urls`，例如 R2 的公网地址：

```js
const CONFIG = {
  // ...
  urls: [
    'https://<你的-R2-公网域名>/cfdata-targets.txt',
  ],
  urlTimeout: 5000,
};
```

脚本同时支持 Untitled.js 原有的 `host[:port]#alias` 文本与 WeTest 优选地址表格（`data-label="线路名称"` 页面）。脚本用法与常见问题见 [substore/README.md](substore/README.md)。

### R2 上传（Worker 兼容接口）

在配置文件 `config` 中启用 R2 上传：

```json
{
  "config": {
    "r2enabled": true,
    "r2workerurl": "https://r2.example.com",
    "r2workertoken": "your-token",
    "r2filename": "cfdata-targets.txt"
  }
}
```

Web 设置菜单的「自动调度 / R2 上传」面板可直接编辑并保存，保存后立即热更新。保存后 token 不再回显到浏览器：输入框显示「已保存，留空保持不变」，留空保存不会覆盖已存储的 token。调度或手动导出完成后，上传内容始终为 Sub-Store 目标列表，与本地导出格式无关；未产生结果时跳过上传。

Worker 协议：

```text
POST {r2workerurl}/storage?token={r2workertoken}
Content-Type: application/json

{"filename": "cfdata-targets.txt", "value": "1.2.3.4:443#LAX-洛杉矶\n"}
```

返回 2xx 视为成功。上传遵循管理面代理配置，单次请求 30 秒超时，失败最多尝试 3 次（间隔 2 秒）；HTTP 401/403 视为 token 配置错误，不重试。上传失败只记录日志，不会覆盖 R2 上的旧文件，也不影响本地导出与后续调度。

故障排查：

- 日志提示 `worker-url 未配置` 或 `worker-token 未配置`：启用 R2 前需同时填写两项。
- 日志提示 `R2 鉴权失败（HTTP 401/403）`：检查 token 是否与 Worker 一致，注意不要带入多余空格或换行。
- 连续失败：检查 Worker 地址、管理面代理、R2 绑定与网络连通性；日志中的地址已去除 query，不会输出 token。
- Sub-Store 拉取不到内容：确认 `r2filename` 与 `urls` 中的地址一致，并确认 Worker 对该文件名提供公开 GET。

## 自动调度

调度由程序进程内完成，不依赖 systemd、root 权限或 Linux 专有机制；Web 模式启动后自动加载，CLI 菜单「自动调度 → 启动常驻调度」也可运行。调度参数来自配置文件 `config`：

```json
{
  "config": {
    "schedule-enabled": true,
    "cron-expression": "0 4 * * *",
    "check-interval": 720
  }
}
```

- `schedule-enabled`：总开关，默认 `false`。
- `cron-expression`：5 段标准 Cron（分 时 日 月 周），非空且有效时优先于检测间隔，例如 `0 4 * * *` 表示每天 04:00。
- `check-interval`：检测间隔（分钟）。`<=0` 按 2880 分钟、小于 60 按 60 分钟处理；仅在 Cron 为空或无效时生效。

计划行为：

- Cron 模式：进程启动后不立即扫描，按表达式等待下一次触发。
- 间隔模式：进程启动后立即执行一次，之后按间隔重复；`Reload` 只重建计划，不补发立即执行。
- 每次触发前检查是否有任务运行中，忙碌时跳过本次并记录日志。
- 调度扫描完成后固定按 Sub-Store 目标列表导出（本地文件为 `.txt`），启用 R2 时同步上传；空结果跳过上传，上传失败只记录日志，不影响后续调度。
- 修改配置后 Web 设置面板会立即热更新；直接编辑配置文件时，后台每 10 秒检测变更并重载。

Web 设置菜单的「自动调度 / R2 上传」面板可开关调度、编辑 Cron/间隔与 R2 参数，保存即生效。

### 从 systemd 迁移

旧版本的定时任务通过 `cfdata-schedule.timer` 触发外部 CLI。升级后请停用并清理旧 unit，避免与进程内调度重复执行：

```bash
sudo systemctl disable --now cfdata-schedule.timer
sudo systemctl daemon-reload
sudo rm -f /etc/systemd/system/cfdata-schedule.timer /etc/systemd/system/cfdata-schedule.service
```

程序目录中的 `cfdata-schedule.service`、`cfdata-schedule.timer` 与 `cfdata-schedule.log` 不再使用，可按需删除。随后在 Web 设置面板或 CLI 菜单「自动调度」中启用调度并设置 Cron/间隔。同一个配置文件只应由一个进程加载调度，避免多实例重复扫描。

## 测速地址

默认测速地址为 `auto`，表示由后端自动选择内置测速源。

启动时会对全部内置测速地址和已输入的自定义测速地址做可用性测试（小体积探测），自动选择时跳过不可用地址；若全部不可用则回退 CF 官方地址。Web 端会把测试结果同步到测速地址下拉框，不可用项会标注「不可用」。

Web 下拉项：

- 自动选择
- Cloudflare
- CM提供
- 移动专属
- 手动输入

CLI 可通过 `-offurl`/`-nsburl` 指定：

```bash
./cfdata-linux-amd64 -cli -offurl auto
./cfdata-linux-amd64 -cli -offurl speed.cloudflare.com/__down?bytes=99999999
./cfdata-linux-amd64 -cli -offurl https://example.com/file.bin
```

说明：测速只读取响应字节流计算速度，不会把测速文件保存到本地。

## 常用参数

```text
-cli              启用 CLI 模式
-mode             official 或 nsb
-scanmode         扫描方式：tcping（默认，TCP 握手延迟）或 httping（HTTP TTFB，延迟比 tcping 高属正常，不同模式数据不可对比）
-offthreads       官方扫描并发数
-nsbthreads       非标扫描并发数
-offport          官方测试/测速端口
-offdelay         官方延迟阈值，单位毫秒
-nsbdelay         非标延迟阈值，单位毫秒；0=不筛延迟
-offurl           官方测速下载地址，默认 auto
-nsburl           非标测速下载地址，默认 auto
-dns              自定义 DNS 服务器
-debug            调试日志等级：false、error、all
-skipgeo          跳过地区/代理环境验证（启动时不再提示代理警告）
-out              输出文件名（官方/非标通用），默认 cfdata-results
```

非标常用参数：

```text
-nsbfile          本地输入文件
-nsbsourceurl     网络输入 URL
-nsbfallbackport  非标输入缺省端口；不传时随 TLS 自动使用 443/80
-nsbtls           非标是否启用 TLS
-nsbspeedtest     非标测速线程数，0 表示不测速。多 IP 并发影响实际速度，需要准确应设为 1
-nsbresultlimit   非标延迟测试结果上限
-nsbspeedmin      非标测速合格阈值，单位 MB/s
-nsbspeedlimit    非标测速合格结果上限
-relink           链路复验开关（默认开启）；扫描完成时对结果做真实链路验证，不可用的候选将被剔除
-relinkhost       链路复验探针域名；默认使用内置公共探针，配合 -relink 开关使用
-proxy            管理面代理地址（支持 socks5://、socks5h://、http://、https://）；下载 ASN 库/locations、版本检查、GitHub/订阅上传走此代理，扫描测速不受影响；留空走系统环境变量代理
```

完整参数可运行：

```bash
./cfdata-linux-amd64 -h
```

## 本地缓存

Web 右上角设置菜单提供“恢复全部默认配置”，会清理本地缓存文件，例如 `ips-v4.txt`、`ips-v6.txt`、`locations.json`、ASN 数据库等。任务运行中不会直接清理，避免影响测试。

## 免责声明

本程序仅限用于学习与研究目的。请在下载后24小时内自行删除。使用本程序时，应自行遵守所在地区的法律法规。作者不对使用本程序所产生的任何后果承担责任。下载或使用本程序即视为已阅读、理解并同意上述声明。

## 致谢

- TG 频道：[CF中转IP](https://t.me/CF_NAT)
- GitHub：[Kwisma/iptest](https://github.com/Kwisma/iptest)

## License

Copyright (C) 2026 PoemMisty

This project is licensed under the GNU General Public License v3.0 or later.
See the LICENSE file for details.
