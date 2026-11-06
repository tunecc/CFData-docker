# 目标

两件事：① 把「结果操作」页面的显示设置（字段与顺序、来源、格式、行范围、筛选、分隔符、IPv6、R2 相关显示项）从只存在浏览器 localStorage 改为**服务器持久化**，解决换访问地址（局域网 IP ↔ localhost ↔ 域名）、换浏览器或容器重建后配置丢失；② 官方批量测速遇到 HTTP 429 速率限制时不再直接中止，改为退避重试并自动切换可用测速源后继续。

# 范围

## 目录结构

### 新建

- `combined_refactor/result_panel_config.go`：结果面板设置的读写（`/data/result-panel.json`，schema 版本、原子写入、损坏文件回退空配置）。
- `combined_refactor/result_panel_config_test.go`：设置文件的保存/读取/损坏回退与字段白名单测试。

### 修改

- `combined_refactor/protocol.go`：新增 `get_result_panel_config` / `save_result_panel_config` 请求与 `result_panel_config` / `result_panel_config_saved` 消息结构。
- `combined_refactor/server.go`：注册两个消息处理器；读取失败与写入失败返回错误消息。
- `combined_refactor/web/app.js`：连接后请求服务器设置并按服务器值恢复；改动/保存时先写 localStorage（本地兜底），再防抖同步到服务器；服务器无记录而本地有设置时上送完成迁移；同步失败显示提示。
- `combined_refactor/tasks_speed.go`：官方批量测速加入 429 退避重试与测速源切换；终止条件改为连续多个 IP 仍全部 429 才停止，并在日志说明是否尝试过换源。
- `combined_refactor/speed_url.go`：提供「当前源之外的可用测速源」选择函数，复用 `isSpeedURLAvailable` / `speedURLCandidates`。
- `combined_refactor/tasks_speed_test.go`：429 退避、换源、连续失败终止与成功重置的单元测试（使用 httptest 与可注入的等待时长）。

### 不修改

- 不改动任务参数（官方/非标）、调度、通知、上传账号的存储格式与接口。
- 不改动结果导出内容生成、字段清单与默认勾选。
- 不改动扫描、详细测试、NSB 测速与 R2 上传的业务逻辑。

## 现状事实

- `combined_refactor/web/app.js`：结果面板设置只写入 localStorage 键 `cfdata-result-panel`（`saveResultPanelSettings`），初始化时由 `restoreResultPanelSettings` 恢复；服务器侧没有任何对应存储。
- localStorage 按「协议 + 主机 + 端口」隔离：用户通过 `http://192.168.8.16:13335` 访问时保存的设置，在 `http://localhost:13335`、换端口或换域名后不可见；换浏览器/设备同理。
- 用户部署实例（192.168.8.16:13335）版本为 `sha-9d01c3d`，`/static/app.js` 含 `saveResultPanelSettings` 与 `saveResultsPage`，`Cache-Control: no-cache` 生效；在该实例上用全新浏览器实测「改字段顺序 → 保存 → 刷新」可正常恢复，说明代码路径可用，问题来自 origin/浏览器存储边界。
- `combined_refactor/main.go:286-296`：`/static/` 已带 `Cache-Control: no-cache`。
- 容器入口脚本把二进制同步到 `/data/cfdata` 后执行，`defaultCLIConfigPath()` 取可执行文件所在目录（容器内即 `/data`），配置与 `history/` 都落在 `/data`，重建容器后保留。
- `combined_refactor/tasks_speed.go:131-136`：HTTP 429 被转成「测速失败（速率限制）」；`:179-218` 统计连续 429，连续 3 个即调用 `onRateLimited` 并中止整批；`:275/293` 输出「检测到触发速率限制，请稍后重试或更换网络环境」与「官方批量测速已因速率限制停止」。
- `combined_refactor/speed_url.go`：内置测速源为 `speed.cloudflare.com/__down?bytes=99999999`、`cf.090227.xyz/__down?bytes=99999999`、`speed.okl.abrdns.com`；已有 `isSpeedURLAvailable`、`firstAvailableSpeedURL`、`speedURLCandidates`。
- 当前批量测速每个 IP 下载最长 6 秒、间隔 1.2 秒；同一出口 IP 的连续大流量下载容易被 Cloudflare 限流（429）。

