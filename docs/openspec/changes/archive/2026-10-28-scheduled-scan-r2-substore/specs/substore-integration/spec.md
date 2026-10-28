# 规格增量

## Purpose

让扫描结果以 Sub-Store operator 可直接消费的目标列表形式产出，并保证配套 operator 脚本在真实网络与各种输入格式下稳定解析、去重与回退。

## ADDED Requirements

### Requirement: 目标列表导出格式

系统 SHALL 提供专用的 Sub-Store 目标列表导出/上传格式，每行格式为 `host[:port]#alias`，且每行 MUST 包含非空别名。IPv6 带端口时 MUST 输出 `[v6]:port` 形式。别名默认由数据中心与城市组成，相关字段缺失时 MUST 回退为至少包含数据中心或 IP 的非空值。

#### Scenario: IPv4 目标行

- **WHEN** 导出结果为 IPv4 `1.2.3.4`、端口 443、数据中心 LAX、城市洛杉矶
- **THEN** 输出行形如 `1.2.3.4:443#LAX-洛杉矶`

#### Scenario: IPv6 目标行

- **WHEN** 导出结果为 IPv6 `2606:4700::1`、端口 443
- **THEN** 输出行形如 `[2606:4700::1]:443#HKG-香港`，方括号不可省略

#### Scenario: 别名缺失时回退

- **WHEN** 城市字段为空但数据中心为 NRT
- **THEN** 输出的别名至少包含 `NRT`，不会输出空别名行

#### Scenario: 别名包含破坏分隔的字符

- **WHEN** 别名包含 `#`、换行或多余空白
- **THEN** 输出前清洗这些字符，仍保持单行 `host[:port]#alias` 结构
- **AND** 清洗后为空时回退为数据中心或 IP，不输出空别名

#### Scenario: 关闭 IPv6 括号不影响目标列表

- **WHEN** `outv6bracket` 为 false 且导出 IPv6 目标
- **THEN** 目标列表仍输出 `[v6]:port#alias`，方括号不可省略

### Requirement: 导出内容受筛选与行数约束

目标列表 SHALL 遵循现有导出筛选（合格结果、IP 类型）与行数上限配置；超出上限的行 MUST NOT 被导出或上传。

#### Scenario: 行数上限生效

- **WHEN** 目标列表格式配置行数上限为 20 且合格结果超过 20 行
- **THEN** 导出与上传的文件只包含前 20 行

### Requirement: 上传与调度衔接

启用 R2 上传时，目标列表 SHALL 作为默认上传内容，上传文件名可配置。调度完成后系统 SHALL 自动生成并按配置上传目标列表；未启用上传时仅执行本地导出。

#### Scenario: 调度后自动上传目标列表

- **WHEN** 调度触发完成且启用了 R2 上传
- **THEN** 上传到 R2 的内容为目标列表格式文本

#### Scenario: 未启用上传仅本地导出

- **WHEN** R2 上传未启用且调度触发完成
- **THEN** 系统仅写出本地导出文件，不发起网络上传

### Requirement: operator 解析兼容

仓库内维护的 Sub-Store operator 脚本 SHALL 同时支持纯文本 `host[:port]#alias` 与 WeTest HTML 表格输入。无效行 SHALL 被跳过而不是导致整体失败；某个 URL 获取失败时 SHALL 继续使用其余来源与静态目标。别名重复时 SHALL 通过后缀去重，确保不产生重名节点。

#### Scenario: 无效行被跳过

- **WHEN** 输入文本中包含缺少 `#` 或端口非法的行
- **THEN** 脚本忽略这些行，其余有效目标仍被生成

#### Scenario: URL 获取失败时回退

- **WHEN** 配置的某个远程 URL 超时或返回错误
- **THEN** 脚本记录失败并继续使用其他 URL 与静态目标

#### Scenario: 重复别名去重

- **WHEN** 多个目标使用相同别名
- **THEN** 脚本为重复项追加可区分的后缀，节点名称不重复

### Requirement: operator 网络与运行环境

operator 脚本 SHALL 使用可配置的请求超时，并对多个 URL 并发获取。运行环境既无 `$substore` 也无 `fetch` 时，脚本 SHALL 明确报错而不是静默返回空结果。

#### Scenario: 超时配置生效

- **WHEN** 远程 URL 响应时间超过配置的超时值
- **THEN** 该 URL 视为获取失败并按回退规则处理

#### Scenario: 无 HTTP 客户端时报错

- **WHEN** 运行环境不提供可用的 HTTP 客户端
- **THEN** 脚本抛出明确错误，说明无法发起请求
