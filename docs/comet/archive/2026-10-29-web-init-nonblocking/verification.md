---
generated_from_state_version: 18
---

# 验证

## 当前结果

- 结果: **已归档**
- 验证情况: **已完成检查，验证结果已确认**
- 目标周期: 3
- 迭代: 2
- 验证器尝试次数: 1
- 完成时间: 2026-10-29T03:36:02.476Z
- 摘要: 独立复核 A1–A11 全部通过。本轮库选择模式零结果提示已核实：server.go 在 fromLibrary 时把标志传入 tasks_official.go，库模式发送“数据中心库中 <DC> 的 IP 均不可用，请更新数据中心库后重试。”且手动/全量路径保持原提示，TestOfficialScanZeroMatchesFromLibraryHintsUpdate 通过。Runtime 的 go-test/go-build 对应当前候选并通过，关键单测与全量 go test 独立复跑通过；A6/A9 容器级持久化与真实零结果场景未重测，已作为残余风险列出。

## 验收

| 编号 | 结果 | 来源 | 验收项 | 原因 |
| --- | --- | --- | --- | --- |
| A1 | passed | brief.md | A1：构建并运行指定版本（如 `sha-53c9986`）后打开或刷新页面，首屏 HTML 中的版本行即为该版本标识，不依赖 WebSocket；WebSocket 延迟或断开时仍显示正确版本。 | version.go 的 renderIndexHTML 仅替换唯一占位符 <div class="version-line" id="versionLine">dev</div>（index.html 中该串出现次数为 1）并对版本值 html.EscapeString；main.go 的 / 处理器每次请求写入 renderIndexHTML(data, appVersionLabel()) 且保持 Cache-Control: no-cache，版本来源与 init_config/CLI 输出的 appVersionLabel() 一致（Dockerfile 以 appVersion/appRevision 注入）。TestRenderIndexHTMLInjectsVersion 验证只替换目标占位、转义且不影响其他 dev 文本；上一轮对 18080 实例的只读 HTTP 探测已看到首屏注入，本轮相关文件未再改动。 |
| A2 | passed | brief.md | A2：在 Cloudflare `cdn-cgi/trace` 与 ISP 探测均不可达（模拟超时）的环境下，WebSocket 连接后 `init_config` 立即到达，页面立即进入已连接状态并应用测速源默认值、测速源状态与代理状态，不再等待探测超时。 | server.go 的 buildInitConfigPayload 仅读取启动缓存（startupSpeedDefault）、本地 speedURLStatus 与代理状态、本地数据中心库摘要，handleWebSocket 在连接建立后直接发送 init_config，同步路径中不再有 detectCloudflareTraceCountry 或 resolveStartupSpeedTestURL 网络调用（对比 HEAD 已移除）。speedTestDefault 的兜底 resolveSpeedTestURL 也只用本地可用性缓存。TestBuildInitConfigPayloadUsesStartupCache 验证构造耗时与缓存值；上一轮黑洞代理实例实测 init_config 1–33ms 到达。 |
| A3 | passed | brief.md | A3：开始扫描前的代理检测行为不变：中国/未知/测试标签放行，其他地区或检测失败仍按现有规则弹出警告；7 秒超时与三次尝试保持。 | check_proxy_country 处理器未改动：skipGeoCheck 短路，否则 7 秒 context + detectCloudflareTraceCountry 三次尝试 + shouldWarnProxyCountry 判定；geo_warning.go 未在改动列表。前端 requireProxyWarning 在开始官方扫描与更新数据中心库前均照旧发送 check_proxy_country，init_config 中 proxyWarning 只影响日志且 requireCachedProxyWarning 未被调用，页面加载不会新增代理弹窗。 |
| A4 | passed | brief.md | A4：测速源默认值与可用状态仍正确应用；自动选择逻辑不变；`version_info` 仍能在后台更新检查后追加“发现新版本”链接。 | main.go 在启动时设置 startupSpeedDefault（自动模式为 auto，自定义为原值），init_config 使用该缓存，前端 applySpeedUrlConfig 对 auto 显示“自动选择”并 applySpeedURLStatus 渲染本地状态；自动选择逻辑（resolveStartupSpeedTestURL）仍在启动阶段执行。version_info 处理保留在 server.go，前端 renderVersionInfo 在 hasUpdate 时继续追加“发现新版本”链接。全量测试通过且相关路径未在本轮改动。 |
| A5 | passed | brief.md | A5：`go build ./...` 与 `go test ./...` 在 `combined_refactor` 下通过；新增测试覆盖版本 HTML 注入和初始化配置立即构造，现有版本与扫描测试不回归。 | Runtime 记录的 go-test（go test ./... -count=1，cwd combined_refactor）exit 0、2212ms，go-build exit 0，操作版本对应当前候选；日志内容 ok cfdata 1.253s。新增 web_init_test.go（版本注入/转义、init_config 立即构造）、dc_library_test.go（保存/加载、候选、替换、损坏回退、空状态、reset 保留）、TestOfficialScanZeroMatchesFromLibraryHintsUpdate；我另行只读复跑关键单测与全量 go test 均 PASS。 |
| A6 | passed | brief.md | A6：点击「更新数据中心库」并使用当前 IP 类型/端口/线程数/合格延迟/扫描方式时，系统扫描官方候选池，记录成功 IP 的 `colo`，将结果保存为 `/data/dc-library-v4.json` 或 `/data/dc-library-v6.json`，完成后页面显示更新时间、数据中心数与 IP 数；重建容器后库数据仍存在。 | tasks_library.go 用当前 ipType/threads/port/delay/scanMode 扫描 officialCandidatePool，成功结果含扫描时 trace 实测 colo；dc_library.go 按 IP 类型原子写入 dc-library-v4.json/dc-library-v6.json（storage.go atomicWriteFile 临时文件+rename），schema 为 cfdata.dc-library.v1；Dockerfile WORKDIR /data、entrypoint cd /data、compose ./data:/data，docs/docker.md 已说明持久化。成功后发送 dc_library_status，页面显示更新时间/数据中心数/IP 数。TestDCLibrarySaveLoadAndCandidates 覆盖落盘与重载。 |
| A7 | passed | brief.md | A7：库中存在 HKG 等数据中心时，「数据中心」下拉以单个 `colo` 为选项，显示城市/国家与 IP 数量；选择 HKG 后启动官方扫描，只使用库中 HKG 的 IP 作为候选，扫描结果只包含 HKG，早停数量仍按现有语义生效。 | dcLibraryStatusFor 按单个 colo 汇总城市/国家/IP 数量并排序，前端 officialDcSelect 以 colo 为 option；server.go 在 fromLibrary=true 时只取 libraryIPsForDC 且不回退全量池，并传 targetDC 做实测 colo 过滤、stopCount 沿用现有早停语义。库内 IP 全部失效时 tasks_official.go 分支发送“数据中心库中 <DC> 的 IP 均不可用，请更新数据中心库后重试。”，TestOfficialScanZeroMatchesFromLibraryHintsUpdate 断言该文案且不含“请更换数据中心”，手动路径的旧文案由 TestOfficialScanZeroMatchesReportsTargetDC 覆盖，均通过。 |
| A8 | passed | brief.md | A8：当前 IP 类型没有库数据时，「数据中心」下拉禁用并提示先扫描一次；「全部」全量扫描仍按现有行为可用；库扫描完成后下拉自动启用并显示新数据。 | index.html renderOfficialDcLibrary 在当前 IP 类型 ipCount 为 0 时禁用下拉、强制选择“全部”并显示“尚无数据中心库，请点击右侧按钮更新；「全部」仍可直接扫描”，未禁用开始按钮，getOfficialTargetDC 返回空串使扫描走全量池；切换 IP 类型重渲染；收到 dc_library_status 后按新库自动启用并显示选项。库成功写入后 dc_library_status 在任务完成前下发，渲染即时刷新。 |
| A9 | passed | brief.md | A9：用户手动更新库成功后，库内容、更新时间和下拉选项刷新为新结果；库扫描被取消或失败时保留旧库并给出提示；普通官方扫描不会自动改动库。 | tasks_library.go 仅在扫描完成且结果非空时调用 saveDCLibrary，取消时输出“数据中心库更新已终止，保留原有库数据”并返回，零结果/保存失败发送错误且不改内存库；saveDCLibrary 原子替换成功后立即发送 dc_library_status（含新 UpdatedAt/DC/IP 数），前端刷新内容、时间与下拉选项。普通官方扫描路径不调用 saveDCLibrary。TestDCLibrarySaveLoadAndCandidates 验证快照替换与重载保留，reset_all_config 不删除库由 TestResetAllConfigKeepsDCLibrary 覆盖。 |
| A10 | passed | brief.md | A10：库扫描使用当前 IP 类型/端口/线程数/合格延迟/扫描方式，不设置目标数据中心、不早停；进度、日志与停止按钮工作正常，扫描期间不能同时启动其他官方任务，库扫描结果不进入官方扫描结果表。 | start_dc_library_scan 校验并透传当前 ipType/threads/port/delay/scanMode；runDCLibraryScan 不设置 targetDC、使用 runBoundedWorkers（无 shouldStop）因此不早停，发送 dc_library_progress 与日志，复用任务框架的 stop/task_stopped；beginTask 的 globalTaskMutex/anyTaskRunning 保证与其他官方任务互斥；它不写 session.scanResults 也不发送 scan_result，故结果不进入官方结果表。前端更新按钮在运行时禁用、任务结束由 finishRunningTask 恢复。 |
| A11 | passed | brief.md | A11：选择「全部」时官方扫描行为与现状一致；选择「手动输入其他代码」时仍按现有全量候选池与目标 DC 过滤扫描；NSB、详细测试、测速、导出、R2/GitHub 上传与自动调度流程不受影响。 | 选择“全部”时 getOfficialTargetDC 返回空串、officialDcFromLibrary=false，server 走 runOfficialTask 全量候选；选择“手动输入其他代码”时 officialDcSelection='__manual__'、发送官方目标 DC 且 fromLibrary=false，仍用全量池并做 targetDC 过滤（server.go 的 else 分支）。tasks_official.go 仅抽取 officialCandidatePool，全量池/解析错误语义不变。git status 显示改动仅限版本、初始化、库相关文件与文档，NSB/详细测试/测速/导出/R2/GitHub/自动调度代码未变动，全量测试通过。 |

