# Docker 使用说明

镜像地址：`ghcr.io/tunecc/cfdata-web`，支持 `linux/amd64` 与 `linux/arm64`。

## 标签策略

| 触发方式 | 生成的镜像标签 | 说明 |
| -------- | -------------- | ---- |
| 推送稳定 tag（如 `v1.2.3`） | `v1.2.3`、`latest`、`sha-<短哈希>` | 三个标签指向同一多架构镜像 |
| 推送预发布 tag（如 `v1.3.0-beta.1`） | `v1.3.0-beta.1`、`sha-<短哈希>` | 不更新 `latest` |
| Actions 页面手动触发 | `latest`、`edge`、`sha-<短哈希>` | 使用所选分支最新提交；不写入任何 `v*` 标签 |

`latest` 指向最近一次完成的稳定 tag 发布或手动构建；并发发布时以最后完成的运行为准。需要可复现的稳定版本时请固定使用 `v*` 版本标签。

## 拉取与运行

```bash
docker pull ghcr.io/tunecc/cfdata-web:latest
docker run -d --name cfdata \
  -p 13335:13335 \
  -v cfdata-data:/data \
  --restart unless-stopped \
  ghcr.io/tunecc/cfdata-web:latest
```

浏览器打开 `http://localhost:13335` 即可使用。容器默认以 Web 模式启动，监听 `0.0.0.0:13335`。

以 CLI 交互菜单运行（需要 TTY）：

```bash
docker run --rm -it -v cfdata-data:/data ghcr.io/tunecc/cfdata-web:latest -cli
```

## Docker Compose

仓库根目录提供 `docker-compose.yml`，数据目录挂载到当前目录的 `./data`：

```bash
docker compose up -d
docker compose logs -f
docker compose down
```

Docker 会创建 `./data`，容器启动时会自动修正其属主，无需手动 `chown`。数据文件在宿主机上默认属于 UID/GID `10001`；希望归属你自己的账号时，在 compose 中设置 `PUID`/`PGID`：

```yaml
environment:
  PUID: 501   # 宿主机 id -u
  PGID: 20    # 宿主机 id -g
```

## Web 认证

通过启动参数开启认证（需同时提供用户名和密码）：

```bash
docker run -d --name cfdata \
  -p 13335:13335 \
  -v cfdata-data:/data \
  ghcr.io/tunecc/cfdata-web:latest \
  -user admin -password 'change-me'
```

`docker-compose.yml` 中提供了对应的注释示例。

## 端口与健康检查

镜像内置健康检查，默认请求 `http://127.0.0.1:13335/favicon.png`。修改程序监听端口时，必须同步设置 `CFDATA_HEALTHCHECK_PORT`，否则容器会显示 unhealthy；如果通过 `-host` 绑定了非回环地址，还需设置 `CFDATA_HEALTHCHECK_HOST` 指向该地址：

```bash
docker run -d --name cfdata \
  -p 8080:8080 \
  -e CFDATA_HEALTHCHECK_PORT=8080 \
  -v cfdata-data:/data \
  ghcr.io/tunecc/cfdata-web:latest \
  -port 8080
```

查看健康状态：

```bash
docker inspect --format '{{.State.Health.Status}}' cfdata
```

## 数据持久化

容器的配置、IP 列表、位置信息、ASN 数据库与调试日志统一写入 `/data`。镜像启动时会把内置二进制同步到 `/data/cfdata`，因此配置与 `.bak`、迁移备份都留在数据卷内，镜像升级后卷内二进制会自动更新。

- 宿主机目录（Compose 默认挂载 `./data`）：`-v ./data:/data`，入口会自动把目录属主修正为 `10001`（或 `PUID`/`PGID` 指定的用户），无需手动 `chown`。
- 命名卷：`-v cfdata-data:/data`，属主自动继承镜像内的 `10001:10001`，无需手工授权。

入口脚本以 root 启动仅用于修正数据目录属主，随后立即降权，应用进程始终以非 root 用户运行。若运行环境禁止容器以 root 启动（如 Kubernetes `runAsNonRoot`），可设置 `user: "10001:10001"`，并确保挂载目录已可写。

同一个数据卷只应运行一个容器，避免多个实例同时修改配置。