# 非目标

- 不把任务参数、调度、通知、账号迁移到新的存储文件。
- 不做多用户隔离或账号级配置；服务器设置对所有浏览器共享。
- 不引入前端框架、数据库或新依赖。
- 不改变 429 之外的错误处理（超时、连接错误、5xx 仍按现状记为失败）。
- 不自动修改用户配置的测速源选项；换源只在本次批量测速内生效。

# 验收示例

- A1：结果操作页改动字段勾选/顺序/自定义字段/格式/来源/行范围/筛选/分隔符/IPv6 后，刷新页面设置全部保留；右上角「保存」同样落盘并提示成功。
- A2：换浏览器，或改用另一个访问地址（如 `http://localhost:端口` 与 `http://局域网IP:端口`）打开页面，仍恢复服务器上最后一次保存的结果面板设置。
- A3：服务器没有设置记录、但浏览器本地已有旧设置时，首次加载沿用本地设置并自动写入服务器（迁移），刷新与换浏览器后不再丢失。
- A4：服务器写入失败（如 `/data` 不可写）时界面给出可见提示，不显示保存成功；本地 localStorage 仍保留当前设置。
- A5：批量测速中单个 IP 返回 429 时，先等待 3 秒重试、仍 429 再等 6 秒重试；重试成功则正常记录速度并按阈值判定达标。
- A6：重试后仍为 429 时自动切换到另一个可用测速源继续测速，日志说明「已切换测速源」。
- A7：连续多个 IP（6 个）在换源后仍全部 429 时才停止批量测速，并提示已尝试更换测速源，达标统计与已测结果保持一致。
- A8：`go build ./...`、`go test ./...` 与前端 Node 测试通过；新增测试覆盖 429 退避/换源/终止与设置文件读写。

# 约束与不变量

- 服务器设置文件使用独立文件与 schema 版本，写入原子化（临时文件 + rename）；损坏或不可读时按空配置处理，不阻止启动。
- 只接受已知的键集合（与 `cfdata-result-panel` 现有字段一致），未知键忽略；字段顺序数组只保留字符串。
- 服务器设置是权威来源：连接后以服务器值覆盖本地；服务器为空时才用本地值并上送。
- localStorage 继续作为本地缓存与故障兜底，键名 `cfdata-result-panel` 与结构不变。
- 429 退避只影响官方批量测速；单个 IP 遇到非 429 失败仍按原逻辑记为失败，不重试。
- 换源仅使用内置测速源列表，不改变用户显式配置的测速源设置。
- 终止条件与提示必须保持达标计数准确，已测结果不丢失。

# 决策

- D1（2026-11-06，用户确认）：结果面板设置迁移到服务器持久化，localStorage 保留为本地兜底。
- D2（2026-11-06，用户确认）：官方批量测速遇 429 采用「退避重试 + 自动换源」。
- D3（实现选择）：设置存储在 `/data/result-panel.json`，与任务参数配置分离，避免 CLI 配置结构膨胀。
- D4（实现选择）：退避间隔 3s、6s，单个 IP 最多重试 2 次；重试后仍 429 则换源。
- D5（实现选择）：连续 6 个 IP 在换源后仍全部 429 才终止批量测速，日志注明已尝试换源。
- D6（实现选择）：前端改动即防抖同步服务器（约 500ms），显式保存立即同步；两者都先写 localStorage。

# 待解决问题

（无）

# 验证预期

- `combined_refactor` 下 `go build ./...`、`go test ./...` 通过；`node --test web/test/*.test.js` 通过。
- 浏览器实测：同一地址改设置后刷新保留；换端口/换主机地址打开同一实例仍恢复；清空 localStorage 后从服务器恢复。
- 测速实测：用本地 httptest 模拟 429 → 验证退避与换源；真实环境晚点重跑官方批量测速确认不再一次 429 即中止。
- 回归检查：任务参数保存、调度、通知、R2、账号、导出、R2 别名与历史记录行为不变。
