# Comet Design Handoff

- Change: webui-scp-style-redesign
- Phase: design
- Mode: compact
- Context hash: 3a37ebf5dfa59a0e70e24ae1a94d2ac6f70dbdce67c986ca6ef6a6f401fa00e6

Generated-by: comet-handoff.sh
Task hash policy: task-content-v1. Read tasks.md for live completion; excerpts are design-time context.

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## docs/openspec/changes/webui-scp-style-redesign/proposal.md

- Source: docs/openspec/changes/webui-scp-style-redesign/proposal.md
- Lines: 1-33
- SHA256: d610c42bcb86bb7094d80f154ceaa36d22d9cb229f478be883289094f5424ec2

```md
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

```

## docs/openspec/changes/webui-scp-style-redesign/design.md

- Source: docs/openspec/changes/webui-scp-style-redesign/design.md
- Lines: 1-55
- SHA256: 7914b03719e42f1c9482a8241ce0101bd02189613712d60a9257ff24206354fb

```md
---
comet_change: webui-scp-style-redesign
role: technical-design
canonical_spec: openspec
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

```

## docs/openspec/changes/webui-scp-style-redesign/tasks.md

- Source: docs/openspec/changes/webui-scp-style-redesign/tasks.md
- Lines: 1-40
- SHA256: 39eacd24a89e596f74cc2d7eaead0bf97ce3728c4b551ce62b3bce4e3bf513df

