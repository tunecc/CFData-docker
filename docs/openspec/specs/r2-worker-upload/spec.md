# r2-worker-upload Specification

## Purpose
通过 Cloudflare Worker 兼容接口把扫描结果写入 R2 存储，供 Sub-Store 等下游按 URL 读取，并在网络波动时自动重试、失败不破坏已有数据。

## Requirements

### Requirement: Worker 上传协议

配置 `worker-url` 与 `worker-token` 后，系统 SHALL 以 `POST {worker-url}/storage?token={worker-token}`、`Content-Type: application/json`、请求体 `{"filename":..., "value":...}` 的形式上传内容。`worker-url` MUST 为 http 或 https 地址。配置缺失或 URL 非法时系统 SHALL 跳过上传并记录明确错误，不阻塞扫描与导出流程。

#### Scenario: 成功上传

- **WHEN** 已配置有效的 worker-url、worker-token、文件名与内容
- **THEN** 系统向 Worker 的 `/storage` 端点发送 POST 请求，服务端返回 2xx 后记录上传成功

#### Scenario: 配置缺失时跳过

- **WHEN** worker-url 或 worker-token 为空且启用了 R2 上传
- **THEN** 系统记录配置不完整错误并跳过上传，扫描与本地导出仍正常完成

#### Scenario: URL 非法时跳过

- **WHEN** worker-url 不是有效的 http/https 地址
- **THEN** 系统记录配置错误并跳过上传，不发起网络请求

#### Scenario: 令牌不出现在日志

- **WHEN** 上传成功或失败
- **THEN** 日志与调试输出不包含 worker-token 明文

### Requirement: 重试与超时

单次上传失败时系统 SHALL 最多尝试 3 次，每次间隔 2 秒；单次请求 SHALL 使用 30 秒超时。鉴权失败（HTTP 401/403）SHALL NOT 重试。全部尝试失败后系统 SHALL 返回错误并记录最后一次失败原因。

#### Scenario: 前两次失败后成功

- **WHEN** 第 1、2 次上传失败而第 3 次成功
- **THEN** 系统记录最终上传成功，不再继续重试

#### Scenario: 三次均失败

- **WHEN** 连续 3 次上传均失败
- **THEN** 系统返回失败结果，记录最后一次错误，并保留 R2 上的旧内容

#### Scenario: 鉴权失败不重试

- **WHEN** Worker 返回 HTTP 401 或 403
- **THEN** 系统不再重试，直接记录配置或鉴权错误

### Requirement: 上传内容与空结果保护

上传内容 SHALL 来自当前导出结果（默认为 Sub-Store 目标列表）。当导出结果为空或上传最终失败时，系统 SHALL NOT 覆盖 R2 上的既有文件。

#### Scenario: 空结果不上传

- **WHEN** 本次扫描没有产生可导出的结果
- **THEN** 系统跳过 R2 上传，R2 上的旧文件保持不变

#### Scenario: 上传失败保留旧文件

- **WHEN** 本次上传在重试后仍失败
- **THEN** R2 上的旧文件保持原样，系统记录失败日志

### Requirement: 管理面代理

R2 上传请求 SHALL 遵循现有管理面代理配置；未配置代理时使用系统环境变量代理。扫描与测速流量 SHALL NOT 受该代理影响。

#### Scenario: 配置代理后走代理

- **WHEN** 管理面代理配置为 socks5/http 地址并发起 R2 上传
- **THEN** 上传请求经该代理发出，扫描与测速仍保持直连
