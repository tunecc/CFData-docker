# Rulings

## Review acceptance — r2-upload-naming-config (2026-10-31)

独立审查结论：PASS_WITH_MINOR（无 CRITICAL/IMPORTANT）。

- MINOR-1（字节长度→字符数）：已修复，校验改用 utf8.RuneCountInString，测试覆盖 200 汉字接受 / 201 拒绝。
- MINOR-3（_config_help 缺 r2aliasformat）：已修复，补齐帮助条目。
- SUGGESTION-2（模板字面量 # 剥离）：已修复，substoreAliasWithTemplate 对最终别名统一剥离 #。
- SUGGESTION-3（输入框 maxlength=200）：已修复。
- MINOR-2（手动路径双读配置）：接受，不改签名。理由：uploadManualRowsToR2 保留自身读配置与错误日志的既有契约（6 处既有测试依赖），两次读取窗口极小且下次上传自愈，无正确性风险。
- SUGGESTION-1（逐行重建 Replacer）：接受，不改。理由：替换值逐行不同，Replacer 无法跨行复用，无可提升项。

复查轮次使用：0（初审通过，无阻断问题，无需复查轮）。

## Task 3.2 手动验收偏差记录

- 上传内容按模板命名、清空回落默认：由 httptest 端到端测试覆盖（TestScheduledExportUsesAliasTemplate / TestTriggerManualR2UploadUsesAliasTemplate），等同真实 HTTP 上传流程。
- 界面渲染与元素：本地真实运行冒烟验证（/ 页面 200，r2AliasFormat 输入框与 maxlength 正确）。
- 真实 Worker 网络上传与浏览器内保存回显交互：留待用户环境验收（本环境无 R2 Worker 与浏览器），已记录偏差。