```md
# Tasks

## 1. 设计体系与布局骨架

- [ ] 1.1 重写 `index.html` 基础层：`:root` 设计变量（teal 主色、语义色、玻璃拟态、暗色变量）+ `data-theme` 切换与 localStorage 持久化 + 系统偏好默认；验证：切主题刷新保持，`prefers-color-scheme` 未手选时生效 <!-- comet-task:1b52202a-fb8c-46a3-be34-f65707c68581 -->
- [ ] 1.2 三栏 Grid 布局骨架：左侧导航（品牌 + 导航项）、中栏（控制卡 + 日志卡）、右栏（配置卡）；窄屏单列折叠 + 安全区适配；验证：桌面/窄屏两种宽度下无横向溢出、功能可达 <!-- comet-task:c91a1e02-0e46-401c-b920-307f6532cfa6 -->
- [ ] 1.3 WS 连接与消息分发框架迁移：`wsSend`/`connect`/`handleMessage` 分发表以 21 种请求/39 种响应为核对清单逐项保留语义；验证：登录后连接建立，版本信息与初始化配置正常显示 <!-- comet-task:fa5bfd47-0422-4aca-9a4a-11f40c8505c8 -->
- [ ] 1.4 Android 桥兼容迁移（`window.CFDataAndroid` 相关调用行为不变）；验证：桥存在时旧调用路径不报错 <!-- comet-task:69307b1f-3ece-4186-b42e-b38863e6ec27 -->

## 2. 任务运行视图

- [ ] 2.1 圆形启停按钮状态机（空闲绿/准备蓝/运行红/停止中）+ 乐观 UI + toast 反馈，状态由服务端消息回填；验证：启动→运行→停止全程状态正确、连点有锁 <!-- comet-task:983b45e6-354b-4f11-906c-c569814decd8 -->
- [ ] 2.2 官方优选与非标优选参数表单迁入分组卡片（含 DC 库选择、端口组、测速源、链路复验等现有全部字段），字段收发与现有消息兼容；验证：用新旧参数各跑一次扫描行为一致 <!-- comet-task:9f6d0f3b-40e4-452d-92e6-3a76cf8cf076 -->
- [ ] 2.3 进度条组件：阶段名 + 4px 渐变进度 + 计数 + 无总量阶段的活动指示；后台任务确认卡与跟随入口重绘；验证：扫描中进度实时更新，转后台/跟随流程正常 <!-- comet-task:e3a87f02-ea6a-47ca-bf1e-d3e4ef9578cb -->

## 3. 日志区

- [ ] 3.1 终端风日志组件：等宽凹槽背景、级别着色（INF/WRN/ERR/DBG）、时间戳置灰、URL/数字高亮、ANSI 解析、新行上浮动画、500 行环形缓冲；验证：三种级别行着色正确，超量日志页面不卡顿 <!-- comet-task:bcb08df1-be03-4dcf-af18-273a4bd48f96 -->
- [ ] 3.2 滚动行为：上滚暂停自动滚动 + 毛玻璃「回到底部」胶囊、点击平滑滚回、框选文本不打断；验证：长日志下上滚/回底/选择复制均正常 <!-- comet-task:12709680-6695-488b-8c76-e886d3efac63 -->

## 4. 结果与分析视图

- [ ] 4.1 统计摘要 chips 与数据中心/地区排行（前端从结果行聚合，rank 徽章 + 百分比条，语义色三档）；验证：完成后数值与结果行统计一致 <!-- comet-task:986f84a0-2d00-490a-924b-11d3afc60be6 -->
- [ ] 4.2 结果明细表迁移：排序、筛选、一键复制、详细测试视图；验证：四类操作与现行为一致 <!-- comet-task:cfb48e93-28fd-4144-b4bb-77f663f69325 -->
- [ ] 4.3 导出与上传入口重排：结果操作弹窗（txt/csv/substore、行范围、字段选择、分隔符）、GitHub/edgetunnel/R2 上传（含账户管理与令牌加密存储）；验证：各导出与上传路径功能与现行为一致 <!-- comet-task:9eb0936c-61c5-4271-9f84-c6163369687e -->

## 5. 配置编辑

- [ ] 5.1 右栏配置卡：自动调度（开关/Cron/间隔）、R2 上传（地址/令牌/文件名/别名模板/最终地址反馈）、管理面代理分组表单 + hint 说明 + 保存反馈；验证：保存后重载回显一致，调度器热重载生效 <!-- comet-task:0013a895-0ef3-46a7-8628-8d239b87a3a6 -->
- [ ] 5.2 设置菜单迁移：主题、恢复默认配置、配置保存/加载/同步（localStorage 与服务器配置）到导航「设置」视图；验证：各设置项功能与现行为一致 <!-- comet-task:17e0bfd1-ff37-4943-b694-665e371d595b -->

## 6. 收尾与集成

- [ ] 6.1 `login.html` 统一设计变量与主题（含 `?error=1` 错误态样式）；验证：登录页两种主题下渲染正常、登录流程不受影响 <!-- comet-task:93597278-7719-4787-8287-2d514ee24d32 -->
- [ ] 6.2 全量构建与测试：在 `combined_refactor/` 执行 `go build ./...` 与 `go test ./...` 全部通过，并按 AGENTS.md 产出 `release_assets/cfdata-test` 测试版 <!-- comet-task:8fbd20fc-23de-434b-943c-c9b92d0a3dd2 -->
- [ ] 6.3 端到端验收：Docker 方式运行测试版，逐项走查 spec 场景（布局/主题/启停/进度/日志/结果分析/配置/移动端），确认全部通过 <!-- comet-task:fe654106-525a-4e67-8527-45d9d19a18fb -->

## Workflow follow-up

- 审查要求满足后归档本 change。

```

## docs/openspec/changes/webui-scp-style-redesign/.openspec.yaml

- Source: docs/openspec/changes/webui-scp-style-redesign/.openspec.yaml
- Lines: 1-2
- SHA256: c6ab3a95d8d0fd469ca8750ef79234025bf4a8356d5d57856e97d7e47fc8b6c2

```md
schema: spec-driven
created: 2026-10-31

```

## docs/openspec/changes/webui-scp-style-redesign/specs/web-console/spec.md

- Source: docs/openspec/changes/webui-scp-style-redesign/specs/web-console/spec.md
- Lines: 1-130
- SHA256: b9f45cea791d2f5a7718e821f75f4b083ad7a62f7af87fd1b858bf164bd7b685

[TRUNCATED]

