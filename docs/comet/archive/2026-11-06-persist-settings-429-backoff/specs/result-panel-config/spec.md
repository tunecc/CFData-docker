# 结果面板设置服务器持久化

## Purpose

结果操作页面的显示设置不再只依赖浏览器 localStorage：服务器保存最后一次设置，换访问地址（局域网 IP、localhost、域名、端口）、换浏览器或重建容器后仍能恢复，同时保留 localStorage 作为本地缓存与故障兜底。

## Requirements

### Requirement: 服务器设置存储

系统 SHALL 在数据目录（与 `cfdata-config.json` 同级，容器内为 `/data`）维护 `result-panel.json`，保存结果面板设置的完整键集合：`source`、`format`、`fileName`、`startRow`、`endRow`、`ipType`、`qualified`、`latency`、`loss`、`fieldOrder`、`customFields`、`separatorPreset`、`separatorCustom`、`ipv6Brackets`。

读取 MUST 忽略未知键与非字符串的 `fieldOrder` 元素；`fieldOrder` 只保留字符串数组。文件损坏、缺失或不可读时 MUST 按空配置处理，不得阻止程序启动。

写入 MUST 原子化（临时文件 + rename），并带有 schema 版本字段，便于后续扩展。

### Requirement: 保存与恢复

WebSocket 连接建立后，前端 SHALL 请求服务器设置；收到后 MUST 以服务器值覆盖本地缓存并应用，服务器无记录而本地已有设置时 MUST 把本地值上送完成迁移。

结果面板设置发生改动（字段勾选、顺序、自定义字段、来源、格式、行范围、筛选、分隔符、IPv6）时，前端 MUST 立即写入 localStorage，并 SHALL 防抖（约 500ms）同步到服务器；右上角「保存」MUST 立即同步服务器。

服务器写入失败或连接未就绪时，界面 MUST 给出可见提示，MUST NOT 显示保存成功；localStorage 仍 MUST 保留当前设置供本地恢复。

### Requirement: 兼容与回归

localStorage 键 `cfdata-result-panel` 与其结构 MUST 保持不变，已有本地设置 MUST 能继续恢复。

本次改动 MUST NOT 改变任务参数、调度、通知、上传账号、R2 配置的存储格式与接口，也不改变结果导出内容生成与字段清单。