## 检查

| 检查 | 命令 | 工作目录 | 状态 | 退出码 | 耗时 |
| --- | --- | --- | --- | ---: | ---: |
| Go 全量测试 | test ./... -count=1 | combined_refactor | passed | 0 | 2212 ms |
| Go 全量构建 | build ./... | combined_refactor | passed | 0 | 215 ms |

### Builder 报告的证据

以下为 Builder 报告，不等同于 Runtime 检查凭据或独立验收结果。

- go build ./...: passed — combined_refactor 下执行成功
- go test ./...: passed — ok cfdata 1.228s，含新增 TestOfficialScanZeroMatchesFromLibraryHintsUpdate；既有 Official/DC 库/初始化测试全部通过
- Linux amd64 测试二进制构建: passed — CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build 输出 release_assets/cfdata-test（14602400 字节）
- Web 与容器实测（上一轮候选）: passed — A1–A4/A6–A11 的浏览器、黑洞代理与重启实测来自上一轮候选；本轮仅改库模式零结果提示及其单测，相关路径未改动
- 已知限制: 本轮仅修改库模式零结果提示并补充单测，未重新执行浏览器与容器重建实测；A6/A9 的容器持久化与手动更新实测来自上一轮候选，相关代码本轮未改动。
- 已知限制: 真实网络下“库内 IP 全部失效”的场景未构造浏览器复现，由单元测试 TestOfficialScanZeroMatchesFromLibraryHintsUpdate 覆盖消息与保留行为。