```md
# Spec Delta

## Purpose

提供 CFData 的 Web 控制台界面行为：以三栏卡片式布局组织任务运行、日志、结果分析与配置编辑，支持深浅主题与移动端，全部操作经现有 WebSocket 通道完成。

## ADDED Requirements

### Requirement: 三栏布局与导航

桌面端控制台 SHALL 采用三栏布局：左侧窄导航栏（品牌与导航项）、中栏（任务控制卡与日志卡）、右栏（配置卡）。导航 SHALL 至少包含：任务运行、官方优选、非标优选、分析报告、设置。移动端（窄屏）SHALL 折叠为单列，导航项以顶部或卡片内图标组呈现。

#### Scenario: 桌面端三栏渲染

- **WHEN** 在桌面宽度浏览器打开控制台
- **THEN** 页面呈现侧边导航、中栏控制与日志、右栏配置三个区域，无横向溢出

#### Scenario: 移动端单列折叠

- **WHEN** 在窄屏（移动端宽度）打开控制台
- **THEN** 布局折叠为单列，功能仍全部可达

#### Scenario: 导航切换视图

- **WHEN** 用户点击导航项「官方优选」「非标优选」「分析报告」
- **THEN** 主区域切换到对应视图，当前导航项高亮

### Requirement: 深浅主题

控制台 SHALL 提供深色与浅色主题，跟随系统偏好为默认值，用户手动选择后持久化；刷新后保持所选主题。登录页 SHALL 与控制台使用同一主题体系。

#### Scenario: 切换主题并持久化

- **WHEN** 用户切换为深色主题后刷新页面
- **THEN** 页面仍为深色主题

#### Scenario: 跟随系统默认

- **WHEN** 用户未手动选择过主题且系统为深色模式
- **THEN** 控制台以深色主题呈现

### Requirement: 任务启停交互

控制台 SHALL 提供圆形启停按钮作为任务状态机入口：空闲（绿色）、准备中（蓝色）、运行中（红色，可点击停止）、停止中（禁止操作）。点击后 SHALL 立即给出乐观反馈（按钮状态与 toast 提示），并按服务端实际状态回填确认。运行中任务 SHALL 可转入后台，新会话可重新跟随后台任务。

#### Scenario: 启动任务的状态流转

- **WHEN** 空闲状态点击启停按钮发起扫描
- **THEN** 按钮立即进入准备中（蓝色）并显示启动提示，随后按服务端进度转为运行中（红色）

#### Scenario: 运行中停止任务

- **WHEN** 运行中点击启停按钮
- **THEN** 按钮进入停止中状态并提示正在停止，任务停止后回到空闲（绿色）

#### Scenario: 跟随后台任务

- **WHEN** 任务转入后台后刷新页面
- **THEN** 控制台提示存在后台任务，并可重新跟随其进度

### Requirement: 进度呈现

任务运行期间，控制台 SHALL 展示当前阶段名称、进度条与完成计数（已处理/总数）。进度条 SHALL 有渐变填充与流光动画；无总量的阶段 SHALL 以活动指示代替进度。

#### Scenario: 扫描进度更新

- **WHEN** 官方优选扫描进行中
- **THEN** 控制台显示阶段名、进度条随进度增长、计数实时更新

#### Scenario: 无总量阶段

- **WHEN** 后端未提供总数
- **THEN** 显示活动指示与阶段名，不显示误导性的进度百分比

### Requirement: 实时日志

日志区 SHALL 以终端风格呈现：等宽字体、深色凹槽背景、按级别着色（信息/警告/错误/调试）、时间戳置灰、URL 与数字高亮。新日志行 SHALL 有上浮淡入动画。用户向上滚动时 SHALL 暂停自动滚动并显示「回到底部」胶囊，点击后平滑滚回并恢复自动滚动。日志 SHALL 维持有上限的缓冲，超出后移除最旧行。

#### Scenario: 日志级别着色


```

Full source: docs/openspec/changes/webui-scp-style-redesign/specs/web-console/spec.md
