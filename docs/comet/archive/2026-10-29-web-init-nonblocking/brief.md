# 目标

一次交付两件事：打开或刷新 Web 页面时立即看到与镜像标签一致的版本号，并且 WebSocket 初始化配置在连接建立后立即下发，不再被 Cloudflare 地区探测和测速源 ISP 探测阻塞；同时为官方扫描建立可持久化的「数据中心库」——扫描候选 IP 并记录实际 `colo`，保存到容器 `/data`，页面按库中数据中心选择只扫描某个地区，支持手动更新、空库提示与重启后保留。

# 范围

## 目录结构

### 新建

- `combined_refactor/dc_library.go`：数据中心库的数据结构、加载、保存、状态摘要与按数据中心取候选 IP。
- `combined_refactor/tasks_library.go`（或并入 `tasks_official.go`）：执行库扫描，复用官方 `trace` 扫描器与有界并发，记录成功 IP 的 `colo`，完成后更新库并发送状态。
- `combined_refactor/dc_library_test.go`：库加载/保存/状态摘要、候选选择、空库与损坏文件回退的单元测试。

### 修改

- `combined_refactor/main.go`：返回 `index.html` 时把 `versionLine` 的初始占位文本替换为 `appVersionLabel()`；启动时加载本地数据中心库；保持 `Cache-Control: no-cache`。
- `combined_refactor/server.go`：移除 WebSocket 握手阶段的同步地区探测与测速源 ISP 探测，连接后立即发送 `init_config`；`init_config` 附带数据中心库摘要；新增库扫描与状态查询处理器；官方扫描在选定数据中心时使用库中候选 IP。
- `combined_refactor/tasks_official.go`：支持从数据中心库传入指定 DC 的候选 IP；保留目标 DC 过滤、早停与零结果提示。
- `combined_refactor/protocol.go`：新增库扫描请求、库状态与完成消息结构。
- `combined_refactor/index.html`：官方参数中的「数据中心」改为从库读取的下拉（含「全部」与「手动输入其他代码」），显示库更新时间/数据中心数/IP 数；新增「更新数据中心库」按钮、扫描进度与空库提示；保持现有配置保存/加载。
- `combined_refactor/storage.go`：本地库文件纳入持久化路径说明；不改变现有缓存清理语义（库不被「恢复全部默认配置」删除）。
- `combined_refactor/*_test.go`：补充库持久化、候选选择、空库与初始化快速构造测试。
- `docs/docker.md`：说明 `dc-library-v4.json` / `dc-library-v6.json` 保存在 `/data`，重建容器后保留。

### 不修改

- 不改变扫描、详细测试、测速、导出、R2/GitHub 上传、自动调度、NSB 的业务逻辑。
- 不改变代理检测规则、7 秒超时、三次尝试与启动任务前的 `check_proxy_country` 门禁。
- 不改变 `version_info` 更新检查、版本比较与更新链接。
- 不改变 Dockerfile、镜像标签、CI 工作流与 `/data` 挂载方式。
- 不改变 `reset_all_config` 现有缓存文件清理列表（不删除数据中心库）。
- 不改变 CLI 与自动调度入口；本次数据中心库只覆盖 Web 页面。

## 现状事实

