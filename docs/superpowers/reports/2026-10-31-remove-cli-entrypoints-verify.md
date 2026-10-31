# Verification Report: remove-cli-entrypoints

- Date: 2026-10-31
- Mode: full（任务 7）
- Implementation commit: b531b65（main，基线 6949ae3）
- Build review: 独立审查初判 FAIL（3 IMPORTANT：README 非标参数块残留 11 个已删 flag、-proxy 提法、-nsbspeedtest 语义错位）→ 修复 → 复查仅剩 1 行阻塞（README:219 未同步）→ 已修复；复查确认其余全部通过（核心删除逻辑、共享管线保全、构建测试均干净）

## Summary

| 维度 | 结果 |
| --- | --- |
| Completeness | 7/7 任务完成 |
| Correctness | 20 个 CLI 专属函数删除且零残留引用；共享管线/配置模型/导出上传零行为变化 |
| Coherence | 与 design.md 决策一致（偏差已记录于 Implementation Deviation 节） |

## 检查项

1. **tasks.md 全部完成**：7/7（逐 ID 登记）
2. **实现符合 design.md**：边界以调用图为准——审查逐个核对 20 个被删函数零引用；describeSchedulePlan/readMenuLine 等共享符号迁移保留；newCLISession/handleCLIMessage 经核实为调度器终端渲染器而保留（与原计划的偏差记录于 design.md Implementation Deviation，已通过 revise-design 重新批准）
3. **能力规格**：skip_specs: true（CLI 从未被规格覆盖，Web/调度行为规格零变化）——无场景需核对
4. **proposal 目标满足**：CLI 入口与交互代码全删；README/docs 同步；-cli 明确报错
5. **无回归**：go build/vet/test 全绿（含调度管线测试）；Web 冒烟 200 + 新界面；-cli 报错验证
6. **最终集成代码审查**：两轮独立审查，遗留 1 行阻塞已修复（README:219 与 main.go flag 描述一致、补 -url 行、配置模板元信息去 CLI 化），审查者明示该修复后无需再全量复查
7. **行为保持**：-nsbspeedtest 默认 0 恢复原有效默认链；-proxy 删除不改变 Web 行为（Web 从未消费该 flag）

## 测试与构建证据

- `go -C combined_refactor test ./...` 全绿（guard 执行并绑定收据）
- Web 冒烟：首页 200 + 新界面元素；`-cli` → "flag provided but not defined: -cli"
- `release_assets/cfdata-test` 已重建（Oct 31 19:16+，晚于全部源码改动）

## 已接受偏差（记录）

- design.md Implementation Deviation 三项（rewriteBoolFlagArgs 保留、-nsbspeedtest 升公共 flag、终端渲染器保留）——已经 revise-design 重新批准
- main.go 去门化后的裸块保留（行为等价）；7 个 HEAD 既有孤儿函数不计入本 change

## 结论

全部检查通过，无 CRITICAL/IMPORTANT 问题。Ready for archive。
