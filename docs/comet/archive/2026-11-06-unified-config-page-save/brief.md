# 目标

统一右侧全部参数配置页面的保存入口：每个页面右上角固定一个「保存当前页面的配置」按钮，位置、样式与文案一致；并修好「结果操作 → 字段与顺序」刷新后回退的问题——当前字段清单虽已固定为完整字段，但字段顺序、自定义字段、格式、行范围等改动没有写入浏览器本地存储，刷新页面就恢复默认值。

# 范围

## 目录结构

### 修改

- `combined_refactor/index.html`：为自动化、结果操作、上传账号、设置四个分组补齐统一的 `cfg-panel-head`（标题在左、保存按钮在右）；移除或下沉各分区内位置不一致的保存按钮；保持官方优选 / 非标优选已有右上角「保存参数」。
- `combined_refactor/web/app.js`：补齐结果面板设置的保存触发（字段顺序、勾选、自定义字段、数据来源、格式、行范围、筛选、分隔符、IPv6 方括号）；新增各页面级保存入口函数；保持既有保存语义（服务器参数 / 调度 / 本地存储 / 账号加密存储）。
- `combined_refactor/web/enhance.js`：若页面头部由增强层渲染，则同步面板头部结构；不改动模式切换与布局逻辑。
- `combined_refactor/web/test/`：为新增的纯逻辑（保存载荷收集、页面保存路由）补 Node 单测。
- `combined_refactor/main.go`：为 `/static/` 响应补 `Cache-Control: no-cache`，保证升级后刷新能拿到新的 `app.js` / `enhance.js`。

### 不修改

- 不改动任务参数、调度、通知、R2、上传账号的服务器/本地存储格式与字段名。
- 不改动结果导出内容生成与字段清单本身（上一 change 已完成）。
- 不改动左栏任务控制卡与日志区布局。

## 现状事实

- `combined_refactor/index.html:212-218`、`357-362`：官方优选 / 非标优选面板已有 `cfg-panel-head`，右侧为「💾 保存参数」，调用 `saveTaskConfigToServer(scope)` 写入服务器配置。
- `combined_refactor/index.html:555`：自动化保存按钮位于页面底部（`saveScheduleConfig()`），没有页面头部。
- `combined_refactor/index.html:665-668`：结果操作面板有 `cfg-panel-head` 但只有标题，没有保存按钮。
- `combined_refactor/index.html:558-597`：上传账号面板没有页面头部，GitHub 与 edgetunnel 各自在分区底部有保存按钮。
- `combined_refactor/index.html:599-656`：设置面板没有页面头部，主题即时生效；代理有独立「保存/清除」；配置管理有「保存当前配置」（`saveConfigToLS`）、加载、自动加载偏好等分散按钮。
- `combined_refactor/web/app.js:4465-4487`：`saveResultPanelSettings()` 已实现，写入 localStorage 键 `cfdata-result-panel`（含 source/format/fileName/startRow/endRow/筛选/fieldOrder/customFields/分隔符/IPv6）。
- `combined_refactor/web/app.js:4489-4552`：`restoreResultPanelSettings()` 已在初始化时调用（`app.js:5331`），负责把 `cfdata-result-panel` 恢复到面板；当前源码的保存触发依赖预览刷新路径，没有页面级显式保存入口。
- `combined_refactor/web/app.js:4442-4453`：`updateResultPreview()` 末尾已调用 `saveResultPanelSettings()`；`renderResultFieldOptions()`、格式切换、自定义字段、筛选项与行范围输入都经过该路径，因此当前代码里结果面板设置实际会即时写入 localStorage（本机实测：全选字段、调序、加自定义字段、切 CSV 后刷新，26 项顺序与自定义字段均保留）。
- 用户本机在 52527 端口运行的 `./cfdata` 进程启动于 2026-10-31 21:47，早于加入结果面板持久化的 `c42dadd`（2026-11-01 12:26）；该实例前端是旧 bundle，所以刷新会回到默认值。当前源码已包含持久化。
- `combined_refactor/main.go:286-292`：`/static/` 使用 `http.FileServer` 直接返回，没有 `Cache-Control`、`ETag` 或 `Last-Modified`；升级镜像后浏览器可能继续使用缓存的旧 `app.js` / `enhance.js`，导致新保存入口不生效。
- 参考项目 `subs-check-pro`（`/Users/tune/Downloads/subs-check-pro`）的前端为压缩产物；其可确认的交互是「显式保存 + 未保存内容确认」（`有内容未保存 / 继续编辑 / 确认离开`、`保存中...`），保存入口位于页面右上角操作区。本次只借用该交互形态，不复制其组件或样式实现。

