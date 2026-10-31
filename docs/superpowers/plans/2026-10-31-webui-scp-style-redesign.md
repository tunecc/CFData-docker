---
change: webui-scp-style-redesign
design-doc: docs/openspec/changes/webui-scp-style-redesign/design.md
base-ref: 0340a5cdbec47ae03e70bb88b044633c8d590249
archived-with: 2026-10-31-webui-scp-style-redesign
---

<!-- comet-task-authority: docs/openspec/changes/webui-scp-style-redesign/tasks.md -->

# Plan: Web 界面照 subs-check-pro 大改

实现策略：新写 `index.html` 替换旧文件（旧版在 git 历史可回滚）。JS 层以旧文件的 `handleMessage` 分发与各处理器为迁移底本（21 请求/39 响应语义不变），DOM 结构与样式全部按新设计体系重写。前端任务无 JS 测试设施，TDD 的 RED/GREEN 适用于其中可测的 Go 侧改动（本计划无 Go 行为变更，编译即验证）；界面任务以构建 + 本地运行走查 spec 场景验收。

## 1. 设计体系与布局骨架

<!-- comet-task-ref:1b52202a-fb8c-46a3-be34-f65707c68581 -->
- **1.1** 新 index.html 基础层：`:root` 设计变量（teal/语义色/玻璃拟态/暗色）+ `data-theme` 切换持久化（localStorage `cfdata_theme`，默认跟随 `prefers-color-scheme`）。
  验收：本地运行，切主题刷新保持；无手选时跟随系统。

<!-- comet-task-ref:c91a1e02-0e46-401c-b920-307f6532cfa6 -->
- **1.2** 三栏 Grid 骨架：左导航（品牌/导航项/主题切换/退出）、中栏（控制卡+日志卡）、右栏配置卡；窄屏单列 + 安全区。
  验收：桌面/窄屏无横向溢出，导航切换视图（任务运行/官方/非标/分析/设置）。

<!-- comet-task-ref:fa5bfd47-0422-4aca-9a4a-11f40c8505c8 -->
- **1.3** WS 框架迁移：`wsSend`/`connect`/重连/`handleMessage` 分发表覆盖 39 种响应，逐项保留语义；`init_config`/`version_info` 正常渲染。
  验收：登录后连接建立，版本与初始化配置显示；分发表与 protocol.go 清单核对。

<!-- comet-task-ref:69307b1f-3ece-4186-b42e-b38863e6ec27 -->
- **1.4** Android 桥（`window.CFDataAndroid`）调用路径迁移。
  验收：桥存在/不存在两种情形均不报错。

## 2. 任务运行视图

<!-- comet-task-ref:983b45e6-354b-4f11-906c-c569814decd8 -->
- **2.1** 圆形启停按钮状态机 + 乐观 UI + toast + 连点锁；状态由 scan_progress/test_progress/task_complete/task_stopped/backgroud 消息回填。
  验收：启动→运行→停止全程状态正确。

<!-- comet-task-ref:9f6d0f3b-40e4-452d-92e6-3a76cf8cf076 -->
- **2.2** 官方/非标参数表单迁入分组卡片：全部现有字段（IP 类型、端口组、并发、延迟、DC 库、早停、测速源、链路复验、输出文件名等）与现有消息字段收发一致。
  验收：两种参数各跑一次任务行为一致（消息层字段核对 + 本地运行）。

<!-- comet-task-ref:e3a87f02-ea6a-47ca-bf1e-d3e4ef9578cb -->
- **2.3** 进度条组件 + 后台任务确认卡与跟随入口重绘（run_in_background/follow/get_background_task_status 语义不变）。
  验收：扫描中进度实时更新；转后台/跟随流程正常。

## 3. 日志区

<!-- comet-task-ref:bcb08df1-be03-4dcf-af18-273a4bd48f96 -->
- **3.1** 终端风日志组件：级别着色、时间戳置灰、URL/数字高亮、ANSI 解析、上浮动画、500 行环形缓冲。
  验收：三种级别行着色正确；超量日志页面不卡顿。

<!-- comet-task-ref:12709680-6695-488b-8c76-e886d3efac63 -->
- **3.2** 滚动行为：上滚暂停 + 「回到底部」胶囊、平滑滚回、框选不打断。
  验收：长日志下交互正常。

## 4. 结果与分析视图

<!-- comet-task-ref:986f84a0-2d00-490a-924b-11d3afc60be6 -->
- **4.1** 统计 chips + DC/地区排行（rank 徽章 + 百分比条 + 语义色三档），数据源为当前任务结果行聚合。
  验收：完成后数值与结果行统计一致。

<!-- comet-task-ref:cfb48e93-28fd-4144-b4bb-77f663f69325 -->
- **4.2** 结果明细表迁移：排序、筛选、一键复制、详细测试视图（test_result/test_complete 渲染）。
  验收：四类操作与现行为一致。

<!-- comet-task-ref:9eb0936c-61c5-4271-9f84-c6163369687e -->
- **4.3** 导出与上传入口：结果操作弹窗（格式/行范围/字段/分隔符）、GitHub 账户管理+上传（WebCrypto 加密存 localStorage）、edgetunnel 上传、R2 自动上传选择（含别名模板与地址反馈字段）。
  验收：各路径消息语义与现行为一致。

## 5. 配置编辑

<!-- comet-task-ref:0013a895-0ef3-46a7-8628-8d239b87a3a6 -->
- **5.1** 右栏配置卡：自动调度 + R2 上传（含 r2AliasFormat、生效地址反馈）+ 管理面代理分组表单，收发与 set/get_schedule_config、get/set_management_proxy 兼容。
  验收：保存后重载回显一致，调度器热重载。

<!-- comet-task-ref:17e0bfd1-ff37-4943-b694-665e371d595b -->
- **5.2** 设置视图：主题、恢复默认配置（reset_all_config）、配置保存/加载/同步（localStorage 与 get_config/check_proxy_country）。
  验收：各设置项功能与现行为一致。

## 6. 收尾与集成

<!-- comet-task-ref:93597278-7719-4787-8287-2d514ee24d32 -->
- **6.1** login.html 统一设计变量与主题（含 `?error=1`）。
  验收：登录页两种主题渲染正常、登录流程不受影响。

<!-- comet-task-ref:8fbd20fc-23de-434b-943c-c9b92d0a3dd2 -->
- **6.2** 全量构建与测试：`go build ./...`、`go vet`、`go test ./...` 全绿；按 AGENTS.md 产出 `release_assets/cfdata-test`。
  验收：命令退出码 0，产物更新。

<!-- comet-task-ref:fe654106-525a-4e67-8527-45d9d19a18fb -->
- **6.3** 端到端验收：本地真实运行逐项走查 spec 17 场景；Docker 方式构建运行冒烟。
  验收：场景逐项记录，偏差如实记录。

## 回退

新旧文件均为单提交系列，`git revert` 可整体回退；服务端仅在消息字段不足时小幅补充，同样可回退。
