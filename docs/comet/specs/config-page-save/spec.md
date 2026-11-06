# 参数配置页统一保存

## Purpose

右侧参数配置区的五个页面（官方优选/非标优选、自动化、结果操作、上传账号、设置）使用同一套页面头部与保存入口：标题在左，右上角一个「保存」按钮，保存该页面上所有可持久化的配置，并给出可见的保存结果。结果操作页的显示设置在改动后即时保存，刷新不再回到默认值。

## Requirements

### Requirement: 统一的页面头部与保存入口

五个配置页面 SHALL 使用同一 HTML 结构与样式：`cfg-panel-head` 内左侧为页面标题，右侧为一个「💾 保存」按钮。按钮文案与位置 MUST 一致，具体保存内容通过 `title` 说明。

官方优选与非标优选页面的保存 MUST 继续调用现有 `saveTaskConfigToServer(scope)` 写入服务器配置；按钮文案由「保存参数」统一为「保存」，行为不变。

保存动作 MUST 给出可见结果：成功与失败都要有提示或状态反馈，不得静默失败。

### Requirement: 结果操作页保存

结果操作页的右上角「保存」SHALL 把当前页面配置写入 localStorage 键 `cfdata-result-panel`：数据来源、文件格式、文件名、开始行、结束行、IP 类型筛选、合格筛选、延迟筛选、丢包筛选、字段顺序、自定义字段、TXT 分隔符预设与自定义值、IPv6 方括号开关。

设置改动（字段勾选、顺序调整、自定义字段、格式、来源、行范围、筛选、分隔符、IPv6 开关）MUST 即时保存，刷新页面后 MUST 完整恢复；点击保存按钮 MUST 再次落盘并提示成功。

该页同时承载 R2 上传设置：页面保存 MUST 一并提交 R2 配置（复用 `saveScheduleConfig()`）。原分区内的重复保存按钮 MUST 被移除。

恢复 MUST 保持现有优先级与容错：存储损坏或无记录时使用默认值，不阻塞页面初始化。既有 `fieldOrder` 中不在当前字段清单的键 MUST 被忽略。

### Requirement: 自动化页保存

自动化页右上角「保存」MUST 复用 `saveScheduleConfig()`，提交自动调度、Bark/Telegram 通知与 R2 字段，成功后按现有回显逻辑更新状态提示。页面底部重复的「保存自动化设置」按钮 MUST 被移除。

### Requirement: 上传账号页保存

上传账号页右上角「保存」MUST 分别处理 GitHub 与 edgetunnel 两个表单：

- 两个表单都为空时 MUST 跳过对应表单，不报错、不写入。
- 已填写且校验通过的账号 MUST 按现有加密存储逻辑保存。
- 已填写但校验失败时 MUST 给出与现有分区保存按钮一致的提示，并且 MUST NOT 写入无效账号；一个表单失败不影响另一个表单的保存。

### Requirement: 设置页保存

设置页右上角「保存」MUST 依次完成：保存当前表单参数到 localStorage（`saveConfigToLS`）、保存自动加载偏好（`saveAutoLoadPreference`）、在管理面代理输入框非空时提交代理保存；代理输入为空时 MUST 保持现有代理不变。

主题切换 MUST 保持即时生效；现有「加载保存配置」「从服务器同步配置」「删除保存配置」「清除代理」按钮 MUST 保持可用与原有行为。

### Requirement: 静态资源刷新

`/static/` 下的 JS/CSS 响应 MUST 带 `Cache-Control: no-cache`，使升级后端后浏览器刷新即可获得新的前端文件；HTML 与 API 的缓存策略 MUST 保持不变。
