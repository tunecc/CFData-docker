### Preset file-count authorization

- status: active
- workflow: hotfix
- decision: continue-on-file-count-only
- scope: 更新检测改为 tunecc/CFData-docker 的 Release/Tags；内嵌 locations/ips-v4/ips-v6 快照作为远程失败兜底；不改变扫描、筛选、测速和镜像发布行为，不修改 .github/
- allowed-file-categories: implementation, tests, user-docs, config, generated
- authorization-basis: user-explicit
- reason: 用户在 2026-10-27 明确选择「继续 hotfix」，确认范围与风险不变、仅文件数超过 4 个提示阈值（预计 7 个左右，实际约 8 个）
