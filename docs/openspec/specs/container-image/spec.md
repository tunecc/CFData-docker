# container-image Specification

## Purpose
定义 CFData-WEB 容器镜像的构建与运行契约，使镜像可在 amd64/arm64 主机上以非 root 方式稳定运行 Web 服务，并在容器重建后保留配置与缓存数据。

## Requirements

### Requirement: 可复现的镜像构建

系统 MUST 能从仓库源码使用标准 Docker 构建命令生成可运行镜像，构建过程不依赖仓库外的本地构建产物，也不要求构建机预装 Go 工具链。

#### Scenario: 从干净克隆构建镜像

- **WHEN** 在干净克隆的仓库根目录执行标准 Docker 构建命令
- **THEN** 构建成功并生成可用于启动 Web 服务的镜像

#### Scenario: 构建失败时给出明确错误

- **WHEN** 源码编译失败或构建依赖不可用
- **THEN** 构建以非零状态结束，且日志包含失败阶段与原因

### Requirement: 多架构镜像

镜像 MUST 同时提供 linux/amd64 与 linux/arm64 两种架构，并在对应架构主机上以原生方式运行。

#### Scenario: 在 amd64 主机运行

- **WHEN** 在 linux/amd64 主机拉取并运行镜像
- **THEN** 容器正常启动且 Web 服务可访问

#### Scenario: 在 arm64 主机运行

- **WHEN** 在 linux/arm64 主机拉取并运行镜像
- **THEN** 容器正常启动且 Web 服务可访问

### Requirement: 默认 Web 运行模式

容器 MUST 默认以 Web 模式启动，监听容器内所有地址的 13335 端口，并允许通过附加启动参数覆盖端口、监听地址、Web 认证等运行选项。

#### Scenario: 默认启动

- **WHEN** 启动容器且不附加任何参数
- **THEN** Web 服务在容器内 `0.0.0.0:13335` 监听，访问 `/` 返回 Web 界面

#### Scenario: 覆盖端口与认证

- **WHEN** 启动容器时传入 `-port 8080 -user admin -password secret`
- **THEN** Web 服务监听 8080 端口，且未认证请求被引导至登录页

### Requirement: 数据持久化

容器 MUST 将配置文件、IP 列表、位置信息、ASN 数据库与调试日志写入可挂载的数据目录；挂载卷后，这些数据在容器删除或重建后仍保留。

#### Scenario: 重建后保留配置

- **WHEN** 挂载数据卷启动容器，修改配置后删除并重新创建容器
- **THEN** 新容器读取到修改后的配置

#### Scenario: 重建后保留缓存

- **WHEN** 挂载数据卷完成一次扫描与测速后重新创建容器
- **THEN** 已有 IP 列表、位置信息与 ASN 数据库无需重新下载即可使用

#### Scenario: 未挂载卷时仍可运行

- **WHEN** 启动容器时不挂载数据卷
- **THEN** Web 服务仍能启动并完成基本功能，数据仅保存在容器可写层

### Requirement: 非 root 运行与健康检查

容器 MUST 以非 root 用户运行；镜像 MUST 提供健康检查，用于判断 Web 服务是否可用。

#### Scenario: 以非 root 用户运行

- **WHEN** 检查容器内运行进程的用户
- **THEN** 进程 UID 不为 0

#### Scenario: 服务正常时健康

- **WHEN** Web 服务已启动并响应健康检查请求
- **THEN** Docker 健康状态为 healthy

#### Scenario: 服务不可用时 unhealthy

- **WHEN** Web 进程异常退出或端口不可达
- **THEN** Docker 健康状态为 unhealthy

### Requirement: 镜像版本可追溯

镜像 MUST 在应用内展示构建版本，并通过 OCI 标签记录版本、提交修订与源码地址；基于 tag 构建的版本 MUST 与 tag 一致。

#### Scenario: tag 构建版本一致

- **WHEN** 使用 tag `v1.2.3` 构建并启动镜像
- **THEN** 应用内版本输出为 `v1.2.3`，镜像标签包含相同版本与源码信息

### Requirement: 精简运行镜像

运行镜像 MUST 不包含 Go 工具链、源码与构建缓存，仅包含运行所需的二进制、证书与基础系统文件。

#### Scenario: 运行镜像不含构建工具

- **WHEN** 检查运行镜像中的 Go 工具链或源码目录
- **THEN** 这些内容不存在

### Requirement: HTTPS 信任

容器 MUST 能对公网 HTTPS 服务完成证书校验，用于测速地址探测、版本检查与数据下载。

#### Scenario: 访问 HTTPS 测速地址

- **WHEN** 容器内程序访问任一内置 HTTPS 测速地址
- **THEN** TLS 握手与证书校验成功，不出现证书错误
