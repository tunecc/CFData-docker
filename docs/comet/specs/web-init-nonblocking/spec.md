# Web 初始化快速呈现

## Purpose

让用户打开或刷新 CFData-WEB 页面时立即看到正确的版本号，并让 WebSocket 初始化配置在连接建立后立即到达，不再被 Cloudflare 地区探测或测速源 ISP 探测阻塞；同时保持现有代理警告、测速源状态、版本更新检查与业务任务行为不变。

## Requirements

### Requirement: 首屏版本号

系统 SHALL 在返回内嵌 `index.html` 时，把版本行 `#versionLine` 的初始占位文本替换为 `appVersionLabel()` 的当前值。版本号 MUST 与启动 banner、CLI 版本输出以及 GHCR `sha-<短哈希>` 镜像标签使用同一来源，并 MUST 在首屏 HTML 中可见，不依赖 WebSocket 连接是否成功。

注入 MUST 只替换版本行的占位内容，不得影响页面其他同名文本或 HTML 结构；注入值 MUST 进行 HTML 转义。页面响应的 `Cache-Control: no-cache` MUST 保持不变。

### Requirement: 初始化配置立即下发

WebSocket 连接建立后，系统 MUST 立即发送 `init_config`。发送前 MUST NOT 执行同步外部网络请求，包括 Cloudflare `cdn-cgi/trace` 地区探测与测速源 ISP 探测。

`init_config` SHALL 继续携带现有字段：`version`、`speedTestURL`、`speedTestDefault`、`debug`、`releaseURL`、`cfCountry`、`proxyWarning`、`geoCheckOK`、`skipGeoCheck`、`speedURLStatus`、`proxyStatus`。其中测速源可用性状态 MUST 使用启动阶段已经生成的本地缓存，代理状态 MUST 使用本地配置，二者均不得触发新的网络探测。

### Requirement: 保留任务前代理检测

页面加载时不再同步执行地区探测 MUST NOT 改变用户开始任务前的代理检测行为。`check_proxy_country` SHALL 继续按现有规则执行：7 秒超时、三次尝试、`shouldWarnProxyCountry` 判定；中国、未知和测试标签放行，其他地区或检测失败 SHALL 按现有方式显示代理警告。页面加载时 MUST NOT 因此新增自动弹出的代理警告。

### Requirement: 测速源状态与版本更新检查

`init_config` 中的测速源默认值与可用性状态 SHALL 保持现有含义和前端应用方式；自动选择逻辑 MUST 保持不变。后台 `version_info` 更新检查 SHALL 继续执行，失败只记录调试日志；成功且存在新版本时 SHALL 继续在版本行追加更新链接。

### Requirement: 一致性与回归

版本 HTML 注入、`init_config.version`、启动 banner、CLI 版本输出 MUST 使用同一个 `appVersionLabel()`。扫描、详细测试、测速、导出、R2/GitHub 上传、自动调度与 NSB 流程 MUST 保持不变。本次变更 MUST NOT 引入新的外部网络依赖，也 MUST NOT 改变 Docker 镜像标签、CI 工作流或 `/data` 持久化方式。

## 验收映射

本规格的验收项定义在 `brief.md` 的「验收示例」中（A1–A5），分别对应首屏版本号、初始化配置立即下发、任务前代理检测保持、测速源与更新检查保持、构建与测试回归。
