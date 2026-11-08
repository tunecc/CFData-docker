# Capability: web-config-panel（右栏配置面板统一设计系统）

归档后右栏配置面板（`.cfg-panel`，5 页：官方优选 / 非标优选 / 自动化 / 结果操作 / 设置）的完整行为与外观规格。设计手法学习 `subs-check-pro-webui@v1.8.2`（实现参考），行为约束以本 Spec 为准。

## 1. 布局骨架（不变式）

- 右栏结构保持：`.config-col` → `.cfg-panel` → `.cfg-tabs`（4 个 tab）+ 5 个 `.cfg-body` 面板；「官方优选」tab 仍按任务模式在 cfgPanelOfficial / cfgPanelNSB 间切换（`nzShowConfigPanel`，含滚动位置记忆）。
- 每页头部 `.cfg-panel-head` 保持：标题 + 保存按钮；保存入口 `saveTaskConfigToServer('official'|'nsb')` / `saveAutomationPage` / `saveSettingsPage` / `saveResultsPage` 不变。
- 所有表单控件的 DOM id 不变；`oninput` / `onclick` 入口不变；保存请求体格式不变。
- `.cfg-tabs` 单行不换行（`flex-wrap: nowrap`，溢出时允许横向滚动）。

## 2. 统一行组件 `.cfg-field`

配置面板内全部表单项使用统一的 `.cfg-field` 行结构，替代 `.option-item` / `.form-group` / `.modal-field` 三套标记（`.inline-check` 收编为开关行变体）：

- **普通行**（默认）：CSS Grid 双列 `minmax(0,1fr) minmax(0,1.2fr)`，label 列在左、控件列在右，垂直居中，`column-gap: 10px`。
  - 行结构：`.cfg-field > .cfg-label-text + .cfg-ctrl`，hint 为第三行。
  - hint（`.cfg-label-hint`）：跨两列的独立行，10.5–11px、muted 色、行高 ≤1.6；支持内联 `<code>` 示例（等宽小字号、可选中复制）。
  - 行间用 hairline 下边框分隔（`color-mix(var(--border) 45%, transparent)`），hover 整行轻微高亮。
  - 行内边距统一（约 `10px 8px` 量级），消除此前 `.option-item` 7px / `.form-group` 8px / `.cfg-body` 12px / `.section-box` 14px 的多套值。
- **全宽行** `.cfg-field.full-width`：单列布局，顺序为 label（含行内动作）→ hint → 控件；用于字段选择、输出顺序、测速网址等宽内容。
- **开关行** `.cfg-field--toggle`：`1fr auto` 双列，右侧为 checkbox 开关，剩余空间给左侧文本。
- 禁用/只读态字段统一弱化样式（opacity ≈0.4、pointer-events none）。

### 场景：官方优选页渲染统一行结构

页面加载后，官方优选页的 16 个配置项全部渲染为 `.cfg-field` 结构（含 full-width / toggle 变体），每项的 label 在界面上可见，无 `.option-item` 残留。验收：A1

## 3. 说明可见化

- 原藏于 `title` 属性的配置项说明全部提升为可见 `.cfg-label-hint`（实现时允许精简改写为 ≤2 行的短句，语义不变）。
- `readonly input` 当提示牌的用法清零：静态提示改为 hint 文本或状态徽标；「数据中心库」「扫描方式」等状态型 readonly 值改为不可编辑但样式明确区分的展示控件（非输入框外观或弱化输入框外观，带 aria 说明）。
- label 纯文字化：配置面板内 emoji 全部去除（含 cfg-tab 图标、分组头、按钮、说明文字）；层次感由字号/字重/小节竖条承担。

### 场景：长说明可见

「提前结束数量」「测速网址」「链路复验」等原 title 长说明项，在其控件下方可见对应 hint；对应控件 id 不变。验收：A2

### 场景：readonly 提示牌清零

配置面板 HTML 中 `input[readonly]` 作为说明载体的用法为 0 处；grep 校验。验收：A2

## 4. 行内提示组件 `.cfg-inline-hint`

