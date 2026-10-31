# Proposal

## Why

当前 Web 界面（7236 行单文件）功能堆叠杂乱：参数区块平铺、结果表与日志观感差、无整体布局体系。用户确认以 subs-check-pro-webui 的交互与输出呈现为范本大改：只做 Web 控制（CLI 弃用不删），做好 Docker 部署，让界面"显示得特别好"。

## What Changes

- **布局骨架**：重写 `index.html` 为三栏 Grid 一屏式布局（左侧窄导航栏 / 中栏控制卡+日志卡 / 右栏配置卡），移动端折叠为单列 + 顶部图标组；页面区块按导航组织（任务运行 / 官方优选 / 非标优选 / 分析报告 / 设置）
- **任务启停交互**：48px 圆形启停状态机按钮（空闲绿→准备蓝→运行红→停止中），乐观 UI + toast 反馈，后台任务跟随入口
- **进度与日志**：4px 细进度条（阶段名 + 计数 + 渐变 shimmer）；终端风日志区（等宽、内阴影、级别着色 INF/WRN/ERR/DBG、时间戳置灰、新行上浮动画、上滚自动暂停 + "回到底部"胶囊、环形缓冲）
- **结果与分析呈现**：统计 chip 网格（大数字 + 语义色）、地区/数据中心排行（rank 徽章 + 百分比条）、结果表格（排序/筛选/一键复制）、导出与上传（GitHub/edgetunnel/R2）入口重排入导航
- **配置编辑**：配置按分组表单呈现（横向 tab + 组内 label-控件行 + 灰色 hint），含自动调度、R2 上传、管理面代理
- **设计体系**：CSS 自定义属性设计变量（主色 teal、语义色 success/warning/danger、玻璃拟态、color-mix 半透明）、深浅主题切换并持久化、登录页样式统一
- **技术形态不变**：保持零依赖原生 JS/CSS 单文件 + go:embed 内嵌 + WebSocket 通道；不引入框架与构建步骤

## Capabilities

### New Capabilities

- `web-console`: Web 控制台的布局与主题、任务启停交互、进度与日志呈现、结果与分析展示、配置编辑与移动端适配等界面行为。

### Modified Capabilities

（无——任务管线、调度、上传与部署行为不变）

## Impact

- `combined_refactor/index.html`：整体重写（保持单文件内嵌与 WS 通道）
- `combined_refactor/login.html`：样式与主题统一
- `combined_refactor/server.go`/`protocol.go`/`session.go`：按需新增状态聚合或分析摘要消息，现有 21 种消息保持兼容
- `Dockerfile`/部署配置：仅在内嵌与运行方式需要时小幅调整
- 不改动：任务管线（tasks_*.go）、调度器、CLI 代码（弃用不删）、R2 Worker 协议