## 自动调度

调度在容器内的应用进程中运行，不使用 systemd。容器以 Web 模式启动后会自动读取 `/data/cfdata-config.json` 中的调度配置：

```json
{
  "config": {
    "schedule-enabled": true,
    "cron-expression": "0 4 * * *",
    "check-interval": 720
  }
}
```

- `cron-expression` 为 5 段标准 Cron，有效时优先；留空或无效时使用 `check-interval`（分钟）。
- 也可在 Web 右上角设置菜单的「自动调度 / R2 上传」面板中开关和编辑，保存后立即热更新。
- 调度扫描完成后固定导出 Sub-Store 目标列表并触发已启用的 R2 上传；空结果跳过上传，失败只记录日志。
- 需要长期运行请使用 `--restart unless-stopped`（Compose 示例已包含），并确保容器只有一个实例使用该数据卷。
- 升级前已用宿主机 systemd `cfdata-schedule.timer` 调度的，请先停用旧 timer 并清理 unit，避免与容器内调度重复执行。

## 升级

```bash
docker pull ghcr.io/tunecc/cfdata-web:latest
docker rm -f cfdata
docker run -d --name cfdata \
  -p 13335:13335 \
  -v cfdata-data:/data \
  --restart unless-stopped \
  ghcr.io/tunecc/cfdata-web:latest
```

新版本启动时会自动用镜像内二进制替换卷内旧二进制，`/data` 中的配置与缓存保持不变。使用 Compose 时执行 `docker compose pull && docker compose up -d`。

## 从源码构建

在仓库根目录执行：

```bash
docker build \
  --build-arg VERSION=dev \
  --build-arg REVISION=$(git rev-parse HEAD) \
  -t cfdata-web:dev .
docker run --rm -p 13335:13335 cfdata-web:dev
```

传入 `REVISION` 后，启动日志与 Web 界面会显示 `sha-<短哈希>`，与 GHCR 的 `sha-<短哈希>` 镜像标签一致，便于确认实际运行版本；未传时显示 `dev`。指定发布版本号时同样建议带上提交：

```bash
docker build \
  --build-arg VERSION=v1.2.3 \
  --build-arg REVISION=$(git rev-parse HEAD) \
  -t cfdata-web:v1.2.3 .
```

此时界面显示 `v1.2.3 (sha-<短哈希>)`。

同时构建两种架构（需要 buildx）：

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=v1.2.3 \
  --build-arg REVISION=$(git rev-parse HEAD) \
  -t cfdata-web:v1.2.3 \
  --output type=oci,dest=/tmp/cfdata-multi.tar .
```

构建阶段使用 `golang:1.26-bookworm` 交叉编译，运行镜像基于 `alpine:3.22`，不包含 Go 工具链与源码。

## GHCR 包可见性

GHCR 包首次发布后默认是私有可见性。首次构建成功后，仓库所有者需要在 GitHub 的 **Packages → cfdata-web → Package settings → Change visibility** 中改为 public，其他人才能匿名拉取。

## 手动构建提醒

在 Actions 页面手动触发 Docker 工作流时，会使用所选分支的最新代码构建，并把 `latest` 指向这次构建。请不要在未经验证的功能分支上手动触发发布，测试用途请使用 `edge` 或 `sha-<短哈希>` 标签。

## 常见问题

- **容器启动即退出，日志提示 `/data` 不可写**：入口会自动修正属主；仍失败通常是因为目录只读、NFS root-squash，或用 `user:` 强制非 root 启动。改用 `PUID`/`PGID` 指定可写用户，或修正挂载目录属主。
- **健康状态为 unhealthy**：确认 `-port` 与 `CFDATA_HEALTHCHECK_PORT` 一致、`-host` 与 `CFDATA_HEALTHCHECK_HOST` 一致，且容器内配置的监听端口未被其他进程占用。
- **拉取镜像提示未授权**：包仍为私有，按“GHCR 包可见性”改为 public，或使用有 `read:packages` 权限的凭据登录 `ghcr.io`。
- **多架构构建很慢**：非本机架构的运行阶段依赖 QEMU 仿真；CI 中已启用 GHA 缓存，本地首次构建耗时属正常现象。
