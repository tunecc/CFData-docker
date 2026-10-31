---
comet_change: webui-scp-style-redesign
role: technical-design
canonical_spec: openspec
archived-with: 2026-10-31-webui-scp-style-redesign
status: final
---

# Design

## Context

- 现有前端为 7236 行单文件 `index.html`（内联 CSS ~1643 行 + JS ~4720 行），零依赖、go:embed 内嵌，全部交互经 `/ws` 一条 WebSocket（21 种请求 / 39 种响应，见 `protocol.go`/`server.go`/`session.go`）；另有无认证、全局单任务锁、后台任务跟随、Android 桥（`window.CFDataAndroid`）等既定行为
- 目标范本 subs-check-pro-webui（github.com/sinspired/subs-check-pro-webui）同为零框架手写实现：三栏 Grid 布局、CSS 自定义属性 + `color-mix` 设计体系、48px 圆形启停状态机按钮、动态频率短轮询、终端风日志 colorize、统计 chip + 排行条分析页。本 change 移植其设计体系与交互模式，但数据通道仍用现有 WebSocket（不需要改为轮询）
- 范本的设计细节已核对：主色 teal `#0ea5a0`、语义色 success/warning/danger/idle/prepare、暗色 `#121316/#18191b`、卡片 12px 圆角 + 双层柔和阴影、玻璃拟态 blur 10-12px、日志 13px 等宽内阴影 + 级别徽章 + 上浮动画、进度条 4px 渐变 + shimmer

## Goals / Non-Goals

**Goals:**

- 按范本重写界面层：布局、启停交互、进度、日志、结果分析、配置编辑、主题、移动端
- 现有全部 WebSocket 消息语义保持兼容，服务端最小改动
- 保持零依赖单文件 + 内嵌 + Docker 部署方式

**Non-Goals:**

- 不改任务管线、调度器、上传逻辑与认证模型
- 不删除 CLI（弃用不删）
- 不引入前端框架、构建工具或外部 CDN
- 不做多用户/多任务并行改造

## Decisions

1. **整体重写而非渐进改造**：新建 `index.html` 结构（CSS 变量区 → 布局骨架 → 组件样式 → HTML → JS 模块分区注释），以现有 WS 消息为兼容清单逐项迁移 `handleMessage` 语义。理由：现有 UI 无可保留的样式体系，渐进改造的兼容成本高于重写。旧版本可经 git 回滚。
2. **设计变量直接移植范本体系**：`:root` 定义 `--bg/--fg/--card/--border/--accent(#0ea5a0)/--success/--warning/--danger/--glass-*`，`:root[data-theme='dark']` 暗色变量；组件用 `color-mix(in srgb, var(--accent) N%, transparent)` 做半透明底色。不引入 Tailwind。
3. **单页视图切换，不做多页面路由**：分析报告作为导航视图（同页切换），登录页维持独立 `login.html`（样式统一到同一变量体系）。理由：WS 会话与单文件内嵌架构下，独立分析页需要额外的页面通道与状态广播，收益低。
4. **数据通道保持 WebSocket，不改为轮询**：范本用短轮询是因为其无 WS；我们已有推送通道，进度/日志/结果继续走现有消息。前端状态机从 `task_complete`/`task_stopped`/进度消息推导按钮状态；点击启动即乐观转「准备中」。
5. **分析统计由前端聚合**：控制台已接收全部扫描/测速结果行，统计 chips 与地区/DC 排行直接在前端从结果行计算，不新增后端聚合消息；后续如需跨任务历史统计再议（超出本 change）。
6. **日志 colorize 在前端实现**：移植范本的着色规则（级别词 INF/WRN/ERR/DBG、时间戳置灰、URL/数字高亮、ANSI 码解析），先 `escapeHtml` 再着色；环形缓冲上限 500 行（现有 logBox 无上限，长时间任务会卡顿，一并修复）。
7. **后台任务与全局单任务行为不变**：保留「转入后台」确认卡与跟随后台任务入口，仅重绘样式。
8. **Android 桥兼容保留**：`window.CFDataAndroid` 相关调用迁入新 JS 结构，行为不变。
9. **浏览器目标为现代浏览器**：`color-mix()`/CSS Grid 需要较新内核；关键容器（卡片、按钮）提供纯色 fallback 背景，避免旧内核完全不可用。

## Risks / Trade-offs

- [整体重写的功能回归风险] → 以 protocol.go 的 21 请求/39 响应为迁移核对清单，tasks.md 按功能区块分组验收；重写期间旧版保留在 git 历史可即时回滚
- [单文件体积进一步增大] → 用清晰的分区注释组织（变量/布局/组件/视图/JS 模块），不引框架；如后续持续膨胀再考虑拆分内嵌多文件（需要改 embed 与 Docker，不在本次）
- [范本观感依赖其字体/配色细节] → 变量集中定义便于微调；验收以布局结构、交互状态机、语义色一致性为准，不逐像素对照
- [前端聚合统计与后端真实数据偏差] → 统计口径以结果行为准并在 chips 副注标明口径（如"达标延迟 ≤ N ms"）

## Migration Plan

一次性替换 `index.html`（必要时微调 `login.html`）；服务端仅在消息字段不足以支撑界面时小幅补充。回滚 = `git revert` 对应提交。Docker 构建、内嵌方式与入口脚本不变。

## Open Questions

无——范本设计要素已核对完毕，占位与命名细节在 Build 阶段按设计变量执行。
