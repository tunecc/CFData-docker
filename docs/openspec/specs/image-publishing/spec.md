# image-publishing Specification

## Purpose
定义 CFData-WEB 镜像的发布契约，使仓库在推送版本 tag 时自动发布可追溯的 GHCR 镜像，并允许维护者从 Actions 页面以最新代码手动构建。

## Requirements

### Requirement: tag 推送自动发布

当推送匹配 `v*` 的 tag 时，发布流水线 MUST 构建 linux/amd64 与 linux/arm64 镜像并推送到 GHCR，标签 MUST 包含该 tag 名称与基于提交的 `sha-<短哈希>`；不含 `-` 的稳定版本 tag MUST 同时更新 `latest`。

#### Scenario: 推送稳定版本 tag

- **WHEN** 仓库推送 tag `v1.2.3`
- **THEN** GHCR 中出现 `v1.2.3`、`latest` 与 `sha-<短哈希>` 三个镜像标签，且均指向同一多架构镜像索引

#### Scenario: 预发布 tag 不更新 latest

- **WHEN** 仓库推送 tag `v1.3.0-beta.1`
- **THEN** GHCR 中出现 `v1.3.0-beta.1` 与 `sha-<短哈希>`，`latest` 保持不变

#### Scenario: tag 构建失败

- **WHEN** tag 推送后构建或测试失败
- **THEN** 工作流失败，且不更新 `latest` 与版本标签

### Requirement: 手动触发最新代码构建

维护者 MUST 能在 GitHub Actions 页面手动触发镜像构建，构建所选分支或提交的最新代码，无需修改仓库文件。

#### Scenario: 手动构建默认分支

- **WHEN** 在 Actions 页面选择默认分支并触发 Docker 工作流
- **THEN** 流水线使用该分支最新提交构建镜像并推送成功

#### Scenario: 手动构建指定分支

- **WHEN** 在 Actions 页面选择功能分支并触发
- **THEN** 流水线使用该分支最新提交构建，并在结果中记录提交哈希

### Requirement: 手动构建标签策略

手动构建 MUST 推送 `latest`、`edge` 与 `sha-<短哈希>` 标签，且 MUST NOT 覆盖任何 `v*` 版本标签。

#### Scenario: 手动构建标签

- **WHEN** 手动触发构建成功
- **THEN** `latest`、`edge` 与 `sha-<短哈希>` 指向本次构建镜像，所有 `v*` 版本标签保持不变

### Requirement: 免额外凭据发布

流水线 MUST 使用仓库自带的 `GITHUB_TOKEN` 完成 GHCR 登录与推送，不要求维护者配置额外的镜像仓库凭据。

#### Scenario: 未配置额外 secrets

- **WHEN** 仓库仅启用默认 `GITHUB_TOKEN` 且未配置 Docker Hub 等凭据
- **THEN** tag 与手动构建均能成功推送 GHCR

### Requirement: 构建来源可追溯

每次发布 MUST 记录源码仓库、提交修订与构建版本，并写入镜像 OCI 标签与工作流摘要，便于从镜像回溯到提交。

#### Scenario: 从镜像回溯提交

- **WHEN** 检查已发布镜像的 OCI 标签
- **THEN** 可读到源码地址、提交 SHA 与构建版本

### Requirement: 与现有发布流程共存

镜像发布 MUST NOT 影响现有 `build.yml` 的二进制、APK 构建与 GitHub Release 发布流程。

#### Scenario: tag 同时触发两条流水线

- **WHEN** 推送 `v*` tag
- **THEN** 二进制/APK Release 流程与镜像发布流程均正常完成，互不阻塞

### Requirement: 发布结果可见

流水线 MUST 在工作流摘要中列出本次发布的镜像地址、标签与提交哈希，失败时说明失败阶段。

#### Scenario: 成功运行后查看摘要

- **WHEN** 流水线运行成功
- **THEN** Actions 运行摘要包含完整镜像地址、标签列表与提交哈希

#### Scenario: 失败运行后查看摘要

- **WHEN** 流水线运行失败
- **THEN** Actions 运行摘要或失败步骤日志指出失败阶段