- [index.html:1668](file:///Users/tune/Develop/GitHub/CFData-docker/combined_refactor/index.html#L1668) 的版本行初始文本为 `dev`，只有收到 WebSocket `init_config` 后才由 `renderVersionInfo` 更新（[index.html:6546](file:///Users/tune/Develop/GitHub/CFData-docker/combined_refactor/index.html#L6546)）。
- 服务端在发送 `init_config` 之前同步执行地区探测（`detectCloudflareTraceCountry`，3 次尝试共用 7 秒超时，[geo_warning.go:17](file:///Users/tune/Develop/GitHub/CFData-docker/combined_refactor/geo_warning.go#L17)）和测速源解析（`resolveStartupSpeedTestURL`，自动选择时探测 ISP，5 秒超时，[speed_url.go:70](file:///Users/tune/Develop/GitHub/CFData-docker/combined_refactor/speed_url.go#L70)），网络不可达时页面最长等待约 12 秒。
- 启动阶段 `main.go` 已执行 `ensureSpeedURLAvailability()` 并解析过测速源（[main.go:237](file:///Users/tune/Develop/GitHub/CFData-docker/combined_refactor/main.go#L237)），`speedURLStatus` 来自本地缓存，不需要每个连接重复探测。
- 前端在开始任务前通过 `requireProxyWarning` 发送 `check_proxy_country` 重新检测（[index.html:3355](file:///Users/tune/Develop/GitHub/CFData-docker/combined_refactor/index.html#L3355)），启动时的地区探测不是任务门禁；`init_config.proxyWarning` 当前仅用于日志和未被调用的缓存判断。
- HTTP 层 HTML 已设置 `Cache-Control: no-cache`，版本延迟不是浏览器缓存导致。
- 官方扫描从 `https://www.baipiao.eu.org/cloudflare/ips-v4`（或 v6）获取候选池；IPv4 内置快照 6534 行，按 /24 展开后每个子网随机取一个 IP；IPv6 快照 633 行，按 /48 展开后随机取一个 IP。当前官方扫描会扫描全部候选（除目标 DC 早停或用户取消）。
- 数据中心只能通过候选 IP 请求 `speed.cloudflare.com/cdn-cgi/trace` 得到的 `colo` 判定；`locations.json` 只负责把 `colo` 映射为城市、地区与国家，不是 IP→数据中心库。
- 页面当前「数据中心」是自由文本输入（[index.html:1793](file:///Users/tune/Develop/GitHub/CFData-docker/combined_refactor/index.html#L1793)），没有数据中心库、库扫描按钮、库更新时间或空库提示。
- Docker 运行目录为 `/data`（[Dockerfile:47](file:///Users/tune/Develop/GitHub/CFData-docker/Dockerfile#L47)），compose 使用 `./data:/data`（[docker-compose.yml:9](file:///Users/tune/Develop/GitHub/CFData-docker/docker-compose.yml#L9)）；`ips-v4.txt`、`locations.json` 等文件都直接保存在该目录，因此本地库文件也会随 `/data` 持久化。
- 任务框架 `startTaskNamed` 保证同一会话一次只运行一个任务，并提供进度、日志、停止与 `task_complete`/`task_stopped` 消息；库扫描可以复用该机制。

# 非目标

- 不把版本号改成独立 HTTP 接口，也不新增外部依赖。
- 不在页面加载时异步弹出代理警告弹窗；代理警告仍只在用户开始任务时按现有流程检查。
- 不改变 `init_config` 字段的名称与语义。
- 不调整测速源选择策略、ISP 判定或测速源可用性探测。
- 不自动刷新数据中心库，也不在普通官方扫描中自动写入或更新库；库只由用户手动触发的库扫描更新。
- 不宣称库中的 IP→数据中心映射永久有效；每次按 DC 扫描仍通过实际 `trace` 重新验证 `colo`。
- 不新增第三方按数据中心划分的 IP 数据源；库只来自本机扫描观测。
- 不实现按国家/地区聚合选择或多选数据中心；下拉以单个 `colo` 为选项。
- 不实现「库中 IP 不足时自动回退全量候选池」；库中选定 DC 的候选只来自库，结果不足时提示更新库。
- 不改变现有「恢复全部默认配置」的删除范围，不因该操作清空数据中心库。
- 不把数据中心库用于 NSB 模式、详细测试候选或 GitHub/R2 上传格式。
- 不改变 CLI `-offdc`、自动调度 `offdc` 与相关配置字段的行为。

# 验收示例

- A1：构建并运行指定版本（如 `sha-53c9986`）后打开或刷新页面，首屏 HTML 中的版本行即为该版本标识，不依赖 WebSocket；WebSocket 延迟或断开时仍显示正确版本。
- A2：在 Cloudflare `cdn-cgi/trace` 与 ISP 探测均不可达（模拟超时）的环境下，WebSocket 连接后 `init_config` 立即到达，页面立即进入已连接状态并应用测速源默认值、测速源状态与代理状态，不再等待探测超时。
- A3：开始扫描前的代理检测行为不变：中国/未知/测试标签放行，其他地区或检测失败仍按现有规则弹出警告；7 秒超时与三次尝试保持。
- A4：测速源默认值与可用状态仍正确应用；自动选择逻辑不变；`version_info` 仍能在后台更新检查后追加“发现新版本”链接。
- A5：`go build ./...` 与 `go test ./...` 在 `combined_refactor` 下通过；新增测试覆盖版本 HTML 注入和初始化配置立即构造，现有版本与扫描测试不回归。
- A6：点击「更新数据中心库」并使用当前 IP 类型/端口/线程数/合格延迟/扫描方式时，系统扫描官方候选池，记录成功 IP 的 `colo`，将结果保存为 `/data/dc-library-v4.json` 或 `/data/dc-library-v6.json`，完成后页面显示更新时间、数据中心数与 IP 数；重建容器后库数据仍存在。
- A7：库中存在 HKG 等数据中心时，「数据中心」下拉以单个 `colo` 为选项，显示城市/国家与 IP 数量；选择 HKG 后启动官方扫描，只使用库中 HKG 的 IP 作为候选，扫描结果只包含 HKG，早停数量仍按现有语义生效。
- A8：当前 IP 类型没有库数据时，「数据中心」下拉禁用并提示先扫描一次；「全部」全量扫描仍按现有行为可用；库扫描完成后下拉自动启用并显示新数据。
- A9：用户手动更新库成功后，库内容、更新时间和下拉选项刷新为新结果；库扫描被取消或失败时保留旧库并给出提示；普通官方扫描不会自动改动库。
- A10：库扫描使用当前 IP 类型/端口/线程数/合格延迟/扫描方式，不设置目标数据中心、不早停；进度、日志与停止按钮工作正常，扫描期间不能同时启动其他官方任务，库扫描结果不进入官方扫描结果表。
- A11：选择「全部」时官方扫描行为与现状一致；选择「手动输入其他代码」时仍按现有全量候选池与目标 DC 过滤扫描；NSB、详细测试、测速、导出、R2/GitHub 上传与自动调度流程不受影响。

# 约束与不变量

- 首屏版本号、`init_config.version`、启动 banner 与 CLI 版本输出必须使用同一个 `appVersionLabel()`，展示与 GHCR `sha-<短哈希>` 标签一致。
- 版本 HTML 注入必须只替换 `versionLine` 的占位内容，不能影响页面其他 `dev` 文本或 HTML 结构；注入值需 HTML 转义。
- `init_config` 的发送不能包含任何同步外部网络请求；`speedURLStatus` 使用启动缓存，代理状态与数据中心库摘要均来自本地数据。
- 页面 HTML 仍使用 `Cache-Control: no-cache`；热刷新应获取最新内嵌页面。
- 代理检测仍使用原有 7 秒超时、三次尝试、`shouldWarnProxyCountry` 判定与任务前检查流程。
- 数据中心库只存储实际扫描观测到的 `IP → colo` 与观测时间；每次按 DC 扫描必须重新请求 `trace` 验证 `colo`，不得把库内容当作静态事实直接输出。
- 库文件写入必须原子化（临时文件 + rename），损坏或无法读取时按空库处理并记录日志，不得阻止程序启动。
- 库按 IP 类型分开保存（IPv4/IPv6）；文件结构使用明确 schema 版本，便于后续扩展。
- 库扫描与官方扫描共用任务互斥；库扫描不设置目标 DC、不启用早停，使用当前 IP 类型、端口、线程数、合格延迟与扫描方式。
- 库扫描成功完成后替换该 IP 类型的库快照；取消或失败时保留旧库，不写入半成品。
- 选定库中数据中心时，候选 IP 只来自库；不得自动回退全量候选池。结果为空时提示更新库。
- 选择「全部」或「手动输入其他代码」时，保持现有全量候选扫描与目标 DC 过滤行为。
- 库选项只能按 `colo` 展示；城市、地区、国家只作为 `locations.json` 的展示标签，不改变选择粒度。
- 不改变 `reset_all_config` 的删除列表；库文件不因恢复默认配置被删除。
- 不引入新的外部网络依赖；候选池仍使用现有 `ips-v4.txt`/`ips-v6.txt` 及离线快照回退。

# 决策

- D1（2026-10-29，用户要求）：修复刷新页面后版本号延迟显示的问题，按“版本首屏渲染 + 初始化配置不等待网络探测”的方案实施。
- D2（实现选择）：版本号在服务端返回内嵌 `index.html` 时通过字符串替换渲染，不新增版本接口。
- D3（实现选择）：从 WebSocket 初始化路径移除同步地区探测与测速源 ISP 探测；地区检测继续由任务前的 `check_proxy_country` 按需执行。
- D4（实现选择）：`init_config` 继续携带 `speedTestURL`、`speedTestDefault`、`speedURLStatus`、`proxyStatus` 等字段，其中可用性状态来自启动缓存。
- D5（2026-10-29，用户要求）：数据中心库并入本 change，不另开第二个 change。
- D6（实现选择）：库扫描复用官方候选池与 `trace` 扫描器，记录成功 IP 的 `colo`，按 IP 类型保存到 `/data/dc-library-v4.json` / `dc-library-v6.json`；库扫描使用当前扫描参数，不设目标 DC、不早停。
- D7（实现选择）：库成功完成后整体替换该 IP 类型的快照；取消或失败保留旧库；普通扫描不自动写入库。
- D8（实现选择）：库文件不纳入 `reset_all_config` 的删除列表，避免用户丢失手动建立的库。
- D9（2026-10-29，用户确认）：「数据中心」下拉按单个数据中心（`colo`，如 HKG）选择，显示城市/国家与 IP 数量；不做国家/地区聚合或多选。
- D10（2026-10-29，用户确认）：选中库中数据中心后，官方扫描只使用库内该数据中心的 IP 作为候选；不自动回退全量候选池，结果不足时提示更新库。
- D11（2026-10-29，用户确认）：当前 IP 类型库为空时，仅禁用「数据中心」下拉并提示先扫描；「全部」全量扫描仍可用，不禁用官方扫描开始按钮。
- D12（2026-10-29，用户确认）：数据中心库本次只覆盖 Web 页面；CLI `-offdc` 与自动调度保持现状。
- D13（实现选择）：保留下拉中的「全部」与「手动输入其他代码」选项；手动输入仍走现有全量候选池与目标 DC 过滤，避免丢失已有能力。

# 待解决问题

（无）

# 验证预期

- `combined_refactor` 下 `go build ./...` 与 `go test ./...` 通过。
- 新增单元测试：版本 HTML 注入只替换目标占位且正确转义；`init_config` 负载构造不触发网络探测；库加载/保存、状态摘要、空库、按 DC 选候选、损坏文件回退均有覆盖。
- 浏览器实测：在 Cloudflare trace 不可达网络下刷新页面，版本号首屏可见；连接后立即收到 `init_config`，测速源状态和代理状态正常渲染。
- 数据中心库实测：点击更新库后完成扫描并写入 `/data`；重建容器后库仍在；下拉显示库中数据中心与数量；选择某个 DC 后只扫描该 DC 的库内 IP 并仍按早停结束；空库时下拉禁用并提示，全量扫描仍可用。
- 回归检查：任务前代理警告、自动测速、官方全量扫描、手动输入 DC、NSB 扫描、版本更新提示、Docker 启动 banner 与「恢复全部默认配置」行为不变。
- `docker compose up -d --force-recreate` 后页面版本号与镜像 `sha-<短哈希>` 标签一致，库文件保留在 `/data`。
