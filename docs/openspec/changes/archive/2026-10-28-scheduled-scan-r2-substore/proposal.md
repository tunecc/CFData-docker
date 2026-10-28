# 提案

## 为什么（Why）

当前项目只有 systemd 定时方案（仅 Linux CLI + root，Docker 内不可用），扫描结果导出后需要手工上传，无法形成「定时扫描 → 产出目标列表 → Sub-Store 消费」的自动闭环。参考 subs-check-pro 的 Cron/间隔双调度与 R2 Worker 上传重试后，本项目需要内置调度与 R2 上传能力，并产出 Untitled.js 可直接消费的目标列表。

## 变更内容（What Changes）

- **BREAKING**：移除 systemd 定时方案（`combined_refactor/schedule.go` 的 unit 生成、启停菜单与相关 CLI 入口），改为进程内调度：
  - `cron-expression`：标准 5 段 Cron 表达式，配置有效时优先；
  - `check-interval`：检测间隔（分钟），Cron 为空时生效；`≤0` 按 2880、`<60` 按 60 处理；
  - Web、CLI、Docker 通用；间隔模式启动即执行一次后按间隔循环，Cron 模式按表达式等待；
  - 配置文件变更后热更新定时计划。
- 新增 R2 上传：兼容 subs-check-pro 的 Worker 协议（`POST {worker-url}/storage?token={worker-token}`，JSON `{filename,value}`），默认最多 3 次尝试、固定 2 秒间隔、30 秒超时，走管理面代理；Web 设置与 CLI 配置提供 worker-url、worker-token、上传文件名与开关。
- 新增 Sub-Store 目标列表导出格式：每行 `host[:port]#alias`，IPv6 带端口强制 `[v6]:port`，别名默认 `DC-城市`，保证每行都有别名，导出/上传行数上限可配置。
- 调度任务完成后自动执行「扫描 → 目标列表导出 → R2 上传」；空结果或上传失败不覆盖 R2 上的旧文件，失败记录日志且不影响服务运行。
- 优化并纳入 Untitled.js（Sub-Store operator）：超时与失败回退、别名去重、IPv6/带端口格式兼容；放入仓库 `substore/` 目录统一维护。
- 更新 README 与 Docker 文档；旧 systemd 用户迁移说明（程序不再创建 unit，已安装的 unit 由用户手动关闭）。

不拆分原因：三部分围绕同一条自动流水线（调度触发 → 导出 → 上传 → operator 消费），拆分会造成跨 change 的验收依赖；单 change 内仍按能力独立验收。

## 能力（Capabilities）

### 新增能力

- `scan-scheduling`：进程内扫描调度，覆盖 Cron 表达式与间隔分钟两套计划、启动行为、配置热更新与调度触发范围。
- `r2-worker-upload`：通过 Cloudflare Worker 协议上传结果到 R2，覆盖配置校验、重试与超时、代理、空结果保护与错误处理。
- `substore-integration`：Sub-Store 目标列表导出格式（别名、IPv6、行数限制、上传衔接）与配套 operator 脚本行为。

### 修改的能力

- 无（现有 `container-image`、`image-publishing` 的行为要求不变）。

## 范围与非目标

- 范围：`combined_refactor/` 后端与 Web UI、CLI 配置、导出与上传链路、仓库新增 `substore/` operator 脚本、README 与 Docker 文档。
- 非目标：R2 的 S3 直传、WeTest HTML 生成、扫描/测速核心算法变更、Cloudflare Worker 与 R2 桶的部署本身、自动删除系统级 systemd unit。

## 影响（Impact）

- 代码：`combined_refactor/schedule.go`（替换）、`cli.go`（配置与上传）、`server.go` 与 `index.html`（Web 设置与上传）、`tasks_official.go`/`tasks_nsb.go`（调度触发衔接）、新增 R2 上传与调度模块、新增 `substore/` 脚本。
- 依赖：引入 `robfig/cron/v3`（与 subs-check-pro 同款）。
- 配置：CLI JSON 配置新增 `cron-expression`、`check-interval` 与 R2 字段；Web 设置新增对应项。
- 文档：README、`docs/docker.md`。
