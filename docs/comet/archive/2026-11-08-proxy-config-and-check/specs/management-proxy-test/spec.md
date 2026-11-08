# Capability: management-proxy-test

## Purpose

定义 CFData-WEB 管理面代理连通性测试的完整行为：用户可在 Web 设置页对当前填写或已生效的管理面代理执行一键连通性测试，获得可用性、出口 IP、出口地区与延迟；测试不改变任何已保存配置与全局代理状态。管理面代理的设置、持久化与优先级行为（程序配置 > 系统环境变量回退）维持现状，本 capability 只覆盖「测试」能力。

## Requirements

### Requirement: 代理连通性测试请求

系统 SHALL 提供 WS 消息 `test_management_proxy`：载荷 `value` 可选，为代理地址字符串。`value` 非空时，系统对该代理地址做一次性测试（复用 `parseManagementProxy` 校验，支持 socks5://、socks5h://、http://、https://），不修改全局 `managementProxyRaw`/`managementProxyURL`、配置文件或任何已保存状态；`value` 为空或缺失时，系统对当前生效出口测试，目标依次为：程序配置代理 → 系统环境变量代理（HTTPS_PROXY/https_proxy/HTTP_PROXY/http_proxy/ALL_PROXY/all_proxy，与 `managementProxyInUse` 回退链一致）→ 直连。测试超时 SHALL 为 10 秒。

### Requirement: 测试结果上报

系统 SHALL 以 WS 消息 `proxy_test_result` 返回测试结论，载荷至少包含：`ok`（是否成功）、`error`（失败原因，成功时为空）、`exitIP`（出口 IP，取自 Cloudflare trace 的 `ip=`）、`country`（出口地区，取自 `loc=`，大写）、`latencyMs`（请求耗时毫秒）、`proxyLabel`（脱敏代理地址 `scheme://host`）、`source`（来源标签：「程序配置」「系统环境变量」或「直连」）。测试失败时 `ok` 为 false 且 `error` 说明具体原因（拨号失败、超时、非 2xx 响应、trace 缺字段等）。

### Requirement: 认证信息脱敏

测试请求与结果、日志、错误信息 SHALL NOT 出现代理 URL 中的用户名与密码；展示仅使用 `maskProxyURL` 的 `scheme://host` 形式。

### Requirement: 设置页测试交互

设置页「管理面代理」区块 SHALL 提供「测试」按钮：点击后发送 `test_management_proxy`（携带输入框当前值），等待期间按钮 SHALL 防重复触发；收到 `proxy_test_result` 后就地展示成功（含出口 IP、地区、延迟）或失败（含原因）。输入框为空时按空 `value` 语义测试当前生效出口。

## Scenarios

### Scenario: 已配置代理且可用时测试成功

当程序配置代理可用时，用户在设置页点击「测试」：约 10 秒内显示成功结果，包含出口 IP、出口地区与延迟毫秒数；已保存代理配置与全局状态不变。
验收：A1

### Scenario: 代理不可用时测试失败

当代理地址不可达、超时或认证失败时：显示失败结果与具体原因；已保存代理配置与全局状态不变。
验收：A2

### Scenario: 测试目标跟随输入框与生效链

输入框有未保存值时，测试针对该值进行，不落盘、不改变全局生效代理；输入框为空时，按生效链测试（程序配置 → 环境变量 → 直连）。
验收：A3

### Scenario: 环境变量代理回退测试

未配置程序代理、存在环境变量代理时，测试经环境变量代理进行，结果 source 标注「系统环境变量」。
验收：A4

### Scenario: 直连兜底测试

无任何代理配置时，测试走直连，结果标注直连出口。
验收：A5

### Scenario: 认证信息不泄露

任何测试请求、结果展示、日志与错误信息中只出现 `scheme://host`，不含用户名/密码。
验收：A6

### Scenario: 保存链路回归

保存代理后：状态 chip 更新为「已启用: scheme://host」、输入框清空、出现保存成功提示（现有行为不变）。
验收：A7
