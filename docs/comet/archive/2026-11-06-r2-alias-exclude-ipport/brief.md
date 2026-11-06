# 目标

修复 R2 / Sub-Store 目标列表里 `IP:PORT` 重复的问题：当前按字段与顺序生成的别名模板包含 `{ipport}`，导致上传内容变成 `172.67.76.64:443#172.67.76.64:443-HKG`。改为别名只由非 `IP:PORT` 字段组成，用户选择「数据中心」时输出 `172.67.76.64:443#HKG`。

# 范围

## 目录结构

### 修改

- `combined_refactor/web/app.js`：`buildR2AliasTemplate` 生成别名模板时排除 `ipport`，避免别名重复地址。
- `combined_refactor/cli.go`：新增别名模板净化函数（去掉 `{ipport}` 占位并清理多余分隔符），在 R2 / Sub-Store 内容生成前统一应用。
- `combined_refactor/scheduler.go`：保存 `r2AliasFormat` 时同样净化，已存配置在下次保存后不再含 `{ipport}`。
- `combined_refactor/r2_manual.go`、`combined_refactor/cli.go` 的 R2 上传路径：读取旧配置时应用净化，保证历史配置立即生效。
- `combined_refactor/index.html`：同步 R2 别名说明文案（IP:PORT 不参与别名；未选其他字段时不加别名）。
- 相关测试：`r2_alias_test.go` 增加 `{ipport}` 净化与分隔符清理用例。

### 不修改

- R2 上传的筛选规则、文件名、地址与 token。
- TXT/CSV 导出的字段拼接（`IP:PORT#字段` 保持不变）。
- Sub-Store 目标地址生成规则（endpoint 仍为 IP:PORT）。

## 现状事实

- `combined_refactor/r2_manual.go:20-25`：手动 R2 上传内容由 `formatSubstoreResultsWithAlias(rows, aliasTemplate)` 生成；`combined_refactor/cli.go:992` 的自动/CLI 上传同样使用该函数。
- `combined_refactor/cli.go:1081-1090`：输出格式为 `endpoint#alias`，`endpoint` 由 `substoreEndpoint` 生成，本身就是 `IP:PORT`。
- `combined_refactor/cli.go:1183-1197`：`applyR2AliasTemplate` 支持 `{dc} {city} {region} {dcCountry} {ip} {port} {ipport}` 占位符。
- `combined_refactor/web/app.js:4755-4760`：`buildR2AliasTemplate` 把 `resultFieldOrder` 中每个字段映射为占位符并用 TXT 分隔符拼接；字段顺序含 `ipport` 时模板带 `{ipport}`。
- 用户实例字段顺序为 `ipport, dc`，模板即 `{ipport}-{dc}`，上传结果为 `172.67.76.64:443#172.67.76.64:443-HKG`。
- `combined_refactor/cli.go:1200-1208`：模板为空或替换结果为空时回退默认别名（`dc-city` / `dc` / `city` / `loc` / `ip` / `ipport`）。

# 非目标

- 不改变 Sub-Store 目标地址（endpoint）的生成方式。
- 不允许在别名中重复地址；也不新增占位符。
- 不调整结果面板预览与导出（TXT 仍是 `IP:PORT#字段`）。
- 不改变 R2 上传时机与筛选规则。
- 不改变 Web / CLI 的「Sub-Store 目标列表」导出默认别名（如 `HKG-香港`）。

# 验收示例

- A1：字段顺序为 `IP:PORT, 数据中心`、分隔符 `-` 时，R2 上传内容每行为 `IP:PORT#HKG`，不再出现重复的 `IP:PORT-` 别名。
- A2：字段顺序为 `IP:PORT, 数据中心, 落地区域` 时，别名为 `HKG-HK`；顺序调整后别名顺序随之变化。
- A3：配置文件中已保存 `{ipport}-{dc}` 的旧模板，升级后无需手动保存即可按 `{dc}` 生成（服务端读取时净化）；界面保存后配置文件也写为净化后的模板。
- A4：字段只选 `IP:PORT` 时每行只输出地址（如 `172.67.76.64:443`），不带 `#` 与别名。
- A5：R2 自动上传（自动调度/CLI 管线）与 Web 手动上传使用同一别名规则，输出一致；Sub-Store 目标列表导出保持现有默认别名，不在本次范围。
- A6：`go build ./...`、`go test ./...` 与前端 Node 测试通过，现有 R2 别名与导出测试不回归。

# 约束与不变量

- 别名中不得再出现 `IP:PORT` 的重复值；`{ipport}` 占位符在净化后不再生效；净化后为空的模板按“无别名”处理（只输出地址）。
- 净化必须保留其余占位符与字面量，并清理残留的分隔符（首尾与连续重复），不产生 `-HKG`、`HKG-` 或 `HKG--HK`。
- 已有 R2 配置的筛选、地址、token、文件名保持不变。
- 别名长度限制（≤200 字符）与禁止 `#` 的校验保持。

# 决策

- D1（实现选择）：前端生成模板时排除 `ipport`。
- D2（实现选择）：服务端在读取与保存 `r2AliasFormat` 时净化 `{ipport}`，兼容已保存的旧配置。
- D3（2026-11-06，用户确认）：只选择 `IP:PORT` 时每行只输出地址，不带 `#` 与别名。
- D4（实现选择）：前端在无其他字段时发送显式“无别名”标记 `{none}`，与“旧配置未设置模板（回落默认别名）”区分，避免升级后悄悄丢失别名。
- D5（2026-11-06，用户确认）：本次只覆盖 R2 上传（自动+手动）；Sub-Store 目标列表导出（Web/CLI）保持现有默认别名，不改为字段选择驱动。

# 待解决问题

（无）

# 验证预期

- `combined_refactor` 下 `go build ./...`、`go test ./...` 通过；`node --test web/test/*.test.js` 通过。
- 浏览器与接口实测：按 `IP:PORT + 数据中心` 触发 R2 上传，内容为 `IP:PORT#HKG`；旧配置 `{ipport}-{dc}` 无需手动修改即可生效；Sub-Store 导出与 R2 一致。
- 回归检查：TXT/CSV 导出、字段选择、预览、GitHub/edgetunnel 上传不受影响。
