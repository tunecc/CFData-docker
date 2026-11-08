// 配置字段"活校验"：给建议而非拦截（学习 subs-check-pro config-form.js 的 FIELD_VALIDATORS 思路，
// 见 docs/analyses/subs-check-pro-learnings-v2-ui.md §四.1）。
// 纯函数无 DOM 依赖，可被 node 单测直接加载（web/test/validators.test.js）。
// 在浏览器中先于 app.js 加载，挂 window.CFDataValidators 供 app.js / 体检面板复用。

        // 单条校验结论：level ∈ ok（不显示文案）/ info（灰）/ warn（橙）；warn 永不拦截保存
        function validatorResult(level, msg) {
            return { level: level, msg: msg };
        }

        // 数字解析：空串返回 null（视为未填写，不提示），非有限数字返回 NaN 交由调用方判 warn
        function parseConfigNumber(raw) {
            const s = String(raw ?? '').trim();
            if (s === '') return null;
            const v = Number(s);
            return Number.isFinite(v) ? v : NaN;
        }

        // 并发数量：0=按候选规模自动适配；过高提示官方接口限频/429 退避风险
        function threadsValidator(raw) {
            const v = parseConfigNumber(raw);
            if (v === null) return validatorResult('ok', '');
            if (Number.isNaN(v) || v < 0) return validatorResult('warn', '并发需为不小于 0 的整数，0 表示自动适配');
            if (v === 0) return validatorResult('info', '0 = 按候选规模自动适配');
            if (v > 500) return validatorResult('info', '并发过高易触发官方接口限频（429 退避），建议 ≤300');
            return validatorResult('ok', '');
        }

        // 扫描合格延迟（毫秒）：过低易把网络抖动当超时误判，过高会放进大量高延迟节点
        function delayValidator(raw) {
            const v = parseConfigNumber(raw);
            if (v === null) return validatorResult('ok', '');
            if (Number.isNaN(v) || v < 0) return validatorResult('warn', '延迟阈值需为不小于 0 的整数（毫秒）');
            if (v > 0 && v < 50) return validatorResult('info', '阈值过低易因网络抖动误判，建议 ≥100ms');
            if (v > 2000) return validatorResult('info', '阈值过高会让大量高延迟节点进入合格集');
            return validatorResult('ok', '');
        }

        // 测速合格下限（MB/s）：过高几乎没有节点能达标
        function speedMinValidator(raw) {
            const v = parseConfigNumber(raw);
            if (v === null) return validatorResult('ok', '');
            if (Number.isNaN(v) || v < 0) return validatorResult('warn', '速度下限需为不小于 0 的数值（MB/s）');
            if (v > 10) return validatorResult('info', '下限过高可能几乎没有节点达标');
            return validatorResult('ok', '');
        }

        // 数量上限类：非负整数；过大提示任务耗时
        function countValidator(label, allowZeroNote) {
            return function (raw) {
                const v = parseConfigNumber(raw);
                if (v === null) return validatorResult('ok', '');
                if (Number.isNaN(v) || v < 0 || !Number.isInteger(v)) return validatorResult('warn', label + '需为不小于 0 的整数');
                if (v === 0 && allowZeroNote) return validatorResult('info', allowZeroNote);
                if (v > 100000) return validatorResult('info', '数量过大会明显拉长任务耗时');
                return validatorResult('ok', '');
            };
        }

        // 检测间隔（分钟）：沿用来源经验建议 ≥2880（2 天），过短易触发上游阻断
        function scheduleIntervalValidator(raw) {
            const v = parseConfigNumber(raw);
            if (v === null) return validatorResult('ok', '');
            if (Number.isNaN(v) || v < 0 || !Number.isInteger(v)) return validatorResult('warn', '检测间隔需为不小于 0 的整数（分钟）');
            if (v === 0) return validatorResult('info', '0 = 不使用固定间隔调度（仅 Cron 生效时停用）');
            if (v < 60) return validatorResult('info', '间隔过短易触发运营商/上游阻断，建议 ≥2880（2 天）');
            return validatorResult('ok', '');
        }

        // 端口：1-65535 整数（本仓库唯一端口字段，冲突互查不适用，做合法性校验）
        function portValidator(raw) {
            const v = parseConfigNumber(raw);
            if (v === null) return validatorResult('ok', '');
            if (Number.isNaN(v) || !Number.isInteger(v) || v < 1 || v > 65535) return validatorResult('warn', '端口需为 1-65535 的整数');
            return validatorResult('ok', '');
        }

        // Worker / 订阅类 URL：留空不提示（可选字段），填了必须是 http(s) 地址
        function urlValidator(raw) {
            const s = String(raw ?? '').trim();
            if (s === '') return validatorResult('ok', '');
            if (!/^https?:\/\//i.test(s)) return validatorResult('warn', '地址需以 http:// 或 https:// 开头');
            return validatorResult('ok', '');
        }

        // 字段注册表：key 与 index.html 输入框 id 一致
        const FIELD_VALIDATORS = {
            officialThreads: threadsValidator,
            nsbThreads: threadsValidator,
            officialDelay: delayValidator,
            nsbDelay: delayValidator,
            officialSpeedMin: speedMinValidator,
            nsbSpeedMin: speedMinValidator,
            speedBandwidth: countValidator('总测速带宽上限', '0 = 不限速（官方与非标测速共享）'),
            officialStopCount: countValidator('提前结束数量', '0 = 不提前结束（数据中心选「全部」时同样不生效）'),
            nsbResultLimit: countValidator('扫描合格数量', '0 = 不限制结果数量'),
            scheduleInterval: scheduleIntervalValidator,
            nsbFallbackPort: portValidator,
            r2WorkerUrl: urlValidator,
        };

        // 统一入口：按字段 id 校验值；未注册字段返回 ok（无提示）
        function validateConfigField(fieldId, raw) {
            const fn = FIELD_VALIDATORS[fieldId];
            if (!fn) return validatorResult('ok', '');
            let res;
            try {
                res = fn(raw);
            } catch (e) {
                return validatorResult('ok', '');
            }
            return res && (res.level === 'info' || res.level === 'warn') ? res : validatorResult('ok', '');
        }


window.CFDataValidators = {
    validateConfigField: validateConfigField,
    FIELD_VALIDATORS: FIELD_VALIDATORS,
};