# 非目标

- 不引入前端框架或新依赖，不重写配置页布局。
- 不把结果面板设置迁移到服务器配置；仍使用浏览器本地存储。
- 不改动任务参数、调度、通知、R2 与上传账号的存储字段和接口协议。
- 不实现跨设备同步配置。
- 不修改左栏任务运行区、日志区与结果表格。

# 验收示例

- A1：官方优选、自动化、结果操作、上传账号、设置五个页面顶部都有同一结构的头部：左侧页面标题，右侧「💾 保存」按钮，位置与样式一致；点击后给出保存成功或失败的可见提示。
- A2：结果操作页改动字段勾选、输出顺序、自定义字段、数据来源、格式、行范围、筛选、分隔符或 IPv6 方括号后刷新页面，设置全部保留（自动保存）；点击右上角「保存」同样落盘并提示成功。
- A3：自动化页右上角「保存」提交调度、通知与 R2 字段并在界面上回显，等同于原「保存自动化设置」；页面底部重复的保存按钮被移除。
- A4：上传账号页右上角「保存」分别保存 GitHub 与 edgetunnel 表单中已填写且校验通过的账号；空白表单跳过不报错；填写但校验失败时给出与现有分区按钮一致的提示，且不写入无效账号。
- A5：设置页右上角「保存」写入当前表单参数到本地、保存自动加载偏好，并在代理输入框非空时保存管理面代理；主题切换仍即时生效，现有加载/删除配置按钮行为不变。
- A6：升级镜像后刷新页面，`/static/app.js` 与 `/static/enhance.js` 响应带 `Cache-Control: no-cache`，页面加载的是新前端（含统一保存入口）。
- A7：`combined_refactor` 下 `go build ./...`、`go test ./...` 与 `node --test web/test/*.test.js` 通过，既有配置保存/回显测试不回归。

# 约束与不变量

- 保存按钮只改变入口位置与触发方式，不改变各配置项存储位置与字段名。
- 结果面板设置的存储键 `cfdata-result-panel` 与既有字段结构保持不变，旧存储可继续恢复。
- 保存失败（连接未就绪、localStorage 不可用、校验失败）必须给出与现有提示一致的可见反馈，不得静默失败。
- 五处页面头部使用同一 HTML 结构与样式类，后续新增页面可复用。
- 不改变任务参数在服务器配置文件中的自动回显逻辑。

# 决策

- D1（2026-11-06，用户确认）：结果操作页采用「自动保存 + 右上角保存按钮」；设置改动即时写入 `cfdata-result-panel`，按钮再统一落盘并提示成功。
- D2（2026-11-06，用户确认）：上传账号页右上角保存分别保存 GitHub 与 edgetunnel 两个表单中已填写且校验通过的账号，空白表单跳过。
- D3（2026-11-06，用户确认）：设置页右上角保存 = 当前参数到本地 + 自动加载偏好 + 已填写的管理面代理。
- D4（实现选择）：五个页面统一使用 `cfg-panel-head` + 右侧「💾 保存」按钮，按钮文案统一，具体保存内容写在 `title` 中。
- D5（实现选择）：结果操作页的页面保存同时包含该页上的 R2 上传设置（调用 `saveScheduleConfig()`），并移除分区内重复的 R2 保存按钮。
- D6（实现选择）：为 `/static/` 增加 `Cache-Control: no-cache`，避免升级后浏览器继续使用旧前端；不改变 HTML 与 API 的缓存策略。

# 待解决问题

（无）

# 验证预期

- `combined_refactor` 下 `go build ./...`、`go test ./...` 通过；`node --test web/test/*.test.js` 通过。
- 浏览器实测：五个页面顶部头部与保存按钮位置一致；结果操作页按 Q1 结论修改设置后刷新，设置保留；自动化、账号、设置页保存后刷新/重连仍回显；保存失败提示可见。
- 回归检查：官方/非标参数保存到服务器、调度与通知、R2 配置、账号加密存储、主题切换、代理保存/清除、配置加载与删除行为不变。
