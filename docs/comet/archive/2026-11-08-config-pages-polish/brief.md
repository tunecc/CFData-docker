# 目标

右栏配置面板（cfg-panel 的 5 个页面：官方优选 / 非标优选 / 自动化 / 结果操作 / 设置）整体视觉与结构重构：学习 subs-check-pro WebUI（`github.com/sinspired/subs-check-pro-webui@v1.8.2`）的配置表单设计手法，解决当前「不精致、说明不可见、空间利用率低」的问题。

核心改造方向（借鉴参考项目手法）：

1. **统一表单行组件**：配置面板内 4 套并存标记（`.option-item` / `.form-group` / `.modal-field` / `.inline-check`）统一为一套 `.cfg-field` 行结构——Grid 双列（label 列 + 控件列），说明独立成行跨两列；`full-width` 与开关行两种变体。
2. **说明可见化**：现藏于 `title` 属性的次要说明（部分超 100 字）提升为可见 hint（约 10.5–11px、muted 色、紧凑行高），支持内联 `code` 示例样式；`readonly input` 当提示牌的用法全部替换为文本 hint。
3. **行内提示组件**：统一注意/校验提示为三档（ok / info / warn）行内提示，用 `color-mix(in srgb, var(--semantic) N%, transparent)` 从语义变量派色，双主题自动适配，替代现有零散的 `.automation-hint` / `.cfg-field-hint` / 裸内联样式说明。
4. **视觉细节精修**：字号体系（label 13px / hint 10.5–11px / input 桌面 13px、移动端 16px 防 iOS 缩放）；input 高度收紧（padding 6px 10px、radius 8px）；focus 统一公式（border accent + 2.5px accent 15% 环）；4px 间距体系；行内边距统一（消除 cfg-body 12px / cfg-group-body 2-10px / section-box 14px 三套值）。
5. **结构清理**：删除配置面板内的双重标题（自动化页、设置页）、legacy 类替换（`.section-box` / `.control-title` / `.settings-panel`）、约 25 处内联样式沉淀为 class、cfg-tabs 禁止换行。
6. **主题变量补全**：cron 分段色等硬编码 hex 改走主题变量；`--warning` / `--danger` / `--idle` / `--prepare` 补暗色适配；消除 `--text-secondary` 等新旧别名在配置面板内的混用。

# 范围

## 目录结构

### 修改

- combined_refactor/index.html — 右栏配置面板（约 204–837 行）：5 页 DOM 重写为统一 `.cfg-field` 结构，title 说明提升为可见 hint，删除 readonly 提示框、双重标题、内联样式；DOM id 与保存入口保持不变。
- combined_refactor/web/styles.css — cfg 区域样式重写（约 826–1086 行 cfg-* 段）：新增 `.cfg-field` / `.cfg-label-hint` / `.cfg-inline-hint` 组件样式，表单基础样式（196–286 行）适配新字号体系，主题变量补全（6–57 行暗色适配），cron 硬编码色改变量。
- combined_refactor/web/app.js — 仅适配性修改：`cfg-field-hint` 注入逻辑（约 5361–5374 行）改用新的 `.cfg-inline-hint` 组件；面板切换/保存逻辑与 DOM id 不变。
- combined_refactor/web/enhance.js — 如面板切换需适配新结构（`nzShowConfigPanel` 滚动记忆），保持行为不变。

### 明确不建

- 不新增构建工具或前端框架 — 项目为纯静态 HTML/CSS/JS，保持无构建架构。
- 不做 SCHEMA 数据驱动的表单重构 — 若用户确认方案 A，保持静态 HTML 结构。

# 非目标

- 不改变任何配置项的语义、键名、默认值与保存后端格式（保存逻辑按现有 id 收集）。
- 不改左侧运行面板、日志区、扫描/测试表格、登录页。
- 不改复制弹窗（modal，843–926 行）——`.modal-field` 样式保留原样，仅配置面板内不再使用。
- 不做移动端专属重设计（仅保持现有响应式断点行为，随新样式自然继承）。
- 不引入参考项目的 YAML 双模式编辑器、服务端主题持久化等额外功能——只学设计手法。

# 验收示例

- A1: 右栏 5 个配置页的表单项全部为统一 `.cfg-field` 行结构；配置面板内不再出现 `.option-item` / `.form-group` / `.modal-field` 混用（`grep` + DOM 检查）。
- A2: 官方/非标页原藏于 `title` 的说明在界面上可见（hint 样式），`readonly input` 提示牌用法清零；控件 id 全部保持不变，`saveTaskConfigToServer` / `saveAutomationPage` / `saveSettingsPage` / `saveResultsPage` 收集逻辑不受影响。
- A3: 行内注意提示为统一三档组件（ok/info/warn，语义变量派色）；配置面板内不再有裸内联样式说明文字。
- A4: 暗色主题下配置页无硬编码 hex 残留（cron 分段等），浅色/暗色双主题浏览器截图检查通过。
- A5: 布局紧凑化生效：label 与控件同行双列、input 桌面字号 ≤14px、hint 行高 ≤1.6；`node --test`（combined_refactor/web/test）全部通过。
- A6: cfg-tabs 单行不换行；浏览器实测 5 页切换、滚动位置记忆、保存动作正常。

# 约束与不变式

- 参考项目 subs-check-pro WebUI 定位为**实现参考**（学习设计手法），不作为需求来源文档做逐条覆盖；需求以本 brief 为准。
- DOM id、`onclick` / `oninput` 入口、保存后端 API 请求体格式保持不变。
- macOS 开发机路径适配：Go 在 `/opt/homebrew/bin/go`；编译测试版在 `combined_refactor/` 下执行，输出到 `release_assets/cfdata-test`（覆盖原文件）。
- AGENTS.md 禁改文件（`.github/` 除 docker.yml、注释含"原版源码"的顶层文件）不触碰。

# Decisions

- D1（用户已确认 Q1=A）: 改造方式采用静态 HTML 统一标记 + 样式重写（方案 A），不做 SCHEMA 数据驱动重构（方案 B）。理由：本项目表单项约 82 个但分 5 页静态组织，保存逻辑按 id 收集稳定运行；方案 B 需重写收集链路，风险与收益不成比例。
- D2（用户已确认 Q2=A）: 配置面板内 emoji 全部去除，label 走纯文字，学参考项目以小节标题左侧 3px 强调色竖条等细节建立层次；cfg-tab 的 emoji 图标同删。
- D3: hint 提升策略——title 中全部长说明提升为可见 hint（不保留 tooltip 形态）。依据用户诉求「次要说明、注意事项都要看到且精致」。
- D4: 参考项目定位为实现参考（见约束）。

# Open questions

（无——Q1、Q2 已由用户确认，详见 Decisions D1、D2。）
