# Brainstorm Summary

- Change: webui-scp-style-redesign
- Date: 2026-10-31

## 确认的技术方案

与 Open 阶段已批准的 design.md 完全一致（本次 Step 1c 确认提问未获答复，以其 Open 产物批准为准继续）：

- 整体重写 `index.html`：零依赖单文件 + go:embed + WebSocket 通道不变
- 设计体系：teal 主色 `#0ea5a0`、语义色、玻璃拟态、`color-mix`、暗色 `#121316/#18191b`、深浅主题持久化
- 三栏 Grid：左导航（任务运行/官方优选/非标优选/分析报告/设置）/中栏控制+日志/右栏配置；移动端单列
- 48px 圆形启停状态机 + 乐观 UI + toast；进度条阶段名+计数+shimmer；无总量阶段活动指示
- 终端风日志：级别着色、时间戳置灰、上浮动画、上滚暂停+回到底部、500 行环形缓冲（修复现有无上限）
- 分析：统计 chips + DC/地区排行 + 结果表操作，前端聚合不加后端消息
- 兼容：21 种 WS 请求/39 种响应语义全兼容；Android 桥保留；login.html 统一主题
- 服务端最小改动；CLI 弃用不删；不改任务管线/调度/上传/认证

## 关键取舍与风险

- 整体重写回归风险 → 以 protocol.go 消息清单逐项迁移核对，git 历史可回滚
- 现代浏览器目标（color-mix/Grid），关键容器纯色 fallback
- 不逐像素对照范本，以布局结构、状态机、语义色一致性为验收口径

## 测试策略

- `go build ./...` + `go test ./...` 全绿 + AGENTS.md 测试版构建
- 本地真实运行逐项走查 spec 17 个验收场景；Docker 方式最终验收

## Spec Patch

无——`web-console` 能力 delta spec 已完整覆盖。
