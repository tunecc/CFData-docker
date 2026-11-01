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


window.formatEtaText = formatEtaText;
window.parseThreadsValue = parseThreadsValue;
