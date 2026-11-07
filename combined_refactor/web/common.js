// 纯 UI 工具函数（无 DOM 依赖）：可被 node 单测直接加载（web/test/common.test.js）。
// 在浏览器中先于 app.js 加载，函数挂到 window 供全局使用。

        // 并发数量解析：0=自动按候选规模自适应；非法/负值回落默认 100
        function parseThreadsValue(raw) {
            const v = parseInt(raw, 10);
            if (!Number.isFinite(v) || v < 0) return 100;
            return v;
        }

        // ETA 可选字段（etaSeconds >= 0 时显示），缺失或 -1 自动隐藏
        function formatEtaText(etaSeconds) {
            if (!Number.isFinite(etaSeconds) || etaSeconds < 0) return '';
            const totalSec = Math.max(0, Math.round(etaSeconds));
            const mm = Math.floor(totalSec / 60);
            const ss = totalSec % 60;
            return ` · 预计剩余 ${mm}:${String(ss).padStart(2, '0')}`;
        }


        // 结果面板字段选择清单：固定输出全部内置字段 + 自定义字段。
        // 不读取当前结果行，避免刷新页面/重启后没有结果数据时字段选择只剩 IP:PORT。
        function buildResultFieldChoices(resultFields, customFields) {
            const builtin = Array.isArray(resultFields) ? resultFields : [];
            const custom = Array.isArray(customFields) ? customFields : [];
            return builtin.concat(custom).filter((field) => field && typeof field.key === 'string' && field.key !== '');
        }

        // 结果面板字段清单对应的 key 列表（保持清单顺序，重复 key 只保留首次出现）
        function resultFieldChoiceKeys(resultFields, customFields) {
            const keys = [];
            const seen = new Set();
            for (const field of buildResultFieldChoices(resultFields, customFields)) {
                if (seen.has(field.key)) continue;
                seen.add(field.key);
                keys.push(field.key);
            }
            return keys;
        }


        // 表单级判断：任一字段有非空值即视为已填写（页面级保存据此跳过空白表单）
        function hasAnyFieldValue(values) {
            return (Array.isArray(values) ? values : []).some((v) => String(v ?? '').trim() !== '');
        }


        // 预览行截断：只取前 limit 行，非数组按空处理
        function takePreviewRows(rows, limit) {
            if (!Array.isArray(rows)) return [];
            const max = Number.isFinite(limit) && limit > 0 ? Math.floor(limit) : rows.length;
            return rows.slice(0, max);
        }


        // R2 / Sub-Store 别名占位符（与后端 applyR2AliasTemplate 支持集合一致）
        const R2_ALIAS_PLACEHOLDERS = {
            ipport: '{ipport}',
            ip: '{ip}',
            port: '{port}',
            dc: '{dc}',
            dcCountry: '{dcCountry}',
            region: '{region}',
            city: '{city}',
        };
        // 显式“无别名”标记：与“旧配置未设置模板（回落默认别名）”区分
        const R2_ALIAS_NONE_TEMPLATE = '{none}';

        // 按字段与顺序生成别名模板：IP:PORT 已在行首地址中，不参与别名；
        // 没有其他可用字段时返回 {none}，由后端只输出地址。
        function buildR2AliasTemplateFromOrder(fieldOrder, separator) {
            const sep = typeof separator === 'string' && separator !== '' ? separator : '-';
            const keys = (Array.isArray(fieldOrder) ? fieldOrder : [])
                .filter((key) => key !== 'ipport' && Object.prototype.hasOwnProperty.call(R2_ALIAS_PLACEHOLDERS, key));
            if (!keys.length) return R2_ALIAS_NONE_TEMPLATE;
            return keys.map((key) => R2_ALIAS_PLACEHOLDERS[key]).join(sep);
        }


        // 达标率三态色档（A6）：0% 红 bad / >0% 且 <5% 橙 warn / ≥5% 绿 good。
        // 阈值沿用 subs-check-pro 的通过率分档（docs/analyses/subs-check-pro-learnings-v2-ui.md §五）。
        function qualifiedRateTier(qualifiedCount, resultCount) {
            const q = Number(qualifiedCount) || 0;
            const t = Number(resultCount) || 0;
            if (t <= 0 || q <= 0) return 'bad';
            const rate = (q / t) * 100;
            if (rate < 5) return 'warn';
            return 'good';
        }

        // 中间截断（A9）：保头去中，超长时 head + … + tail；短文本原样返回。
        // 比 CSS ellipsis 保留更多信息，用于长 URL / 别名展示（title 仍显示全文）。
        function truncateMiddle(text, maxLen) {
            const s = String(text ?? '');
            const max = Number.isFinite(maxLen) && maxLen >= 1 ? Math.floor(maxLen) : 0;
            if (max <= 0 || s.length <= max) return s;
            if (max === 1) return s.slice(0, 1);
            const headLen = Math.ceil((max - 1) / 2);
            const tailLen = Math.floor((max - 1) / 2);
            return s.slice(0, headLen) + '…' + (tailLen > 0 ? s.slice(s.length - tailLen) : '');
        }

        // 跨标签页任务完成广播（A10）：写入方与接收方共用同一键与格式。
        const TASK_DONE_BROADCAST_KEY = 'cfdata_task_done';
        function buildTaskDoneBroadcast(mode, ts) {
            return JSON.stringify({ ts: Number.isFinite(ts) ? Math.round(ts) : Date.now(), mode: mode === 'nsb' ? 'nsb' : 'official' });
        }
        // 解析广播值：仅接受比 lastTs 更新的有效载荷（返回 null 表示忽略）
        function parseTaskDoneBroadcast(raw, lastTs) {
            let obj = null;
            try { obj = JSON.parse(String(raw ?? '')); } catch (e) { return null; }
            if (!obj || typeof obj !== 'object') return null;
            const ts = Number(obj.ts);
            if (!Number.isFinite(ts) || ts <= (Number(lastTs) || 0)) return null;
            return { ts: ts, mode: obj.mode === 'nsb' ? 'nsb' : 'official' };
        }

        // 日志框选缓冲器（A3）：选中文本期间新行只进缓冲不进 DOM，选区结束后一次性回灌，
        // 避免日志重绘打断用户复制（学习 subs-check-pro 的框选暂停渲染，适配我们的增量推送）。
        function createLogBuffer(maxLines) {
            const max = Number.isFinite(maxLines) && maxLines > 0 ? Math.floor(maxLines) : 500;
            let lines = [];
            let omitted = 0;
            return {
                // 缓冲一行：超出上限丢弃最旧并累计省略数
                push(text) {
                    lines.push(String(text ?? ''));
                    if (lines.length > max) {
                        lines.shift();
                        omitted++;
                    }
                },
                get size() { return lines.length; },
                get omitted() { return omitted; },
                // 选区结束：取出全部缓冲行与省略计数并复位
                drain() {
                    const out = { lines: lines, omitted: omitted };
                    lines = [];
                    omitted = 0;
                    return out;
                },
                clear() {
                    lines = [];
                    omitted = 0;
                },
            };
        }


window.formatEtaText = formatEtaText;
window.parseThreadsValue = parseThreadsValue;
window.buildResultFieldChoices = buildResultFieldChoices;
window.resultFieldChoiceKeys = resultFieldChoiceKeys;
window.hasAnyFieldValue = hasAnyFieldValue;
window.takePreviewRows = takePreviewRows;
window.R2_ALIAS_PLACEHOLDERS = R2_ALIAS_PLACEHOLDERS;
window.R2_ALIAS_NONE_TEMPLATE = R2_ALIAS_NONE_TEMPLATE;
window.buildR2AliasTemplateFromOrder = buildR2AliasTemplateFromOrder;
window.qualifiedRateTier = qualifiedRateTier;
window.truncateMiddle = truncateMiddle;
window.TASK_DONE_BROADCAST_KEY = TASK_DONE_BROADCAST_KEY;
window.buildTaskDoneBroadcast = buildTaskDoneBroadcast;
window.parseTaskDoneBroadcast = parseTaskDoneBroadcast;
window.createLogBuffer = createLogBuffer;