## 阻塞项

_无。_

## 风险与跳过的工作

- A6/A9 的容器重建持久化未在本轮重新实测；依据 /data WORKDIR、entrypoint cd /data、compose 挂载、启动加载与上一轮重启/库文件哈希实测及本轮未改相关代码判断通过。
- A7 的城市/国家标签依赖 locations.json；缺失时只显示 colo 与数量，且当前 locations.json 无中文国家名，国家列展示 cca2 代码（如 HK）。
- A11 仍存在配置还原歧义：localStorage/配置文件保存的 targetDC 在加载配置或跟随后台任务时经 applyOfficialTargetDCSelection 还原，若该 colo 已在库中会被静默切为库模式，候选来源由全量池变为库内 IP；当场手动选择仍走全量池。
- 库模式零结果的新提示仅由单测覆盖，未在真实网络下构造“库内 IP 全部失效”的浏览器复现。
- A1 注入依赖 index.html 中版本行占位字符串完全一致，后续改动该 HTML 结构会使注入静默失效并回退 dev。
- 测速源默认值缓存固定在进程启动时（D3/D4 设计），运行期 ISP/网络变化不再逐连接重新探测，页面默认值可能与当前网络条件不完全一致。

## 之前的迭代

| 目标周期 | 迭代 | 尝试 | 结果 | 未解决项 | 摘要 | 完成时间 |
| ---: | ---: | ---: | --- | --- | --- | --- |
| 1 | 0 | 0 | recovery | — | Native target specification declarations changed | 2026-10-29T02:42:37.482Z |
| 2 | 0 | 0 | recovery | — | Native confirmed acceptance criteria changed; restore the approved documents, or reconcile any running Builder with its original Action and claim before using the latest continuation to revise-requirements on this change and confirm the current Shape. A legacy flattened acceptance snapshot cannot safely establish Markdown boundaries after an edit. | 2026-10-29T02:42:56.712Z |
| 3 | 1 | 1 | pass | — | 独立复核 A1–A11 全部通过。独立执行 go build ./... 与 go test ./... 通过；对 127.0.0.1:18080 测试实例做只读 WebSocket/HTTP 探测：首屏版本已服务端注入且 no-cache，init_config 1ms 到达并携带版本/测速源缓存/库摘要，代理检测对 HK 返回警告；按库 HKG 扫描使用 60 个库 IP、结果仅 HKG 并按既有语义早停；手动 DC 路径使用 200 条全量候选；库扫描可观测进度、可停止，取消后库文件哈希不变，扫描期间启动官方任务被拒绝，普通扫描不修改库。未重跑完整成功库扫描与容器重建，已按真实库文件、重启加载、单测与部署路径核验，并在风险中列出。 | 2026-10-29T03:12:12.513Z |
| 3 | 1 | 1 | recovery | — | 修复库中数据中心扫描空结果提示：库选择模式提示更新数据中心库，手动输入模式保持原提示；补充测试后重新提交验收。 | 2026-10-29T03:26:24.680Z |
| 3 | 2 | 1 | pass | — | 独立复核 A1–A11 全部通过。本轮库选择模式零结果提示已核实：server.go 在 fromLibrary 时把标志传入 tasks_official.go，库模式发送“数据中心库中 <DC> 的 IP 均不可用，请更新数据中心库后重试。”且手动/全量路径保持原提示，TestOfficialScanZeroMatchesFromLibraryHintsUpdate 通过。Runtime 的 go-test/go-build 对应当前候选并通过，关键单测与全量 go test 独立复跑通过；A6/A9 容器级持久化与真实零结果场景未重测，已作为残余风险列出。 | 2026-10-29T03:36:02.476Z |



## 结论

独立复核 A1–A11 全部通过。本轮库选择模式零结果提示已核实：server.go 在 fromLibrary 时把标志传入 tasks_official.go，库模式发送“数据中心库中 <DC> 的 IP 均不可用，请更新数据中心库后重试。”且手动/全量路径保持原提示，TestOfficialScanZeroMatchesFromLibraryHintsUpdate 通过。Runtime 的 go-test/go-build 对应当前候选并通过，关键单测与全量 go test 独立复跑通过；A6/A9 容器级持久化与真实零结果场景未重测，已作为残余风险列出。
