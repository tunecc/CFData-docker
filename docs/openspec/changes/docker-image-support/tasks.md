# Tasks

## 1. 镜像构建基础

- [x] 1.1 新增根目录多阶段 `Dockerfile`：Go 1.26 构建阶段按 `TARGETARCH` 交叉编译 `combined_refactor`，Alpine 运行阶段安装 CA/时区、创建 UID/GID 10001 用户；验证 `docker build -t cfdata-web:dev .` 成功且镜像可启动 <!-- comet-task:ec3b550a-da8c-45aa-9bb4-90eaa1b4e8b8 -->
- [x] 1.2 新增 `docker/docker-entrypoint.sh`：启动时比较并原子同步 `/app/cfdata` 到 `/data/cfdata`，再 `exec` 传递全部参数；验证镜像内 `/data/cfdata` 存在、重复启动不重复复制、传入 `-h` 能正常退出 <!-- comet-task:181cdb6d-dbbb-4117-823c-34b0d14e738d -->
- [x] 1.3 新增 `.dockerignore`：排除 `.git`、`.github`、`docs`、`app`、Gradle、根目录原版源码、Comet/Qoder 状态与运行期缓存；验证构建上下文不含这些目录且构建仍成功 <!-- comet-task:07224830-c719-4e19-8915-75a86651856d -->
- [x] 1.4 验证镜像基础运行契约：进程 UID 为 10001、默认监听 `0.0.0.0:13335`、`GET /favicon.png` 返回 200、日志中版本号与构建参数一致 <!-- comet-task:5968c313-223c-409f-be6a-767172d5c00f -->

## 2. 数据持久化与运行体验

- [x] 2.1 验证命名卷持久化：挂载卷启动容器，写入/修改配置并生成缓存文件，删除并重建容器后配置与缓存仍存在；验证未挂载卷时服务仍可启动 <!-- comet-task:6b370e12-0c87-4467-9ee6-1ccaf2267c68 -->
- [x] 2.2 配置镜像 `HEALTHCHECK` 与 `CFDATA_HEALTHCHECK_PORT` 覆盖逻辑；验证默认端口下状态为 healthy，停止服务后变为 unhealthy，改 `-port 8080` 且设置环境变量后仍为 healthy <!-- comet-task:b596cf2d-2ebc-4418-8350-0f6c5111b8e8 -->
- [x] 2.3 新增 `docker-compose.yml` 示例（`latest` 镜像、端口 13335、命名卷、`restart: unless-stopped`）；验证 `docker compose config` 通过且 `docker compose up -d` 后服务可访问 <!-- comet-task:a1dadd33-a714-4c56-87c5-6dcca82dfc3e -->
- [x] 2.4 验证多架构构建：执行 `docker buildx build --platform linux/amd64,linux/arm64`（不推送），确认两个平台均构建成功并能生成多架构清单 <!-- comet-task:40fce18e-30ff-43f0-9a30-b657e889c01a -->

## 3. GHCR 发布流水线

- [x] 3.1 新增 `.github/workflows/docker.yml`：`push.tags: v*` 与 `workflow_dispatch` 触发，声明 `contents: read`、`packages: write`，使用 `GITHUB_TOKEN` 登录 GHCR；验证 YAML 可解析且触发与权限声明符合设计 <!-- comet-task:82a3e4ba-2016-4aeb-b25d-454912409f6b -->
- [x] 3.2 实现 tag、`latest`、`edge`、`sha-<短哈希>` 标签与 `VERSION` 生成逻辑，预发布 tag 不更新 `latest`；用本地 shell 模拟 tag 推送与手动触发两种上下文，核对输出与规格一致 <!-- comet-task:507cdac1-b67d-4ba8-b9f1-940362604253 -->
- [x] 3.3 接入 `setup-qemu`、`setup-buildx`、`build-push-action`，构建 `linux/amd64,linux/arm64`，配置 GHA 缓存与按 ref 的 `concurrency`；验证工作流步骤顺序与参数完整 <!-- comet-task:a98e1a93-0f9f-4236-8784-4b728f2fb554 -->
- [x] 3.4 输出工作流摘要（镜像地址、标签、digest、版本、提交哈希），失败时步骤日志可定位阶段；验证摘要步骤使用实际构建输出而非硬编码 <!-- comet-task:04ef6beb-605d-4f01-a184-0d69558fdb09 -->
- [x] 3.5 核对镜像名小写处理与 GHCR 地址生成；用大小写混合的 owner 模拟脚本，确认最终镜像名全小写 <!-- comet-task:35eb4632-e512-4b06-93fd-cc884e1c8634 -->

## 4. 文档与仓库约定

- [x] 4.1 在 `README.md` 增加简短 Docker 小节并新增 `docs/docker.md`，覆盖拉取、运行、compose、认证、端口与健康检查、数据卷、升级、GHCR 可见性、手动构建覆盖 `latest` 的提醒；验证文档中的命令按原样可执行 <!-- comet-task:c741f9e3-2e36-4cb9-ad22-c559c1c2094d -->
- [x] 4.2 更新 `AGENTS.md`：为 `.github/workflows/docker.yml` 增加例外，其余 `.github/` 禁止规则保持不变；验证文字与范围一致 <!-- comet-task:70dda3d1-3aee-4132-a08c-c6c9eac47f56 -->
- [x] 4.3 核对 GPL 合规与 OCI 标签：镜像内包含 `LICENSE`，标签含源码地址、revision、version、licenses；验证 `docker inspect` 输出 <!-- comet-task:3a7d3af6-8ccd-419d-a9af-136e090f436c -->

## 5. 集成验证

- [x] 5.1 执行端到端本地验证：构建镜像 → 启动 → 健康检查 → 认证参数 → 卷持久化 → 版本输出 → 多架构清单，全部通过 <!-- comet-task:f63d6308-dea4-4b30-b0d2-2c15dbc27bfa -->
- [x] 5.2 核对 `.github/workflows/build.yml` 未改动，现有二进制/APK Release 触发与产物语义保持不变 <!-- comet-task:25e460e3-4bc9-46c4-a964-c9bca06e937b -->
- [x] 5.3 静态检查全部新增 YAML 与脚本：YAML 解析通过、脚本 `sh -n` 通过、无未替换占位符 <!-- comet-task:50f141cd-5a2b-4ed7-8651-3808c06ef95b -->

## Workflow follow-up

- 合并后在 Actions 页面手动运行 Docker 工作流，确认 GHCR 出现 `latest`、`edge`、`sha-*`，并将包可见性设为 public。
- 推送 `v*` tag，确认版本 tag、`latest` 与现有 Release 流程并行正常。
- 完成后归档本 change。
