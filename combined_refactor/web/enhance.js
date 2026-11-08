/* ============================================================
   新界面增强层：视图导航 / 圆形启停状态机 / 终端日志 / 分析聚合 / Toast
   以包装（wrap）旧顶层函数的方式集成，不改动旧逻辑本身。
   ============================================================ */
(function () {
    'use strict';

    /* ---------------- 视图导航（主列双面板 + 右侧配置分组） ---------------- */

    // 运行视图内「运行日志 / 分析报告」双面板切换
    window.nzShowRunPanel = function (panel) {
        var panels = { log: 'runPanelLog', report: 'runPanelReport' };
        var tabs = { log: 'runTabLog', report: 'runTabReport' };
        Object.keys(panels).forEach(function (key) {
            var body = document.getElementById(panels[key]);
            if (body) body.classList.toggle('active', key === panel);
        });
        document.querySelectorAll('.run-tab').forEach(function (t) { t.classList.remove('active'); });
        var tab = document.getElementById(tabs[panel]);
        if (tab) tab.classList.add('active');
        if (panel === 'report' && typeof window.nzRefreshAnalysis === 'function') {
            try { window.nzRefreshAnalysis(); } catch (e) { /* 表格尚未渲染 */ }
        }
        try { window.scrollTo({ top: 0, behavior: 'smooth' }); } catch (e) { /* 旧内核 */ }
    };

    // 右侧配置分组切换；任务模式唯一切换入口在任务控制卡：
    // official/nsb/mode 入参只跟随当前模式显示对应参数页，MUST NOT 反向切换任务模式
    var nzPanelScrollMemory = {}; // A12：配置面板 tab 滚动位置记忆（会话内存态）
    window.nzShowConfigPanel = function (name) {
        var key = name;
        if (key === 'mode' || key === 'official' || key === 'nsb') {
            var curMode = 'official';
            try { curMode = currentMode === 'nsb' ? 'nsb' : 'official'; } catch (e) { curMode = 'official'; }
            key = curMode;
        }
        var tabs = { automation: 'cfgTabAutomation', results: 'cfgTabResults', accounts: 'cfgTabAccounts', settings: 'cfgTabSettings' };
        var bodies = { official: 'cfgPanelOfficial', nsb: 'cfgPanelNSB', automation: 'cfgPanelAutomation', results: 'cfgPanelResults', accounts: 'cfgPanelAccounts', settings: 'cfgPanelSettings' };
        // A12：display:none 前先保存各面板滚动位置（存 → 切 → rAF 恢复，参照 subs-check-pro config-form.js:4693-4732）
        Object.keys(bodies).forEach(function (k) {
            var body = document.getElementById(bodies[k]);
            if (body) nzPanelScrollMemory[k] = body.scrollTop;
        });
        Object.keys(bodies).forEach(function (k) {
            var body = document.getElementById(bodies[k]);
            if (!body) return;
            var isActive = k === key;
            // 面板初始 HTML 自带 hidden 类，切换时必须同步增删
            body.classList.toggle('active', isActive);
            body.classList.toggle('hidden', !isActive);
        });
        // A12：恢复目标面板滚动位置（rAF 等布局生效后再滚）
        var targetBody = document.getElementById(bodies[key]);
        if (targetBody) {
            var savedTop = nzPanelScrollMemory[key] || 0;
            requestAnimationFrame(function () { targetBody.scrollTop = savedTop; });
        }
        document.querySelectorAll('.cfg-tab').forEach(function (t) { t.classList.remove('active'); });
        // 当前模式参数标签：文字/说明跟随任务控制卡选择的模式，两个模式只有一个标签
        var modeTab = document.getElementById('cfgTabMode');
        if (modeTab) {
            var label = document.getElementById('cfgTabModeLabel');
            var isNSB = key === 'nsb';
            if (label) label.textContent = isNSB ? '非标优选' : '官方优选';
            modeTab.title = (isNSB ? '非标优选参数与扫描' : '官方优选参数与扫描') + '（模式切换只在任务控制卡）';
            if (isNSB || key === 'official') modeTab.classList.add('active');
        }
        var tab = document.getElementById(tabs[key]);
        if (tab) tab.classList.add('active');
        if (key === 'settings') {
            try { wsSend({ type: 'get_management_proxy', data: {} }); } catch (e) { /* 连接未就绪 */ }
        }
        if (key === 'results' && typeof syncResultSourceOptions === 'function') {
            try { syncResultSourceOptions(); } catch (e) { /* 结果数据未就绪 */ }
        }
    };

    window.nzShowView = function (view) {
        if (view === 'official' || view === 'nsb' || view === 'settings' || view === 'automation' || view === 'accounts' || view === 'results') {
            window.nzShowConfigPanel(view);
            return;
        }
        // 主列只有一个运行视图；run = 日志面板，analysis = 报告面板
        document.querySelectorAll('.view').forEach(function (v) { v.classList.remove('active'); });
        var el = document.getElementById('view-run');
        if (el) el.classList.add('active');
        window.nzShowRunPanel(view === 'analysis' ? 'report' : 'log');
    };

    /* ---------------- Toast ---------------- */

    // 默认展示时长（毫秒）：与 styles.css 中 toast-countdown 动画时长保持一致
    var NZ_TOAST_MS = 2600;

    window.nzToast = function (text, kind, durationMs) {
        var wrap = document.getElementById('toastWrap');
        if (!wrap) return;
        var ms = typeof durationMs === 'number' && durationMs > 0 ? durationMs : NZ_TOAST_MS;
        var t = document.createElement('div');
        t.className = 'toast ' + (kind || 'info');
        t.textContent = String(text == null ? '' : text);
        // 底部倒计时进度条：动画时长与自动关闭时长同源（A5）
        try { t.style.setProperty('--toast-ms', (ms / 1000) + 's'); } catch (e) { /* 旧内核忽略 */ }
        wrap.appendChild(t);
        setTimeout(function () {
            t.style.transition = 'opacity .3s';
            t.style.opacity = '0';
            setTimeout(function () { t.remove(); }, 320);
        }, ms);
    };

    /* ---------------- 复制按钮三段式反馈（A5） ---------------- */

    // 成功后：按钮内容瞬时换对勾 + .copied 绿态 → 停留约 1.8s → 还原内容；
    // 反馈期间 dataset.busy 置位，防重复触发复制。返回 false 表示正忙（调用方跳过本次复制）。
    var NZ_COPIED_MS = 1800;
    window.nzCopyFeedback = function (target) {
        var btn = target && target.closest ? target.closest('button') : null;
        if (!btn) return true;
        if (btn.dataset.busy === '1') return false;
        var original = btn.innerHTML;
        btn.dataset.busy = '1';
        btn.classList.add('copied');
        btn.innerHTML = '<span aria-hidden="true">✓</span>';
        setTimeout(function () {
            btn.classList.remove('copied');
            btn.innerHTML = original;
            delete btn.dataset.busy;
        }, NZ_COPIED_MS);
        return true;
    };

    /* ---------------- 圆形启停状态机 ---------------- */

    var nzToggleState = 'idle';
    var STATE_META = {
        idle: { title: '空闲', sub: '点击按钮开始扫描与测试' },
        preparing: { title: '准备中…', sub: '正在初始化任务' },
        running: { title: '任务运行中', sub: '点击按钮可停止任务' },
        stopping: { title: '正在停止…', sub: '等待任务退出' }
    };

    function nzSetToggleState(state) {
        if (!STATE_META[state]) return;
        nzToggleState = state;
        var btn = document.getElementById('nzToggle');
        if (btn) {
            btn.classList.remove('state-idle', 'state-preparing', 'state-running', 'state-stopping');
            btn.classList.add('state-' + state);
            // A11：启停按钮按下态随任务状态同步（任务活跃 = true）
            var active = state === 'preparing' || state === 'running' || state === 'stopping';
            btn.setAttribute('aria-pressed', active ? 'true' : 'false');
        }
        var title = document.getElementById('nzRunTitle');
        if (title) title.textContent = STATE_META[state].title;
        var sub = document.getElementById('nzRunSub');
        if (sub) sub.textContent = STATE_META[state].sub;
    }

    var nzPendingPrepare = false;
    window.nzToggleClick = function () {
        if (nzToggleState === 'preparing' || nzToggleState === 'stopping') return;
        if (nzToggleState === 'running') {
            try { startTask(); } catch (e) { console.error(e); }
            return;
        }
        nzPendingPrepare = true;
        nzSetToggleState('preparing');
        nzToast('任务启动中…', 'info');
        setTimeout(function () { nzPendingPrepare = false; }, 3000);
        try { startTask(); } catch (e) { nzSetToggleState('idle'); nzPendingPrepare = false; }
    };

    /* 包装旧状态写入函数，派生按钮状态 */
    var _setStartLoading = setStartLoading;
    setStartLoading = function (text, bg, shadow) {
        _setStartLoading(text, bg, shadow);
        var t = String(text || '');
        if (t.indexOf('停止') >= 0) {
            nzSetToggleState('stopping');
            nzPendingPrepare = false;
        } else if (nzPendingPrepare) {
            nzSetToggleState('preparing');
        } else {
            nzSetToggleState('running');
        }
    };
    var _resetButton = resetButton;
    resetButton = function (text) {
        _resetButton(text);
        nzSetToggleState('idle');
    };
    var _showStoppingProgress = showStoppingProgress;
    showStoppingProgress = function () {
        _showStoppingProgress();
        nzSetToggleState('stopping');
    };
    if (typeof finishRunningTask === 'function') {
        var _finishRunningTask = finishRunningTask;
        finishRunningTask = function (buttonText) {
            _finishRunningTask(buttonText);
            nzSetToggleState('idle');
        };
    }
    if (typeof resetStoppedTaskUI === 'function') {
        var _resetStoppedTaskUI = resetStoppedTaskUI;
        resetStoppedTaskUI = function () {
            var r = _resetStoppedTaskUI.apply(this, arguments);
            nzSetToggleState('idle');
            return r;
        };
    }

    /* ---------------- 进度元信息 ---------------- */

    var nzProgressStageText = '';

    function nzSyncProgressMeta() {
        var bar = document.getElementById('progressBar');
        var box = document.getElementById('progressBox');
        var meta = document.getElementById('nzProgressMeta');
        if (!bar || !box || !meta) return;
        var show = box.style.display === 'block';
        meta.classList.toggle('show', show);
        var stage = document.getElementById('nzProgressStage');
        if (stage) stage.textContent = show ? nzProgressStageText : '';
        var count = document.getElementById('nzProgressCount');
        if (count) count.textContent = show ? (bar.textContent || '') : '';
    }

    var _resetProgress = resetProgress;
    resetProgress = function () {
        _resetProgress();
        nzSyncProgressMeta();
    };
    var _updateProgress = updateProgress;
    updateProgress = function () {
        _updateProgress.apply(this, arguments);
        nzSyncProgressMeta();
    };
    if (typeof updateQualifiedSpeedProgress === 'function') {
        var _updateQualifiedSpeedProgress = updateQualifiedSpeedProgress;
        updateQualifiedSpeedProgress = function () {
            _updateQualifiedSpeedProgress.apply(this, arguments);
            nzSyncProgressMeta();
        };
    }
    if (typeof completeProgressWithCurrentCount === 'function') {
        var _completeProgressWithCurrentCount = completeProgressWithCurrentCount;
        completeProgressWithCurrentCount = function () {
            _completeProgressWithCurrentCount.apply(this, arguments);
            nzSyncProgressMeta();
        };
    }

    /* ---------------- 消息后处理：状态/跳转/分析/徽章 ---------------- */

    var nzFirstResultJumped = false;
    var PROGRESS_TYPES = ['scan_progress', 'nsb_progress', 'official_speed_progress', 'test_progress', 'speed_test_result', 'compact_ipv4_progress', 'dc_library_progress'];
    var RESULT_TYPES = ['scan_result', 'nsb_scan_result', 'nsb_scan_sorted', 'test_result', 'test_complete', 'official_speed_complete', 'nsb_speed_complete', 'nsb_csv_complete', 'speed_test_result', 'task_complete'];
    var STAGE_NAMES = {
        scan_progress: '扫描中', nsb_progress: '非标扫描', official_speed_progress: '官方测速',
        test_progress: '详细测试', speed_test_result: '单点测速', compact_ipv4_progress: '精简地址库',
        dc_library_progress: '数据中心库'
    };

    var _handleMessage = handleMessage;
    handleMessage = function (msg) {
        var result = _handleMessage(msg);
        try { nzAfterMessage(msg); } catch (e) { console.error(e); }
        return result;
    };

    function nzAfterMessage(msg) {
        var type = msg && msg.type;
        if (PROGRESS_TYPES.indexOf(type) >= 0) {
            if (STAGE_NAMES[type]) nzProgressStageText = STAGE_NAMES[type];
            if (nzToggleState === 'preparing' || nzToggleState === 'idle') nzSetToggleState('running');
            nzSyncProgressMeta();
        }
        if (RESULT_TYPES.indexOf(type) >= 0) {
            nzScheduleAnalysis();
            nzUpdateNavBadge();
        }
        if ((type === 'scan_result' || type === 'nsb_scan_result' || type === 'nsb_scan_sorted') && !nzFirstResultJumped) {
            nzFirstResultJumped = true;
            nzShowView('analysis');
            nzToast('已收到首批结果，已切换到分析报告', 'info');
        }
        if (type === 'task_complete') {
            nzSetToggleState('idle');
            nzToast('任务完成', 'success');
            nzScheduleAnalysis();
            // A10：跨标签页广播任务完成（其他同源标签页经 storage 事件刷新状态/结果）
            try {
                localStorage.setItem(
                    window.TASK_DONE_BROADCAST_KEY || 'cfdata_task_done',
                    typeof window.buildTaskDoneBroadcast === 'function' ? window.buildTaskDoneBroadcast(currentMode) : String(Date.now())
                );
            } catch (e) { /* 隐私模式等存储不可用时忽略 */ }
            try { nzShowRunPanel('report'); } catch (e) { console.error(e); }
            try { nzUpdateStartReadiness(); } catch (e) { /* 旧函数缺失时忽略 */ }
        }
        if (type === 'task_stopped') {
            nzToast('任务已停止', 'info');
            try { nzUpdateStartReadiness(); } catch (e) { /* 旧函数缺失时忽略 */ }
        }
        if (type === 'error') {
            nzToast(typeof msg.data === 'string' ? msg.data : '发生错误，请查看日志', 'error');
        }
    }

    /* ---------------- 分析聚合（从已渲染表格读取） ---------------- */

    var nzAnalysisTimer = null;
    window.nzScheduleAnalysis = function () {
        clearTimeout(nzAnalysisTimer);
        nzAnalysisTimer = setTimeout(nzRefreshAnalysis, 220);
    };
    window.nzRefreshAnalysis = nzRefreshAnalysis;
    window.nzUpdateLogCount = nzUpdateLogCount;
    window.nzUpdateNavBadge = nzUpdateNavBadge;

    function nzRows(tableId) {
        var tb = document.querySelector('#' + tableId + ' tbody');
        return tb ? Array.prototype.slice.call(tb.children) : [];
    }

    function nzCells(tr) {
        return Array.prototype.slice.call(tr.children).map(function (td) { return td.textContent.trim(); });
    }

    function nzColIndex(tableId, names) {
        var ths = Array.prototype.slice.call(document.querySelectorAll('#' + tableId + ' thead th')).map(function (th) { return th.textContent.trim(); });
        for (var i = 0; i < names.length; i++) {
            var idx = ths.indexOf(names[i]);
            if (idx >= 0) return idx;
        }
        return -1;
    }

    function nzChip(icon, color, num, label, sub) {
        return '<div class="stat-chip"><div class="chip-icon" style="background:color-mix(in srgb,' + color + ' 14%,transparent);color:' + color + ';">' + icon +
            '</div><div><div class="chip-num" style="color:' + color + ';">' + num + '</div><div class="chip-label">' + label +
            '</div>' + (sub ? '<div class="chip-sub">' + sub + '</div>' : '') + '</div></div>';
    }

    /* A6：达标率三态色档（0% 红 / >0% 且 <5% 橙 / ≥5% 绿），阈值纯函数在 common.js */
    var NZ_TIER_COLORS = { good: '#0fd056', warn: '#d97706', bad: '#ef4444' };
    function nzTierClass(qualified, total) {
        var tier = 'good';
        try { tier = window.qualifiedRateTier ? window.qualifiedRateTier(qualified, total) : 'good'; } catch (e) { /* 纯函数缺失时退默认 */ }
        return NZ_TIER_COLORS[tier] ? tier : 'good';
    }
    function nzTierColor(qualified, total) {
        return NZ_TIER_COLORS[nzTierClass(qualified, total)] || NZ_TIER_COLORS.good;
    }
    // Hero 关键数字强调 span；tierClass 传入时按三态着色
    function nzHeroNum(value, tierClass) {
        var cls = 'hero-num' + (tierClass ? ' tier-' + tierClass : '');
        return '<span class="' + cls + '">' + nzEscText(value) + '</span>';
    }

    function nzRefreshAnalysis() {
        // 数据层优先：服务端下发的统计记录（任务完成推送 / 历史查看 / 重启恢复）
        if (window.nzAnalysisData && typeof nzRenderAnalysisFromStats === 'function') {
            try {
                nzRenderAnalysisFromStats(window.nzAnalysisData);
                return;
            } catch (e) { /* 记录字段异常时回落 DOM 聚合 */ }
        }
        var scanRows = nzRows('scanTable');
        var testRows = nzRows('testTable');
        var dcRows = nzRows('dcTable');
        var summaryCard = document.getElementById('analysisSummaryCard');
        var rankCard = document.getElementById('analysisRankCard');
        if (!summaryCard || !rankCard) return;

        var regionIdxScan = nzColIndex('scanTable', ['地区', '落地区域']);
        var regions = {};
        scanRows.forEach(function (tr) {
            var cells = nzCells(tr);
            var v = regionIdxScan >= 0 ? (cells[regionIdxScan] || '') : '';
            if (v) regions[v] = (regions[v] || 0) + 1;
        });
        if (Object.keys(regions).length === 0) {
            var regionIdxTest = nzColIndex('testTable', ['地区']);
            testRows.forEach(function (tr) {
                var cells = nzCells(tr);
                var v = regionIdxTest >= 0 ? (cells[regionIdxTest] || '') : '';
                if (v) regions[v] = (regions[v] || 0) + 1;
            });
        }
        var regionCount = Object.keys(regions).length;
        var qualified = document.querySelectorAll('#testTable tbody tr.row-qualified').length +
            document.querySelectorAll('#scanTable tbody tr.row-qualified').length;

        var hasData = scanRows.length + testRows.length + dcRows.length > 0;
        summaryCard.style.display = hasData ? '' : 'none';
        rankCard.style.display = dcRows.length > 0 ? '' : 'none';
        if (!hasData) return;

        // A6：达标率三态（DOM 兜底路径以扫描结果为分母，无扫描行时退详细测试行）
        var qualifiedTotal = qualified;
        var qualifiedDenom = scanRows.length > 0 ? scanRows.length : testRows.length;
        var tierColor = nzTierColor(qualifiedTotal, qualifiedDenom);
        var qualifiedRate = qualifiedDenom > 0 ? Math.round(qualifiedTotal / qualifiedDenom * 100) : 0;

        var chips = nzChip('📡', '#0ea5a0', scanRows.length, '扫描结果', '当前任务结果行');
        chips += nzChip('🧪', '#8b5cf6', testRows.length, '详细测试', '丢包与延迟明细');
        chips += nzChip('🏢', '#0198d3', dcRows.length, '数据中心', '落地点汇总');
        chips += nzChip('🌍', '#e23b76', regionCount, '覆盖地区', '按结果行地区去重');
        chips += nzChip('✅', tierColor, qualifiedTotal, '达标节点', '达标率 ' + qualifiedRate + '%');
        var chipsEl = document.getElementById('analysisChips');
        if (chipsEl) chipsEl.innerHTML = chips;

        var hero = document.getElementById('analysisHero');
        if (hero) {
            hero.innerHTML = '共 ' + nzHeroNum(scanRows.length) + ' 条扫描结果 · ' + nzHeroNum(testRows.length) + ' 条详细测试' +
                ' · 覆盖 ' + nzHeroNum(regionCount) + ' 个地区 / ' + nzHeroNum(dcRows.length) + ' 个数据中心' +
                ' · 达标率 ' + nzHeroNum(qualifiedRate + '%', nzTierClass(qualifiedTotal, qualifiedDenom));
        }

        /* 数据中心排行：按 IP 数量 */
        var countIdx = nzColIndex('dcTable', ['IP数量', 'IP 数量']);
        var stats = dcRows.map(function (tr) {
            var cells = nzCells(tr);
            var raw = countIdx >= 0 ? parseInt(String(cells[countIdx]).replace(/[^\d]/g, ''), 10) : 0;
            return { name: cells[0] || '-', count: isNaN(raw) ? 0 : raw, meta: cells[1] || '' };
        }).sort(function (a, b) { return b.count - a.count; });
        var rankEl = document.getElementById('analysisRankDC');
        if (rankEl) {
            var max = stats.length ? stats[0].count || 1 : 1;
            var total = stats.reduce(function (s, x) { return s + x.count; }, 0);
            var html = '';
            stats.slice(0, 10).forEach(function (s, i) {
                var pct = total ? Math.round(s.count / total * 100) : 0;
                var nameEsc = nzEscText(s.name);
                html += '<div class="rank-row"><div class="rank-no' + (i < 3 ? ' top' : '') + '">' + (i + 1) + '</div>' +
                    '<div class="rank-name" title="' + nameEsc + '">' + nameEsc + '</div>' +
                    '<div class="rank-bar"><i style="width:' + Math.round(s.count / max * 100) + '%"></i></div>' +
                    '<div class="rank-val">' + s.count + '</div><div class="rank-pct">' + pct + '%</div></div>';
            });
            if (stats.length > 10) {
                html += '<div class="card-tip" style="text-align:center;">其余 ' + (stats.length - 10) + ' 个数据中心未列出</div>';
            }
            rankEl.innerHTML = html || '<div class="analysis-empty">暂无数据</div>';
            var tip = document.getElementById('analysisRankTip');
            if (tip) {
                tip.textContent = stats.length ? ('共 ' + stats.length + ' 个数据中心 · 按落地 IP 数排序') : '';
            }
        }
    }

    // 数据层渲染：用服务端统计记录绘制分析视图（结构与 DOM 聚合路径一致）
    function nzRenderAnalysisFromStats(stats) {
        var summaryCard = document.getElementById('analysisSummaryCard');
        var rankCard = document.getElementById('analysisRankCard');
        if (!summaryCard || !rankCard || !stats) return;
        var dcDist = stats.dcDistribution || {};
        var dcNames = Object.keys(dcDist);
        var regionCount = Object.keys(stats.regionDistribution || {}).length;
        var rate = stats.resultCount > 0 ? Math.round((stats.qualifiedCount || 0) / stats.resultCount * 100) : 0;
        // A6：达标率三态（服务器统计路径）
        var tierColor = nzTierColor(stats.qualifiedCount || 0, stats.resultCount || 0);
        summaryCard.style.display = '';
        rankCard.style.display = dcNames.length > 0 ? '' : 'none';

        var chips = nzChip('📡', '#0ea5a0', stats.resultCount || 0, '总结果', '本次任务统计');
        chips += nzChip('🧪', '#8b5cf6', stats.testCount || 0, '详细测试', '丢包与延迟明细');
        chips += nzChip('🏢', '#0198d3', dcNames.length, '数据中心', '落地点汇总');
        chips += nzChip('🌍', '#e23b76', regionCount, '覆盖地区', '按地区去重');
        chips += nzChip('✅', tierColor, stats.qualifiedCount || 0, '达标节点', '达标率 ' + rate + '%');
        var chipsEl = document.getElementById('analysisChips');
        if (chipsEl) chipsEl.innerHTML = chips;

        var hero = document.getElementById('analysisHero');
        if (hero) {
            var when = stats.startedAt ? new Date(stats.startedAt).toLocaleString('zh-CN', { hour12: false }) : '';
            var modeName = stats.mode === 'nsb' ? '非标优选' : '官方优选';
            hero.innerHTML = nzEscText(stats.label || modeName) + (when ? ' · ' + nzEscText(when) : '') +
                ' · 共 ' + nzHeroNum(stats.resultCount || 0) + ' 条结果 · 覆盖 ' + nzHeroNum(regionCount) + ' 个地区 / ' + nzHeroNum(dcNames.length) + ' 个数据中心' +
                ' · 达标率 ' + nzHeroNum(rate + '%', nzTierClass(stats.qualifiedCount || 0, stats.resultCount || 0)) +
                (stats.failureSummary ? ' · ' + nzEscText(stats.failureSummary) : '');
        }

        var rankEl = document.getElementById('analysisRankDC');
        if (rankEl) {
            var entries = dcNames.map(function (name) { return { name: name, count: dcDist[name] || 0 }; });
            entries.sort(function (a, b) { return b.count - a.count; });
            var max = entries.length ? entries[0].count || 1 : 1;
            var total = entries.reduce(function (sum, x) { return sum + x.count; }, 0);
            var html = '';
            entries.slice(0, 10).forEach(function (entry, i) {
                var pct = total ? Math.round(entry.count / total * 100) : 0;
                var nameEsc = nzEscText(entry.name);
                html += '<div class="rank-row"><div class="rank-no' + (i < 3 ? ' top' : '') + '">' + (i + 1) + '</div>' +
                    '<div class="rank-name" title="' + nameEsc + '">' + nameEsc + '</div>' +
                    '<div class="rank-bar"><i style="width:' + Math.round(entry.count / max * 100) + '%"></i></div>' +
                    '<div class="rank-val">' + entry.count + '</div><div class="rank-pct">' + pct + '%</div></div>';
            });
            if (entries.length > 10) {
                html += '<div class="card-tip" style="text-align:center;">其余 ' + (entries.length - 10) + ' 个数据中心未列出</div>';
            }
            rankEl.innerHTML = html || '<div class="analysis-empty">暂无数据</div>';
            var tip = document.getElementById('analysisRankTip');
            if (tip) {
                tip.textContent = entries.length ? ('共 ' + entries.length + ' 个数据中心 · 按落地 IP 数排序') : '';
            }
        }
        // Top 结果提示（有记录时展示在排行下方）
        var topEl = document.getElementById('analysisTopResults');
        if (topEl) {
            var tops = stats.topResults || [];
            topEl.textContent = tops.length
                ? ('Top: ' + tops.slice(0, 5).map(function (t) { return t.ip + (t.port ? ':' + t.port : '') + (t.latency ? ' ' + t.latency : ''); }).join(' · '))
                : '';
            topEl.style.display = tops.length ? '' : 'none';
        }
    }

    function nzUpdateNavBadge() {
        var n = nzRows('scanTable').length + nzRows('testTable').length;
        var text = n > 999 ? '999+' : String(n);
        var reportBadge = document.getElementById('reportBadge');
        if (reportBadge) {
            reportBadge.textContent = text;
            reportBadge.classList.toggle('show', n > 0);
        }
    }

    /* ---------------- 终端风日志（分级过滤 + 连续重复合并） ---------------- */

    var LOG_MAX = 500;
    var nzLogSelecting = false;
    var nzLogPaused = false;
    var nzLogFilterLevel = 'brief';
    // A3：框选日志期间的行缓冲（选区稳定优先；纯函数在 common.js，旧文件缺失时退直连渲染）
    var nzLogBuffer = typeof window.createLogBuffer === 'function' ? window.createLogBuffer(LOG_MAX) : null;

    // 选区是否有效：shell 内按住且当前选区非空（点击不选中不缓冲）
    function nzHasLogSelection() {
        try {
            var sel = window.getSelection();
            return !!(sel && String(sel).length > 0);
        } catch (e) { return false; }
    }

    // 选区结束：缓冲行按序回灌（nzAppendLogLine 自带 LOG_MAX 截断），丢弃超限先插省略提示
    function nzFlushLogBuffer() {
        if (!nzLogBuffer || nzLogBuffer.size === 0) return;
        var out = nzLogBuffer.drain();
        if (out.omitted > 0) {
            nzAppendLogLine('（框选期间已省略 ' + out.omitted + ' 行日志）');
        }
        out.lines.forEach(function (line) { nzAppendLogLine(line); });
    }

    // 「简要」级别隐藏的高频逐行细节（单点测速、分轮精简、上传重试、界面操作回声等）
    var LOG_DETAIL_RE = /(测速完成: |第 \d+\/\d+ 次上传|^第 \d+ 轮|精简第 \d+ 轮|已复制: |复制失败: |切换至：|已连接服务器|连接断开|已开始跟随后台任务|配置文件已从|当前配置已保存|调试模式已开启)/;
    var LOG_ERROR_RE = /失败|错误|ERR\b|error|鉴权|无效/i;
    var LOG_WARN_RE = /警告|WRN\b|warn/i;

    function nzLogClassify(text) {
        if (LOG_ERROR_RE.test(text)) return 'err';
        if (LOG_WARN_RE.test(text)) return 'warn';
        if (LOG_DETAIL_RE.test(text)) return 'detail';
        return 'info';
    }

    window.nzSetLogFilter = function (level) {
        if (['all', 'brief', 'error'].indexOf(level) < 0) level = 'brief';
        nzLogFilterLevel = level;
        var box = document.getElementById('logBox');
        if (box) {
            box.classList.remove('filter-all', 'filter-brief', 'filter-error');
            box.classList.add('filter-' + level);
        }
        document.querySelectorAll('#logFilter .log-filter-btn').forEach(function (btn) {
            btn.classList.toggle('active', btn.getAttribute('data-level') === level);
        });
    };

    function nzShell() { return document.getElementById('logShell'); }

    function nzNearBottom() {
        var shell = nzShell();
        if (!shell) return true;
        return shell.scrollTop + shell.clientHeight >= shell.scrollHeight - 48;
    }

    function nzUpdateBackCapsule() {
        var cap = document.getElementById('logBackCapsule');
        if (!cap) return;
        cap.classList.toggle('show', nzLogPaused && !nzLogSelecting);
    }

    window.nzLogScrollBottom = function () {
        var shell = nzShell();
        if (!shell) return;
        try { shell.scrollTo({ top: shell.scrollHeight, behavior: 'smooth' }); } catch (e) { shell.scrollTop = shell.scrollHeight; }
        nzLogPaused = false;
        nzUpdateBackCapsule();
    };

    window.nzClearLog = function () {
        var box = document.getElementById('logBox');
        if (box) box.innerHTML = '';
        nzLastLogLine = null;
        nzLastLogText = '';
        if (nzLogBuffer) nzLogBuffer.clear();
        nzUpdateLogCount();
    };

    function nzUpdateLogCount() {
        var box = document.getElementById('logBox');
        var el = document.getElementById('logStatusCount');
        if (box && el) el.textContent = String(box.children.length);
    }

    function nzEscText(s) {
        return window.escapeHTML ? escapeHTML(s) : String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }

    function nzColorizePlain(text) {
        var s = nzEscText(text);
        s = s.replace(/(^|[\s（(：:=＝，,])(\d+(?:\.\d+)?)(?=%|ms|MB\/s|KB\/s|GB|MB|KB|\s|，|,|。|；|;|）|\)|$)/g, function (a, p, n) {
            return p + '<span class="log-num">' + n + '</span>';
        });
        s = s.replace(/(^|\s)(INF|WRN|ERR|DBG)(?=[\s:：]|$)/g, function (a, p, w) {
            var cls = { INF: 'info', WRN: 'warn', ERR: 'error', DBG: 'debug' }[w];
            return p + '<span class="log-level log-level-' + cls + '">' + w + '</span>';
        });
        return s;
    }

    var ANSI_MAP = { 31: 'error', 32: 'info', 33: 'warn', 34: 'debug', 36: 'debug' };
    function nzColorize(raw) {
        raw = String(raw == null ? '' : raw).replace(/\x1b\[(\d+)m/g, function (a, code) {
            if (code === '0') return '\u0000/close\u0000';
            var cls = ANSI_MAP[code];
            return cls ? '\u0000/ansi-' + cls + '\u0000' : '';
        });
        var urlRe = /(https?:\/\/[^\s，。；、（）()<>"]+)/g;
        var out = '';
        var last = 0;
        var m;
        while ((m = urlRe.exec(raw)) !== null) {
            out += nzColorizePlain(raw.slice(last, m.index));
            out += '<span class="log-url">' + nzEscText(m[0]) + '</span>';
            last = m.index + m[0].length;
        }
        out += nzColorizePlain(raw.slice(last));
        out = out.replace(/\u0000\/close\u0000/g, '</span>');
        out = out.replace(/\u0000\/(ansi-[a-z]+)\u0000/g, '<span class="log-level-$1">');
        return out;
    }

    var nzLastLogLine = null;
    var nzLastLogText = '';
    var nzLastLogCount = 0;

    function nzAppendLogLine(text) {
        var shell = nzShell();
        var box = document.getElementById('logBox');
        if (!box) return;
        if (shell) shell.classList.remove('loading');
        text = String(text == null ? '' : text);
        // A3：框选期间不进 DOM（含重复合并、上限删除），只进缓冲，选区结束后统一回灌
        if (nzLogBuffer && nzLogSelecting && nzHasLogSelection()) {
            nzLogBuffer.push(text);
            return;
        }
        // 连续重复的相同日志合并为一行带计数徽标，避免刷屏
        if (nzLastLogLine && nzLastLogText === text && nzLastLogLine.parentNode === box) {
            nzLastLogCount++;
            var dup = nzLastLogLine.querySelector('.log-line-dup');
            if (!dup) {
                dup = document.createElement('span');
                dup.className = 'log-line-dup';
                nzLastLogLine.appendChild(dup);
            }
            dup.textContent = '×' + (nzLastLogCount + 1);
            nzUpdateLogCount();
            return;
        }
        var stick = nzNearBottom();
        var level = nzLogClassify(text);
        var div = document.createElement('div');
        div.className = 'log-line-new log-line-' + level + (level === 'err' ? ' nz-line-err' : (level === 'warn' ? ' nz-line-warn' : ''));
        var time = document.createElement('span');
        time.className = 'log-time';
        time.textContent = '[' + new Date().toLocaleTimeString() + ']';
        div.appendChild(time);
        var body = document.createElement('span');
        body.innerHTML = ' ' + nzColorize(text);
        div.appendChild(body);
        box.appendChild(div);
        nzLastLogLine = div;
        nzLastLogText = text;
        nzLastLogCount = 0;
        while (box.children.length > LOG_MAX) box.removeChild(box.firstChild);
        if (!nzLogPaused && !nzLogSelecting && stick) {
            shell.scrollTop = shell.scrollHeight;
        } else if (!stick) {
            nzLogPaused = true;
        }
        nzUpdateBackCapsule();
        nzUpdateLogCount();
    }

    var _log = log;
    log = function (text) {
        try {
            if (typeof recordWebDebugNotice === 'function') recordWebDebugNotice('web_log', text);
            nzAppendLogLine(String(text == null ? '' : text));
        } catch (e) {
            _log(text);
        }
    };

    /* 日志滚动：上滚暂停 + 回到底部胶囊；框选文本不打断（渲染转缓冲，松开回灌） */
    (function () {
        var shell = nzShell();
        if (!shell) return;
        shell.addEventListener('scroll', function () {
            nzLogPaused = !nzNearBottom();
            nzUpdateBackCapsule();
        });
        shell.addEventListener('mousedown', function () { nzLogSelecting = true; });
        document.addEventListener('mouseup', function () {
            if (nzLogSelecting) {
                nzLogSelecting = false;
                // A3：选区结束，回灌框选期间缓冲的日志行
                try { nzFlushLogBuffer(); } catch (e) { /* 回灌失败不影响主流程 */ }
                nzUpdateBackCapsule();
            }
        });
    })();

    /* ---------------- 主题（data-theme 同步） ---------------- */

    function nzAndroidDark() {
        try { return !!(window.CFDataAndroid && window.CFDataAndroid.isDarkMode()); } catch (e) { return false; }
    }

    var _applyTheme = applyTheme;
    applyTheme = function (theme) {
        _applyTheme(theme);
        var t = theme || localStorage.getItem('cfdata-theme') || 'auto';
        var dark = t === 'dark' || (t === 'auto' && (window.matchMedia('(prefers-color-scheme: dark)').matches || nzAndroidDark()));
        document.documentElement.dataset.theme = dark ? 'dark' : 'light';
        var meta = document.querySelector('meta[name="theme-color"]');
        if (meta) meta.setAttribute('content', dark ? '#121316' : '#f4f6f7');
    };
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function () {
        if ((localStorage.getItem('cfdata-theme') || 'auto') === 'auto') {
            nzSetThemeNow();
        }
    });
    function nzSetThemeNow() {
        try { applyTheme(localStorage.getItem('cfdata-theme') || 'auto'); } catch (e) { /* ignore */ }
    }

    /* ---------------- 状态 pill ---------------- */

    var _updateStatus = updateStatus;
    updateStatus = function (text, connected) {
        var el = document.getElementById('status');
        if (!el) { _updateStatus(text, connected); return; }
        var t = String(text == null ? '' : text);
        el.innerHTML = '<span class="dot"></span>' + nzEscText(t);
        el.classList.toggle('error', !connected);
        el.classList.toggle('running', !!connected && /扫描|运行|测试|测速|进行中/.test(t));
    };

    /* ---------------- 初始化 ---------------- */

    document.addEventListener('DOMContentLoaded', function () {
        // 历史遗留的自动加载偏好值归一化：服务器参数现在总是自动加载
        try {
            var pref = localStorage.getItem('cfdata-autoload');
            if (pref !== 'off' && pref !== 'localStorage') localStorage.setItem('cfdata-autoload', 'off');
        } catch (e) { /* ignore */ }
        // 清理日志框内源码缩进残留的空白文本节点，避免 pre-wrap 把它们渲染成多余空行
        try {
            var logBoxEl = document.getElementById('logBox');
            if (logBoxEl) {
                Array.prototype.slice.call(logBoxEl.childNodes).forEach(function (node) {
                    if (node.nodeType === 3 && !node.textContent.trim()) logBoxEl.removeChild(node);
                });
            }
        } catch (e) { /* ignore */ }
        // 初始日志过滤级别与启动就绪校验
        try { nzSetLogFilter('brief'); } catch (e) { /* ignore */ }
        try { nzUpdateStartReadiness(); } catch (e) { /* 旧函数缺失时忽略 */ }
        // A8：PWA 可安装（零缓存透传 SW，仅满足安装判据；未登录或旧内核时静默失败）
        if ('serviceWorker' in navigator) {
            try {
                navigator.serviceWorker.register('/static/sw.js', { scope: '/' }).catch(function () { /* 未登录/不支持时静默 */ });
            } catch (e) { /* ignore */ }
        }
    });

    nzSetToggleState('idle');
    var pill = document.getElementById('logStatusPill');
    if (pill) pill.innerHTML = '<span class="dot"></span><span id="logStatusCount">0</span>';
    nzSyncProgressMeta();
    try { applyTheme(localStorage.getItem('cfdata-theme') || 'auto'); } catch (e) { /* ignore */ }
})();