- 统一三档：`lvl-ok`（绿）/ `lvl-info`（accent 中性）/ `lvl-warn`（橙警告），每档 = 12px SVG 图标 + 文字，背景/边框/文字三色均由 `color-mix(in srgb, var(--语义变量) N%, transparent)` 派生。
- JS 注入的动态提示（`app.js` 原 `cfg-field-hint` 逻辑，约 5361–5374 行）改用此组件，注入位置在对应字段行内（控件下方跨两列）。
- 静态注意事项（R2 / GitHub 上传组的 `.automation-hint` 长文、自动化页提示）改为 `lvl-info` / `lvl-warn` 行内提示或 hint 文本，不再出现裸内联样式的说明 div。

### 场景：R2 提示为组件化提示

R2 上传分组的注意事项以 `.cfg-inline-hint`（或 hint 文本）呈现，双主题下颜色来自语义变量。验收：A3

## 5. 分组与层次

- `.cfg-group` 保留折叠卡片形态（`cfg-group-head` 点击折叠行为不变），精修样式：head 更紧凑（约 `7px 10px`、12.5–13px/600）、左侧或标题前缀 3px accent 竖条（伪元素）、body 内边距与 `.cfg-field` 行统一。
- 双重标题清除：自动化页 `.section-box h3`「⏱️ 自动化」与设置页重复小节头删除，页头 `.cfg-panel-title` 为唯一页级标题。
- legacy 类在配置面板内清除：`.section-box` / `.control-title` / `.settings-panel` 不再被 5 页使用（左栏与其余页面不受影响，样式规则可保留）。
- 设置页两张卡片保持竖排；设置页底部 `.cfg-panel-footer` 保留。

### 场景：分组头精修

官方优选页 3 个分组（扫描参数/测速参数/上传与维护）的 head 均为紧凑单行 + accent 竖条样式，折叠交互与折叠状态记忆不回归。验收：A1

## 6. 视觉规范

- 字号体系：label 13px/500；hint 10.5–11px；input/select 桌面 13px（placeholder 12px）；移动端（≤760px）input 恢复 16px 防 iOS 聚焦缩放；小节/分组标题 12.5–13px/600。
- input/select 统一：`padding: 6px 10px`、`border-radius: 8px`、边框 1px `--border`、背景 `--input-bg`（新增该变量或等价 color-mix 派色）；高度约 30px。
- focus 统一公式：`border-color: var(--accent)`（或 accent-light），`box-shadow: 0 0 0 2.5px color-mix(in srgb, var(--accent) 15%, transparent)`；全局 outline 处理保持可访问性（focus-visible 补偿）。
- 数字步进器不强制引入（保持原生 number input + 现有校验），但样式按统一公式。
- 间距走 4px 体系（4/8/12px 档位）；`--gap: 10px` 全局值不变。

## 7. 主题与变量

- cron 分段控件硬编码 hex（`#3b82f6` 等）改走主题语义变量或 color-mix 派色。
- `--warning` / `--danger` / `--idle` / `--prepare` 补充 `:root[data-theme='dark']` 暗色适配；新增输入框背景变量（如 `--input-bg`）双主题各有值。
- 配置面板内不再使用 `--text-secondary` / `--text-main` / `--border-color` 等旧别名（styles.css 中别名定义可保留供旧代码过渡，但 cfg 区域改用 `--muted` / `--fg` / `--border`）。

### 场景：双主题无硬编码残留

浅色与暗色主题下分别截图检查 5 页：无硬编码 hex 导致的对比度异常；grep cfg 区域样式无旧别名/硬编码 cron 色。验收：A4

## 8. 密度与切换行为

- 双列布局生效后，官方/非标页无需滚动即可见明显更多配置项（相对旧版行高 ~38–40px 收紧至 ~32px 量级）。
- 页面切换（tab 切换、面板切换）行为不变；`node --test`（`combined_refactor/web/test/`）全部通过；必要时新增/调整样式类断言测试。
- 本地起服务浏览器实测：5 页切换、滚动位置记忆、保存动作、双主题切换正常。验收：A5、A6

### 场景：保存链路回归

修改任一配置项（如官方页并发数）→ 点击保存 → 请求体中该字段值正确；与改造前收集逻辑一致（id 不变保证）。验收：A2、A5
