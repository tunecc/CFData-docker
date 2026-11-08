# 目标

为 CFData-WEB 的管理面代理新增「连通性测试」能力：用户在 Web 设置页保存/填写代理后，可以一键测试代理是否可用，并看到出口 IP、出口地区与延迟；同时核实现有代理设置入口在当前 main 分支的保存链路完好（作为回归验收保留）。

背景与调查结论（2026-11-08，main 分支）：

- 用户痛点「代理必须在环境变量设置」的根因是版本发布链路：代理设置入口于 2026-10-21（`91bd7a7`）加入，但 GHCR 镜像此前仅打 `v*` tag 时构建，最近发布停留在 5 月；用户使用的 latest 镜像不含该入口。2026-11-08 起 CI 已改为每次推 main 自动构建，镜像更新后入口即可用。
- 当前 main 分支保存链路已核实完好：设置页「管理面代理」输入框 →「保存」（`saveSettingsPage`）→ WS `set_management_proxy` → 写配置文件（容器内 `/data/cfdata-config.json`，挂载卷持久化）→ 回 `proxy_updated` 更新状态 chip、清空输入框并提示。
- 程序配置优先于系统环境变量（`managementProxyInUse` 回退链）；只有 `managementHTTPClient` 走程序配置代理，管理面操作（ASN 库/locations 下载、版本检查、GitHub/R2/订阅上传）经它发请求。
- 已有 `check_proxy_country`（任务启动前直连出口地区警告，走 `upstreamHTTPClient`）与本功能的「测试管理面代理」语义互补，互不替代。
- 参考 subs-check-pro 的代理可用性检测体验（`utils/proxy.go`），仅作实现参考，非需求来源。

# 范围

## 目录结构

### 修改

- combined_refactor/proxy.go — 新增管理面代理连通性测试函数：按给定代理地址（或当前生效代理/直连）构建一次性 HTTP 客户端，请求 Cloudflare trace 端点，汇总可用性、出口 IP、出口地区、延迟；认证信息用现有 `maskProxyURL` 脱敏。
- combined_refactor/server.go — 新增 WS handler `test_management_proxy`（接收可选 `value`），返回 `proxy_test_result`。
- combined_refactor/web/app.js — 设置页代理区块新增「测试」按钮、结果展示（成功/失败样式）、测试期间防重复点击；处理 `proxy_test_result` 消息。
- combined_refactor/index.html — 设置页代理区块增加测试按钮与结果行元素。

### 明确不建

- 不新建独立代理测试页/弹窗 — 测试结果就地显示在设置页代理区块内，符合现有 cfg-field 设计系统。
- 不改动 `check_proxy_country` 与扫描测速数据面的直连语义 — 与本功能互补，保持现状。

## Source coverage

subs-check-pro（`/Users/tune/Downloads/subs-check-pro`）为实现参考材料（借鉴其代理可用性检测思路），不作为需求来源，不触发源文档完整覆盖模式。

# 非目标

- 扫描与测速数据面保持直连，不受管理面代理影响（现状语义不变）。
- 不做自动探测本机常见代理端口（subs-check-pro 的 `findAvailableSysProxy` 模式；Docker 容器网络下价值有限，用户未选择）。
- 不引入订阅节点检测等 subs-check-pro 核心业务能力。
- 不修改代理保存/持久化链路的现有行为。

# 验收示例

- A1: 已配置程序代理（如 socks5://127.0.0.1:1080）且代理可用时，设置页点击「测试」约 10 秒内显示测试成功，包含出口 IP、出口地区与延迟（毫秒）。
- A2: 测试失败（代理不可达、超时、认证失败）时，显示测试失败与具体原因，且不改变已保存的代理配置与全局生效代理。
- A3: 输入框已填写但尚未保存的代理地址时，「测试」针对输入框当前值进行（不落盘、不改变全局生效代理）；输入框为空时针对当前生效出口测试（程序配置 → 系统环境变量 → 直连）。
- A4: 未配置程序代理、但存在系统环境变量代理时，测试经环境变量代理进行，结果标注来源为「系统环境变量」。
- A5: 无任何代理配置时，测试走直连，结果标注为直连出口。
- A6: 测试请求与结果不泄露代理认证信息：UI、日志与错误信息中只出现 `scheme://host` 形式（复用 `maskProxyURL`），不含用户名/密码。
- A7: （回归）保存代理后，状态 chip 更新为「已启用: scheme://host」、输入框清空并出现保存成功提示——现有行为保持不变。

# Constraints and invariants

- 代理地址支持与现有解析一致：socks5://、socks5h://、http://、https://（复用 `parseManagementProxy`）。
- 测试为一次性拨号，不得修改全局 `managementProxyURL` / `managementProxyRaw` 状态或配置文件。
- 测试超时不超过 10 秒，测试按钮在等待期间防重复触发。
- 环境变量回退链与 `managementProxyInUse` 保持一致（HTTPS_PROXY/https_proxy/HTTP_PROXY/http_proxy/ALL_PROXY/all_proxy）。

# Decisions

- D1（用户确认，2026-11-08）：「检测的功能」= 管理面代理连通性测试（出口 IP/地区/延迟），不做端口自动探测。
- D2（调查确认）：用户痛点根因是旧 GHCR 镜像未包含 2026-10-21 加入的设置入口（镜像仅 tag 触发构建）；保存链路在 main 分支无缺陷，本次只交付测试功能 + 回归验收 A7，交付说明中提示更新镜像。
- D3（Agent 决定）：检测端点用 Cloudflare trace（`https://www.cloudflare.com/cdn-cgi/trace`），一次请求同时获得出口 IP（`ip=`）与地区（`loc=`），延迟按请求耗时计算；与现有 `check_proxy_country` 的端点选择一致。
- D4（Agent 决定）：WS 消息命名 `test_management_proxy` → `proxy_test_result`，载荷 `{ok, error, exitIP, country, latencyMs, proxyLabel, source}`。
