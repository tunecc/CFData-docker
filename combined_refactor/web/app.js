        let ws;
        let defaultSpeedTestUrl = "speed.okl.abrdns.com";
        let scheduleConfigLoaded = false;
        let lastSchedulePayload = null;
        let isTaskRunning = false;
        let isStoppingTask = false;
        let nsbReallinkRunning = false;
        let scanData = [];
        let testData = [];
        let dcList = [];
        let testedDCs = new Set();
        let currentTestingDC = '';
        let scanResultsCount = 0;
        let testResultsCount = 0;
        let currentMode = 'official';
        let uploadedFileName = '';
        let uploadedFileContent = '';
        let nsbHeaders = [];
        let nsbRows = [];
        let nsbLiveRows = [];
        let nsbLiveIndex = {};
        let nsbLiveRowElements = {};
        let nsbStopHandledByComplete = false;
        let nsbCompactView = true;
        let nsbTableIPTypeFilter = 'all';
        let nsbTableQualifiedFilter = 'all';
        let nsbTableLatencyFilter = 0;
        let nsbTableLossFilter = 100;
        let nsbSpeedMinValue = 0;
        let nsbSpeedEnabledForResults = false;
        const AUTO_SPEED_URL_VALUE = 'auto';
        const AUTO_SPEED_URL_LABEL = '自动选择';
        let officialTestQualifiedFilter = 'all';
        let officialTestLatencyFilter = 0;
        let officialTestLossFilter = 100;
        let officialSpeedMinValue = 0.1;
        let officialSpeedEnabledForResults = false;
        let officialAutoSpeedPending = false;
        let officialAutoStage = '';
        let officialBatchSpeedRunning = false;
        let nsbBatchSpeedRunning = false;
        let nsbContinueSpeedUnlocked = false;
        let officialTestCompletePending = false;
        let officialSpeedEndedByRateLimit = false;
        let officialContinueSpeedUnlocked = false;
        // 任务恢复状态：连接建立后服务端下发 task_restored / task_restore_chunk /
        // task_restore_done，前端据此重建运行视图；seq 用于过滤已包含在恢复载荷里的旧消息
        let taskRestorePending = false;
        let taskRestoreLastSeq = 0;
        let taskRestoreBuffer = [];
        let taskRestoreData = null;
        let webDebugMode = false;
        let webDebugErrors = [];
        let currentSpeedTestingIP = '';
        let runningTaskLabel = '';
        let runningTaskMode = '';
        let runningTaskTab = 'scan';
        let dcLibraryStatusMap = {};
        let officialDcSelection = '';
        let dcLibraryScanning = false;
        let stopWatchdogTimer = null;
        let cfCountry = '检测失败';
        let proxyWarningRequired = false;
        let proxyGeoCheckOK = true;
        let proxyWarningBypassed = false;
        let proxyWarningPendingAction = null;
        let proxyWarningCountdownTimer = null;
        let proxyCountryCheckPending = false;
        let proxyCountryCheckTimer = null;
        let proxyWarningSinglePass = false;
        let resultModalMode = 'scan';
        let copyModalMode = 'scan';
        let sortModalMode = 'official';
        let resultFieldOrder = [];
        let resultCustomFields = [];
        let copyFieldOrder = [];
        let copyCustomFields = [];
        let autoUploadKeys = new Set();
        let noticeConfirmAction = null;
        const githubAccountStorageKey = 'cfdata-github-accounts';
        const githubSessionTokens = new Map();
        const githubTokenCryptoSalt = 'cfdata-github-token-v1';
        const edgetunnelAccountStorageKey = 'cfdata-edgetunnel-accounts';
        const edgetunnelSessionPasswords = new Map();

        const LATENCY_COLORS = [
            { end: 50, color: '#1f8a3b' },
            { end: 100, color: '#4caf50' },
            { end: 200, color: '#9ccc65' },
            { end: 250, color: '#e0c341' },
            { end: 3000, color: '#e67e22' },
            { end: Infinity, color: '#d64541' },
        ];
        const SPEED_GREEN = 10;

        const statusEl = document.getElementById('status');
        const progressBox = document.getElementById('progressBox');
        const progressBar = document.getElementById('progressBar');

        const modeOptionOfficial = document.getElementById('modeOptionOfficial');
        const modeOptionNSB = document.getElementById('modeOptionNSB');
        const officialControls = document.getElementById('officialControls');
        const nsbControls = document.getElementById('nsbControls');
        const fileInput = document.getElementById('nsbFileInput');
        const nsbSourceUrlInput = document.getElementById('nsbSourceUrl');

        document.getElementById('nsbFileInput').addEventListener('change', e => {
            const file = e.target.files[0];
            if (!file) {
                uploadedFileName = '';
                uploadedFileContent = '';
                syncNSBInputMode();
                return;
            }
            uploadedFileName = file.name || 'upload.txt';
            const reader = new FileReader();
            reader.onload = () => {
                uploadedFileContent = String(reader.result || '');
                syncNSBInputMode('file');
                log(`已选择文件: ${uploadedFileName}`);
            };
            reader.onerror = () => log('读取文件失败');
            reader.readAsText(file);
        });

        function syncNSBInputMode(preferred) {
            const hasFile = !!(fileInput && fileInput.files && fileInput.files.length);
            const hasURL = !!(nsbSourceUrlInput && nsbSourceUrlInput.value.trim());
            if (preferred === 'file' && hasFile && nsbSourceUrlInput) {
                nsbSourceUrlInput.value = '';
            } else if (preferred === 'url' && hasURL && fileInput) {
                fileInput.value = '';
                uploadedFileName = '';
                uploadedFileContent = '';
            }
            if (typeof nzUpdateStartReadiness === 'function') nzUpdateStartReadiness();
        }

        function applyTheme(theme) {
            const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            let androidDark = false;
            try { if (window.CFDataAndroid && window.CFDataAndroid.isDarkMode()) androidDark = true; } catch(e) {}

            function setDark(dark) {
                document.documentElement.classList.toggle('dark-theme', dark);
                document.body.classList.toggle('dark-theme', dark);
                try { if (window.CFDataAndroid) window.CFDataAndroid.setDarkModeStatusBar(dark); } catch(e) {}
            }

            if (theme === 'auto') {
                setDark(prefersDark || androidDark);
                localStorage.setItem('cfdata-theme', 'auto');
                const autoBtn = document.getElementById('theme-auto');
                const lightBtn = document.getElementById('theme-light');
                const darkBtn = document.getElementById('theme-dark');
                if (autoBtn) autoBtn.classList.add('active');
                if (lightBtn) lightBtn.classList.remove('active');
                if (darkBtn) darkBtn.classList.remove('active');
                return;
            }

            setDark(theme === 'dark');
            localStorage.setItem('cfdata-theme', theme);
            const autoBtn = document.getElementById('theme-auto');
            const lightBtn = document.getElementById('theme-light');
            const darkBtn = document.getElementById('theme-dark');
            if (autoBtn) autoBtn.classList.remove('active');
            if (lightBtn) lightBtn.classList.toggle('active', theme === 'light');
            if (darkBtn) darkBtn.classList.toggle('active', theme === 'dark');
        }

        function setTheme(theme) {
            applyTheme(theme);
        }

        function toggleSettingsMenu(event) {
            if (event) {
                event.stopPropagation();
            }
            const menu = document.getElementById('settingsMenu');
            menu.classList.toggle('hidden');
            if (!menu.classList.contains('hidden')) {
                menu.classList.add('no-transition');
                setTimeout(() => menu.classList.remove('no-transition'), 0);
                wsSend({ type: 'get_management_proxy' });
            }
        }

        function toggleConfigMenu(event) {
            event.stopPropagation();
            const menu = document.getElementById('configMenu');
            menu.classList.toggle('hidden');
            if (!menu.classList.contains('hidden')) {
                const pref = localStorage.getItem('cfdata-autoload') || 'off';
                document.getElementById('configAutoLoad').value = pref;
            }
        }

        document.addEventListener('click', (event) => {
            const menu = document.getElementById('settingsMenu');
            const btn = document.getElementById('settingsToggle');
            if (!menu || !btn) return;
            if (!menu.contains(event.target) && !btn.contains(event.target)) {
                const proxyInput = document.getElementById('proxyInput');
                // 未保存的手动输入不能因误点空白区域而丢失
                if (!proxyInput || !proxyInput.value.trim()) {
                    menu.classList.add('hidden');
                }
            }
            const configMenu = document.getElementById('configMenu');
            const configBtn = document.getElementById('btnConfigMenu');
            if (configMenu && configBtn) {
                if (!configMenu.contains(event.target) && !configBtn.contains(event.target)) {
                    configMenu.classList.add('hidden');
                }
            }
        });

        function initThemeFromStorage() {
            const saved = localStorage.getItem('cfdata-theme') || 'auto';
            applyTheme(saved);
        }

        function initializeUI() {
            updateNSBSpeedControls();
            updateBatchSpeedButton();
        }

        function updateNSBSpeedControls() {
            const enableSpeedTest = document.getElementById('nsbEnableSpeed');
            const speedTestInput = document.getElementById('nsbSpeedTest');
            if (!enableSpeedTest || !speedTestInput) return;

            const speedEnabled = String(enableSpeedTest.value || 'off').toLowerCase() === 'on';
            if (speedEnabled) {
                speedTestInput.disabled = false;
                if (String(speedTestInput.value).trim() === '0') {
                    speedTestInput.value = '5';
                }
                updateNSBSpeedButtons();
                return;
            }

            speedTestInput.value = '0';
            speedTestInput.disabled = true;
            updateNSBSpeedButtons();
        }

        function updateNSBFallbackPortDefault() {
            const portInput = document.getElementById('nsbFallbackPort');
            const tlsMode = String(document.getElementById('nsbTls')?.value || 'on').toLowerCase();
            if (!portInput) return;
            const current = parseInt(portInput.value, 10) || 0;
            if (tlsMode === 'off' && (current <= 0 || current === 443)) {
                portInput.value = '80';
            } else if (tlsMode !== 'off' && (current <= 0 || current === 80)) {
                portInput.value = '443';
            }
        }

        function applySpeedUrlPreset(targetId, value) {
            const input = document.getElementById(targetId);
            if (!input) return;
            input.readOnly = false;
            if (value === 'custom') {
                input.value = '';
                input.focus();
                return;
            }
            if (value === AUTO_SPEED_URL_VALUE) {
                input.value = AUTO_SPEED_URL_LABEL;
                input.readOnly = true;
                return;
            }
            input.value = value || defaultSpeedTestUrl;
        }

        function resolveSpeedTestUrl(targetId, presetId) {
            const input = document.getElementById(targetId);
            const preset = document.getElementById(presetId);
            if (preset && preset.value === AUTO_SPEED_URL_VALUE) {
                return AUTO_SPEED_URL_VALUE;
            }
            const value = input ? input.value.trim() : '';
            if (!value || value === AUTO_SPEED_URL_LABEL) return AUTO_SPEED_URL_VALUE;
            return value;
        }

        function markSpeedUrlCustom(presetId) {
            const preset = document.getElementById(presetId);
            if (preset) preset.value = 'custom';
        }

        function syncSpeedUrlPreset(targetId, presetId) {
            const input = document.getElementById(targetId);
            const preset = document.getElementById(presetId);
            if (!input || !preset) return;
            const value = input.value.trim();
            if (value === AUTO_SPEED_URL_LABEL) {
                preset.value = AUTO_SPEED_URL_VALUE;
                input.readOnly = true;
                return;
            }
            const option = Array.from(preset.options).find((item) => item.value === value);
            preset.value = option ? value : 'custom';
            input.readOnly = false;
        }

        function resetDefaultSpeedUrl(targetId = 'officialSpeedUrl', presetId = '') {
            const input = document.getElementById(targetId);
            if (!input) return;
            input.value = AUTO_SPEED_URL_LABEL;
            input.readOnly = true;
            const preset = presetId ? document.getElementById(presetId) : null;
            if (preset) preset.value = AUTO_SPEED_URL_VALUE;
            if (presetId) syncSpeedUrlPreset(targetId, presetId);
        }

        function applySpeedUrlConfig(targetId, presetId, value) {
            const input = document.getElementById(targetId);
            const preset = document.getElementById(presetId);
            if (!input || !preset) return;
            const resolved = String(value || '').trim();
            if (!resolved || resolved === AUTO_SPEED_URL_VALUE || resolved === AUTO_SPEED_URL_LABEL) {
                resetDefaultSpeedUrl(targetId, presetId);
                return;
            }
            input.value = resolved;
            input.readOnly = false;
            syncSpeedUrlPreset(targetId, presetId);
        }

        function applyProxyStatus(payload) {
            const source = (payload && payload.source) || '';
            const enabled = source === '程序配置' || source === '系统环境变量';
            const statusEl = document.getElementById('proxyStatusText');
            if (statusEl) {
                statusEl.textContent = enabled ? `已启用: ${payload.label}` : '未启用（可用系统环境变量代理）';
            }
            const sourceEl = document.getElementById('proxySourceText');
            if (sourceEl) sourceEl.textContent = source ? `代理来源: ${source}` : '代理来源: 未设置';
            const hintEl = document.getElementById('uploadProxyHint');
            if (hintEl) {
                hintEl.textContent = enabled
                    ? `上传失败？可在设置菜单更换/清除代理后重试；当前代理: ${payload.label}。扫描测速不受代理影响。`
                    : '上传失败？可在设置菜单配置代理后重试；支持程序配置（socks5://、http:// 等）与系统环境变量代理，扫描测速不受影响。';
            }
        }

        function saveManagementProxy() {
            const input = document.getElementById('proxyInput');
            const value = input ? input.value.trim() : '';
            if (!value) {
                showNoticeCard('请输入代理地址，例如 socks5://127.0.0.1:1080；(清除请用「清除」按钮)');
                return false;
            }
            if (!wsSend({ type: 'set_management_proxy', data: { value } })) {
                showNoticeCard('与服务端的连接尚未就绪，请稍后再试');
                return false;
            }
            return true;
        }

        function clearManagementProxy() {
            if (!wsSend({ type: 'set_management_proxy', data: { value: '' } })) {
                showNoticeCard('与服务端的连接尚未就绪，请稍后再试');
            }
        }

        function applyScheduleConfig(payload) {
            if (!payload) return;
            lastSchedulePayload = payload;
            const setValue = (id, value) => {
                const el = document.getElementById(id);
                if (el) el.value = value ?? '';
            };
            const setChecked = (id, value) => {
                const el = document.getElementById(id);
                if (el) el.checked = !!value;
            };
            setChecked('scheduleEnabled', payload.scheduleEnabled);
            setValue('scheduleCron', payload.cronExpression || '');
            setValue('scheduleInterval', payload.checkInterval || '');
            // 回显后按当前值重绘 Cron 分段展示层与间隔联动
            if (typeof renderScheduleCronDisplay === 'function') renderScheduleCronDisplay();
            setChecked('r2Enabled', payload.r2Enabled);
            setValue('r2WorkerUrl', payload.r2WorkerUrl || '');
            const tokenInput = document.getElementById('r2WorkerToken');
            if (tokenInput) {
                tokenInput.value = '';
                tokenInput.dataset.tokenSet = payload.r2WorkerTokenSet ? '1' : '';
                tokenInput.placeholder = payload.r2WorkerTokenSet ? '已保存，留空保持不变' : 'Worker token';
            }
            setValue('r2Filename', payload.r2Filename || '');
            setValue('r2AliasFormat', payload.r2AliasFormat || '');
            setValue('notifyBarkServer', payload.notifyBarkServer || '');
            const barkInput = document.getElementById('notifyBarkKey');
            if (barkInput) {
                barkInput.value = '';
                barkInput.dataset.tokenSet = payload.notifyBarkKeySet ? '1' : '';
                barkInput.placeholder = payload.notifyBarkKeySet ? '已保存，留空保持不变' : 'Bark key';
            }
            const tgTokenInput = document.getElementById('notifyTelegramToken');
            if (tgTokenInput) {
                tgTokenInput.value = '';
                tgTokenInput.dataset.tokenSet = payload.notifyTelegramTokenSet ? '1' : '';
                tgTokenInput.placeholder = payload.notifyTelegramTokenSet ? '已保存，留空保持不变' : 'Telegram bot token';
            }
            setValue('notifyTelegramChatID', payload.notifyTelegramChatID || '');
            // 别名已字段化：加载后立即按「字段与顺序」重新生成，历史模板值不残留到保存链路
            if (typeof syncR2AliasTemplate === 'function') syncR2AliasTemplate();
            r2AutoUploadReady = !!(payload.r2Enabled && payload.r2WorkerUrl && payload.r2WorkerTokenSet);
            syncAutoUploadSelection();
            renderScheduleStatus(payload);
            renderR2UploadFeedback(payload);
        }

        function renderR2UploadFeedback(payload) {
            const el = document.getElementById('r2UploadFeedback');
            if (!el) return;
            const workerUrl = (payload.r2WorkerUrl || '').trim();
            if (!payload.saved || !payload.r2Enabled || !workerUrl) {
                el.style.display = 'none';
                return;
            }
            const filename = (payload.r2Filename || '').trim() || 'cfdata-targets.txt';
            const url = workerUrl.replace(/\/+$/, '') + '/' + filename;
            el.innerHTML = 'R2 生效地址：<strong>' + escapeHTML(url) + '</strong> —— 旧文件不会被自动删除，下游订阅地址需同步更换。';
            el.style.display = '';
        }

        function formatScheduleTime(value) {
            if (!value) return '';
            const date = new Date(value);
            return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false });
        }

        function renderScheduleStatus(payload) {
            const el = document.getElementById('scheduleStatusText');
            if (!el) return;
            const status = payload.status || {};
            const parts = [];
            if (!payload.scheduleEnabled) {
                parts.push('调度未启用');
            } else if (status.mode === 'cron') {
                parts.push('Cron 模式');
            } else {
                const minutes = status.intervalMinutes || payload.effectiveIntervalMinutes;
                parts.push(`每 ${minutes} 分钟`);
            }
            if (status.running) parts.push('扫描进行中');
            if (status.nextRun) parts.push('下次: ' + formatScheduleTime(status.nextRun));
            if (status.lastRun) {
                parts.push('上次: ' + formatScheduleTime(status.lastRun) + (status.lastResult ? ' ' + status.lastResult : ''));
            }
            if (payload.warning) parts.push('⚠ ' + payload.warning);
            el.textContent = parts.join(' · ');
        }

        // ── Cron 分段控件（照 subs-check-pro：分段着色展示 + 点击编辑 + 实时校验 + 间隔联动） ──
        const CRON_SEGMENTS = [
            { label: '分钟', title: '分钟 (0-59)', min: 0, max: 59 },
            { label: '小时', title: '小时 (0-23)', min: 0, max: 23 },
            { label: '日', title: '日 (1-31)', min: 1, max: 31 },
            { label: '月份', title: '月份 (1-12)', min: 1, max: 12 },
            { label: '星期', title: '星期 (0-7，0=周日)', min: 0, max: 7 },
        ];

        // 校验单个字段（支持 * 、步进 a/b、区间 a-b、列表逗号分隔）
        function cronValidateSegment(seg, fieldIdx) {
            const { label: name, min, max } = CRON_SEGMENTS[fieldIdx];
            const stepMatch = seg.match(/^(.+?)\/(\d+)$/);
            const base = stepMatch ? stepMatch[1] : seg;
            const step = stepMatch ? Number(stepMatch[2]) : null;
            if (step !== null && step < 1) return `${name}：步进值必须 ≥ 1`;
            const inRange = (n) => { const v = Number(n); return Number.isInteger(v) && v >= min && v <= max; };
            if (base === '*') return null;
            const rangeMatch = base.match(/^(\d+)-(\d+)$/);
            if (rangeMatch) {
                const [, lo, hi] = rangeMatch;
                if (!inRange(lo) || !inRange(hi)) return `${name}：范围 ${lo}-${hi} 超出 ${min}–${max}`;
                if (Number(lo) > Number(hi)) return `${name}：起始值 ${lo} 大于结束值 ${hi}`;
                return null;
            }
            if (/^\d+$/.test(base)) {
                if (!inRange(base)) return `${name}：${base} 超出范围 ${min}–${max}`;
                return null;
            }
            return `${name}：含非法字符 "${base}"`;
        }

        // 编辑中：只校验已输入的字段，字段数不足不报错
        function cronValidatePartial(val) {
            if (!val || !val.trim()) return null;
            const parts = val.trim().split(/\s+/);
            for (let i = 0; i < Math.min(parts.length, 5); i++) {
                for (const seg of parts[i].split(',')) {
                    const err = cronValidateSegment(seg, i);
                    if (err) return err;
                }
            }
            return null;
        }

        // 失焦：完整校验（含字段数量）
        function cronValidateFull(val) {
            if (!val || !val.trim()) return null;
            const parts = val.trim().split(/\s+/);
            if (parts.length !== 5) return `需要 5 个字段（分 时 日 月 周），当前 ${parts.length} 个`;
            return cronValidatePartial(val);
        }

        function cronIsValid(expr) {
            if (!expr || !expr.trim()) return false;
            const parts = expr.trim().split(/\s+/);
            if (parts.length !== 5) return false;
            return parts.every((p, i) => p.split(',').every(seg => seg.length > 0 && cronValidateSegment(seg, i) === null));
        }

        function cronToast(msg) {
            if (typeof window.nzToast === 'function') window.nzToast(msg, 'error');
        }

        // 重绘分段展示层，并同步「检测间隔」的灰化联动
        function renderScheduleCronDisplay() {
            const display = document.getElementById('scheduleCronDisplay');
            const labelsRow = document.getElementById('scheduleCronLabels');
            const input = document.getElementById('scheduleCron');
            if (!display || !labelsRow || !input) return;
            const raw = (input.value || '').trim();
            const parts = raw ? raw.split(/\s+/) : [];
            const valid = cronIsValid(raw);
            display.className = 'cfg-cron-display' + (raw && !valid ? ' cfg-cron-display--error' : '');
            display.innerHTML = '';
            if (!raw) {
                labelsRow.style.display = 'none';
                const ph = document.createElement('span');
                ph.className = 'cfg-cron-placeholder';
                ph.textContent = input.placeholder || '0 4 */2 * *';
                display.appendChild(ph);
            } else if (valid) {
                labelsRow.style.display = '';
                parts.forEach((part, i) => {
                    const seg = document.createElement('span');
                    seg.className = `cfg-cron-seg cfg-cron-seg-${i}`;
                    seg.textContent = part;
                    seg.title = CRON_SEGMENTS[i].title;
                    display.appendChild(seg);
                });
            } else {
                labelsRow.style.display = 'none';
                const rawText = document.createElement('span');
                rawText.className = 'cfg-cron-raw-text';
                rawText.textContent = raw;
                display.appendChild(rawText);
            }
            updateScheduleCronLinkage();
        }

        // Cron 合法 → 检测间隔灰化禁用 + 绿色徽标；为空/非法 → 间隔可编辑 + 橙色徽标
        function updateScheduleCronLinkage() {
            const input = document.getElementById('scheduleCron');
            const intervalField = document.getElementById('scheduleIntervalField');
            const intervalInput = document.getElementById('scheduleInterval');
            const badge = document.getElementById('scheduleCronBadge');
            if (!input || !intervalField || !badge) return;
            const clockIcon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><polyline points="12 6 12 12 16 14"></polyline></svg>';
            if (cronIsValid(input.value)) {
                intervalField.classList.add('cfg-field--muted');
                if (intervalInput) intervalInput.disabled = true;
                badge.className = 'cfg-cron-badge cfg-cron-badge--active';
                badge.innerHTML = clockIcon + '<span>定时计划已启用 · 禁用检测间隔</span>';
            } else {
                intervalField.classList.remove('cfg-field--muted');
                if (intervalInput) intervalInput.disabled = false;
                badge.className = 'cfg-cron-badge cfg-cron-badge--idle';
                badge.innerHTML = clockIcon + '<span>定时计划未配置 · 检测间隔生效</span>';
            }
        }

        function scheduleCronEnterEdit() {
            const display = document.getElementById('scheduleCronDisplay');
            const labelsRow = document.getElementById('scheduleCronLabels');
            const input = document.getElementById('scheduleCron');
            if (!display || !input) return;
            display.style.display = 'none';
            if (labelsRow) labelsRow.style.display = 'none';
            input.style.display = '';
            requestAnimationFrame(() => { input.focus(); input.select(); });
        }

        function scheduleCronExitEdit() {
            const display = document.getElementById('scheduleCronDisplay');
            const labelsRow = document.getElementById('scheduleCronLabels');
            const input = document.getElementById('scheduleCron');
            if (!display || !input) return;
            const err = cronValidateFull(input.value);
            if (err) cronToast(err);
            input.classList.remove('cfg-cron-input--error');
            input.style.display = 'none';
            display.style.display = '';
            renderScheduleCronDisplay();
        }

        function bindScheduleCronControl() {
            const display = document.getElementById('scheduleCronDisplay');
            const labelsRow = document.getElementById('scheduleCronLabels');
            const input = document.getElementById('scheduleCron');
            if (!display || !labelsRow || !input || display.dataset.bound === '1') return;
            display.dataset.bound = '1';
            labelsRow.innerHTML = CRON_SEGMENTS.map((s, i) =>
                `<span class="cfg-cron-label cfg-cron-label-${i}" title="${s.title}">${s.label}</span>`).join('');
            let toastTimer = null;
            display.addEventListener('click', scheduleCronEnterEdit);
            display.addEventListener('keydown', (e) => {
                if (e.key === 'Enter' || e.key === 'F2' || (e.key === ' ' && !e.ctrlKey)) {
                    e.preventDefault();
                    scheduleCronEnterEdit();
                }
            });
            input.addEventListener('input', () => {
                const err = cronValidatePartial(input.value);
                input.classList.toggle('cfg-cron-input--error', err !== null);
                clearTimeout(toastTimer);
                if (!err) { updateScheduleCronLinkage(); return; }
                toastTimer = setTimeout(() => cronToast('Cron 错误：' + err), 700);
            });
            input.addEventListener('blur', scheduleCronExitEdit);
            renderScheduleCronDisplay();
        }

        // ---- 任务历史（数据层分析来源 + 列表管理）----
        function nzLoadHistoryList() {
            wsSend({ type: 'history_list', data: {} });
        }
        function nzHistoryView(file) {
            if (!file) return;
            wsSend({ type: 'history_get', data: { file } });
        }
        function nzHistoryDelete(file) {
            if (!file) return;
            wsSend({ type: 'history_delete', data: { file } });
        }
        function nzClearHistory() {
            showConfirmCard('确定删除全部任务历史记录吗？此操作不可恢复。', () => wsSend({ type: 'history_clear', data: {} }), '清空历史');
        }
        function renderHistoryList(items) {
            const list = document.getElementById('historyList');
            if (!list) return;
            if (!items || !items.length) {
                list.innerHTML = '<div class="automation-hint">暂无历史记录</div>';
                return;
            }
            const rows = items.map((it) => {
                const date = new Date(it.startedAt);
                const when = Number.isNaN(date.getTime()) ? String(it.startedAt || '') : date.toLocaleString('zh-CN', { hour12: false });
                const modeName = it.mode === 'nsb' ? '非标' : '官方';
                const rate = it.resultCount > 0 ? Math.round((it.qualifiedCount || 0) / it.resultCount * 100) : 0;
                const dur = it.durationSec >= 60 ? Math.floor(it.durationSec / 60) + 'm' + (it.durationSec % 60) + 's' : (it.durationSec || 0) + 's';
                const file = escapeHTML(it.file || '');
                return '<div class="history-row" data-file="' + file + '" onclick="nzHistoryView(\'' + file + '\')">' +
                    '<span class="history-when">' + when + '</span><span class="history-mode">' + modeName + '</span>' +
                    '<span class="history-count">' + (it.resultCount || 0) + ' 个 · 达标 ' + rate + '%</span>' +
                    '<span class="history-dur">' + dur + '</span>' +
                    '<button class="mini-action-btn" title="删除该记录" onclick="event.stopPropagation(); nzHistoryDelete(\'' + file + '\')">🗑️</button></div>';
            });
            list.innerHTML = rows.join('');
        }

        function sendNotifyTest(channel) {
            const feedback = document.getElementById('notifyTestFeedback');
            if (feedback) {
                feedback.textContent = '正在发送测试通知…';
                feedback.style.display = '';
            }
            if (!wsSend({ type: 'notify_test', data: { channel } })) {
                if (feedback) feedback.textContent = '与服务端的连接尚未就绪，请稍后再试';
            }
        }

        function saveScheduleConfig() {
            const textValue = (id) => document.getElementById(id)?.value ?? '';
            const checkedValue = (id) => !!document.getElementById(id)?.checked;
            const interval = parseInt(textValue('scheduleInterval'), 10);
            const payload = {
                scheduleEnabled: checkedValue('scheduleEnabled'),
                cronExpression: textValue('scheduleCron').trim(),
                checkInterval: Number.isFinite(interval) ? interval : 0,
                r2Enabled: checkedValue('r2Enabled'),
                r2WorkerUrl: textValue('r2WorkerUrl').trim(),
                r2Filename: textValue('r2Filename').trim(),
                r2AliasFormat: textValue('r2AliasFormat').trim(),
                notifyBarkServer: textValue('notifyBarkServer').trim(),
                notifyTelegramChatID: textValue('notifyTelegramChatID').trim(),
            };
            // 留空表示不修改已保存的 token
            const token = textValue('r2WorkerToken').trim();
            if (token) payload.r2WorkerToken = token;
            // 通知凭据同样留空保持不变
            const barkKey = textValue('notifyBarkKey').trim();
            if (barkKey) payload.notifyBarkKey = barkKey;
            const tgToken = textValue('notifyTelegramToken').trim();
            if (tgToken) payload.notifyTelegramToken = tgToken;
            if (!wsSend({ type: 'set_schedule_config', data: payload })) {
                showNoticeCard('与服务端的连接尚未就绪，请稍后再试');
                return false;
            }
            return true;
        }

        function applySpeedURLStatus(payload) {
            if (!payload || !Array.isArray(payload.urls)) return;
            const entries = payload.urls;
            const presetIds = ['officialSpeedUrlPreset', 'nsbSpeedUrlPreset'];
            for (const presetId of presetIds) {
                const preset = document.getElementById(presetId);
                if (!preset) continue;
                for (const opt of preset.options) {
                    if (opt.value === AUTO_SPEED_URL_VALUE || opt.value === 'custom') continue;
                    let label = opt.textContent.replace(/（不可用）$/, '');
                    const entry = entries.find((item) => item.url === opt.value);
                    if (entry && entry.tested && entry.available === false) label += '（不可用）';
                    opt.textContent = label;
                }
            }
        }

        function resetAllConfig() {
            showConfirmCard('这会清掉本地缓存文件：ips-v4.txt、ips-v6.txt、locations.json、GeoLite2-ASN.mmdb。\n要继续吗？', confirmResetAllConfig, '清理本地缓存');
        }

        function confirmResetAllConfig() {
            if (!wsSend({ type: 'reset_all_config' })) {
                showNoticeCard('与服务端的连接尚未就绪，请稍后再试');
                return;
            }
            document.body.classList.add('no-transition');
            setTimeout(() => document.body.classList.remove('no-transition'), 500);
        }

        function readFormValues() {
            function val(id) {
                const el = document.getElementById(id);
                return el ? el.value : '';
            }
            function selVal(id) {
                const el = document.getElementById(id);
                return el ? el.value : 'off';
            }
            return {
                official: {
                    scanMode: selVal('officialScanMode'),
                    ipType: selVal('officialIpType'),
                    port: selVal('officialTestPort'),
                    threads: val('officialThreads'),
                    delay: val('officialDelay'),
                    speedMin: val('officialSpeedMin'),
                    speedLimit: val('officialSpeedLimit'),
                    autoSpeed: selVal('officialAutoSpeed'),
                    targetDC: getOfficialTargetDC(),
                    stopCount: val('officialStopCount'),
                    speedUrl: resolveSpeedTestUrl('officialSpeedUrl', 'officialSpeedUrlPreset'),
                },
                nsb: {
                    scanMode: selVal('nsbScanMode'),
                    threads: val('nsbThreads'),
                    delay: val('nsbDelay'),
                    resultLimit: val('nsbResultLimit'),
                    fallbackPort: val('nsbFallbackPort'),
                    speedMin: val('nsbSpeedMin'),
                    speedLimit: val('nsbSpeedLimit'),
                    speedTest: val('nsbSpeedTest'),
                    enableSpeed: selVal('nsbEnableSpeed'),
                    targetDC: val('nsbTargetDC'),
                    tls: selVal('nsbTls'),
                    compactView: selVal('nsbCompactView'),
                    outFile: val('nsbOutFile'),
                    speedUrl: resolveSpeedTestUrl('nsbSpeedUrl', 'nsbSpeedUrlPreset'),
                    sourceUrl: val('nsbSourceUrl'),
                    relinkPreset: selVal('nsbRelinkPreset'),
                    relinkHost: val('nsbRelinkHost'),
                },
            };
        }

        function setValue(id, value) {
            const el = document.getElementById(id);
            if (el === null || el === undefined || value === undefined || value === null || value === '') return;
            el.value = value;
        }

        function applyConfigValues(config) {
            const o = config.official || {};
            const n = config.nsb || {};
            setValue('officialScanMode', o.scanMode);
            setValue('officialIpType', o.ipType);
            setValue('officialTestPort', o.port);
            setValue('officialThreads', o.threads);
            setValue('officialDelay', o.delay);
            setValue('officialSpeedMin', o.speedMin);
            setValue('officialSpeedLimit', o.speedLimit);
            setValue('officialAutoSpeed', o.autoSpeed);
            if (o.targetDC !== undefined && o.targetDC !== null) applyOfficialTargetDCSelection(o.targetDC);
            setValue('officialStopCount', o.stopCount);
            setValue('nsbScanMode', n.scanMode);
            setValue('nsbThreads', n.threads);
            setValue('nsbDelay', n.delay);
            setValue('nsbResultLimit', n.resultLimit);
            setValue('nsbFallbackPort', n.fallbackPort);
            setValue('nsbSpeedMin', n.speedMin);
            setValue('nsbSpeedLimit', n.speedLimit);
            setValue('nsbSpeedTest', n.speedTest);
            setValue('nsbEnableSpeed', n.enableSpeed);
            setValue('nsbTargetDC', n.targetDC);
            if (n.tls) setValue('nsbTls', n.tls);
            if (n.compactView) setValue('nsbCompactView', n.compactView);
            setValue('nsbOutFile', n.outFile);
            setValue('nsbSourceUrl', n.sourceUrl);
            if (n.relinkPreset !== undefined) {
                setValue('nsbRelinkPreset', n.relinkPreset);
                if (n.relinkHost !== undefined) setValue('nsbRelinkHost', n.relinkHost);
                syncRelinkControls();
            }
            if (o.speedUrl) applySpeedUrlConfig('officialSpeedUrl', 'officialSpeedUrlPreset', o.speedUrl);
            if (n.speedUrl) applySpeedUrlConfig('nsbSpeedUrl', 'nsbSpeedUrlPreset', n.speedUrl);
            syncOfficialSpeedConfigState();
            syncNSBSpeedConfigState();
            updateNSBSpeedControls();
            updateNSBCompactToggle();
            // 恢复保存的任务模式；任务运行中不切换
            if (!isTaskRunning && config.mode && (config.mode === 'official' || config.mode === 'nsb') && config.mode !== currentMode) {
                switchMode(config.mode);
            }
            if (typeof nzUpdateStartReadiness === 'function') nzUpdateStartReadiness();
        }

        function saveConfigToLS() {
            const config = readFormValues();
            try {
                localStorage.setItem('cfdata-config', JSON.stringify(config));
            } catch (e) {
                showNoticeCard('保存配置失败: ' + (e.message || String(e)));
                return false;
            }
            log('当前配置已保存到 localStorage');
            return true;
        }

        function loadConfigFromLS() {
            let raw;
            try {
                raw = localStorage.getItem('cfdata-config');
            } catch (e) {
                showNoticeCard('读取配置失败: ' + (e.message || String(e)));
                return;
            }
            if (!raw) {
                showNoticeCard('localStorage 中未找到保存的配置');
                return;
            }
            try {
                applyConfigValues(JSON.parse(raw));
            } catch (e) {
                showNoticeCard('解析配置失败: ' + (e.message || String(e)));
                return;
            }
            log('配置文件已从 localStorage 加载');
        }

        function mapCLIConfigToForm(cliCfg) {
            function pick(map, keys) {
                for (const k of keys) {
                    if (map[k] !== undefined && map[k] !== null && map[k] !== '') return map[k];
                }
                return undefined;
            }
            const c = cliCfg.config || cliCfg;
            const result = { official: {}, nsb: {} };
            const sm = pick(c, ['offscanmode', 'scanmode', 'scanMode']);
            if (sm !== undefined) result.official.scanMode = sm;
            const nsm = pick(c, ['nsbscanmode', 'scanmode', 'scanMode']);
            if (nsm !== undefined) result.nsb.scanMode = nsm;
            const autospeed = c.offautospeed;
            if (autospeed !== undefined) result.official.autoSpeed = (autospeed === true || String(autospeed).toLowerCase() === 'true' || String(autospeed) === '1') ? 'on' : 'off';
            const enableSpeed = c.nsbenablespeed;
            if (enableSpeed !== undefined) result.nsb.enableSpeed = (enableSpeed === true || String(enableSpeed).toLowerCase() === 'true' || String(enableSpeed) === '1') ? 'on' : 'off';
            const savedMode = pick(c, ['mode']);
            if (savedMode === 'official' || savedMode === 'nsb') result.mode = savedMode;
            result.official.ipType = pick(c, ['offiptype', 'iptype', 'ipType']);
            result.official.port = pick(c, ['offport', 'testport', 'port']);
            result.official.threads = pick(c, ['offthreads', 'threads']);
            result.nsb.threads = pick(c, ['nsbthreads']);
            result.official.delay = pick(c, ['offdelay', 'delay']);
            result.nsb.delay = pick(c, ['nsbdelay']);
            result.official.speedMin = pick(c, ['offspeedmin', 'speedmin']);
            result.nsb.speedMin = pick(c, ['nsbspeedmin']);
            result.official.speedLimit = pick(c, ['offspeedlimit', 'speedlimit']);
            result.nsb.speedLimit = pick(c, ['nsbspeedlimit']);
            result.official.targetDC = pick(c, ['offdc', 'dc']);
            result.official.stopCount = pick(c, ['offstop']);
            result.nsb.targetDC = pick(c, ['nsbdc']);
            result.official.speedUrl = pick(c, ['offurl', 'url']);
            result.nsb.speedUrl = pick(c, ['nsburl']);
            result.nsb.resultLimit = pick(c, ['nsbresultlimit', 'resultlimit']);
            result.nsb.fallbackPort = pick(c, ['nsbfallbackport']);
            result.nsb.speedTest = pick(c, ['nsbspeedtest', 'speedtest']);
            const tls = pick(c, ['nsbtls', 'tls']);
            if (tls !== undefined) result.nsb.tls = tls === true || String(tls).toLowerCase() === 'true' || String(tls) === '1' ? 'on' : 'off';
            const compact = pick(c, ['nsbcompact', 'compact']);
            if (compact !== undefined) result.nsb.compactView = compact === true || String(compact).toLowerCase() === 'true' || String(compact) === '1' ? 'compact' : 'full';
            result.nsb.outFile = pick(c, ['out', 'nsbout', 'offout']);
            result.nsb.sourceUrl = pick(c, ['nsbsourceurl', 'sourceurl']);
            const relinkOff = c.relink === false || String(c.relink).toLowerCase() === 'false' || String(c.relink) === '0';
            const rawRelinkHost = String(c.relinkhost || '').trim();
            if (relinkOff) {
                result.nsb.relinkPreset = 'off';
                result.nsb.relinkHost = '已关闭';
            } else if (rawRelinkHost && rawRelinkHost !== 'probe.609909.xyz') {
                result.nsb.relinkPreset = 'custom';
                result.nsb.relinkHost = rawRelinkHost;
            } else {
                result.nsb.relinkPreset = 'default';
                result.nsb.relinkHost = rawRelinkHost || '已关闭';
            }
            return result;
        }

        // 「上传账号」分组内的账号保存入口：复用结果弹窗的保存逻辑与加密存储。
        // silent=true 时由页面级保存汇总提示；成功返回 true，校验失败返回提示文本。
        async function saveGithubAccountFromPanel(silent = false) {
            const fail = (msg) => {
                if (!silent) showNoticeCard(msg);
                return msg;
            };
            const repoRaw = document.getElementById('githubRepo').value.trim();
            const parts = repoRaw.split('/');
            if (parts.length !== 2 || !parts[0] || !parts[1]) {
                return fail('请先填写仓库 owner/repo，例如 myname/myrepo');
            }
            const token = document.getElementById('githubToken').value.trim();
            if (!token) {
                return fail('请先填写 GitHub Token');
            }
            const payload = {
                owner: parts[0],
                repo: parts[1],
                token: token,
                branch: document.getElementById('githubBranch').value.trim() || 'main',
                path: document.getElementById('githubPath').value.trim() || 'results/ip.txt',
                message: document.getElementById('githubMessage').value.trim() || 'update cfdata results',
            };
            await saveCurrentGitHubAccount(payload);
            renderGitHubAccounts();
            const okMsg = 'GitHub 账号已保存到本机浏览器加密存储';
            if (!silent) showNoticeCard(okMsg);
            log(okMsg);
            return true;
        }

        async function saveEdgetunnelAccountFromPanel(silent = false) {
            const fail = (msg) => {
                if (!silent) showNoticeCard(msg);
                return msg;
            };
            const host = document.getElementById('edgetunnelHost').value.trim();
            const password = document.getElementById('edgetunnelPassword').value.trim();
            if (!host || !password) {
                return fail('请先填写 edgetunnel 域名与密码');
            }
            await saveCurrentEdgetunnelAccount();
            const okMsg = 'edgetunnel 账号已保存到本机浏览器加密存储';
            if (!silent) showNoticeCard(okMsg);
            log(okMsg);
            return true;
        }

        // 当前页面的账号表单是否填写过（名称/token/仓库 任一非空即视为要保存）
        function accountFormHasInput(ids) {
            return hasAnyFieldValue(ids.map((id) => document.getElementById(id)?.value));
        }

        // 页面级保存：结果操作页（结果面板设置 + R2 上传设置）
        function saveResultsPage() {
            const localOK = saveResultPanelSettings();
            const panelSent = flushResultPanelServerSync(true);
            suppressScheduleSavedNotice = true;
            // 响应丢失时兜底：5 秒后自动解除抑制，避免永久屏蔽调度保存提示
            setTimeout(() => { suppressScheduleSavedNotice = false; }, 5000);
            if (!saveScheduleConfig()) suppressScheduleSavedNotice = false;
            if (!localOK && !panelSent) {
                suppressScheduleSavedNotice = false;
                showNoticeCard('结果面板设置保存失败：本地与服务器均不可用');
            }
            // 成功提示由 result_panel_config_saved 统一给出，避免两条提示互相覆盖
        }

        // 页面级保存：自动化页（调度、通知与 R2 与自动化共用同一配置）
        function saveAutomationPage() {
            saveScheduleConfig();
        }

        // 页面级保存：上传账号页（两个表单中的有效账号，空白表单跳过）
        async function saveAccountsPage() {
            const hasGithub = accountFormHasInput(['githubAccountName', 'githubToken', 'githubRepo']);
            const hasEdgetunnel = accountFormHasInput(['edgetunnelAccountName', 'edgetunnelHost', 'edgetunnelPassword']);
            if (!hasGithub && !hasEdgetunnel) {
                showNoticeCard('请先填写要保存的 GitHub 或 edgetunnel 账号信息');
                return;
            }
            const failures = [];
            if (hasGithub) {
                const result = await saveGithubAccountFromPanel(true);
                if (result !== true) failures.push(result);
            }
            if (hasEdgetunnel) {
                const result = await saveEdgetunnelAccountFromPanel(true);
                if (result !== true) failures.push(result);
            }
            if (failures.length) {
                showNoticeCard(failures[0]);
                return;
            }
            showNoticeCard('上传账号已保存到本机浏览器加密存储');
        }

        // 页面级保存：设置页（本地配置 + 自动加载偏好 + 已填写的管理面代理）
        function saveSettingsPage() {
            if (!saveConfigToLS()) return;
            const autoLoad = document.getElementById('configAutoLoad');
            if (autoLoad) saveAutoLoadPreference(autoLoad.value);
            const proxyInput = document.getElementById('proxyInput');
            const proxy = proxyInput ? proxyInput.value.trim() : '';
            if (!proxy || saveManagementProxy()) {
                showNoticeCard('设置已保存');
            }
        }

        function deleteSavedConfig() {
            if (!confirm('确定要删除保存的配置吗？')) return;
            try {
                localStorage.removeItem('cfdata-config');
                localStorage.removeItem('cfdata-autoload');
            } catch (e) {
                showNoticeCard('删除配置失败: ' + (e.message || String(e)));
                return;
            }
            showNoticeCard('保存的配置已删除');
        }

        function saveAutoLoadPreference(value) {
            try {
                localStorage.setItem('cfdata-autoload', value);
            } catch (e) {
                showNoticeCard('保存自动加载偏好失败');
            }
        }

        function autoLoadConfig() {
            // 服务器配置文件是任务参数的主载体，连接建立后总是加载一次；
            // localStorage 仅作兜底缓存，选择「本地保存参数」时在服务器参数之上覆盖。
            wsSend({ type: 'get_config' });
            let pref;
            try {
                pref = localStorage.getItem('cfdata-autoload');
            } catch (e) {
                return;
            }
            if (pref !== 'localStorage') return;
            try {
                const raw = localStorage.getItem('cfdata-config');
                if (raw) {
                    applyConfigValues(JSON.parse(raw));
                }
            } catch (e) {
                console.error('[cfdata] autoLoadConfig failed:', e);
            }
        }

        function saveTaskConfigToServer(scope) {
            const config = readFormValues();
            const payload = {};
            payload.mode = currentMode;
            if (scope === 'official') {
                const o = config.official;
                payload.officialScanMode = o.scanMode;
                payload.officialIpType = parseInt(o.ipType, 10) || 4;
                payload.officialPort = parseInt(o.port, 10) || 443;
                payload.officialThreads = parseThreadsValue(o.threads);
                payload.officialDelay = parseInt(o.delay, 10) || 0;
                payload.officialSpeedMin = parseFloat(o.speedMin) || 0;
                payload.officialSpeedLimit = Math.max(0, parseInt(o.speedLimit, 10) || 0);
                payload.officialAutoSpeed = o.autoSpeed === 'on';
                payload.officialTargetDC = o.targetDC || '';
                payload.officialStopCount = Math.max(0, parseInt(o.stopCount, 10) || 0);
                payload.officialSpeedUrl = o.speedUrl || '';
            } else if (scope === 'nsb') {
                const n = config.nsb;
                payload.nsbScanMode = n.scanMode;
                payload.nsbThreads = parseThreadsValue(n.threads);
                payload.nsbDelay = parseInt(n.delay, 10) || 0;
                payload.nsbResultLimit = Math.max(0, parseInt(n.resultLimit, 10) || 0);
                payload.nsbFallbackPort = parseInt(n.fallbackPort, 10) || 443;
                payload.nsbSpeedTest = Math.max(0, parseInt(n.speedTest, 10) || 0);
                payload.nsbSpeedMin = parseFloat(n.speedMin) || 0;
                payload.nsbSpeedLimit = Math.max(0, parseInt(n.speedLimit, 10) || 0);
                payload.nsbEnableSpeed = n.enableSpeed === 'on';
                payload.nsbTargetDC = n.targetDC || '';
                payload.nsbTls = n.tls === 'on';
                payload.nsbCompact = n.compactView === 'compact';
                payload.nsbOutFile = n.outFile || '';
                payload.nsbSpeedUrl = n.speedUrl || '';
                payload.nsbSourceUrl = n.sourceUrl || '';
                payload.nsbRelink = n.relinkPreset !== 'off';
                payload.nsbRelinkHost = (n.relinkPreset === 'off') ? '' : (n.relinkPreset === 'default' ? 'probe.609909.xyz' : (n.relinkHost || ''));
            } else {
                showNoticeCard('未知的参数分组');
                return;
            }
            if (!wsSend({ type: 'save_task_config', data: payload })) {
                showNoticeCard('与服务端的连接尚未就绪，请稍后再试');
            }
        }

        function renderLastRun(info) {
            const el = document.getElementById('lastRunLine');
            if (!el) return;
            // 无历史时显示空状态占位（服务器重启后摘要清空）
            if (!info || (!info.label && !info.mode)) {
                el.textContent = '上次运行：暂无记录';
                el.classList.add('last-run-empty');
                el.style.display = '';
                return;
            }
            el.classList.remove('last-run-empty');
            const date = new Date((info.endTime || 0) * 1000);
            const timeText = isNaN(date.getTime()) || !info.endTime ? '-' : date.toLocaleString();
            const dur = Number(info.durationMs) || 0;
            const durText = dur >= 60000
                ? Math.floor(dur / 60000) + '分' + Math.round((dur % 60000) / 1000) + '秒'
                : (dur >= 1000 ? Math.round(dur / 1000) + ' 秒' : dur + ' ms');
            const parts = ['上次运行：' + (info.label || '任务'), info.success ? '完成' : '已停止', timeText, '耗时 ' + durText, '结果 ' + (Number(info.results) || 0) + ' 条'];
            if (info.uploadNote) parts.push(info.uploadNote);
            el.textContent = parts.join(' · ');
            el.style.display = '';
        }

        function loadConfigFromServer() {
            wsSend({ type: 'get_config' });
        }

        function wsSend(payload) {
            if (!ws || ws.readyState !== WebSocket.OPEN) {
                recordWebDebugError('ws_send', 'WebSocket 未连接');
                return false;
            }
            try {
                ws.send(JSON.stringify(payload));
                return true;
            } catch (e) {
                recordWebDebugError('ws_send', e && e.message ? e.message : String(e));
                console.error('ws.send failed:', e);
                return false;
            }
        }

        function recordWebDebugError(source, detail) {
            if (!webDebugMode) return;
            const entry = { time: new Date().toISOString(), source, detail: String(detail || '') };
            webDebugErrors.push(entry);
            if (webDebugErrors.length > 200) webDebugErrors.shift();
            console.error('[cfdata-debug]', entry);
        }

        function recordWebDebugNotice(source, detail) {
            const text = String(detail || '');
            if (!/(错误|失败|异常|超时|终止|跳过|未找到|未发现|无效|为空|不匹配|无法|损坏|重试|未就绪|警告|提醒|⚠|panic|error|fail|failed|timeout|invalid|empty|not found|skip|retry|warning|warn|refused|reset|denied|429|500|502|503|504)/i.test(text)) return;
            recordWebDebugError(source, text);
        }

        window.addEventListener('error', (event) => {
            recordWebDebugError('window_error', `${event.message || '脚本错误'} ${event.filename || ''}:${event.lineno || 0}:${event.colno || 0}`);
        });

        window.addEventListener('unhandledrejection', (event) => {
            const reason = event.reason && event.reason.message ? event.reason.message : String(event.reason || 'unhandled rejection');
            recordWebDebugError('unhandled_rejection', reason);
        });

        function startCompactIPv4() {
            if (requireProxyWarning(() => startCompactIPv4())) return;
            if (!ws || ws.readyState !== WebSocket.OPEN) {
                showNoticeCard('与服务端的连接尚未就绪，请等待"已连接服务器"后再试');
                return;
            }
            if (isTaskRunning) {
                showNoticeCard('当前已有任务运行，请先停止或等待完成');
                return;
            }
            showConfirmCard('精简会覆盖本地 ips-v4.txt。\n仅保留探测到活跃的 /24 子网。\n继续？', confirmStartCompactIPv4, '确认精简 IPv4 地址库');
        }

        function confirmStartCompactIPv4() {
            const btn = document.getElementById('btnCompactIPv4');
            if (btn) {
                btn.disabled = true;
                btn.textContent = '⏳';
            }
            isTaskRunning = true;
            runningTaskLabel = 'IPv4 地址库精简';
            runningTaskMode = currentMode;
            runningTaskTab = 'scan';
            setStartLoading(getRunningButtonText());
            resetProgress();
            progressBox.style.display = 'block';
            updateProgress(0, 1, '精简准备中');
            wsSend({ type: 'compact_ipv4', data: {} });
        }

        function finishCompactIPv4(btnText = '🧹') {
            const btn = document.getElementById('btnCompactIPv4');
            if (btn) {
                btn.disabled = false;
                btn.textContent = btnText;
            }
        }

        let compactHitBuffer = [];
        let compactHitFlushTimer = null;

        function flushCompactHits() {
            if (compactHitFlushTimer) {
                clearTimeout(compactHitFlushTimer);
                compactHitFlushTimer = null;
            }
            if (compactHitBuffer.length === 0) return;
            const count = compactHitBuffer.length;
            const sample = compactHitBuffer.slice(0, 3).join(', ');
            log(`[精简] 本轮新增命中 ${count} 个，示例: ${sample}${count > 3 ? ' ...' : ''}`);
            compactHitBuffer = [];
        }

        function connect() {
            const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            ws = new WebSocket(wsProtocol + "//" + window.location.host + "/ws");
            ws.onopen = () => {
                updateStatus('已连接服务器', true);
                log('已连接服务器');
                initThemeFromStorage();
                // 等待服务端下发任务恢复载荷；期间带 seq 的任务消息先缓冲
                taskRestorePending = true;
                taskRestoreBuffer = [];
            };
            ws.onmessage = (event) => {
                try {
                    const msg = JSON.parse(event.data);
                    handleMessage(msg);
                } catch (e) {
                    recordWebDebugError('ws_message_parse', e && e.message ? e.message : String(e));
                    console.error('WebSocket message parse error:', e);
                }
            };
            ws.onclose = () => {
                clearStopWatchdog();
                resetCurrentSpeedButton();
                // 任务在服务端继续运行；重连后由任务恢复载荷自动重建视图
                updateStatus('连接断开，3秒后重连...', false);
                log('连接断开，正在重连...');
                setTimeout(connect, 3000);
            };
        }

        function updateStatus(text, connected) {
            statusEl.textContent = text;
            statusEl.classList.toggle('error', !connected);
        }

        function log(text) {
            recordWebDebugNotice('web_log', text);
            const box = document.getElementById('logBox');
            const div = document.createElement('div');
            const span = document.createElement('span');
            span.style.cssText = 'color:#94a3b8;opacity:.75;';
            span.textContent = `[${new Date().toLocaleTimeString()}]`;
            div.appendChild(span);
            div.appendChild(document.createTextNode(' ' + text));
            box.appendChild(div);
            box.scrollTop = box.scrollHeight;
        }

        function showNoticeCard(message, title = '提示') {
            noticeConfirmAction = null;
            const mask = document.getElementById('noticeCardMask');
            const titleEl = document.getElementById('noticeCardTitle');
            const messageEl = document.getElementById('noticeCardMessage');
            const actions = document.getElementById('noticeCardActions');
            if (!mask || !titleEl || !messageEl || !actions) return;
            titleEl.textContent = title;
            messageEl.textContent = String(message || '');
            actions.innerHTML = '<button class="action-btn export-btn" onclick="closeNoticeCard()">知道了</button>';
            mask.classList.add('active');
        }

        function showConfirmCard(message, onConfirm, title = '确认操作') {
            noticeConfirmAction = typeof onConfirm === 'function' ? onConfirm : null;
            const mask = document.getElementById('noticeCardMask');
            const titleEl = document.getElementById('noticeCardTitle');
            const messageEl = document.getElementById('noticeCardMessage');
            const actions = document.getElementById('noticeCardActions');
            if (!mask || !titleEl || !messageEl || !actions) return;
            titleEl.textContent = title;
            messageEl.textContent = String(message || '');
            actions.innerHTML = '<button class="action-btn" onclick="closeNoticeCard()">取消</button><button class="action-btn export-btn" onclick="confirmNoticeCard()">确认</button>';
            mask.classList.add('active');
        }

        function closeNoticeCard() {
            const mask = document.getElementById('noticeCardMask');
            if (mask) mask.classList.remove('active');
            noticeConfirmAction = null;
        }

        function confirmNoticeCard() {
            const action = noticeConfirmAction;
            closeNoticeCard();
            if (typeof action === 'function') action();
        }

        function requireProxyWarning(action) {
            if (proxyWarningSinglePass) {
                proxyWarningSinglePass = false;
                return false;
            }
            if (proxyWarningBypassed) return false;
            if (proxyCountryCheckPending) return true;
            proxyWarningPendingAction = action;
            proxyCountryCheckPending = true;
            if (proxyCountryCheckTimer) window.clearTimeout(proxyCountryCheckTimer);
            proxyCountryCheckTimer = window.setTimeout(() => {
                proxyCountryCheckPending = false;
                proxyCountryCheckTimer = null;
                cfCountry = '检测失败';
                proxyGeoCheckOK = false;
                proxyWarningRequired = true;
                log('代理检测警告：地区验证超时，已阻止自动开始测试');
                showProxyWarningModal();
            }, 8000);
            if (!wsSend({ type: 'check_proxy_country', data: {} })) {
                proxyCountryCheckPending = false;
                window.clearTimeout(proxyCountryCheckTimer);
                proxyCountryCheckTimer = null;
                proxyWarningPendingAction = null;
                showNoticeCard('连接未就绪，请等待重连后再试');
            }
            return true;
        }

        function handleProxyCountryResult(data) {
            cfCountry = normalizeProxyCountry(data);
            proxyWarningRequired = !!data?.proxyWarning;
            proxyGeoCheckOK = data?.geoCheckOK !== false;
            proxyCountryCheckPending = false;
            if (proxyCountryCheckTimer) {
                window.clearTimeout(proxyCountryCheckTimer);
                proxyCountryCheckTimer = null;
            }
            if (proxyWarningRequired) {
                log(`代理检测警告：当前网络标签 ${cfCountry}`);
                showProxyWarningModal();
                return;
            }
            const action = proxyWarningPendingAction;
            proxyWarningPendingAction = null;
            proxyWarningSinglePass = true;
            if (typeof action === 'function') action();
        }

        function normalizeProxyCountry(data) {
            const value = String(data?.cfCountry || '').trim().toUpperCase();
            if (value) return value;
            return data?.skipGeoCheck ? 'SKIPPED' : '检测失败';
        }

        function requireCachedProxyWarning(action) {
            if (!proxyWarningRequired || proxyWarningBypassed) return false;
            proxyWarningPendingAction = action;
            showProxyWarningModal();
            return true;
        }

        function showProxyWarningModal() {
            const mask = document.getElementById('noticeCardMask');
            const btn = document.getElementById('proxyWarningContinueBtn');
            const titleEl = document.getElementById('noticeCardTitle');
            const messageEl = document.getElementById('noticeCardMessage');
            const actions = document.getElementById('noticeCardActions');
            if (!mask || !titleEl || !messageEl || !actions) return;
            titleEl.textContent = '代理检测警告';
            messageEl.textContent = [
                `读取到的网络标签：${cfCountry || '未知'}`,
                proxyGeoCheckOK ? '检测到您当前很可能处于代理/VPN环境中。' : '地区验证失败，无法确认当前是否处于代理/VPN环境中。',
                '在代理状态下进行的 IP 优选测试结果将不准确。',
                '建议关闭代理软件后刷新页面重新测试。',
            ].join('\n');
            actions.innerHTML = '<button class="action-btn danger-btn" onclick="cancelProxyWarning()">停止测速</button><button class="action-btn export-btn" id="proxyWarningContinueBtn" onclick="confirmProxyWarning()" disabled>强制继续 (5)</button>';
            mask.classList.add('active');
            let remaining = 5;
            const nextBtn = document.getElementById('proxyWarningContinueBtn');
            if (!nextBtn) return;
            nextBtn.disabled = true;
            nextBtn.textContent = `强制继续 (${remaining})`;
            if (proxyWarningCountdownTimer) window.clearInterval(proxyWarningCountdownTimer);
            proxyWarningCountdownTimer = window.setInterval(() => {
                remaining--;
                if (remaining <= 0) {
                    window.clearInterval(proxyWarningCountdownTimer);
                    proxyWarningCountdownTimer = null;
                    nextBtn.disabled = false;
                    nextBtn.textContent = '强制继续';
                    return;
                }
                nextBtn.textContent = `强制继续 (${remaining})`;
            }, 1000);
        }

        function hideProxyWarningModal() {
            const mask = document.getElementById('noticeCardMask');
            if (mask) mask.classList.remove('active');
            if (proxyWarningCountdownTimer) {
                window.clearInterval(proxyWarningCountdownTimer);
                proxyWarningCountdownTimer = null;
            }
            noticeConfirmAction = null;
        }

        function cancelProxyWarning() {
            hideProxyWarningModal();
            proxyWarningPendingAction = null;
            proxyCountryCheckPending = false;
            log(`代理检测警告：已取消测速（当前网络标签 ${cfCountry || '未知'}）`);
        }

        function confirmProxyWarning() {
            const action = proxyWarningPendingAction;
            proxyWarningBypassed = true;
            proxyWarningPendingAction = null;
            hideProxyWarningModal();
            log(`代理检测警告：用户强制继续（当前网络标签 ${cfCountry || '未知'}）`);
            proxyWarningSinglePass = true;
            if (typeof action === 'function') action();
        }

        function getScanMode() {
            if (currentMode === 'official') {
                const el = document.getElementById('officialScanMode');
                return el ? el.value : 'tcping';
            }
            const el = document.getElementById('nsbScanMode');
            return el ? el.value : 'tcping';
        }

        function getScanModeHint() {
            if (currentMode === 'official') {
                const el = document.getElementById('officialScanModeHint');
                return el || null;
            }
            const el = document.getElementById('nsbScanModeHint');
            return el || null;
        }

        function getScanModeLabel() {
            return getScanMode() === 'httping' ? 'HTTPing' : 'TCPing';
        }

        function updateScanModeHint() {
            const mode = getScanMode();
            const hint = getScanModeHint();
            if (!hint) return;
            if (mode === 'httping') {
                hint.value = '测量 HTTP GET 请求的完整 TTFB（含 TLS 握手），因此HTTPing 延迟比 TCPing 高属正常现象，不同模式数据不可互相比较';
            } else {
                hint.value = '仅测量 TCP 握手延迟，不同模式数据不可互相比较';
            }
        }

        function switchMode(mode) {
            currentMode = mode;
            // 模式高亮反映在任务控制卡的分段选择器上，并联动右侧官方/非标参数页
            if (modeOptionOfficial) modeOptionOfficial.classList.toggle('active', mode === 'official');
            if (modeOptionNSB) modeOptionNSB.classList.toggle('active', mode === 'nsb');
            nzUpdateStartReadiness();

            if (mode === 'official') {
                officialControls.classList.remove('hidden');
                nsbControls.classList.add('hidden');
                document.querySelector('.tabs .tab:last-child').style.display = 'block';
                document.getElementById('dcSection').style.display = dcList.length ? 'block' : 'none';
                if (!isTaskRunning) resetButton('✨ 开始扫描与测试');
                updateNSBCompactToggle();
                if (isTaskRunning && runningTaskMode === 'official') {
                    showRunningTaskView();
                } else if (!isTaskRunning) {
                    switchTab('scan');
                    renderCurrentScanResults();
                }
            } else {
                officialControls.classList.add('hidden');
                nsbControls.classList.remove('hidden');
                const testTab = document.querySelectorAll('.tab')[1];
                testTab.style.display = 'none';
                if (document.querySelector('.tab.active') !== document.querySelector('.tabs .tab:first-child')) {
                    switchTab('scan');
                }
                document.getElementById('dcSection').style.display = 'none';
                if (!isTaskRunning) resetButton('✨ 开始扫描与测试');
                updateNSBCompactToggle();
                if (isTaskRunning && runningTaskMode === 'nsb') {
                    showRunningTaskView();
                } else if (!isTaskRunning) {
                    renderCurrentScanResults();
                }
            }
            if (!isTaskRunning) resetProgress();
            updateScanModeHint();
            // 结果操作面板的数据来源随任务模式变化（官方：扫描/详细测试；非标：NSB）
            if (typeof syncResultSourceOptions === 'function') {
                try { syncResultSourceOptions(); } catch (e) { /* 面板未初始化 */ }
            }
            // 模式与参数页联动：切到对应模式即打开右侧对应参数页（currentMode 已同步，不会递归）
            if (typeof window.nzShowConfigPanel === 'function') {
                try { window.nzShowConfigPanel(mode); } catch (e) { /* 面板未初始化 */ }
            }
            log(`切换至：${mode === 'official' ? '官方优选' : '非标优选'}`);
        }

        // 启动前就绪校验：非标优选必须有 IP 来源（文件或网络 URL）；
        // 官方优选无硬性前置。缺失时禁用启动并高亮对应配置项。
        function nzStartReadiness() {
            if (currentMode !== 'nsb') return { ready: true, missing: [] };
            const hasFile = !!(uploadedFileContent && uploadedFileContent.trim());
            const urlInput = document.getElementById('nsbSourceUrl');
            const hasURL = !!(urlInput && urlInput.value.trim());
            const missing = [];
            if (!hasFile && !hasURL) missing.push('source');
            return { ready: missing.length === 0, missing };
        }

        function nzUpdateStartReadiness() {
            const { ready } = nzStartReadiness();
            const fileRow = document.getElementById('nsbFileInput')?.closest('.option-item');
            const urlRow = document.getElementById('nsbSourceUrl')?.closest('.option-item');
            if (fileRow) fileRow.classList.toggle('missing-hint', !ready);
            if (urlRow) urlRow.classList.toggle('missing-hint', !ready);
            // 任务运行中不允许禁用启停按钮：停止运行中的任务必须始终可用
            if (isTaskRunning || isStoppingTask) return;
            for (const id of ['nzToggle', 'btnStart']) {
                const btn = document.getElementById(id);
                if (!btn) continue;
                btn.disabled = !ready;
                btn.title = ready ? '' : '非标优选需要先提供 IP 文件或网络 URL';
            }
        }

        // 软性缺失提醒：不阻止任务启动
        function nzSoftWarnings() {
            if (lastSchedulePayload && lastSchedulePayload.r2Enabled && (!lastSchedulePayload.r2WorkerUrl || !lastSchedulePayload.r2WorkerTokenSet)) {
                nzToast('R2 上传已启用但 Worker 地址或 token 不完整，任务完成后将跳过自动上传', 'error');
            }
        }

        function switchTab(tabId) {
            document.querySelectorAll('.tab').forEach((t) => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach((c) => c.classList.remove('active'));

            const tabs = document.querySelectorAll('.tab');
            const first = tabs[0];
            const second = tabs[1];

            if (tabId === 'scan') {
                first.classList.add('active');
            } else {
                if (second.style.display !== 'none') second.classList.add('active');
                else first.classList.add('active');
            }
            const target = document.getElementById(tabId);
            if (target) target.classList.add('active');

            if (tabId === 'test' && currentMode === 'nsb') {
                switchTab('scan');
                return;
            }

            if (currentMode === 'official' && tabId === 'test') {
                renderAllTestResults();
            } else if (currentMode === 'official' && tabId === 'scan' && scanData.length) {
                renderCurrentScanResults();
            } else if (currentMode === 'nsb' && tabId === 'scan') {
                renderCurrentScanResults();
            }
        }

        function renderCurrentScanResults() {
            if (currentMode === 'official') {
                renderScanHeaders('official');
                if (scanData.length) sortAndRenderScanResults();
                else updateBadge('scanCount', 0);
                return;
            }
            if (nsbRows.length) {
                renderScanRowsCSV(nsbHeaders, nsbRows);
                return;
            }
            renderScanHeaders('nsb');
            renderAllNSBLiveRows();
        }

        function updateProgress(current, total, text, etaSeconds) {
            const etaSuffix = formatEtaText(etaSeconds);
            if (!total) {
                progressBar.style.width = '100%';
                progressBar.style.animation = 'none';
                progressBar.textContent = `${text || ''} ${current || 0}${etaSuffix}`;
                return;
            }
            const p = Math.min(Math.max((current / total) * 100, 0), 100);
            progressBar.style.width = `${Math.round(p)}%`;
            const displayCurrent = p >= 100 ? total : (current || 0);
            progressBar.textContent = `${text || ''} ${displayCurrent}/${total} (${Math.round(p)}%)${etaSuffix}`;
            progressBar.style.animation = p >= 100 ? 'none' : 'stripe 1s linear infinite';
        }

        function completeProgressWithCurrentCount(text = '完成') {
            const currentText = progressBar.textContent || '';
            const match = currentText.match(/(\d+)\s*\/\s*(\d+)/);
            const total = match ? parseInt(match[2], 10) : 0;
            if (Number.isFinite(total) && total > 0) {
                updateProgress(total, total, text);
                return;
            }
            progressBar.style.width = '100%';
            progressBar.style.animation = 'none';
            progressBar.textContent = text;
        }

        function updateQualifiedSpeedProgress(current, total) {
            const safeCurrent = current || 0;
            const safeTotal = total || 1;
            const p = Math.min(Math.max((safeCurrent / safeTotal) * 100, 0), 100);
            progressBar.style.width = `${Math.round(p)}%`;
            progressBar.textContent = `测速中 ${safeCurrent}/${safeTotal} (${Math.round(p)}%)`;
            progressBar.style.animation = p >= 100 ? 'none' : 'stripe 1s linear infinite';
        }

        function showStoppingProgress() {
            progressBox.style.display = 'block';
            progressBar.style.background = 'var(--warning-color)';
            progressBar.style.width = '100%';
            progressBar.style.animation = 'none';
            progressBar.textContent = '正在手动终止...';
        }

        function resetProgress() {
            progressBar.style.width = '0%';
            progressBar.style.background = 'linear-gradient(45deg, rgba(255,255,255,0.15) 25%, transparent 25%, transparent 50%, rgba(255,255,255,0.15) 50%, rgba(255,255,255,0.15) 75%, transparent 75%, transparent), linear-gradient(to right, var(--primary-color), #60a5fa)';
            progressBar.style.backgroundSize = '20px 20px, 100% 100%';
            progressBar.style.animation = 'stripe 1s linear infinite';
            progressBar.textContent = '0%';
        }

        function setStartLoading(text, background = 'linear-gradient(135deg, #ef4444, #dc2626)', boxShadow = '0 4px 12px rgba(239, 68, 68, 0.3), inset 0 1px 1px rgba(255,255,255,.2)') {
            const btn = document.getElementById('btnStart');
            const lb = document.getElementById('btnStartLabel');
            const sp = document.getElementById('btnStartSpinner');
            lb.textContent = text;
            btn.style.background = background;
            btn.style.boxShadow = boxShadow;
            if (sp) sp.classList.remove('hidden');
            if (sp && sp.classList) {
                sp.className = 'btn-spinner';
            }
            btn.disabled = false;
        }

        // ---------------- 任务恢复（连接建立后自动重建运行视图） ----------------

        function applyTaskRestoredState(data) {
            // 恢复载荷元信息：任务状态、进度与参数先行恢复，结果表格在 task_restore_done 统一重建
            taskRestoreData = { scan: [], test: [], speed: [], nsbLive: [], dc: [], nsbHeaders: [], nsbRows: [] };
            if (!data || !data.hasTask) return;
            const snapshot = data.snapshot || {};
            const mode = data.mode;
            isTaskRunning = !!data.running;
            isStoppingTask = false;
            runningTaskMode = mode || '';
            runningTaskLabel = data.label || '任务';
            runningTaskTab = 'scan';
            if ((mode === 'official' || mode === 'nsb') && currentMode !== mode) switchMode(mode);
            if (snapshot.params) applyBackgroundParams(snapshot.params);
            progressBox.style.display = 'block';
            if (data.running) {
                setStartLoading(getRunningButtonText());
                updateProgress(snapshot.current || 0, snapshot.total || 1, snapshot.phase || '任务运行中');
            } else {
                resetButton('✨ 开始扫描与测试');
                updateProgress(snapshot.current || 0, Math.max(snapshot.total || 0, snapshot.current || 0, 1), snapshot.phase || '完成');
            }
            (data.logs || []).forEach((line) => log(line));
        }

        function applyTaskRestoreChunk(chunk) {
            if (!taskRestoreData || !chunk) return;
            const rows = chunk.rows || [];
            if (!Array.isArray(taskRestoreData[chunk.kind])) return;
            taskRestoreData[chunk.kind] = taskRestoreData[chunk.kind].concat(rows);
        }

        function applyTaskRestoreDone(data) {
            taskRestoreLastSeq = (data && data.lastSeq) || 0;
            const buffered = taskRestoreBuffer;
            taskRestoreBuffer = [];
            taskRestorePending = false;
            const d = taskRestoreData;
            taskRestoreData = null;
            if (d) {
                scanData = d.scan;
                testData = d.test;
                dcList = d.dc;
                nsbHeaders = d.nsbHeaders;
                nsbRows = d.nsbRows;
                nsbLiveRows = [];
                nsbLiveIndex = {};
                nsbLiveRowElements = {};
                d.nsbLive.forEach((row) => addNSBLiveResult(row || {}, false));
                updateBadge('scanCount', currentMode === 'nsb' ? nsbLiveRows.length : scanData.length);
                updateBadge('testCount', testData.length);
                if (d.speed.length) d.speed.forEach((res) => updateSpeedResult(res || {}));
                if (dcList.length && currentMode === 'official') showDCSelection(dcList);
                showRunningTaskView();
            }
            // 回放恢复期间缓冲的增量消息：丢弃已包含在恢复载荷里的旧消息，其余按序应用
            buffered.forEach((msg) => {
                if (typeof msg.seq === 'number' && msg.seq <= taskRestoreLastSeq) return;
                handleMessage(msg);
            });
        }

        function applyBackgroundParams(params) {
            const setValue = (id, value) => {
                const el = document.getElementById(id);
                if (el === null || el === undefined || value === undefined || value === null || value === '') return;
                el.value = value;
            };
            setValue('officialIpType', params.ipType);
            setValue('officialThreads', params.threads || params.maxThreads);
            setValue('officialTestPort', params.port);
            setValue('officialDelay', params.delay);
            setValue('officialSpeedLimit', params.speedLimit);
            setValue('officialSpeedMin', params.speedMin);
            applyOfficialTargetDCSelection(params.dc);
            setValue('nsbThreads', params.maxThreads);
            setValue('nsbFallbackPort', params.fallbackPort);
            setValue('nsbResultLimit', params.resultLimit);
            setValue('nsbSpeedLimit', params.speedLimit);
            setValue('nsbSpeedMin', params.speedMin);
            setValue('nsbSpeedTest', params.speedTest);
            setValue('nsbTargetDC', params.dc);
            setValue('nsbOutFile', params.outFile);
            if (params.enableTLS !== undefined) setValue('nsbTls', params.enableTLS ? 'on' : 'off');
            syncOfficialSpeedConfigState();
            syncNSBSpeedConfigState();
            updateNSBSpeedControls();
            updateNSBCompactToggle();
        }

        function getRunningButtonText() {
            return `${runningTaskLabel || '任务'}测试中 - 终止测试`;
        }

        function getStopConfirmText() {
            return `确定要终止当前${runningTaskLabel || '任务'}吗？`;
        }

        function clearRunningTaskLabel() {
            runningTaskLabel = '';
            runningTaskMode = '';
            runningTaskTab = 'scan';
        }

        function showRunningTaskView() {
            if (!runningTaskMode || currentMode !== runningTaskMode) return;
            if (runningTaskTab === 'test') {
                switchTab('test');
                renderAllTestResults();
                return;
            }
            switchTab('scan');
            if (runningTaskMode === 'official') {
                renderScanHeaders('official');
                sortAndRenderScanResults();
            } else if (nsbRows.length) {
                renderScanRowsCSV(nsbHeaders, nsbRows);
            } else {
                renderScanHeaders('nsb');
                renderAllNSBLiveRows();
                    updateBadge('scanCount', nsbLiveRows.length);
            }
        }

        function finishRunningTask(buttonText = '✨ 开始扫描与测试') {
            isTaskRunning = false;
            isStoppingTask = false;
            dcLibraryScanning = false;
            const dcLibraryBtn = document.getElementById('btnUpdateDcLibrary');
            if (dcLibraryBtn) dcLibraryBtn.disabled = false;
            clearStopWatchdog();
            clearRunningTaskLabel();
            resetButton(buttonText);
            updateNSBCompactToggle();
        }

        function clearStopWatchdog() {
            if (!stopWatchdogTimer) return;
            window.clearTimeout(stopWatchdogTimer);
            stopWatchdogTimer = null;
        }

        function startStopWatchdog() {
            clearStopWatchdog();
            nsbStopHandledByComplete = false;
            stopWatchdogTimer = window.setTimeout(() => {
                if (!isStoppingTask) return;
                resetStoppedTaskUI('已手动终止，后台仍在清理；如立即重试提示已有任务，请稍后再试');
            }, 30000);
        }

        function resetStoppedTaskUI(message = '已手动终止') {
            clearStopWatchdog();
            isStoppingTask = false;
            officialAutoSpeedPending = false;
            officialAutoStage = '';
            officialBatchSpeedRunning = false;
            nsbBatchSpeedRunning = false;
            nsbReallinkRunning = false;
            officialTestCompletePending = false;
            officialSpeedEndedByRateLimit = false;
            currentTestingDC = '';
            resetCurrentSpeedButton();
            finishRunningTask('✨ 开始扫描与测试');
            updateBatchSpeedButton();
            updateNSBSpeedButtons();
            finishCompactIPv4();
            showDCSelection(dcList);
            const status = document.getElementById('currentTestDC');
            if (status && runningTaskMode !== 'nsb') status.textContent = message;
            if (currentMode === 'official' && document.getElementById('test').classList.contains('active')) {
                renderAllTestResults();
            } else {
                renderCurrentScanResults();
            }
            progressBar.style.background = 'var(--warning-color)';
            progressBar.style.width = '100%';
            progressBar.style.animation = 'none';
            progressBar.textContent = message;
        }

        function resetButton(text = '✨ 开始扫描与测试') {
            const btn = document.getElementById('btnStart');
            const lb = document.getElementById('btnStartLabel');
            const sp = document.getElementById('btnStartSpinner');
            if (!btn || !lb) return;
            lb.textContent = text;
            if (sp) sp.classList.add('hidden');
            btn.style.background = 'linear-gradient(135deg, var(--primary-color), #2563eb)';
            btn.style.boxShadow = '0 4px 12px rgba(59,130,246,.3), inset 0 1px 1px rgba(255,255,255,0.2)';
            btn.disabled = false;
            window.setTimeout(updateBatchSpeedButton, 0);
        }

        function renderScanHeaders(mode) {
            const thead = document.querySelector('#scanTable thead');
            const tbody = document.querySelector('#scanTable tbody');
            tbody.innerHTML = '';

            if (mode === 'official') {
                thead.innerHTML = '<tr><th>IP</th><th>端口</th><th>数据中心</th><th>落地区域</th><th>地区</th><th>城市</th><th>扫描方式</th><th>延迟</th></tr>';
            } else {
                const headers = getVisibleNSBHeaders();
                const headerRow = headers.map(h => `<th>${h}</th>`).join('');
                thead.innerHTML = `<tr>${headerRow}</tr>`;
            }
        }

        function updateNSBCompactToggle() {
            const scanActions = document.getElementById('nsbScanActions');
            if (scanActions) {
                scanActions.classList.toggle('official-scan-actions', currentMode !== 'nsb');
            }
            const compactToggle = document.getElementById('nsbCompactView');
            if (compactToggle) {
                compactToggle.value = nsbCompactView ? 'compact' : 'full';
                compactToggle.disabled = isTaskRunning;
            }
            updateNSBSpeedButtons();
        }

        function getNSBSpeedTargetRows() {
            if (currentMode !== 'nsb') return [];
            return getNSBResultObjects()
                .filter((row) => String(row.ip || '').trim() && String(row.port || '').trim());
        }

        function updateNSBSpeedButtons() {
            const btn = document.getElementById('btnNSBBatchSpeed');
            const continueBtn = document.getElementById('btnNSBContinueSpeed');
            const visible = currentMode === 'nsb';
            const rows = visible ? getNSBSpeedTargetRows() : [];
            const disabled = !visible || isTaskRunning || nsbBatchSpeedRunning || rows.length === 0;
            if (btn) {
                btn.style.display = visible ? 'inline-flex' : 'none';
                btn.disabled = disabled;
                btn.textContent = nsbBatchSpeedRunning ? '测速中...' : '🚀 一键测速';
            }
            if (continueBtn) {
                continueBtn.style.display = visible ? 'inline-flex' : 'none';
                continueBtn.disabled = disabled || !nsbContinueSpeedUnlocked || !rows.some((row) => getSpeedStatus(row.speed, isNSBSpeedQualified(row.speed)) === 'untested');
                continueBtn.textContent = nsbBatchSpeedRunning ? '测速中...' : '⏭️ 继续测速';
            }
            const sortBtn = document.getElementById('btnNSBSort');
            if (sortBtn) sortBtn.style.display = visible ? 'inline-flex' : 'none';
        }

        function syncNSBSpeedConfigState() {
            const speedMin = parseFloat(document.getElementById('nsbSpeedMin')?.value) || 0.1;
            const speedLimit = parseInt(document.getElementById('nsbSpeedLimit')?.value, 10) || 0;
            const speedTest = parseInt(document.getElementById('nsbSpeedTest')?.value, 10) || 0;
            const tlsMode = String(document.getElementById('nsbTls')?.value || 'on').toLowerCase();
            nsbSpeedMinValue = speedMin;
            return {
                speedMin,
                speedLimit,
                speedTest,
                speedURL: resolveSpeedTestUrl('nsbSpeedUrl', 'nsbSpeedUrlPreset'),
                enableTLS: tlsMode !== 'off',
            };
        }

        const NSB_RELINK_DEFAULT_HOST = 'probe.609909.xyz';

        function syncRelinkControls() {
            const preset = document.getElementById('nsbRelinkPreset');
            const input = document.getElementById('nsbRelinkHost');
            if (!preset || !input) return;
            if (preset.value === 'off') {
                input.value = '已关闭';
            } else if (preset.value === 'custom') {
                if (!input.value.trim() || input.value === '已关闭' || input.value === '公共探针') input.value = '';
            } else {
                preset.value = 'default';
                input.value = '公共探针';
            }
        }

        function applyRelinkPreset() {
            syncRelinkControls();
        }

        function markRelinkCustom() {
            const preset = document.getElementById('nsbRelinkPreset');
            const input = document.getElementById('nsbRelinkHost');
            if (!preset || !input) return;
            if (input.readOnly) input.readOnly = false;
            preset.value = 'custom';
        }

        function resetRelinkDefault() {
            const preset = document.getElementById('nsbRelinkPreset');
            const input = document.getElementById('nsbRelinkHost');
            if (input) {
                input.value = '公共探针';
                input.readOnly = false;
            }
            if (preset) preset.value = 'default';
        }

        function getNSBRelinkHost(followAutoSpeed = false) {
            const preset = document.getElementById('nsbRelinkPreset');
            const input = document.getElementById('nsbRelinkHost');
            if (preset && preset.value === 'off') {
                if (followAutoSpeed) return NSB_RELINK_DEFAULT_HOST;
                return '';
            }
            if (preset && preset.value === 'custom') {
                const custom = (input && input.value ? input.value : '').trim();
                return custom || NSB_RELINK_DEFAULT_HOST;
            }
            return NSB_RELINK_DEFAULT_HOST;
        }

        function startNSBReallink() {
            if (nsbReallinkRunning || isTaskRunning || nsbBatchSpeedRunning) {
                showNoticeCard('当前有任务正在运行，请等待完成后再复验');
                return;
            }
            if (currentMode !== 'nsb') {
                showNoticeCard('请切换到非标模式后再复验');
                return;
            }
            const rows = getNSBSpeedTargetRows();
            if (!rows.length) {
                showNoticeCard('当前没有可复验的非标结果');
                return;
            }
            const probeHost = getNSBRelinkHost() || NSB_RELINK_DEFAULT_HOST;
            showConfirmCard(`确认开始链路复验？将对 ${rows.length} 个候选进行真实链路验证（TCP+TLS+HTTP），不可用的候选将从结果中剔除。`, () => {
                const speedCfg = syncNSBSpeedConfigState();
                const outFile = document.getElementById('nsbOutFile')?.value.trim() || 'ip.csv';
                nsbReallinkRunning = true;
                isTaskRunning = true;
                isStoppingTask = false;
                runningTaskLabel = '链路复验';
                runningTaskMode = 'nsb';
                setStartLoading('链路复验中...');
                resetProgress();
                progressBox.style.display = 'block';
                updateNSBSpeedButtons();
                if (!wsSend({ type: 'start_nsb_reallink', data: {
                    results: rows,
                    relinkHost: probeHost,
                    outFile,
                    speedTest: speedCfg.speedTest,
                    speedURL: speedCfg.speedURL,
                    enableTLS: speedCfg.enableTLS,
                    speedMin: speedCfg.speedMin,
                    speedLimit: speedCfg.speedLimit,
                    compact: nsbCompactView,
                    scanMode: getScanMode(),
                    r2AutoUpload: isR2AutoUploadSelected('nsbAutoUploadAccount'),
                } })) {
                    nsbReallinkRunning = false;
                    isTaskRunning = false;
                    finishRunningTask('✨ 开始扫描与测试');
                    showNoticeCard('连接未就绪，请等待重连后再试');
                }
            }, '确认链路复验');
        }

        function setNSBCompactView(enabled) {
            if (isTaskRunning) {
                const compactToggle = document.getElementById('nsbCompactView');
                if (compactToggle) compactToggle.value = nsbCompactView ? 'compact' : 'full';
                showNoticeCard('非标精简表格开关只能在任务开始前选择，本次任务启动后输出内容已固定。');
                return;
            }
            nsbCompactView = !!enabled;
            if (currentMode !== 'nsb') return;
            if (nsbRows.length) {
                renderScanRowsCSV(nsbHeaders, nsbRows);
                return;
            }
            renderScanHeaders('nsb');
            renderAllNSBLiveRows();
        }

        function setNSBTableIPTypeFilter(value) {
            const normalized = normalizeIPTypeFilter(value);
            nsbTableIPTypeFilter = normalized;
            const filter = document.getElementById('nsbTableIPTypeFilter');
            if (filter && filter.value !== nsbTableIPTypeFilter) filter.value = nsbTableIPTypeFilter;
            if (currentMode === 'official') {
                return;
            }
            if (nsbRows.length) {
                renderScanRowsCSV(nsbHeaders, nsbRows);
                return;
            }
            renderAllNSBLiveRows();
        }

        function setNSBTableQualifiedFilter(value) {
            const normalized = normalizeQualifiedFilter(value);
            nsbTableQualifiedFilter = normalized;
            const filter = document.getElementById('nsbTableQualifiedFilter');
            if (filter && filter.value !== nsbTableQualifiedFilter) filter.value = nsbTableQualifiedFilter;
            if (currentMode === 'official') {
                return;
            }
            if (nsbRows.length) {
                renderScanRowsCSV(nsbHeaders, nsbRows);
                return;
            }
            renderAllNSBLiveRows();
        }

        function renderScanRowOfficial(row) {
            const tbody = document.querySelector('#scanTable tbody');
            const latencyVal = parseFloat(row.LatencyStr);
            const lcFactor = getLatencyColorFactor();
            const latencyColor = getLatencyColor(latencyVal, lcFactor);
            const html = `<tr><td>${renderIPCell(row.IP, row.OriginalInput || row.originalInput)}</td><td><span class="copy-ip" onclick="copyToClipboard('${row.Port ?? ''}', event)">${row.Port ?? '-'}</span></td><td>${row.DataCenter || '-'}</td><td>${row.DCCountry || '-'}</td><td>${row.Region || '-'}</td><td>${row.City || '-'}</td><td>${getScanModeLabel()}</td><td style="color:${latencyColor}; font-weight:600;">${row.LatencyStr || '-'}</td></tr>`;
            tbody.insertAdjacentHTML('beforeend', html);
        }

        function getOfficialIPType(row) {
            const ip = String(row?.IP || row?.ip || '').trim();
            return ip.includes(':') ? 'ipv6' : 'ipv4';
        }

        function isOfficialSpeedQualified(value) {
            const speed = parseSpeedMB(value);
            return speed >= 0 && speed >= officialSpeedMinValue;
        }

        function getOfficialRowSpeed(row) {
            if (row?.Speed || row?.speed) return row.Speed || row.speed;
            const ip = row?.IP || row?.ip || '';
            const test = testData.find((item) => item.IP === ip || item.ip === ip);
            return test?.Speed || test?.speed || '';
        }

        function getSpeedStatus(value, isQualified) {
            const text = String(value || '').trim();
            if (!text || text === '-' || text === '未测速') return 'untested';
            return isQualified ? 'qualified' : 'unqualified';
        }

        function renderSpeedStatusTag(status) {
            if (status === 'qualified') return '<span class="result-status-slot"><span class="result-status-tag qualified" title="合格">合格</span></span>';
            if (status === 'untested') return '<span class="result-status-slot"></span>';
            return '<span class="result-status-slot"><span class="result-status-tag unqualified" title="不合格">不合格</span></span>';
        }

        function matchesOfficialQualifiedFilter(row, filter) {
            filter = normalizeQualifiedFilter(filter);
            if (filter === 'all') return true;
            const speed = getOfficialRowSpeed(row);
            const status = getSpeedStatus(speed, isOfficialSpeedQualified(speed));
            return status === filter;
        }

        function getFilteredOfficialScanData() {
            return scanData;
        }

        function escapeJSString(v) {
            return String(v ?? '').replace(/\\/g, '\\\\').replace(/'/g, "\\'");
        }

        function renderIPCell(ip, originalInput) {
            const rawIP = String(ip ?? '');
            const safeIP = escapeHTML(rawIP);
            const jsIP = escapeJSString(rawIP);
            const hrefIP = encodeURIComponent(rawIP);
            let domainIcon = '';
            if (originalInput && originalInput !== rawIP) {
                const isDomain = /[a-zA-Z]/.test(originalInput);
                if (isDomain) {
                    const safeSource = escapeHTML(originalInput);
                    domainIcon = `<span class="ip-detail-link" title="来源域名: ${safeSource}" onclick="event.stopPropagation()" style="cursor:default;">ⓗ</span>`;
                }
            }
            return `<span class="ip-cell"><span class="copy-ip" onclick="copyToClipboard('${jsIP}', event)">${safeIP}</span>${domainIcon}<a class="ip-detail-link" href="https://ippure.com/?ip=${hrefIP}" target="_blank" rel="noopener noreferrer" title="前往 ippure.com 查看 IP 详情" onclick="event.stopPropagation()">ⓘ</a></span>`;
        }

        function renderNSBRowFromCSV(headers, r, originalInput) {
            let ipIndex = headers.indexOf('IP地址');
            let portIndex = headers.indexOf('端口号');
            let latencyIndex = headers.indexOf('网络延迟');
            let lossIndex = headers.indexOf('丢包率');
            let speedIndex = headers.indexOf('下载速度');
            const speedQualified = speedIndex < 0 || isNSBSpeedQualified(r[speedIndex]);
            const speedStatus = speedIndex < 0 ? 'qualified' : getSpeedStatus(r[speedIndex], speedQualified);
            return headers.map((_, idx) => {
                const val = escapeHTML(r[idx] ?? '');
                const rawVal = String(r[idx] ?? '');
                if (idx === ipIndex && ipIndex !== -1) {
                    return `<td>${renderIPCell(rawVal, originalInput)}</td>`;
                } else if (idx === portIndex && portIndex !== -1) {
                    return `<td><span class="copy-ip" onclick="copyToClipboard('${escapeJSString(rawVal)}', event)">${val}</span></td>`;
                } else if (idx === latencyIndex && latencyIndex !== -1) {
                    const latencyVal = parseFloat(rawVal);
                    const lcFactor = getLatencyColorFactor();
                    const color = getLatencyColor(latencyVal, lcFactor);
                    return `<td style="color:${color}; font-weight:600;">${val}</td>`;
                } else if (idx === lossIndex && lossIndex !== -1) {
                    const lossVal = parseFloat(rawVal);
                    const color = Number.isFinite(lossVal) && lossVal > 0 ? '#ef4444' : '#10b981';
                    return `<td style="color:${color}; font-weight:600;">${val}</td>`;
                } else if (idx === speedIndex && speedIndex !== -1) {
                    const speedVal = parseFloat(rawVal);
                    const color = speedStatus === 'untested' ? 'inherit' : (speedVal > SPEED_GREEN ? '#10b981' : (speedVal > 0 ? '#f59e0b' : '#ef4444'));
                    const cls = speedQualified ? '' : ' class="speed-unqualified"';
                    return `<td${cls} style="color:${color}; font-weight:600;"><span class="speed-cell-content"><span>${val}</span>${renderSpeedStatusTag(speedStatus)}</span></td>`;
                } else {
                    return `<td>${val}</td>`;
                }
            }).join('');
        }

        function nsbRowStatusFromCSV(headers, row) {
            const speedIndex = (headers || []).map(normalizeNSBHeader).indexOf('下载速度');
            if (speedIndex < 0) return 'qualified';
            return getSpeedStatus(row[speedIndex], isNSBSpeedQualified(row[speedIndex]));
        }

        function renderScanRowsCSV(headers, rows) {
            const tbody = document.querySelector('#scanTable tbody');
            tbody.innerHTML = '';
            renderScanHeaders('nsb');
            const visibleHeaders = filterNSBHeaders(headers);
            const originalInputIndex = headers.map(normalizeNSBHeader).indexOf('原始输入');

            const filteredRows = filterNSBRowsByLatencyLoss(filterNSBRowsByQualified(filterNSBRowsByIPType(headers, rows, nsbTableIPTypeFilter), headers, nsbTableQualifiedFilter), headers);
            filteredRows.forEach((r) => {
                const visibleRow = filterNSBRow(headers, r);
                const originalInput = originalInputIndex !== -1 ? r[originalInputIndex] : undefined;
                const tds = renderNSBRowFromCSV(visibleHeaders, visibleRow, originalInput);
                const rowStatus = nsbRowStatusFromCSV(headers, r);
                tbody.insertAdjacentHTML('beforeend', `<tr class="${rowStatus === 'qualified' ? 'row-qualified' : (rowStatus === 'unqualified' ? 'row-unqualified' : '')}">${tds}</tr>`);
            });

            updateBadge('scanCount', filteredRows.length);
            updateNSBSpeedButtons();
        }

        const NSB_LIVE_HEADERS = ['IP地址', '端口号', 'TLS', '丢包率', '扫描方式', '网络延迟', '下载速度', '出站IP', 'IP类型', 'OriginalInput', '数据中心', '源IP位置', '地区', '城市', 'ASN号码', 'ASN组织', '访问协议', 'TLS版本', 'SNI', 'HTTP版本', 'WARP', 'Gateway', 'RBI', '密钥交换', '时间戳'];
        const NSB_COMPACT_HEADERS = ['IP地址', '端口号', 'TLS', '丢包率', '扫描方式', '网络延迟', '下载速度', '出站IP', 'IP类型', 'OriginalInput', '数据中心', '源IP位置', '地区', '城市', 'ASN号码', 'ASN组织'];

        function normalizeNSBHeader(header) {
            if (header === 'IP') return 'IP地址';
            if (header === '端口') return '端口号';
            if (header === 'OriginalInput') return '原始输入';
            return header;
        }

        function filterNSBHeaders(headers) {
            const normalized = (headers || []).map(normalizeNSBHeader);
            if (!nsbCompactView) return normalized;
            return NSB_COMPACT_HEADERS.map(normalizeNSBHeader).filter((h) => normalized.includes(h));
        }

        function filterNSBRow(headers, row) {
            const normalized = (headers || []).map(normalizeNSBHeader);
            if (!nsbCompactView) return row;
            const compactNorm = NSB_COMPACT_HEADERS.map(normalizeNSBHeader);
            return compactNorm.filter((h) => normalized.includes(h)).map((h) => {
                const idx = normalized.indexOf(h);
                return idx !== -1 ? row[idx] : '';
            });
        }

        function getVisibleNSBHeaders() {
            if (nsbHeaders.length) return filterNSBHeaders(nsbHeaders);
            const base = nsbCompactView ? NSB_COMPACT_HEADERS : NSB_LIVE_HEADERS;
            return base.map(normalizeNSBHeader);
        }

        function nsbRowToArray(row) {
            return [
                row.ip || '', row.port || '', row.tls ?? '-', row.lossRate || '0%', getScanModeLabel(), row.latency || '-', row.speed || '-',
                row.outboundIP || '-', row.ipType || '-', row.originalInput || '', row.dc || '-', row.loc || '-',
                row.region || '-', row.city || '-', row.asnNumber || '-', row.asnOrg || '-',
                row.visitScheme || '-', row.tlsVersion || '-', row.sni || '-', row.httpVersion || '-',
                row.warp || '-', row.gateway || '-', row.rbi || '-',
                row.kex || '-', row.timestamp || '-'
            ];
        }

        function renderAllNSBLiveRows() {
            const tbody = document.querySelector('#scanTable tbody');
            tbody.innerHTML = '';
            nsbLiveRowElements = {};
            const rows = nsbLiveRows.filter((row) => matchesIPTypeFilter(row.ipType, nsbTableIPTypeFilter) && matchesQualifiedFilter(row, nsbTableQualifiedFilter) && matchesLatencyLossFilter(latencyMsForFilter(row.latency), parseFilterNumber(row.lossRate)));
            rows.forEach((row) => {
                appendNSBLiveRow(row);
            });
            updateBadge('scanCount', rows.length);
            updateNSBSpeedButtons();
        }

        function renderNSBLiveRowHTML(row) {
            const visibleHeaders = getVisibleNSBHeaders();
            const visibleRow = filterNSBRow(NSB_LIVE_HEADERS, nsbRowToArray(row));
            const tds = renderNSBRowFromCSV(visibleHeaders, visibleRow, row.originalInput);
            const rowStatus = getSpeedStatus(row.speed, isNSBSpeedQualified(row.speed));
            return `<tr class="${rowStatus === 'qualified' ? 'row-qualified' : (rowStatus === 'unqualified' ? 'row-unqualified' : '')}">${tds}</tr>`;
        }

        function appendNSBLiveRow(row) {
            const key = (row.ip || '') + ':' + (row.port || '');
            const tbody = document.querySelector('#scanTable tbody');
            tbody.insertAdjacentHTML('beforeend', renderNSBLiveRowHTML(row));
            nsbLiveRowElements[key] = tbody.lastElementChild;
        }

        function updateNSBLiveRowElement(row) {
            const key = (row.ip || '') + ':' + (row.port || '');
            const existing = nsbLiveRowElements[key];
            const visible = matchesIPTypeFilter(row.ipType, nsbTableIPTypeFilter) && matchesQualifiedFilter(row, nsbTableQualifiedFilter) && matchesLatencyLossFilter(latencyMsForFilter(row.latency), parseFilterNumber(row.lossRate));
            if (!visible) {
                if (existing) {
                    existing.remove();
                    delete nsbLiveRowElements[key];
                }
                return;
            }
            if (!existing) {
                appendNSBLiveRow(row);
                return;
            }
            existing.insertAdjacentHTML('afterend', renderNSBLiveRowHTML(row));
            const replacement = existing.nextElementSibling;
            existing.remove();
            nsbLiveRowElements[key] = replacement;
        }

        function updateNSBLiveBadgeFromData() {
            updateBadge('scanCount', nsbLiveRows.filter((item) => matchesIPTypeFilter(item.ipType, nsbTableIPTypeFilter) && matchesQualifiedFilter(item, nsbTableQualifiedFilter) && matchesLatencyLossFilter(latencyMsForFilter(item.latency), parseFilterNumber(item.lossRate))).length);
        }

        function addNSBLiveResult(row, render = true) {
            const key = (row.ip || '') + ':' + (row.port || '');
            const existingIdx = nsbLiveIndex[key];

            if (existingIdx !== undefined) {
                Object.assign(nsbLiveRows[existingIdx], row);
                if (!render) {
                    updateNSBLiveBadgeFromData();
                    updateNSBSpeedButtons();
                    return;
                }
                updateNSBLiveRowElement(nsbLiveRows[existingIdx]);
                updateBadge('scanCount', document.querySelector('#scanTable tbody').children.length);
                updateNSBSpeedButtons();
                return;
            }

            nsbLiveIndex[key] = nsbLiveRows.length;
            nsbLiveRows.push(row);
            if (!render) {
                updateNSBLiveBadgeFromData();
                updateNSBSpeedButtons();
                return;
            }
            if (!matchesIPTypeFilter(row.ipType, nsbTableIPTypeFilter) || !matchesQualifiedFilter(row, nsbTableQualifiedFilter) || !matchesLatencyLossFilter(latencyMsForFilter(row.latency), parseFilterNumber(row.lossRate))) {
                updateBadge('scanCount', document.querySelector('#scanTable tbody').children.length);
                updateNSBSpeedButtons();
                return;
            }
            appendNSBLiveRow(row);
            updateBadge('scanCount', document.querySelector('#scanTable tbody').children.length);
            updateNSBSpeedButtons();
        }

        function updateNSBDisplayedResult(row) {
            if (!row) return;
            const ip = String(row.ip || row.IP || '').trim();
            const port = String(row.port || row.Port || '').trim();
            if (!ip || !port) return;
            const headerSource = nsbHeaders.length ? nsbHeaders : NSB_LIVE_HEADERS;
            const normalizedHeaders = headerSource.map(normalizeNSBHeader);
            const key = `${ip}:${port}`;
            const patch = {
                IP地址: ip,
                端口号: port,
                TLS: row.tls ?? row.TLS ?? '-',
                丢包率: row.lossRate || row.LossRate || '0%',
                网络延迟: row.latency || row.Latency || '-',
                下载速度: row.speed || row.Speed || '-',
                出站IP: row.outboundIP || row.OutboundIP || '-',
                IP类型: row.ipType || row.IPType || '-',
                原始输入: row.originalInput || row.OriginalInput || '-',
                数据中心: row.dc || row.DC || '-',
                源IP位置: row.loc || row.Loc || '-',
                地区: row.region || row.Region || '-',
                城市: row.city || row.City || '-',
                ASN号码: row.asnNumber || row.ASNNumber || '-',
                ASN组织: row.asnOrg || row.ASNOrg || '-',
                访问协议: row.visitScheme || row.VisitScheme || '-',
                TLS版本: row.tlsVersion || row.TLSVersion || '-',
                SNI: row.sni || row.SNI || '-',
                HTTP版本: row.httpVersion || row.HTTPVersion || '-',
                WARP: row.warp || row.Warp || '-',
                Gateway: row.gateway || row.Gateway || '-',
                RBI: row.rbi || row.RBI || '-',
                密钥交换: row.kex || row.Kex || '-',
                时间戳: row.timestamp || row.Timestamp || '-',
            };
            if (nsbRows.length) {
                const idx = nsbRows.findIndex((r) => String(r[normalizedHeaders.indexOf('IP地址')] ?? '').trim() === ip && String(r[normalizedHeaders.indexOf('端口号')] ?? '').trim() === port);
                if (idx >= 0) {
                    const updated = nsbRows[idx].slice();
                    normalizedHeaders.forEach((header, hIdx) => {
                        if (patch[header] !== undefined && hIdx < updated.length) updated[hIdx] = patch[header];
                    });
                    nsbRows[idx] = updated;
                    renderScanRowsCSV(nsbHeaders, nsbRows);
                }
                return;
            }
            const liveIdx = nsbLiveIndex[key];
            if (liveIdx === undefined) return;
            Object.assign(nsbLiveRows[liveIdx], {
                ip,
                port,
                tls: patch.TLS,
                lossRate: patch['丢包率'],
                latency: patch['网络延迟'],
                speed: patch['下载速度'],
                outboundIP: patch['出站IP'],
                ipType: patch['IP类型'],
                dc: patch['数据中心'],
                loc: patch['源IP位置'],
                region: patch['地区'],
                city: patch['城市'],
                asnNumber: patch['ASN号码'],
                asnOrg: patch['ASN组织'],
                visitScheme: patch['访问协议'],
                tlsVersion: patch['TLS版本'],
                sni: patch.SNI,
                httpVersion: patch['HTTP版本'],
                warp: patch.WARP,
                gateway: patch.Gateway,
                rbi: patch.RBI,
                kex: patch['密钥交换'],
                timestamp: patch['时间戳'],
            });
            updateNSBLiveRowElement(nsbLiveRows[liveIdx]);
        }

        function normalizeIPTypeFilter(value) {
            const normalized = String(value || 'all').trim().toLowerCase();
            if (normalized === 'ipv4' || normalized === '4') return 'ipv4';
            if (normalized === 'ipv6' || normalized === '6') return 'ipv6';
            return 'all';
        }

        function matchesIPTypeFilter(value, filter) {
            filter = normalizeIPTypeFilter(filter);
            if (filter === 'all') return true;
            return String(value || '').trim().toLowerCase() === filter;
        }

        function filterNSBRowsByIPType(headers, rows, filter) {
            filter = normalizeIPTypeFilter(filter);
            if (filter === 'all') return rows || [];
            const normalized = (headers || []).map(normalizeNSBHeader);
            const ipTypeIndex = normalized.indexOf('IP类型');
            if (ipTypeIndex < 0) return [];
            return (rows || []).filter((row) => matchesIPTypeFilter(row[ipTypeIndex], filter));
        }

        function normalizeQualifiedFilter(value) {
            const normalized = String(value || 'all').trim().toLowerCase();
            if (normalized === 'qualified' || normalized === '合格') return 'qualified';
            if (normalized === 'unqualified' || normalized === '不合格') return 'unqualified';
            return 'all';
        }

        function isNSBSpeedQualified(value) {
            const speed = parseSpeedMB(value);
            return speed >= 0 && speed >= nsbSpeedMinValue;
        }

        function matchesQualifiedFilter(row, filter) {
            filter = normalizeQualifiedFilter(filter);
            if (filter === 'all') return true;
            const status = getSpeedStatus(row.speed, isNSBSpeedQualified(row.speed));
            return status === filter;
        }

        function parseFilterNumber(value) {
            const text = String(value ?? '').trim().replace('%', '').replace(/ms$/i, '').trim();
            const parsed = parseFloat(text);
            return Number.isFinite(parsed) ? parsed : NaN;
        }

        function latencyMsForFilter(latency) {
            if (Array.isArray(latency)) return parseFilterNumber(latency[latency.length - 1]);
            return parseFilterNumber(latency);
        }

        function matchesLatencyLossFilter(latencyMs, lossPct) {
            latencyMs = Number(latencyMs);
            lossPct = Number(lossPct);
            if (nsbTableLatencyFilter > 0 && !(Number.isFinite(latencyMs) && latencyMs < nsbTableLatencyFilter)) return false;
            if (nsbTableLossFilter < 100 && !(Number.isFinite(lossPct) && lossPct < nsbTableLossFilter)) return false;
            return true;
        }

        function officialLatencyLossMatch(item) {
            const latencyMs = (item.AvgLatency || 0) / 1_000_000;
            const lossPct = (item.LossRate ?? 0) * 100;
            if (officialTestLatencyFilter > 0 && !(latencyMs < officialTestLatencyFilter)) return false;
            if (officialTestLossFilter < 100 && !(lossPct < officialTestLossFilter)) return false;
            return true;
        }

        function filterNSBRowsByLatencyLoss(rows, headers) {
            if (nsbTableLatencyFilter <= 0 && nsbTableLossFilter >= 100) return rows || [];
            const normalized = (headers || []).map(normalizeNSBHeader);
            const latencyIndex = normalized.indexOf('网络延迟');
            const lossIndex = normalized.indexOf('丢包率');
            if (latencyIndex < 0 && lossIndex < 0) return rows || [];
            return (rows || []).filter((row) => {
                const latencyMs = latencyIndex >= 0 ? parseFilterNumber(row[latencyIndex]) : NaN;
                const lossPct = lossIndex >= 0 ? parseFilterNumber(row[lossIndex]) : NaN;
                return matchesLatencyLossFilter(latencyMs, lossPct);
            });
        }

        function filterNSBRowsByQualified(rows, headers, filter) {
            filter = normalizeQualifiedFilter(filter);
            if (filter === 'all') return rows || [];
            const normalized = (headers || []).map(normalizeNSBHeader);
            const speedIndex = normalized.indexOf('下载速度');
            if (speedIndex < 0) return filter === 'qualified' ? (rows || []) : [];
            return (rows || []).filter((row) => {
                const status = getSpeedStatus(row[speedIndex], isNSBSpeedQualified(row[speedIndex]));
                return status === filter;
            });
        }

        function addScanResult(row, render = true) {
            scanResultsCount++;
            scanData.push(row);
            updateBadge('scanCount', getFilteredOfficialScanData().length);
            if (render) renderScanRowOfficial(row);
        }

        function sortAndRenderScanResults() {
            scanData.sort((a, b) => parseFloat(a.LatencyStr) - parseFloat(b.LatencyStr));
            const tbody = document.querySelector('#scanTable tbody');
            tbody.innerHTML = '';
            const rows = getFilteredOfficialScanData();
            rows.forEach(renderScanRowOfficial);
            updateBadge('scanCount', rows.length);
        }

        function compareOfficialResultObjects(a, b) {
            const speedA = parseSpeedMB(a.speed);
            const speedB = parseSpeedMB(b.speed);
            const hasSpeedA = speedA >= 0;
            const hasSpeedB = speedB >= 0;
            if (hasSpeedA !== hasSpeedB) return hasSpeedA ? -1 : 1;
            if (hasSpeedA && speedA !== speedB) return speedB - speedA;
            const latencyA = parseFloat(a.latency);
            const latencyB = parseFloat(b.latency);
            return (Number.isFinite(latencyA) ? latencyA : Number.MAX_SAFE_INTEGER) - (Number.isFinite(latencyB) ? latencyB : Number.MAX_SAFE_INTEGER);
        }

        function parseSpeedMB(value) {
            const text = String(value || '');
            if (!text.includes('MB/s')) return -1;
            const speed = parseFloat(text);
            return Number.isFinite(speed) ? speed : -1;
        }

        function updateBadge(id, v) {
            document.getElementById(id).textContent = v;
        }

        function showDCSelection(list) {
            dcList = list || [];
            const tbody = document.querySelector('#dcTable tbody');
            tbody.innerHTML = '';
            dcList.forEach((dc) => {
                const isTested = testedDCs.has(dc.DataCenter);
                const isTesting = currentTestingDC === dc.DataCenter;
                const lcFactor = getLatencyColorFactor();
                const latencyColor = getLatencyColor(dc.MinLatency, lcFactor);
                const btn = isTesting ? `<button class="action-btn" disabled>测试中...</button>` : (isTested ? `<button class="action-btn selected" onclick="selectDC('${dc.DataCenter}')">重新测试</button>` : `<button class="action-btn" onclick="selectDC('${dc.DataCenter}')">选择测试</button>`);
                const tr = `<tr><td><strong>${dc.DataCenter}</strong></td><td>${dc.DCCountry || '-'}</td><td>${dc.City}</td><td>${dc.IPCount}</td><td style="color:${latencyColor}; font-weight:600;">${dc.MinLatency} ms</td><td>${btn}</td></tr>`;
                tbody.insertAdjacentHTML('beforeend', tr);
            });
            updateBadge('dcCount', dcList.length);
            document.getElementById('dcSection').style.display = dcList.length ? 'block' : 'none';
        }

        function selectDC(dc) {
            if (requireProxyWarning(() => selectDC(dc))) return;
            if (isTaskRunning && currentTestingDC !== '') return;
            const port = parseInt(document.getElementById('officialTestPort').value);
            const delay = parseInt(document.getElementById('officialDelay').value) || 500;

            testResultsCount = 0;
            autoUploadKeys = new Set();
            updateBadge('testCount', 0);
            testData = [];
            officialSpeedEnabledForResults = false;
            officialContinueSpeedUnlocked = false;
            setOfficialTestQualifiedFilter('all');
            officialTestLatencyFilter = 0;
            officialTestLossFilter = 100;
            currentTestingDC = dc;
            document.querySelector('#testTable tbody').innerHTML = '';
            document.getElementById('currentTestDC').textContent = `测试中: ${dc}`;
            isTaskRunning = true;
            runningTaskLabel = `官方优选 ${dc}`;
            runningTaskMode = 'official';
            runningTaskTab = 'test';
            setStartLoading(getRunningButtonText());
            resetProgress();
            progressBox.style.display = 'block';
            showDCSelection(dcList);
            if (!wsSend({ type: 'start_test', data: { dc, port, delay, scanMode: getScanMode(), deferR2Upload: officialAutoSpeedPending && officialSpeedLimitForAuto() > 0, r2AutoUpload: isR2AutoUploadSelected('officialAutoUploadAccount') } })) {
                currentTestingDC = '';
                isTaskRunning = false;
                showDCSelection(dcList);
                finishRunningTask('✨ 重新开始扫描');
                showNoticeCard('连接未就绪，请等待重连后再试');
                return;
            }
            switchTab('test');
        }

        function renderTestRow(res) {
            const port = res.Port || parseInt(document.getElementById('officialTestPort').value) || 443;
            const meta = testResultToObject(res);
            const speedText = res.Speed || '未测速';
            const speedStatus = getSpeedStatus(speedText, isOfficialSpeedQualified(speedText));
            const speedColor = speedText.includes('MB/s') ? (parseFloat(speedText) > SPEED_GREEN ? '#10b981' : '#f59e0b') : (speedText === '未测速' ? 'inherit' : '#ef4444');
            const isTesting = currentSpeedTestingIP === res.IP;
            const speedDisabled = isTesting || isTaskRunning || officialBatchSpeedRunning;
            const buttonText = isTesting ? '测速中' : (res.Speed ? '重新测速' : '测速');
            const lcFactor = getLatencyColorFactor();
            const row = `<tr class="${speedStatus === 'qualified' ? 'row-qualified' : (speedStatus === 'unqualified' ? 'row-unqualified' : '')}"><td>${renderIPCell(res.IP, res.OriginalInput || res.originalInput)}</td><td><span class="copy-ip" onclick="copyToClipboard('${port}', event)">${port}</span></td><td>${meta.dc || '-'}</td><td>${meta.dcCountry || '-'}</td><td>${meta.region || '-'}</td><td>${meta.city || '-'}</td><td>${getScanModeLabel()}</td><td>${(res.LossRate * 100).toFixed(0)}%</td><td>${(res.MinLatency / 1_000_000).toFixed(0)} ms</td><td>${(res.MaxLatency / 1_000_000).toFixed(0)} ms</td><td style="color:${getLatencyColor(res.AvgLatency / 1_000_000, lcFactor)}; font-weight:600;">${(res.AvgLatency / 1_000_000).toFixed(0)} ms</td><td style="display:flex;justify-content:space-between;align-items:center;gap:8px;border-bottom:none;"><span class="speed-result" id="speed-val-${res.IP.replace(/[:.]/g, '_')}" style="color:${speedColor};"><span>${speedText}</span>${renderSpeedStatusTag(speedStatus)}</span><button class="action-btn speed-btn" id="btn-speed-${res.IP.replace(/[:.]/g, '_')}" onclick="runSpeedTest('${res.IP}')" ${speedDisabled ? 'disabled' : ''}>${buttonText}</button></td></tr>`;
            document.querySelector('#testTable tbody').insertAdjacentHTML('beforeend', row);
        }

        function addTestResult(res, render = true) {
            testResultsCount++;
            testData.push(res);
            updateBadge('testCount', getFilteredOfficialTestData().length);
            if (render && matchesOfficialQualifiedFilter(res, officialTestQualifiedFilter)) renderTestRow(res);
            updateBatchSpeedButton();
        }

        function getFilteredOfficialTestData() {
            return testData.filter((row) => matchesOfficialQualifiedFilter(row, officialTestQualifiedFilter) && officialLatencyLossMatch(row));
        }

        function setOfficialTestQualifiedFilter(value) {
            syncOfficialSpeedConfigState();
            officialTestQualifiedFilter = normalizeQualifiedFilter(value);
            const filter = document.getElementById('officialTestQualifiedFilter');
            if (filter && filter.value !== officialTestQualifiedFilter) filter.value = officialTestQualifiedFilter;
            if (currentMode === 'official') renderAllTestResults();
        }

        function renderAllTestResults() {
            syncOfficialSpeedConfigState();
            const tbody = document.querySelector('#testTable tbody');
            tbody.innerHTML = '';
            const rows = getFilteredOfficialTestData();
            testResultsCount = rows.length;
            updateBadge('testCount', testResultsCount);
            rows.forEach(renderTestRow);
            updateBatchSpeedButton();
        }

        function onTestComplete() {
            if (currentTestingDC) testedDCs.add(currentTestingDC);
            currentTestingDC = '';
            showDCSelection(dcList);
            const status = document.getElementById('currentTestDC');
            status.textContent = '测试完成';
        }

        function runSpeedTest(ip) {
            if (requireProxyWarning(() => runSpeedTest(ip))) return;
            syncOfficialSpeedConfigState();
            if (isTaskRunning || officialBatchSpeedRunning) {
                log('当前已有任务运行，单个测速已忽略');
                return;
            }
            if (currentSpeedTestingIP && currentSpeedTestingIP !== ip) {
                log(`已有测速任务进行中: ${currentSpeedTestingIP}`);
                return;
            }

            const id = `btn-speed-${ip.replace(/[:.]/g, '_')}`;
            const btn = document.getElementById(id);
            if (btn) {
                btn.disabled = true;
                btn.textContent = '测速中';
            }
            const speedVal = document.getElementById(`speed-val-${ip.replace(/[:.]/g, '_')}`);
            if (speedVal) speedVal.textContent = '⏳...';
            currentSpeedTestingIP = ip;
            const port = parseInt(document.getElementById('officialTestPort').value);
            const customUrl = resolveSpeedTestUrl('officialSpeedUrl', 'officialSpeedUrlPreset');
            if (!wsSend({ type: 'start_speed_test', data: { ip, port, url: customUrl } })) {
                if (speedVal) speedVal.textContent = '连接未就绪';
                if (btn) {
                    btn.textContent = '重新测速';
                    btn.disabled = false;
                }
                currentSpeedTestingIP = '';
            }
        }

        function getLatencyColorFactor() {
            if (getScanMode() !== 'httping') return 1;
            if (currentMode === 'official') {
                const port = parseInt(document.getElementById('officialTestPort')?.value) || 443;
                return [443, 2053, 2083, 2087, 2096, 8443].includes(port) ? 4.0 : 1.3;
            }
            const tls = String(document.getElementById('nsbTls')?.value || 'on').toLowerCase();
            return tls !== 'off' ? 4.0 : 1.3;
        }

        function getLatencyColor(val, factor) {
            const v = parseFloat(val);
            if (isNaN(v)) return 'inherit';
            const adjusted = factor ? v / factor : v;
            const matched = LATENCY_COLORS.find((item) => adjusted <= item.end);
            return matched ? matched.color : 'inherit';
        }

        function updateSpeedResult(data) {
            syncOfficialSpeedConfigState();
            const btn = document.getElementById(`btn-speed-${data.ip.replace(/[:.]/g, '_')}`);
            const val = document.getElementById(`speed-val-${data.ip.replace(/[:.]/g, '_')}`);
            const item = testData.find((r) => r.IP === data.ip);
            if (item) item.Speed = data.speed;
            officialSpeedEnabledForResults = testData.some((r) => isOfficialSpeedQualified(r.Speed));
            const speedStatus = getSpeedStatus(data.speed, isOfficialSpeedQualified(data.speed));
            if (currentSpeedTestingIP === data.ip) {
                currentSpeedTestingIP = '';
            }
            if (val) {
                val.innerHTML = `<span>${escapeHTML(data.speed)}</span>${renderSpeedStatusTag(speedStatus)}`;
                val.style.color = data.speed.includes('MB/s') ? (parseFloat(data.speed) > SPEED_GREEN ? '#10b981' : '#f59e0b') : (speedStatus === 'untested' ? 'inherit' : '#ef4444');
                const tr = val.closest('tr');
                if (tr) {
                    tr.classList.toggle('row-qualified', speedStatus === 'qualified');
                    tr.classList.toggle('row-unqualified', speedStatus === 'unqualified');
                }
            }
            if (btn) {
                btn.textContent = '重新测速';
                btn.disabled = isTaskRunning || officialBatchSpeedRunning;
            }
            updateBatchSpeedButton();
        }

        // 自动测速只有在上限大于 0 时才会真正执行；上限为 0 时详细测试就是本次任务的终点，
        // 不能延后 R2 上传，否则该场景不会有任何上传。
        function officialSpeedLimitForAuto() {
            return parseInt(document.getElementById('officialSpeedLimit')?.value || '0', 10) || 0;
        }

        function getOfficialSpeedConfig() {
            const port = parseInt(document.getElementById('officialTestPort')?.value || '443', 10) || 443;
            const url = resolveSpeedTestUrl('officialSpeedUrl', 'officialSpeedUrlPreset');
            const speedMin = parseFloat(document.getElementById('officialSpeedMin')?.value || '0.1') || 0.1;
            const speedLimit = parseInt(document.getElementById('officialSpeedLimit')?.value || '0', 10) || 0;
            return { port, url, speedMin, speedLimit, results: testData.slice() };
        }

        function syncOfficialSpeedConfigState(cfg = getOfficialSpeedConfig()) {
            officialSpeedMinValue = cfg.speedMin;
            officialSpeedEnabledForResults = cfg.speedLimit > 0;
            return cfg;
        }

        function updateBatchSpeedButton() {
            const btn = document.getElementById('btnBatchSpeed');
            const continueBtn = document.getElementById('btnContinueBatchSpeed');
            const disabled = officialBatchSpeedRunning || isTaskRunning || !testData.length || currentMode !== 'official';
            if (btn) {
                btn.disabled = disabled;
                btn.textContent = officialBatchSpeedRunning ? '测速中...' : '🚀 一键测速';
            }
            if (!continueBtn) return;
            continueBtn.disabled = disabled || !officialContinueSpeedUnlocked || !testData.some((row) => getSpeedStatus(row.Speed, isOfficialSpeedQualified(row.Speed)) === 'untested');
            continueBtn.textContent = officialBatchSpeedRunning ? '测速中...' : '⏭️ 继续测速';
        }

        function startOfficialBatchSpeed(auto = false, continued = false) {
            if (requireProxyWarning(() => startOfficialBatchSpeed(auto, continued))) return false;
            if (officialBatchSpeedRunning) return false;
            if (!testData.length) {
                if (!auto) showNoticeCard('请先完成数据中心详细测试');
                return false;
            }
            const cfg = syncOfficialSpeedConfigState();
            if (cfg.speedLimit <= 0) {
                if (!auto) showNoticeCard('测速结果上限必须大于 0');
                else log('官方自动测速已跳过：测速结果上限为 0');
                return false;
            }
            officialBatchSpeedRunning = true;
            isTaskRunning = true;
            isStoppingTask = false;
            runningTaskLabel = '官方一键测速';
            runningTaskMode = 'official';
            runningTaskTab = 'test';
            if (!continued) {
                testData.forEach((row) => { row.Speed = ''; });
            }
            setOfficialTestQualifiedFilter('all');
            officialTestLatencyFilter = 0;
            officialTestLossFilter = 100;
            renderAllTestResults();
            setStartLoading(getRunningButtonText());
            resetProgress();
            progressBox.style.display = 'block';
            updateBatchSpeedButton();
            if (!wsSend({ type: 'start_official_speed_batch', data: { ...cfg, skipTested: continued, r2AutoUpload: isR2AutoUploadSelected('officialAutoUploadAccount') } })) {
                officialBatchSpeedRunning = false;
                finishRunningTask('✨ 开始扫描与测试');
                updateBatchSpeedButton();
                showNoticeCard('连接未就绪，请等待重连后再试');
                return false;
            }
            return true;
        }

        function startNSBBatchSpeed(continued = false) {
            if (requireProxyWarning(() => startNSBBatchSpeed(continued))) return false;
            if (nsbBatchSpeedRunning) return false;
            if (currentMode !== 'nsb') {
                showNoticeCard('请切换到非标模式后再测速');
                return false;
            }
            const enableSpeedTest = document.getElementById('nsbEnableSpeed');
            if (enableSpeedTest && String(enableSpeedTest.value || 'off').toLowerCase() !== 'on') {
                enableSpeedTest.value = 'on';
                updateNSBSpeedControls();
            }
            const rows = getNSBSpeedTargetRows();
            if (!rows.length) {
                showNoticeCard('当前没有可测速的非标结果');
                return false;
            }
            const cfg = syncNSBSpeedConfigState();
            if (cfg.speedTest <= 0) {
                showNoticeCard('测速线程必须大于 0');
                return false;
            }
            if (cfg.speedLimit <= 0) {
                showNoticeCard('测速结果上限必须大于 0');
                return false;
            }
            nsbBatchSpeedRunning = true;
            isTaskRunning = true;
            isStoppingTask = false;
            runningTaskLabel = continued ? '非标继续测速' : '非标一键测速';
            runningTaskMode = 'nsb';
            runningTaskTab = 'scan';
            if (!continued) {
                rows.forEach((row) => { row.speed = ''; });
            }
            setNSBTableQualifiedFilter('all');
            nsbTableLatencyFilter = 0;
            nsbTableLossFilter = 100;
            setStartLoading(getRunningButtonText());
            resetProgress();
            progressBox.style.display = 'block';
            updateNSBSpeedButtons();
            if (!wsSend({ type: 'start_nsb_speed_batch', data: { results: rows, speedTest: cfg.speedTest, speedURL: cfg.speedURL, enableTLS: cfg.enableTLS, speedMin: cfg.speedMin, speedLimit: cfg.speedLimit, skipTested: continued, compact: nsbCompactView, r2AutoUpload: isR2AutoUploadSelected('nsbAutoUploadAccount') } })) {
                nsbBatchSpeedRunning = false;
                finishRunningTask('✨ 开始扫描与测试');
                updateNSBSpeedButtons();
                showNoticeCard('连接未就绪，请等待重连后再试');
                return false;
            }
            return true;
        }

        function resetCurrentSpeedButton() {
            if (!currentSpeedTestingIP) return;
            const ip = currentSpeedTestingIP;
            const btn = document.getElementById(`btn-speed-${ip.replace(/[:.]/g, '_')}`);
            const val = document.getElementById(`speed-val-${ip.replace(/[:.]/g, '_')}`);
            const item = testData.find((r) => r.IP === ip);
            if (btn) {
                btn.textContent = '重新测速';
                btn.disabled = false;
            }
            if (val && val.textContent === '⏳...') {
                const fallbackSpeed = item && item.Speed ? item.Speed : '未测速';
                val.textContent = fallbackSpeed;
                val.style.color = fallbackSpeed.includes('MB/s') ? (parseFloat(fallbackSpeed) > 10 ? '#10b981' : '#f59e0b') : '';
            }
            currentSpeedTestingIP = '';
            updateBatchSpeedButton();
        }

        function copyToClipboard(text, event) {
            const showHint = (msg, color) => {
                const hint = document.createElement('span');
                hint.className = 'copy-hint';
                hint.style.color = color;
                hint.textContent = msg;
                hint.style.left = (event.clientX - 20) + 'px';
                hint.style.top = (event.clientY - 20) + 'px';
                document.body.appendChild(hint);
                setTimeout(() => hint.remove(), 800);
            };

            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(text).then(() => {
                    showHint('复制成功！', '#10b981');
                    log('已复制: ' + text);
                }).catch(() => {
                    fallbackCopy();
                });
            } else {
                fallbackCopy();
            }

            function fallbackCopy() {
                const textarea = document.createElement('textarea');
                textarea.value = text;
                textarea.style.position = 'fixed';
                textarea.style.left = '-999999px';
                textarea.style.top = '-999999px';
                document.body.appendChild(textarea);
                textarea.select();
                try {
                    const successful = document.execCommand('copy');
                    if (successful) {
                        showHint('复制成功！', '#10b981');
                        log('已复制: ' + text);
                    } else {
                        showHint('复制失败！', '#ef4444');
                        log('复制失败: ' + text);
                    }
                } catch (err) {
                    showHint('复制失败！', '#ef4444');
                    log('复制失败: ' + text);
                } finally {
                    document.body.removeChild(textarea);
                }
            }
        }

        function startTask() {
            if (isTaskRunning || officialBatchSpeedRunning || nsbBatchSpeedRunning) {
                showConfirmCard(getStopConfirmText(), confirmStopCurrentTask, '终止当前任务');
                return;
            }

            if (requireProxyWarning(() => startTask())) return;

            const readiness = nzStartReadiness();
            if (!readiness.ready) {
                nzUpdateStartReadiness();
                showNoticeCard('非标优选需要先提供 IP 文件或网络 URL，请在右侧「非标优选」分组中配置');
                nzShowConfigPanel('nsb');
                return;
            }
            nzSoftWarnings();

            if (currentMode === 'official') {
                startOfficialTask();
            } else {
                startNSBTask();
            }
        }

        function confirmStopCurrentTask() {
            if (!isTaskRunning) return;
            if (!wsSend({ type: 'stop_task', data: {} })) {
                showNoticeCard('连接未就绪，无法发送停止命令');
                return;
            }
            isStoppingTask = true;
            setStartLoading('正在停止...');
            showStoppingProgress();
            startStopWatchdog();
        }

        function getOfficialIpType() {
            return String(document.getElementById('officialIpType')?.value) === '6' ? 6 : 4;
        }

        function currentDcLibraryStatus() {
            const ipType = getOfficialIpType();
            return dcLibraryStatusMap[String(ipType)] || { ipType, exists: false, updatedAt: 0, ipCount: 0, dcCount: 0, dataCenters: [] };
        }

        function applyDcLibraryStatus(libraries) {
            if (libraries && typeof libraries === 'object') dcLibraryStatusMap = libraries;
            renderOfficialDcLibrary();
        }

        function formatDcLibraryTime(ts) {
            const value = Number(ts) || 0;
            if (!value) return '未知时间';
            const date = new Date(value * 1000);
            return Number.isNaN(date.getTime()) ? '未知时间' : date.toLocaleString();
        }

        function formatDcLibraryLabel(dc) {
            const place = [dc.city, dc.country].filter(v => v && String(v).trim()).join(' / ');
            return place ? `${dc.colo} · ${place} (${dc.ipCount || 0})` : `${dc.colo} (${dc.ipCount || 0})`;
        }

        function renderOfficialDcLibrary() {
            const select = document.getElementById('officialDcSelect');
            if (!select) return;
            const manualRow = document.getElementById('officialDcManualRow');
            const statusInput = document.getElementById('officialDcLibraryStatus');
            const status = currentDcLibraryStatus();
            const dataCenters = Array.isArray(status.dataCenters) ? status.dataCenters : [];
            const hasLibrary = Number(status.ipCount || 0) > 0;
            const previous = officialDcSelection;

            select.innerHTML = '';
            const allOption = document.createElement('option');
            allOption.value = '';
            allOption.textContent = '全部';
            select.appendChild(allOption);
            dataCenters.forEach(dc => {
                if (!dc || !dc.colo) return;
                const option = document.createElement('option');
                option.value = String(dc.colo).toUpperCase();
                option.textContent = formatDcLibraryLabel(dc);
                select.appendChild(option);
            });
            const manualOption = document.createElement('option');
            manualOption.value = '__manual__';
            manualOption.textContent = '手动输入其他代码';
            select.appendChild(manualOption);

            if (!hasLibrary) {
                officialDcSelection = '';
                select.value = '';
                select.disabled = true;
                if (manualRow) manualRow.classList.add('hidden');
                if (statusInput) statusInput.value = '尚无数据中心库，请点击右侧按钮更新；「全部」仍可直接扫描';
                return;
            }

            select.disabled = false;
            const validValues = Array.from(select.options).map(option => option.value);
            if (previous && validValues.includes(previous)) {
                select.value = previous;
                officialDcSelection = previous;
            } else if (previous && previous !== '__manual__') {
                const manualInput = document.getElementById('officialTargetDC');
                if (manualInput && !manualInput.value.trim()) manualInput.value = previous;
                select.value = '__manual__';
                officialDcSelection = '__manual__';
            } else {
                select.value = previous === '__manual__' ? '__manual__' : '';
                officialDcSelection = select.value;
            }
            if (manualRow) manualRow.classList.toggle('hidden', officialDcSelection !== '__manual__');
            if (statusInput) {
                statusInput.value = `更新于 ${formatDcLibraryTime(status.updatedAt)} · ${status.dcCount || dataCenters.length} 个数据中心 · ${status.ipCount || 0} 个 IP`;
            }
        }

        function syncOfficialDcSelection() {
            const select = document.getElementById('officialDcSelect');
            if (!select) return;
            officialDcSelection = select.value || '';
            const manualRow = document.getElementById('officialDcManualRow');
            if (manualRow) manualRow.classList.toggle('hidden', officialDcSelection !== '__manual__');
            if (officialDcSelection === '__manual__') {
                document.getElementById('officialTargetDC')?.focus();
            }
        }

        function applyOfficialTargetDCSelection(value) {
            const raw = String(value || '').trim().toUpperCase();
            const manualInput = document.getElementById('officialTargetDC');
            if (manualInput) manualInput.value = raw;
            officialDcSelection = '';
            if (raw) {
                const status = currentDcLibraryStatus();
                const inLibrary = (status.dataCenters || []).some(dc => String(dc.colo || '').toUpperCase() === raw);
                officialDcSelection = inLibrary ? raw : '__manual__';
            }
            renderOfficialDcLibrary();
        }

        function getOfficialTargetDC() {
            if (officialDcSelection === '__manual__') {
                return String(document.getElementById('officialTargetDC')?.value || '').trim().toUpperCase();
            }
            return officialDcSelection || '';
        }

        function isOfficialDcFromLibrary() {
            return !!officialDcSelection && officialDcSelection !== '__manual__';
        }

        function startDcLibraryScan() {
            if (isTaskRunning) {
                showNoticeCard('已有任务正在运行，请等待完成后再更新数据中心库');
                return;
            }
            if (requireProxyWarning(() => startDcLibraryScan())) return;
            const ipType = getOfficialIpType();
            const threads = parseThreadsValue(document.getElementById('officialThreads').value);
            const port = parseInt(document.getElementById('officialTestPort').value) || 443;
            const delay = parseInt(document.getElementById('officialDelay').value) || 0;
            const scanMode = getScanMode();
            dcLibraryScanning = true;
            isTaskRunning = true;
            isStoppingTask = false;
            runningTaskLabel = '更新数据中心库';
            runningTaskMode = 'official';
            runningTaskTab = 'scan';
            setStartLoading(getRunningButtonText());
            resetProgress();
            progressBox.style.display = 'block';
            const updateBtn = document.getElementById('btnUpdateDcLibrary');
            if (updateBtn) updateBtn.disabled = true;
            if (!wsSend({ type: 'start_dc_library_scan', data: { ipType, threads, port, delay, scanMode } })) {
                dcLibraryScanning = false;
                finishRunningTask('✨ 开始扫描与测试');
                showNoticeCard('连接未就绪，请等待重连后再试');
            }
        }

        function startOfficialTask() {
            const ipType = parseInt(document.getElementById('officialIpType').value);
            const threads = parseThreadsValue(document.getElementById('officialThreads').value);
            const port = parseInt(document.getElementById('officialTestPort').value) || 443;
            const delay = parseInt(document.getElementById('officialDelay').value) || 0;
            const officialSpeedConfig = syncOfficialSpeedConfigState();
            const autoSpeed = String(document.getElementById('officialAutoSpeed')?.value || 'off').toLowerCase() === 'on';
            const stopCountRaw = document.getElementById('officialStopCount')?.value?.trim() || '';
            const parsedStopCount = stopCountRaw === '' ? 20 : parseInt(stopCountRaw, 10);
            const officialStopCount = Number.isFinite(parsedStopCount) && parsedStopCount >= 0 ? parsedStopCount : 20;
            officialSpeedEnabledForResults = autoSpeed && officialSpeedConfig.speedLimit > 0;
            officialAutoSpeedPending = autoSpeed;
            officialAutoStage = autoSpeed ? 'scan' : '';
            officialBatchSpeedRunning = false;
            officialContinueSpeedUnlocked = false;
            isStoppingTask = false;

            isTaskRunning = true;
            runningTaskLabel = '官方优选扫描';
            runningTaskMode = 'official';
            runningTaskTab = 'scan';
            setStartLoading(getRunningButtonText());
            resetProgress();
            progressBox.style.display = 'block';
            renderScanHeaders('official');
            document.querySelector('#scanTable tbody').innerHTML = '';
            document.querySelector('#dcTable tbody').innerHTML = '';
            document.getElementById('dcSection').style.display = 'none';
            document.querySelector('#testTable tbody').innerHTML = '';
            document.getElementById('currentTestDC').textContent = '';

            scanData = [];
            testData = [];
            dcList = [];
            testedDCs.clear();
            currentTestingDC = '';
            scanResultsCount = 0;
            testResultsCount = 0;
            autoUploadKeys = new Set();
            updateBadge('scanCount', 0);
            updateBadge('testCount', 0);
            updateBadge('dcCount', 0);

            if (!wsSend({ type: 'start_task', data: {
                ipType, threads, port, delay, scanMode: getScanMode(),
                autoSpeed: autoSpeed,
                officialTargetDC: getOfficialTargetDC(),
                officialDcFromLibrary: isOfficialDcFromLibrary(),
                officialStopCount: officialStopCount,
                officialSpeedPort: officialSpeedConfig.port,
                officialSpeedURL: officialSpeedConfig.url,
                officialSpeedMin: officialSpeedConfig.speedMin,
                officialSpeedLimit: officialSpeedConfig.speedLimit,
                r2AutoUpload: isR2AutoUploadSelected('officialAutoUploadAccount'),
            } })) {
                officialAutoSpeedPending = false;
                officialAutoStage = '';
                finishRunningTask('✨ 开始扫描与测试');
                showNoticeCard('连接未就绪，请等待重连后再试');
                return;
            }
            switchTab('scan');
        }

        function startNSBTask() {
            syncNSBInputMode();
            const sourceURL = nsbSourceUrlInput ? nsbSourceUrlInput.value.trim() : '';
            const hasFile = !!uploadedFileContent.trim();
            const hasURL = !!sourceURL;
            if (hasFile === hasURL) {
                showNoticeCard('请选择本地文件或填写网络 URL（二选一）');
                return;
            }

            const maxThreads = parseThreadsValue(document.getElementById('nsbThreads').value);
            const fallbackPort = parseInt(document.getElementById('nsbFallbackPort').value) || 0;
            const speedEnabled = String(document.getElementById('nsbEnableSpeed').value || 'off').toLowerCase() === 'on';
            const speedTestInput = parseInt(document.getElementById('nsbSpeedTest').value) || 0;
            const speedURL = resolveSpeedTestUrl('nsbSpeedUrl', 'nsbSpeedUrlPreset');
            const outFile = document.getElementById('nsbOutFile').value.trim() || 'ip.csv';
            const speedTest = speedEnabled ? speedTestInput : 0;
            nsbSpeedEnabledForResults = speedTest > 0 && (parseInt(document.getElementById('nsbSpeedLimit').value) || 0) > 0;
            nsbContinueSpeedUnlocked = false;
            const tlsMode = String(document.getElementById('nsbTls').value || 'on').toLowerCase();
            const enableTLS = tlsMode !== 'off';
            const delay = parseInt(document.getElementById('nsbDelay').value) || 0;
            const resultLimit = parseInt(document.getElementById('nsbResultLimit').value) || 0;
            const targetDC = document.getElementById('nsbTargetDC').value.trim();
            const speedMin = parseFloat(document.getElementById('nsbSpeedMin').value) || 0;
            const speedLimit = parseInt(document.getElementById('nsbSpeedLimit').value) || 0;
            nsbSpeedMinValue = speedMin;
            nsbCompactView = document.getElementById('nsbCompactView')?.value !== 'full';
            const compact = nsbCompactView;
            nsbTableQualifiedFilter = 'all';
            nsbTableLatencyFilter = 0;
            nsbTableLossFilter = 100;
            setNSBTableQualifiedFilter('all');

            if (isNaN(speedTestInput) || speedTestInput < 0) {
                showNoticeCard('测速线程必须为非负整数');
                return;
            }
            if (resultLimit <= 0) {
                showNoticeCard('延迟结果上限必须填写非 0 正整数');
                return;
            }
            if (fallbackPort <= 0 || fallbackPort > 65535) {
                showNoticeCard('备用端口必须为 1-65535');
                return;
            }

            isTaskRunning = true;
            isStoppingTask = false;
            runningTaskLabel = '非标优选';
            runningTaskMode = 'nsb';
            runningTaskTab = 'scan';

            updateNSBCompactToggle();
            setStartLoading(getRunningButtonText(), 'linear-gradient(135deg, var(--error-color), #dc2626)', '0 4px 12px rgba(239, 68, 68, 0.3), inset 0 1px 1px rgba(255,255,255,0.2)');
            renderScanHeaders('nsb');
            resetProgress();
            progressBox.style.display = 'block';
            document.querySelector('#scanTable tbody').innerHTML = '';
            document.getElementById('dcSection').style.display = 'none';
            document.querySelector('#testTable tbody').innerHTML = '';

            nsbHeaders = [];
            nsbRows = [];
            nsbLiveRows = [];
            nsbLiveIndex = {};
            nsbLiveRowElements = {};
            updateBadge('scanCount', 0);
            autoUploadKeys = new Set();

            if (!wsSend({
                type: 'start_nsb_task',
                data: {
                    fileName: uploadedFileName,
                    fileContent: hasFile ? uploadedFileContent : '',
                    sourceURL: hasURL ? sourceURL : '',
                    outFile,
                    maxThreads,
                    fallbackPort,
                    speedTest,
                    speedURL,
                    enableTLS,
                    delay,
                    resultLimit,
                    dc: targetDC,
                    speedMin,
                    speedLimit,
                    compact,
                    scanMode: getScanMode(),
                    relinkHost: getNSBRelinkHost(speedTest > 0),
                    r2AutoUpload: isR2AutoUploadSelected('nsbAutoUploadAccount'),
                }
            })) {
                finishRunningTask('✨ 开始扫描与测试');
                showNoticeCard('连接未就绪，请等待重连后再试');
            }
        }

        function escapeHTML(v) {
            return String(v)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/\"/g, '&quot;')
                .replace(/'/g, '&#039;');
        }

        const RESULT_FIELDS = [
            { key: 'ipport', label: 'IP:PORT', txt: true },
            { key: 'ip', label: 'IP地址' },
            { key: 'port', label: '端口号' },
            { key: 'tls', label: 'TLS' },
            { key: 'lossRate', label: '丢包率' },
            { key: 'latency', label: '网络延迟' },
            { key: 'speed', label: '下载速度' },
            { key: 'outboundIP', label: '出站IP' },
            { key: 'ipType', label: 'IP类型' },
            { key: 'originalInput', label: '原始输入' },
            { key: 'dc', label: '数据中心', txt: true },
            { key: 'dcCountry', label: '落地区域', txt: true },
            { key: 'loc', label: '源IP位置' },
            { key: 'region', label: '地区', txt: true },
            { key: 'city', label: '城市' },
            { key: 'asnNumber', label: 'ASN号码' },
            { key: 'asnOrg', label: 'ASN组织' },
            { key: 'visitScheme', label: '访问协议' },
            { key: 'tlsVersion', label: 'TLS版本' },
            { key: 'sni', label: 'SNI' },
            { key: 'httpVersion', label: 'HTTP版本' },
            { key: 'warp', label: 'WARP' },
            { key: 'gateway', label: 'Gateway' },
            { key: 'rbi', label: 'RBI' },
            { key: 'kex', label: '密钥交换' },
            { key: 'timestamp', label: '时间戳' },
        ];

        function openResultModal(mode) {
            resultModalMode = currentMode === 'nsb' ? 'nsb' : mode;
            const rows = getCurrentResultRows(resultModalMode);
            if (!rows.length) return showNoticeCard('暂无可操作结果');
            const formatSelect = document.getElementById('resultFormat');
            if (formatSelect && !formatSelect.value) formatSelect.value = 'txt';
            const nameInput = document.getElementById('resultFileName');
            if (nameInput && !nameInput.value.trim()) nameInput.value = resultModalMode === 'nsb' ? 'Cloudflare_NSB_Results' : (resultModalMode === 'test' ? 'Cloudflare_SpeedTest_Results' : 'Cloudflare_Scan_Results');
            applyResultModeFields();
            if (!resultFieldOrder.length) resultFieldOrder = getDefaultResultFieldOrder(document.getElementById('resultFormat').value);
            renderGitHubAccounts();
            renderResultFieldOptions();
            // 原弹窗改为右侧「结果操作」标签页；跳转时预选数据来源
            if (typeof nzShowConfigPanel === 'function') nzShowConfigPanel('results');
        }

        // 结果操作面板：同步数据来源选项可用性与模式相关字段的显隐，并即入即渲染动态配置
        function syncResultSourceOptions() {
            const sourceSelect = document.getElementById('resultSourceSelect');
            if (!sourceSelect) return;
            Array.from(sourceSelect.options).forEach(opt => {
                if (opt.value === 'nsb') opt.hidden = currentMode !== 'nsb';
                else opt.hidden = currentMode === 'nsb';
            });
            if (currentMode === 'nsb') {
                resultModalMode = 'nsb';
            } else if (resultModalMode === 'nsb' || !resultModalMode) {
                resultModalMode = 'scan';
            }
            if (sourceSelect.value !== resultModalMode) sourceSelect.value = resultModalMode;
            applyResultModeFields();
            // 即入即渲染：直接进入面板时字段选择/输出顺序/预览/账号下拉全部就绪，无需先点表格按钮
            try {
                if (!resultFieldOrder.length) resultFieldOrder = getDefaultResultFieldOrder((document.getElementById('resultFormat') || {}).value || 'txt');
                renderResultFieldOptions();
                renderGitHubAccounts();
                if (typeof renderEdgetunnelAccounts === 'function') renderEdgetunnelAccounts();
                syncR2AliasTemplate();
            } catch (e) { /* 结果数据未就绪时保持当前渲染 */ }
        }

        // 模式相关字段显隐与默认值（与原弹窗行为一致）
        function applyResultModeFields() {
            const filterField = document.getElementById('resultIPTypeFilterField');
            if (filterField) filterField.style.display = resultModalMode === 'nsb' ? '' : 'none';
            const filterSelect = document.getElementById('resultIPTypeFilter');
            if (filterSelect && resultModalMode !== 'nsb') filterSelect.value = 'all';
            const qualifiedField = document.getElementById('resultQualifiedFilterField');
            if (qualifiedField) qualifiedField.style.display = '';
            const qualifiedSelect = document.getElementById('resultQualifiedFilter');
            if (qualifiedSelect) qualifiedSelect.value = 'all';
            const nameInput = document.getElementById('resultFileName');
            if (nameInput && !nameInput.value.trim()) nameInput.value = resultModalMode === 'nsb' ? 'Cloudflare_NSB_Results' : (resultModalMode === 'test' ? 'Cloudflare_SpeedTest_Results' : 'Cloudflare_Scan_Results');
        }

        // 面板内手动切换数据来源
        function handleResultSourceChange(mode) {
            const rows = getCurrentResultRows(mode);
            if (!rows.length) {
                showNoticeCard('该来源暂无可操作结果');
                syncResultSourceOptions();
                return;
            }
            resultModalMode = mode;
            applyResultModeFields();
            renderResultFieldOptions();
            updateResultPreview();
        }

        function openSortModal(mode) {
            sortModalMode = mode === 'nsb' ? 'nsb' : 'official';
            const count = sortModalMode === 'nsb' ? (nsbRows.length || nsbLiveRows.length) : testData.length;
            if (!count) return showNoticeCard('暂无可排序结果');
            publishSortFilterControls();
            document.getElementById('sortModalTitle').textContent = sortModalMode === 'nsb' ? '非标测速 筛选排序' : '官方详细测试 筛选排序';
            document.getElementById('sortModalMask').classList.add('active');
        }

        function publishSortFilterControls() {
            const ipTypeField = document.getElementById('sortFilterIPTypeField');
            if (ipTypeField) ipTypeField.style.display = sortModalMode === 'nsb' ? '' : 'none';
            const latencyInput = document.getElementById('sortFilterLatency');
            const lossInput = document.getElementById('sortFilterLoss');
            if (sortModalMode === 'nsb') {
                const ipTypeSelect = document.getElementById('sortFilterIPType');
                if (ipTypeSelect) ipTypeSelect.value = nsbTableIPTypeFilter;
                const qualifiedSelect = document.getElementById('sortFilterQualified');
                if (qualifiedSelect) qualifiedSelect.value = nsbTableQualifiedFilter;
                if (latencyInput) latencyInput.value = nsbTableLatencyFilter;
                if (lossInput) lossInput.value = nsbTableLossFilter;
            } else {
                const qualifiedSelect = document.getElementById('sortFilterQualified');
                if (qualifiedSelect) qualifiedSelect.value = officialTestQualifiedFilter;
                if (latencyInput) latencyInput.value = officialTestLatencyFilter;
                if (lossInput) lossInput.value = officialTestLossFilter;
            }
        }

        function applySortModalFilters() {
            let latencyFilter = parseFloat(document.getElementById('sortFilterLatency')?.value);
            if (!Number.isFinite(latencyFilter) || latencyFilter < 0) latencyFilter = 0;
            let lossFilter = parseFloat(document.getElementById('sortFilterLoss')?.value);
            if (!Number.isFinite(lossFilter)) lossFilter = 100;
            lossFilter = Math.min(100, Math.max(0, lossFilter));
            if (lossFilter <= 0) lossFilter = 0.01;
            const qualifiedSelect = document.getElementById('sortFilterQualified');
            if (sortModalMode === 'nsb') {
                nsbTableIPTypeFilter = normalizeIPTypeFilter(document.getElementById('sortFilterIPType')?.value || 'all');
                nsbTableQualifiedFilter = normalizeQualifiedFilter(qualifiedSelect?.value || 'all');
                nsbTableLatencyFilter = latencyFilter;
                nsbTableLossFilter = lossFilter;
            } else {
                officialTestQualifiedFilter = normalizeQualifiedFilter(qualifiedSelect?.value || 'all');
                officialTestLatencyFilter = latencyFilter;
                officialTestLossFilter = lossFilter;
            }
            return { latencyFilter, lossFilter };
        }

        function closeSortModal() {
            document.getElementById('sortModalMask').classList.remove('active');
        }

        function getSortPriorities() {
            const values = ['sortPriority1', 'sortPriority2', 'sortPriority3'].map((id) => document.getElementById(id)?.value || '').filter(Boolean);
            return values.filter((value, index) => values.indexOf(value) === index);
        }

        function compareSortValues(a, b, key) {
            if (key === 'speed') {
                const speedA = parseSortSpeed(a.speed);
                const speedB = parseSortSpeed(b.speed);
                if (speedA.rank !== speedB.rank) return speedA.rank - speedB.rank;
                if (speedA.rank === 0 && speedA.value !== speedB.value) return speedB.value - speedA.value;
                return 0;
            }
            if (key === 'loss') return compareAscending(parseSortNumber(a.lossRate), parseSortNumber(b.lossRate));
            if (key === 'latency') return compareLatencyValues(a.latency, b.latency);
            return 0;
        }

        function compareLatencyValues(a, b) {
            const valuesA = Array.isArray(a) ? a : [a];
            const valuesB = Array.isArray(b) ? b : [b];
            const length = Math.max(valuesA.length, valuesB.length);
            for (let i = 0; i < length; i++) {
                const result = compareAscending(parseSortNumber(valuesA[i]), parseSortNumber(valuesB[i]));
                if (result !== 0) return result;
            }
            return 0;
        }

        function compareAscending(a, b) {
            const safeA = Number.isFinite(a) ? a : Number.MAX_SAFE_INTEGER;
            const safeB = Number.isFinite(b) ? b : Number.MAX_SAFE_INTEGER;
            return safeA - safeB;
        }

        function parseSortNumber(value) {
            const text = String(value ?? '').trim().replace('%', '').replace(/ms$/i, '').trim();
            const parsed = parseFloat(text);
            return Number.isFinite(parsed) ? parsed : Number.MAX_SAFE_INTEGER;
        }

        function parseSortSpeed(value) {
            const text = String(value ?? '').trim();
            if (!text || text === '-' || text === '未测速') return { rank: 1, value: 0 };
            if (text.includes('MB/s')) {
                const parsed = parseFloat(text);
                if (Number.isFinite(parsed)) return { rank: 0, value: parsed };
            }
            return { rank: 2, value: 0 };
        }

        function compareSortableRows(a, b, priorities) {
            for (const key of priorities) {
                const result = compareSortValues(a.row, b.row, key);
                if (result !== 0) return result;
            }
            return a.index - b.index;
        }

        function applySortModal() {
            applySortModalFilters();
            if (sortModalMode === 'nsb') {
                if (nsbRows.length) {
                    renderScanRowsCSV(nsbHeaders, nsbRows);
                } else if (nsbLiveRows.length) {
                    renderAllNSBLiveRows();
                }
            } else if (currentMode === 'official') {
                renderAllTestResults();
            }
            const priorities = getSortPriorities();
            if (!priorities.length) return showNoticeCard('请选择排序优先级');
            if (sortModalMode === 'nsb') applyNSBSort(priorities);
            else applyOfficialTestSort(priorities);
            closeSortModal();
        }

        function applyOfficialTestSort(priorities) {
            testData = testData.map((item, index) => ({ item, index, row: officialTestSortObject(item) }))
                .sort((a, b) => compareSortableRows(a, b, priorities))
                .map((entry) => entry.item);
            if (currentMode === 'official') renderAllTestResults();
            log('官方详细测试结果已按自定义优先级排序');
        }

        function officialTestSortObject(item) {
            return {
                lossRate: `${(item.LossRate || 0) * 100}%`,
                latency: [Math.round((item.MinLatency || 0) / 1_000_000), Math.round((item.MaxLatency || 0) / 1_000_000), Math.round((item.AvgLatency || 0) / 1_000_000)],
                speed: item.Speed || '未测速',
            };
        }

        function applyNSBSort(priorities) {
            if (nsbRows.length) {
                const normalizedHeaders = nsbHeaders.map(normalizeNSBHeader);
                const lossIndex = normalizedHeaders.indexOf('丢包率');
                const latencyIndex = normalizedHeaders.indexOf('网络延迟');
                const speedIndex = normalizedHeaders.indexOf('下载速度');
                nsbRows = nsbRows.map((item, index) => ({ item, index, row: { lossRate: item[lossIndex], latency: item[latencyIndex], speed: item[speedIndex] } }))
                    .sort((a, b) => compareSortableRows(a, b, priorities))
                    .map((entry) => entry.item);
                renderScanRowsCSV(nsbHeaders, nsbRows);
            } else {
                nsbLiveRows = nsbLiveRows.map((item, index) => ({ item, index, row: item }))
                    .sort((a, b) => compareSortableRows(a, b, priorities))
                    .map((entry) => entry.item);
                rebuildNSBLiveIndex();
                if (currentMode === 'nsb') renderAllNSBLiveRows();
            }
            log('非标测速结果已按自定义优先级排序');
        }

        function rebuildNSBLiveIndex() {
            nsbLiveIndex = {};
            nsbLiveRows.forEach((row, index) => {
                nsbLiveIndex[(row.ip || '') + ':' + (row.port || '')] = index;
            });
        }

        function getDefaultResultFieldOrder(format) {
            return getDefaultResultFieldOrderFor(resultModalMode, format);
        }

        function getDefaultResultFieldOrderFor(mode, format) {
            if (format === 'substore') return ['ipport'];
            if (format === 'txt') return mode === 'nsb' ? ['ipport', 'dc', 'loc'] : ['ipport', 'dc', 'dcCountry'];
            if (format === 'csv') return getCompactResultFieldOrderFor(mode);
            const rows = getCurrentResultRows(mode);
            if (!rows.length) return ['ipport'];
            return RESULT_FIELDS.filter((f) => rows.some((r) => String(r[f.key] ?? '').trim() !== '') && f.key !== 'ipport').map((f) => f.key);
        }

        function getAllResultFieldOrder() {
            return resultFieldChoiceKeys(RESULT_FIELDS, resultCustomFields);
        }

        function getCompactResultFieldOrder() {
            return getCompactResultFieldOrderFor(resultModalMode);
        }

        function getCompactResultFieldOrderFor(mode) {
            if (mode === 'nsb') return ['ip', 'port', 'tls', 'lossRate', 'latency', 'speed', 'outboundIP', 'ipType', 'originalInput', 'dc', 'loc', 'region', 'city', 'asnNumber', 'asnOrg'];
            return getOfficialResultFieldOrder(mode);
        }

        function getOfficialResultFieldOrder(mode = resultModalMode) {
            return mode === 'test'
                ? ['ip', 'port', 'latency', 'speed', 'dc', 'dcCountry', 'region', 'city']
                : ['ip', 'port', 'latency', 'dc', 'dcCountry', 'region', 'city'];
        }

        function renderResultFieldOptions() {
            const format = document.getElementById('resultFormat').value;
            if (!resultFieldOrder.length) resultFieldOrder = getDefaultResultFieldOrder(format);
            if (format === 'txt' && !resultFieldOrder.includes('ipport')) resultFieldOrder.unshift('ipport');
            const separatorField = document.getElementById('resultTextSeparatorField');
            if (separatorField) separatorField.style.display = format === 'txt' ? '' : 'none';
            const bracketsField = document.getElementById('resultIPv6BracketsField');
            if (bracketsField) bracketsField.style.display = format === 'txt' ? '' : 'none';
            // 字段选择固定列出全部内置字段 + 自定义字段，不按当前结果行裁剪：
            // 刷新页面或程序重启后前端没有结果行时，仍可预先勾选要导出的列。
            const available = buildResultFieldChoices(RESULT_FIELDS, resultCustomFields);
            document.querySelectorAll('.github-field').forEach((el) => el.style.display = '');
            document.querySelectorAll('.edgetunnel-field').forEach((el) => el.style.display = '');
            document.getElementById('resultFieldPicker').innerHTML = available.map((f) => `<label class="field-pill"><input type="checkbox" value="${f.key}" ${resultFieldOrder.includes(f.key) ? 'checked' : ''} onchange="toggleResultField('${f.key}', this.checked)"> ${f.label}</label>`).join('');
            updateResultFieldQuickButtons();
            renderResultOrderList();
            updateResultPreview();
        }

        function handleResultFormatChange() {
            const format = document.getElementById('resultFormat').value;
            resultFieldOrder = getDefaultResultFieldOrder(format);
            const pathInput = document.getElementById('githubPath');
            if (pathInput && pathInput.value.trim()) {
                const ext = format === 'substore' ? 'txt' : format;
                pathInput.value = pathInput.value.trim().replace(/\.(csv|txt)$/i, `.${ext}`);
            }
            renderResultFieldOptions();
        }

        function getGitHubAccounts() {
            try {
                const parsed = JSON.parse(localStorage.getItem(githubAccountStorageKey) || '[]');
                return Array.isArray(parsed) ? parsed : [];
            } catch (_) {
                return [];
            }
        }

        function saveGitHubAccounts(accounts) {
            localStorage.setItem(githubAccountStorageKey, JSON.stringify(accounts));
        }

        let r2AutoUploadReady = false;
        const autoUploadTouched = new Set();

        function markAutoUploadTouched(id) {
            autoUploadTouched.add(id);
        }

        function isR2AutoUploadSelected(id) {
            return document.getElementById(id)?.value === 'r2';
        }

        function renderAutoUploadSelect(id, accounts) {
            const select = document.getElementById(id);
            if (!select) return;
            const prev = select.value;
            select.innerHTML = '<option value="">关闭自动上传</option><option value="r2">R2 上传</option>' + accounts.map((a, i) => `<option value="${i}:txt">${escapeHTML(a.name || a.repo || 'GitHub账户')} - TXT</option><option value="${i}:csv">${escapeHTML(a.name || a.repo || 'GitHub账户')} - CSV</option>`).join('');
            const hasPrev = prev && Array.from(select.options).some((opt) => opt.value === prev);
            if (hasPrev) {
                select.value = prev;
            } else if (r2AutoUploadReady && !autoUploadTouched.has(id)) {
                select.value = 'r2';
            }
        }

        function syncAutoUploadSelection() {
            for (const id of ['nsbAutoUploadAccount', 'officialAutoUploadAccount']) {
                const select = document.getElementById(id);
                if (!select || autoUploadTouched.has(id)) continue;
                if (r2AutoUploadReady) {
                    select.value = 'r2';
                } else if (select.value === 'r2') {
                    select.value = '';
                }
            }
        }

        function renderGitHubAccounts() {
            const select = document.getElementById('githubAccountSelect');
            const accounts = getGitHubAccounts();
            const accountOptions = '<option value="">新账户/不使用保存配置</option>' + accounts.map((a, i) => `<option value="${i}">${escapeHTML(a.name || a.repo || 'GitHub账户')} ${a.tokenHash ? '(' + escapeHTML(a.tokenHash.slice(0, 10)) + ')' : ''}</option>`).join('');
            if (select) select.innerHTML = accountOptions;
            syncGitHubAccountPick();
            renderAutoUploadSelect('nsbAutoUploadAccount', accounts);
            renderAutoUploadSelect('officialAutoUploadAccount', accounts);
        }

        // 结果弹窗内的已保存账号选择：与「上传账号」面板字段双向同步
        function syncGitHubAccountPick() {
            const pick = document.getElementById('githubAccountPick');
            const select = document.getElementById('githubAccountSelect');
            if (!pick) return;
            const accounts = getGitHubAccounts();
            pick.innerHTML = '<option value="">使用当前填写信息</option>' + accounts.map((a, i) => `<option value="${i}">${escapeHTML(a.name || a.repo || 'GitHub账户')}</option>`).join('');
            pick.value = select ? select.value : '';
        }

        function pickGitHubAccountFromModal() {
            const pick = document.getElementById('githubAccountPick');
            const select = document.getElementById('githubAccountSelect');
            if (!pick || !select) return;
            select.value = pick.value;
            loadSelectedGitHubAccount();
        }

        function normalizeGitHubAccountKey(account) {
            return account?.name || account?.repo || '';
        }

        async function migrateLegacyGitHubToken(account, idx) {
            if (!account || !account.token) return '';
            const token = String(account.token || '');
            const accounts = getGitHubAccounts();
            const current = accounts[idx];
            if (!current) return token;
            delete current.token;
            current.tokenHash = current.tokenHash || await sha256Hex(token);
            current.tokenCipher = current.tokenCipher || await encryptGitHubToken(normalizeGitHubAccountKey(current), token);
            saveGitHubAccounts(accounts);
            githubSessionTokens.set(normalizeGitHubAccountKey(current), token);
            renderGitHubAccounts();
            return token;
        }

        function loadSelectedGitHubAccount() {
            const select = document.getElementById('githubAccountSelect');
            syncGitHubAccountPick();
            const idx = parseInt(select.value, 10);
            if (Number.isNaN(idx)) {
                clearGitHubAccountInputs();
                return;
            }
            const account = getGitHubAccounts()[idx];
            if (!account) return;
            document.getElementById('githubAccountName').value = account.name || '';
            document.getElementById('githubRepo').value = account.repo || '';
            document.getElementById('githubBranch').value = account.branch || 'main';
            document.getElementById('githubPath').value = account.path || '';
            document.getElementById('githubMessage').value = account.message || 'update cfdata results';
            const accountKey = normalizeGitHubAccountKey(account);
            const sessionToken = githubSessionTokens.get(accountKey) || '';
            document.getElementById('githubToken').value = sessionToken;
            document.getElementById('githubSaveToken').checked = !!account.tokenCipher || !!account.token || githubSessionTokens.has(accountKey);
            if (!sessionToken && account.token) {
                migrateLegacyGitHubToken(account, idx).then((token) => {
                    if (token && document.getElementById('githubAccountSelect').value === String(idx)) {
                        document.getElementById('githubToken').value = token;
                    }
                });
            } else if (!sessionToken && account.tokenCipher) {
                decryptStoredGitHubToken(account).then((token) => {
                    if (token) {
                        githubSessionTokens.set(accountKey, token);
                        if (document.getElementById('githubAccountSelect').value === String(idx)) {
                            document.getElementById('githubToken').value = token;
                        }
                    }
                });
            }
        }

        function clearGitHubAccountInputs() {
            ['githubAccountName', 'githubRepo', 'githubPath', 'githubToken'].forEach((id) => document.getElementById(id).value = '');
            document.getElementById('githubBranch').value = 'main';
            document.getElementById('githubMessage').value = 'update cfdata results';
            document.getElementById('githubSaveToken').checked = false;
        }

        function deleteSelectedGitHubAccount() {
            const select = document.getElementById('githubAccountSelect');
            const idx = parseInt(select.value, 10);
            if (Number.isNaN(idx)) return;
            const accounts = getGitHubAccounts();
            const deleted = accounts[idx];
            if (deleted) githubSessionTokens.delete(normalizeGitHubAccountKey(deleted));
            accounts.splice(idx, 1);
            saveGitHubAccounts(accounts);
            renderGitHubAccounts();
        }

        async function sha256Hex(text) {
            if (!text || !crypto.subtle) return '';
            const data = new TextEncoder().encode(text);
            const digest = await crypto.subtle.digest('SHA-256', data);
            return Array.from(new Uint8Array(digest)).map((b) => b.toString(16).padStart(2, '0')).join('');
        }

        function bytesToBase64(bytes) {
            return btoa(String.fromCharCode(...bytes));
        }

        function base64ToBytes(text) {
            return Uint8Array.from(atob(text), (c) => c.charCodeAt(0));
        }

        async function getGitHubTokenCryptoKey(accountName) {
            if (!crypto.subtle) return null;
            const material = await crypto.subtle.importKey('raw', new TextEncoder().encode(`${location.origin}|${accountName}|${githubTokenCryptoSalt}`), 'PBKDF2', false, ['deriveKey']);
            return crypto.subtle.deriveKey({ name: 'PBKDF2', salt: new TextEncoder().encode(githubTokenCryptoSalt), iterations: 120000, hash: 'SHA-256' }, material, { name: 'AES-GCM', length: 256 }, false, ['encrypt', 'decrypt']);
        }

        async function encryptGitHubToken(accountName, token) {
            const key = await getGitHubTokenCryptoKey(accountName);
            if (!key) return '';
            const iv = crypto.getRandomValues(new Uint8Array(12));
            const cipher = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, new TextEncoder().encode(token));
            return `${bytesToBase64(iv)}.${bytesToBase64(new Uint8Array(cipher))}`;
        }

        async function decryptStoredGitHubToken(account) {
            try {
                if (!account.tokenCipher) return '';
                const [ivText, cipherText] = account.tokenCipher.split('.');
                const key = await getGitHubTokenCryptoKey(account.name || account.repo || '');
                if (!key) return '';
                const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: base64ToBytes(ivText) }, key, base64ToBytes(cipherText));
                return new TextDecoder().decode(plain);
            } catch (_) {
                return '';
            }
        }

        async function saveCurrentGitHubAccount(payload) {
            const name = document.getElementById('githubAccountName').value.trim() || payload.repo;
            if (!name) return;
            const rememberToken = document.getElementById('githubSaveToken').checked;
            const accounts = getGitHubAccounts();
            const tokenHash = await sha256Hex(payload.token);
            const next = {
                name,
                repo: `${payload.owner}/${payload.repo}`,
                branch: payload.branch,
                path: payload.path,
                message: payload.message,
                tokenHash,
            };
            if (rememberToken) {
                githubSessionTokens.set(name, payload.token);
                next.tokenCipher = await encryptGitHubToken(name, payload.token);
                if (!next.tokenCipher) {
                    showNoticeCard('当前浏览器不支持 WebCrypto，无法安全保存 token；本次仍会上传，但不会持久保存 token');
                }
            } else {
                githubSessionTokens.delete(name);
            }
            const existing = accounts.findIndex((a) => a.name === name);
            if (existing >= 0) accounts[existing] = next;
            else accounts.push(next);
            saveGitHubAccounts(accounts);
            renderGitHubAccounts();
        }

        function getEdgetunnelAccounts() {
            try {
                const parsed = JSON.parse(localStorage.getItem(edgetunnelAccountStorageKey) || '[]');
                return Array.isArray(parsed) ? parsed : [];
            } catch (_) {
                return [];
            }
        }

        function saveEdgetunnelAccounts(accounts) {
            localStorage.setItem(edgetunnelAccountStorageKey, JSON.stringify(accounts));
        }

        function renderEdgetunnelAccounts() {
            const select = document.getElementById('edgetunnelAccountSelect');
            const accounts = getEdgetunnelAccounts();
            const accountOptions = '<option value="">新账户/不使用保存配置</option>' + accounts.map((a, i) => `<option value="${i}">${escapeHTML(a.name || a.host || 'edgetunnel账户')}</option>`).join('');
            if (select) select.innerHTML = accountOptions;
            syncEdgetunnelAccountPick();
        }

        function syncEdgetunnelAccountPick() {
            const pick = document.getElementById('edgetunnelAccountPick');
            const select = document.getElementById('edgetunnelAccountSelect');
            if (!pick) return;
            const accounts = getEdgetunnelAccounts();
            pick.innerHTML = '<option value="">使用当前填写信息</option>' + accounts.map((a, i) => `<option value="${i}">${escapeHTML(a.name || a.host || 'edgetunnel账户')}</option>`).join('');
            pick.value = select ? select.value : '';
        }

        function pickEdgetunnelAccountFromModal() {
            const pick = document.getElementById('edgetunnelAccountPick');
            const select = document.getElementById('edgetunnelAccountSelect');
            if (!pick || !select) return;
            select.value = pick.value;
            loadSelectedEdgetunnelAccount();
        }

        function normalizeEdgetunnelAccountKey(account) {
            return account?.name || account?.host || '';
        }

        function loadSelectedEdgetunnelAccount() {
            const select = document.getElementById('edgetunnelAccountSelect');
            syncEdgetunnelAccountPick();
            const idx = parseInt(select.value, 10);
            if (Number.isNaN(idx)) {
                clearEdgetunnelAccountInputs();
                return;
            }
            const account = getEdgetunnelAccounts()[idx];
            if (!account) return;
            document.getElementById('edgetunnelAccountName').value = account.name || '';
            document.getElementById('edgetunnelHost').value = account.host || '';
            document.getElementById('edgetunnelMode').value = account.mode || 'overwrite';
            document.getElementById('edgetunnelSavePassword').checked = !!account.passwordCipher || !!account.password || edgetunnelSessionPasswords.has(normalizeEdgetunnelAccountKey(account));
            const accountKey = normalizeEdgetunnelAccountKey(account);
            const sessionPassword = edgetunnelSessionPasswords.get(accountKey) || '';
            document.getElementById('edgetunnelPassword').value = sessionPassword;
            if (!sessionPassword && account.password) {
                document.getElementById('edgetunnelPassword').value = account.password;
                edgetunnelSessionPasswords.set(accountKey, account.password);
            } else if (!sessionPassword && account.passwordCipher) {
                decryptStoredEdgetunnelPassword(account).then((password) => {
                    if (password) {
                        edgetunnelSessionPasswords.set(accountKey, password);
                        if (document.getElementById('edgetunnelAccountSelect').value === String(idx)) {
                            document.getElementById('edgetunnelPassword').value = password;
                        }
                    }
                });
            }
        }

        function clearEdgetunnelAccountInputs() {
            ['edgetunnelAccountName', 'edgetunnelHost', 'edgetunnelPassword'].forEach((id) => document.getElementById(id).value = '');
            document.getElementById('edgetunnelMode').value = 'overwrite';
            document.getElementById('edgetunnelSavePassword').checked = false;
        }

        function deleteSelectedEdgetunnelAccount() {
            const select = document.getElementById('edgetunnelAccountSelect');
            const idx = parseInt(select.value, 10);
            if (Number.isNaN(idx)) return;
            const accounts = getEdgetunnelAccounts();
            const deleted = accounts[idx];
            if (deleted) edgetunnelSessionPasswords.delete(normalizeEdgetunnelAccountKey(deleted));
            accounts.splice(idx, 1);
            saveEdgetunnelAccounts(accounts);
            renderEdgetunnelAccounts();
        }

        async function getEdgetunnelPasswordCryptoKey(accountName) {
            if (!crypto.subtle) return null;
            const material = await crypto.subtle.importKey('raw', new TextEncoder().encode(`${location.origin}|${accountName}|cfdata-edgetunnel-token-v1`), 'PBKDF2', false, ['deriveKey']);
            return crypto.subtle.deriveKey({ name: 'PBKDF2', salt: new TextEncoder().encode('cfdata-edgetunnel-token-v1'), iterations: 120000, hash: 'SHA-256' }, material, { name: 'AES-GCM', length: 256 }, false, ['encrypt', 'decrypt']);
        }

        async function encryptEdgetunnelPassword(accountName, password) {
            const key = await getEdgetunnelPasswordCryptoKey(accountName);
            if (!key) return '';
            const iv = crypto.getRandomValues(new Uint8Array(12));
            const cipher = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, new TextEncoder().encode(password));
            return bytesToBase64(iv) + ':' + bytesToBase64(new Uint8Array(cipher));
        }

        async function decryptStoredEdgetunnelPassword(account) {
            if (!account?.passwordCipher || !crypto.subtle) return '';
            try {
                const key = await getEdgetunnelPasswordCryptoKey(normalizeEdgetunnelAccountKey(account));
                if (!key) return '';
                const parts = account.passwordCipher.split(':');
                if (parts.length !== 2) return '';
                const iv = base64ToBytes(parts[0]);
                const data = base64ToBytes(parts[1]);
                const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, data);
                return new TextDecoder().decode(plain);
            } catch (_) {
                return '';
            }
        }

        async function saveCurrentEdgetunnelAccount() {
            const name = document.getElementById('edgetunnelAccountName').value.trim() || document.getElementById('edgetunnelHost').value.trim();
            if (!name) return;
            const host = document.getElementById('edgetunnelHost').value.trim();
            const password = document.getElementById('edgetunnelPassword').value.trim();
            const mode = document.getElementById('edgetunnelMode').value;
            const rememberPassword = document.getElementById('edgetunnelSavePassword').checked;
            const accounts = getEdgetunnelAccounts();
            const next = { name, host, mode };
            if (rememberPassword && password) {
                edgetunnelSessionPasswords.set(name, password);
                next.passwordCipher = await encryptEdgetunnelPassword(name, password);
                if (!next.passwordCipher) {
                    showNoticeCard('当前浏览器不支持 WebCrypto，无法安全保存密码；本次仍会上传，但不会持久保存密码');
                }
            } else {
                edgetunnelSessionPasswords.delete(name);
            }
            const existing = accounts.findIndex((a) => a.name === name);
            if (existing >= 0) accounts[existing] = next;
            else accounts.push(next);
            saveEdgetunnelAccounts(accounts);
            renderEdgetunnelAccounts();
        }

        function renderResultOrderList() {
            const byKey = Object.fromEntries(RESULT_FIELDS.concat(resultCustomFields).map((f) => [f.key, f]));
            document.getElementById('resultOrderList').innerHTML = resultFieldOrder.map((key, idx) => `<div class="order-item"><span>${byKey[key]?.label || key}</span><span><button onclick="moveResultField(${idx}, -1)">↑</button> <button onclick="moveResultField(${idx}, 1)">↓</button></span></div>`).join('');
            updateResultPreview();
        }

        function parseResultCustomFields(spec) {
            const seen = new Map();
            return String(spec || '').split(',').map((part) => {
                let item = part.trim();
                let key = '';
                const eq = item.indexOf('=');
                if (eq >= 0) {
                    key = item.slice(0, eq).trim().toLowerCase();
                    item = item.slice(eq + 1).trim();
                }
                const rest = item;
                const colon = rest.indexOf(':');
                if (colon < 0) return null;
                const label = rest.slice(0, colon).trim();
                const value = rest.slice(colon + 1).trim();
                if (!label || !value) return null;
                if (!key) key = label.toLowerCase();
                const baseKey = key;
                const used = seen.get(baseKey) || 0;
            if (used > 0) {
                let count = used;
                do {
                    key = `${baseKey}${count}`;
                    count++;
                } while (seen.has(key));
            }
            key = key.replace(/[\s,:#]/g, '_');
            seen.set(baseKey, used + 1);
                if (key !== baseKey) seen.set(key, (seen.get(key) || 0) + 1);
                return { key, label, value };
            }).filter(Boolean);
        }

        function handleCustomResultFieldsChange() {
            const next = parseResultCustomFields(document.getElementById('resultCustomFields')?.value || '');
            const nextKeys = new Set(next.map((f) => f.key));
            resultFieldOrder = resultFieldOrder.filter((key) => !resultCustomFields.some((f) => f.key === key) || nextKeys.has(key));
            resultCustomFields = next;
            for (const field of resultCustomFields) {
                if (!resultFieldOrder.includes(field.key)) resultFieldOrder.push(field.key);
            }
            renderResultFieldOptions();
        }

        function toggleResultField(key, checked) {
            if (checked && !resultFieldOrder.includes(key)) resultFieldOrder.push(key);
            if (!checked) resultFieldOrder = resultFieldOrder.filter((k) => k !== key);
            renderResultOrderList();
            syncR2AliasTemplate();
        }

        function moveResultField(index, delta) {
            const next = index + delta;
            if (next < 0 || next >= resultFieldOrder.length) return;
            [resultFieldOrder[index], resultFieldOrder[next]] = [resultFieldOrder[next], resultFieldOrder[index]];
            renderResultFieldOptions();
        }

        function selectResultFields(all) {
            resultFieldOrder = all ? getAllResultFieldOrder() : ['ipport'];
            renderResultFieldOptions();
        }

        function getAvailableResultFieldKeys() {
            return resultFieldChoiceKeys(RESULT_FIELDS, resultCustomFields);
        }

        function areAllResultFieldsSelected() {
            const available = getAvailableResultFieldKeys();
            return available.length > 0 && available.every((key) => resultFieldOrder.includes(key));
        }

        function updateResultFieldQuickButtons() {
            const btn = document.getElementById('resultSelectAllBtn');
            if (!btn) return;
            btn.textContent = areAllResultFieldsSelected() ? '全部选' : '全选';
            btn.title = areAllResultFieldsSelected() ? '恢复默认选择' : '选择全部可用字段';
        }

        function toggleAllResultFields() {
            if (areAllResultFieldsSelected()) {
                selectDefaultResultFields();
                return;
            }
            resultFieldOrder = getAllResultFieldOrder();
            renderResultFieldOptions();
        }

        function selectIPPortOnly() {
            resultFieldOrder = ['ipport'];
            renderResultFieldOptions();
        }

        function selectDefaultResultFields() {
            resultFieldOrder = getDefaultResultFieldOrder(document.getElementById('resultFormat').value);
            renderResultFieldOptions();
        }

        function openCopyModal(mode) {
            copyModalMode = currentMode === 'nsb' ? 'nsb' : mode;
            document.getElementById('copyModalTitle').textContent = copyModalMode === 'nsb' ? '非标结果一键复制' : (copyModalMode === 'test' ? '官方详细测试一键复制' : '官方扫描结果一键复制');
            const filterField = document.getElementById('copyIPTypeFilterField');
            if (filterField) filterField.style.display = copyModalMode === 'nsb' ? '' : 'none';
            const filterSelect = document.getElementById('copyIPTypeFilter');
            if (filterSelect && copyModalMode !== 'nsb') filterSelect.value = 'all';
            const qualifiedField = document.getElementById('copyQualifiedFilterField');
            if (qualifiedField) qualifiedField.style.display = '';
            const qualifiedSelect = document.getElementById('copyQualifiedFilter');
            if (qualifiedSelect) qualifiedSelect.value = 'all';
            const rows = getCopyRowsBeforeRange();
            if (!rows.length) return showNoticeCard('暂无可复制结果');
            if (!copyFieldOrder.length) copyFieldOrder = getDefaultResultFieldOrderFor(copyModalMode, 'txt');
            renderCopyFieldOptions();
            document.getElementById('copyModalMask').classList.add('active');
        }

        function closeCopyModal() {
            document.getElementById('copyModalMask').classList.remove('active');
        }

        function getCopyRowsBeforeRange() {
            const filter = document.getElementById('copyIPTypeFilter')?.value || 'all';
            const qualified = document.getElementById('copyQualifiedFilter')?.value || 'all';
            if (currentMode === 'nsb') {
                return getNSBResultObjects().filter((row) => matchesIPTypeFilter(row.ipType, filter) && matchesQualifiedFilter(row, qualified));
            }
            const rows = getCurrentResultRows(copyModalMode);
            return rows.filter((row) => matchesIPTypeFilter(getOfficialIPType(row), filter) && matchesOfficialQualifiedFilter(row, qualified));
        }

        function getSelectedCopyRows() {
            const rows = getCopyRowsBeforeRange();
            const start = Math.max(1, parseInt(document.getElementById('copyStartRow')?.value || '1', 10) || 1);
            const end = Math.max(start, parseInt(document.getElementById('copyEndRow')?.value || '20', 10) || 20);
            return rows.slice(start - 1, end).map((row) => {
                const next = { ...row };
                for (const field of copyCustomFields) next[field.key] = field.value;
                return next;
            });
        }

        function getAvailableCopyFieldKeys() {
            const rows = getCopyRowsBeforeRange();
            return RESULT_FIELDS.filter((f) => f.key === 'ipport' || rows.some((r) => String(r[f.key] ?? '').trim() !== '')).concat(copyCustomFields).map((f) => f.key);
        }

        function areAllCopyFieldsSelected() {
            const available = getAvailableCopyFieldKeys();
            return available.length > 0 && available.every((key) => copyFieldOrder.includes(key));
        }

        function updateCopyFieldQuickButtons() {
            const btn = document.getElementById('copySelectAllBtn');
            if (!btn) return;
            btn.textContent = areAllCopyFieldsSelected() ? '全部选' : '全选';
            btn.title = areAllCopyFieldsSelected() ? '恢复默认选择' : '选择全部可用字段';
        }

        function renderCopyFieldOptions() {
            if (!copyFieldOrder.length) copyFieldOrder = getDefaultResultFieldOrderFor(copyModalMode, 'txt');
            if (!copyFieldOrder.includes('ipport')) copyFieldOrder.unshift('ipport');
            const rows = getCopyRowsBeforeRange();
            const available = RESULT_FIELDS.filter((f) => f.key === 'ipport' || rows.some((r) => String(r[f.key] ?? '').trim() !== '')).concat(copyCustomFields);
            document.getElementById('copyFieldPicker').innerHTML = available.map((f) => `<label class="field-pill"><input type="checkbox" value="${f.key}" ${copyFieldOrder.includes(f.key) ? 'checked' : ''} onchange="toggleCopyField('${f.key}', this.checked)"> ${f.label}</label>`).join('');
            updateCopyFieldQuickButtons();
            renderCopyOrderList();
            updateCopyPreview();
        }

        function renderCopyOrderList() {
            const byKey = Object.fromEntries(RESULT_FIELDS.concat(copyCustomFields).map((f) => [f.key, f]));
            document.getElementById('copyOrderList').innerHTML = copyFieldOrder.map((key, idx) => `<div class="order-item"><span>${byKey[key]?.label || key}</span><span><button onclick="moveCopyField(${idx}, -1)">↑</button> <button onclick="moveCopyField(${idx}, 1)">↓</button></span></div>`).join('');
            updateCopyPreview();
        }

        function handleCustomCopyFieldsChange() {
            const next = parseResultCustomFields(document.getElementById('copyCustomFields')?.value || '');
            const nextKeys = new Set(next.map((f) => f.key));
            copyFieldOrder = copyFieldOrder.filter((key) => !copyCustomFields.some((f) => f.key === key) || nextKeys.has(key));
            copyCustomFields = next;
            for (const field of copyCustomFields) {
                if (!copyFieldOrder.includes(field.key)) copyFieldOrder.push(field.key);
            }
            renderCopyFieldOptions();
        }

        function toggleCopyField(key, checked) {
            if (checked && !copyFieldOrder.includes(key)) copyFieldOrder.push(key);
            if (!checked) copyFieldOrder = copyFieldOrder.filter((k) => k !== key);
            renderCopyOrderList();
        }

        function moveCopyField(index, delta) {
            const next = index + delta;
            if (next < 0 || next >= copyFieldOrder.length) return;
            [copyFieldOrder[index], copyFieldOrder[next]] = [copyFieldOrder[next], copyFieldOrder[index]];
            renderCopyFieldOptions();
        }

        function toggleAllCopyFields() {
            if (areAllCopyFieldsSelected()) {
                selectDefaultCopyFields();
                return;
            }
            copyFieldOrder = getAvailableCopyFieldKeys();
            renderCopyFieldOptions();
        }

        function selectCopyIPPortOnly() {
            copyFieldOrder = ['ipport'];
            renderCopyFieldOptions();
        }

        function selectDefaultCopyFields() {
            copyFieldOrder = getDefaultResultFieldOrderFor(copyModalMode, 'txt');
            renderCopyFieldOptions();
        }

        function buildCopyContent() {
            const rows = getSelectedCopyRows();
            if (!rows.length) throw new Error('暂无可复制结果');
            const fields = copyFieldOrder.length ? copyFieldOrder : getDefaultResultFieldOrderFor(copyModalMode, 'txt');
            return buildResultContent('txt', rows, fields, getTextSeparator('copy'), useIPv6BracketsFor('copy'));
        }

        function buildCopyPreview() {
            const rows = getSelectedCopyRows();
            if (!rows.length) return '暂无预览';
            const fields = copyFieldOrder.length ? copyFieldOrder : getDefaultResultFieldOrderFor(copyModalMode, 'txt');
            const first = rows[0];
            const ipport = first.ip ? formatIPPort(first.ip, first.port, useIPv6BracketsFor('copy')) : (first.ipport || `${first.ip || ''}:${first.port || ''}`);
            const extras = fields.filter((key) => key !== 'ipport').map((key) => first[key]).filter((value) => String(value ?? '').trim() !== '');
            const separator = getTextSeparator('copy');
            return extras.length ? `${ipport}#${extras.join(separator)}` : ipport;
        }

        function updateCopyPreview() {
            const el = document.getElementById('copyPreview');
            if (!el) return;
            try {
                el.textContent = buildCopyPreview();
            } catch (err) {
                el.textContent = err.message || '预览失败';
            }
        }

        function copyConfiguredResults(event) {
            let content;
            try {
                content = buildCopyContent();
            } catch (err) {
                showNoticeCard(err.message || '暂无可复制结果');
                return;
            }
            copyToClipboard(content, event);
        }

        function copySubstoreResults(event) {
            let content;
            try {
                content = buildSubstoreContent(getSelectedCopyRows());
            } catch (err) {
                showNoticeCard(err.message || '暂无可复制结果');
                return;
            }
            copyToClipboard(content, event);
        }

        function getCurrentResultRows(mode) {
            if (currentMode === 'nsb') return getNSBResultObjects();
            if (mode === 'test') return testData.map(testResultToObject);
            return scanData.map(scanResultToObject).slice().sort(compareOfficialResultObjects);
        }

        function getCurrentExportRows(mode) {
            const rows = getCurrentResultRows(mode);
            const filter = document.getElementById('resultIPTypeFilter')?.value || 'all';
            const qualified = document.getElementById('resultQualifiedFilter')?.value || 'all';
            if (currentMode === 'nsb') {
                return rows.filter((row) => matchesIPTypeFilter(row.ipType, filter) && matchesQualifiedFilter(row, qualified) && exportLatencyLossMatch(row));
            }
            return rows.filter((row) => matchesIPTypeFilter(getOfficialIPType(row), filter) && matchesOfficialQualifiedFilter(row, qualified) && exportLatencyLossMatch(row));
        }

        function exportLatencyLossMatch(row) {
            let latencyFilter = parseFloat(document.getElementById('resultLatencyFilter')?.value);
            if (!Number.isFinite(latencyFilter) || latencyFilter < 0) latencyFilter = 0;
            let lossFilter = parseFloat(document.getElementById('resultLossFilter')?.value);
            if (!Number.isFinite(lossFilter)) lossFilter = 100;
            lossFilter = Math.min(100, Math.max(0, lossFilter));
            const latencyMs = latencyMsForFilter(row.latency);
            const lossPct = parseFilterNumber(row.lossRate);
            if (latencyFilter > 0 && !(Number.isFinite(latencyMs) && latencyMs < latencyFilter)) return false;
            if (lossFilter < 100 && !(Number.isFinite(lossPct) && lossPct < lossFilter)) return false;
            return true;
        }

        function getDisplayedResultRows(mode) {
            if (currentMode === 'nsb') {
                return getNSBResultObjects().filter((row) => matchesIPTypeFilter(row.ipType, nsbTableIPTypeFilter) && matchesQualifiedFilter(row, nsbTableQualifiedFilter) && matchesLatencyLossFilter(latencyMsForFilter(row.latency), parseFilterNumber(row.lossRate)));
            }
            if (mode === 'test') {
                return getFilteredOfficialTestData().map(testResultToObject);
            }
            return getFilteredOfficialScanData().map(scanResultToObject);
        }

        function scanResultToObject(r) {
            return { ip: r.IP || '', port: String(r.Port ?? ''), ipport: `${r.IP || ''}:${r.Port ?? ''}`, dc: r.DataCenter || '', dcCountry: r.DCCountry || '', region: r.Region || '', city: r.City || '', latency: r.LatencyStr || '', lossRate: r.LossRate != null ? `${(r.LossRate * 100).toFixed(0)}%` : '' };
        }

        function testResultToObject(r) {
            const port = r.Port || parseInt(document.getElementById('officialTestPort').value) || 443;
            const scan = scanData.find((item) => item.IP === r.IP) || {};
            return {
                ip: r.IP || '',
                port: String(port),
                ipport: `${r.IP || ''}:${port}`,
                dc: scan.DataCenter || r.DataCenter || '',
                dcCountry: scan.DCCountry || r.DCCountry || '',
                region: scan.Region || r.Region || '',
                city: scan.City || r.City || '',
                latency: `${(r.AvgLatency / 1_000_000).toFixed(0)}ms`,
                speed: r.Speed || '未测速',
                lossRate: r.LossRate != null ? `${(r.LossRate * 100).toFixed(0)}%` : '',
            };
        }

        function getNSBResultObjects() {
            const sourceHeaders = nsbHeaders.length ? nsbHeaders : NSB_LIVE_HEADERS;
            const sourceRows = nsbRows.length ? nsbRows : nsbLiveRows.map(nsbRowToArray);
            const normalizedHeaders = sourceHeaders.map(normalizeNSBHeader);
            const ipIndex = normalizedHeaders.indexOf('IP地址');
            const portIndex = normalizedHeaders.indexOf('端口号');
            return sourceRows.map((row) => {
                const obj = {};
                normalizedHeaders.forEach((header, idx) => {
                    const field = RESULT_FIELDS.find((f) => f.label === header);
                    if (field) obj[field.key] = row[idx] ?? '';
                });
                obj.ip = obj.ip || (ipIndex >= 0 ? row[ipIndex] : '') || '';
                obj.port = obj.port || (portIndex >= 0 ? row[portIndex] : '') || '';
                obj.ipport = `${obj.ip}:${obj.port}`;
                return obj;
            });
        }

        function buildSelectedResultContent(format) {
            const rows = getSelectedResultRows();
            if (!rows.length) throw new Error('暂无可操作结果');
            const fields = resultFieldOrder.length ? resultFieldOrder : getDefaultResultFieldOrder(format);
            return buildResultContent(format, rows, fields, getTextSeparator('result'), useIPv6BracketsFor('result'));
        }

        function applyTextSeparatorPreset(prefix, value) {
            const input = document.getElementById(`${prefix}TextSeparatorCustom`);
            if (input && value === 'custom') {
                input.value = '';
                input.focus();
            } else if (input) {
                input.value = value === 'space' ? '空格' : value;
            }
            updateTextSeparatorPreview(prefix);
        }

        function markTextSeparatorCustom(prefix) {
            const preset = document.getElementById(`${prefix}TextSeparatorPreset`);
            if (preset && preset.value !== 'custom') preset.value = 'custom';
            updateTextSeparatorPreview(prefix);
        }

        function updateTextSeparatorPreview(prefix) {
            if (prefix === 'copy') updateCopyPreview();
            else updateResultPreview();
        }

        function getTextSeparator(prefix) {
            const raw = document.getElementById(`${prefix}TextSeparatorCustom`)?.value ?? '-';
            if (raw === '空格') return ' ';
            return raw;
        }

        function buildSubstoreContent(rows) {
            if (!rows.length) throw new Error('暂无可操作结果');
            const lines = rows.map((row) => {
                const endpoint = buildSubstoreEndpoint(row);
                return endpoint ? `${endpoint}#${buildSubstoreAlias(row)}` : '';
            }).filter(Boolean);
            if (!lines.length) throw new Error('没有可导出的目标地址');
            return lines.join('\n') + '\n';
        }

        function buildSubstoreEndpoint(row) {
            let host = String(row.ip ?? '').trim();
            let port = String(row.port ?? '').trim();
            if (!host) {
                const parsed = splitSubstoreIPPort(row.ipport);
                host = parsed.host;
                port = parsed.port;
            }
            return formatSubstoreEndpoint(host, port);
        }

        function splitSubstoreIPPort(value) {
            const endpoint = String(value ?? '').trim();
            if (!endpoint) return { host: '', port: '' };
            if (endpoint.startsWith('[')) {
                if (endpoint.endsWith(']')) return { host: endpoint.slice(1, -1), port: '' };
                const match = endpoint.match(/^\[([^\]]+)\]:(\d+)$/);
                if (!match || !isValidSubstorePort(match[2])) return { host: '', port: '' };
                return { host: match[1], port: match[2] };
            }
            const parts = endpoint.split(':');
            if (parts.length === 1) return { host: endpoint, port: '' };
            if (parts.length > 2) {
                // 未加括号的 IPv6 只接受含 :: 或 8 组完整写法，避免把 host:port:port 当成地址
                const likelyIPv6 = /^[0-9a-fA-F:.]+$/.test(endpoint) && (endpoint.includes('::') || parts.length === 8);
                return likelyIPv6 ? { host: endpoint, port: '' } : { host: '', port: '' };
            }
            if (!parts[0] || !isValidSubstorePort(parts[1])) return { host: '', port: '' };
            return { host: parts[0], port: parts[1] };
        }

        function isValidSubstorePort(port) {
            return /^\d+$/.test(port) && Number(port) >= 1 && Number(port) <= 65535;
        }

        function formatSubstoreEndpoint(host, port) {
            let safeHost = String(host ?? '').trim();
            if (!safeHost) return '';
            if (safeHost.startsWith('[') && safeHost.endsWith(']')) safeHost = safeHost.slice(1, -1);
            const safePort = String(port ?? '').trim();
            const formattedHost = safeHost.includes(':') ? `[${safeHost}]` : safeHost;
            return safePort && safePort !== '0' ? `${formattedHost}:${safePort}` : formattedHost;
        }

        function sanitizeSubstoreAliasPart(value) {
            return String(value ?? '').replaceAll('#', '').split(/\s+/).filter(Boolean).join(' ');
        }

        function buildSubstoreAlias(row) {
            const dc = sanitizeSubstoreAliasPart(row.dc);
            const city = sanitizeSubstoreAliasPart(row.city);
            if (dc && city) return `${dc}-${city}`;
            if (dc) return dc;
            if (city) return city;
            const loc = sanitizeSubstoreAliasPart(row.loc);
            if (loc) return loc;
            const ip = sanitizeSubstoreAliasPart(row.ip);
            if (ip) return ip;
            return sanitizeSubstoreAliasPart(row.ipport);
        }

        function buildResultContent(format, rows, fields, textSeparator = '-', useBrackets = true) {
            if (!rows.length) throw new Error('暂无可操作结果');
            if (format === 'substore') return buildSubstoreContent(rows);
            if (format === 'txt') {
                return rows.map((r) => {
                    const ipport = r.ip ? formatIPPort(r.ip, r.port, useBrackets) : (r.ipport || `${r.ip || ''}:${r.port || ''}`);
                    const extras = fields.filter((k) => k !== 'ipport').map((k) => r[k]).filter((v) => String(v ?? '').trim() !== '');
                    return extras.length ? `${ipport}#${extras.join(textSeparator)}` : ipport;
                }).join('\n') + '\n';
            }
            const byKey = Object.fromEntries(RESULT_FIELDS.concat(resultCustomFields).map((f) => [f.key, f]));
            const headers = fields.map((k) => byKey[k]?.label || k);
            const body = rows.map((r) => fields.map((k) => formatCSVCell(r[k])).join(',')).join('\n');
            return headers.join(',') + '\n' + body + '\n';
        }

        function getSelectedResultRows() {
            const rows = getCurrentExportRows(resultModalMode);
            const start = Math.max(1, parseInt(document.getElementById('resultStartRow')?.value || '1', 10) || 1);
            const end = Math.max(start, parseInt(document.getElementById('resultEndRow')?.value || '20', 10) || 20);
            return rows.slice(start - 1, end).map((row) => {
                const next = { ...row };
                for (const field of resultCustomFields) next[field.key] = field.value;
                return next;
            });
        }

        function formatCSVCell(value) {
            const text = String(value ?? '');
            if (/[",\n\r]/.test(text)) return `"${text.replaceAll('"', '""')}"`;
            return text;
        }

        function isIPv6Address(ip) {
            return typeof ip === 'string' && ip.indexOf(':') !== -1;
        }

        function formatIPPort(ip, port, useBrackets) {
            const safeIP = String(ip || '');
            if (!isIPv6Address(safeIP)) return `${safeIP}:${port ?? ''}`;
            return useBrackets ? `[${safeIP}]:${port ?? ''}` : `${safeIP}:${port ?? ''}`;
        }

        function useIPv6BracketsFor(prefix) {
            const el = document.getElementById(`${prefix}IPv6Brackets`);
            return el ? el.checked : true;
        }

        // 预览最多展示的行数；无结果时用内置示例展示格式效果（使用文档保留地址，避免与真实结果混淆）
        const RESULT_PREVIEW_MAX_ROWS = 5;
        const RESULT_PREVIEW_SAMPLE_ROWS = [
            { ip: '192.0.2.10', port: '443', ipport: '192.0.2.10:443', dc: 'HKG', dcCountry: 'HK', region: '香港', city: '香港', latency: '12ms', lossRate: '0%', speed: '25.30MB/s', ipType: 'IPv4', tls: 'on' },
            { ip: '192.0.2.20', port: '443', ipport: '192.0.2.20:443', dc: 'NRT', dcCountry: 'JP', region: '日本', city: '东京', latency: '35ms', lossRate: '0%', speed: '18.40MB/s', ipType: 'IPv4', tls: 'on' },
            { ip: '192.0.2.30', port: '443', ipport: '192.0.2.30:443', dc: 'SJC', dcCountry: 'US', region: '美国', city: '圣何塞', latency: '128ms', lossRate: '1%', speed: '6.20MB/s', ipType: 'IPv4', tls: 'on' },
        ];
        let resultPreviewShowingSample = false;

        function buildResultPreview(format) {
            let rows = getSelectedResultRows();
            resultPreviewShowingSample = false;
            if (!rows.length) {
                // 没有真实结果时展示示例，让用户仍能看到字段顺序与分隔符效果
                rows = RESULT_PREVIEW_SAMPLE_ROWS;
                resultPreviewShowingSample = true;
            }
            rows = takePreviewRows(rows, RESULT_PREVIEW_MAX_ROWS);
            if (!rows.length) return '暂无预览';
            const fields = resultFieldOrder.length ? resultFieldOrder : getDefaultResultFieldOrder(format);
            if (format === 'substore') return buildSubstoreContent(rows).trimEnd();
            if (format === 'txt') {
                return rows.map((row) => {
                    const ipport = row.ip ? formatIPPort(row.ip, row.port, useIPv6BracketsFor('result')) : (row.ipport || `${row.ip || ''}:${row.port || ''}`);
                    const extras = fields.filter((k) => k !== 'ipport').map((k) => row[k]).filter((v) => String(v ?? '').trim() !== '');
                    return extras.length ? `${ipport}#${extras.join(getTextSeparator('result'))}` : ipport;
                }).join('\n');
            }
            const byKey = Object.fromEntries(RESULT_FIELDS.concat(resultCustomFields).map((f) => [f.key, f]));
            const headers = fields.map((k) => byKey[k]?.label || k).join(',');
            const lines = rows.map((row) => fields.map((k) => formatCSVCell(row[k])).join(','));
            return [headers, ...lines].join('\n');
        }

        function updateResultPreview() {
            const el = document.getElementById('resultPreview');
            if (!el) return;
            try {
                el.textContent = buildResultPreview(document.getElementById('resultFormat').value);
            } catch (err) {
                el.textContent = err.message || '预览失败';
                resultPreviewShowingSample = false;
            }
            const badge = document.getElementById('resultPreviewBadge');
            if (badge) badge.style.display = resultPreviewShowingSample ? '' : 'none';
            // R2 别名随「字段与顺序」与分隔符即时同步（预览刷新是所有字段/顺序/分隔符变更的公共路径）
            syncR2AliasTemplate();
            saveResultPanelSettings();
        }

        // ---------------- 结果操作面板显示设置持久化 ----------------
        // 面板显示设置（来源/格式/筛选/字段/顺序/分隔符等）保存在独立 localStorage 键；
        // 不写入 cfdata-config（任务参数）。恢复必须晚于 switchMode 的初始化渲染：
        // 初始化期间的自动回写被 resultPanelLoaded 拦截，避免默认值覆盖已存设置；
        // 恢复时不走 handleResultFormatChange/applyResultModeFields 的重置语义，
        // 只借用它们的显隐逻辑后再回填已存筛选值。

        const RESULT_PANEL_STORAGE_KEY = 'cfdata-result-panel';
        let resultPanelLoaded = false;
        // 服务器持久化：localStorage 只是本地缓存，服务器设置是跨访问地址/浏览器的权威来源。
        // 应用服务器值期间抑制回写，避免把刚读到的设置又推回服务器。
        let resultPanelApplying = false;
        let resultPanelSyncTimer = null;
        let resultPanelManualSavePending = false;
        // 结果操作页保存同时提交 R2 配置时，抑制调度页面那条独立提示，只显示合并后的「结果操作配置已保存」
        let suppressScheduleSavedNotice = false;
        let resultPanelSyncOfflineNotified = false;

        function collectResultPanelSettings() {
            const val = (id) => document.getElementById(id)?.value ?? '';
            const chk = (id) => !!document.getElementById(id)?.checked;
            return {
                source: val('resultSourceSelect'),
                format: val('resultFormat'),
                fileName: val('resultFileName'),
                startRow: val('resultStartRow'),
                endRow: val('resultEndRow'),
                ipType: val('resultIPTypeFilter'),
                qualified: val('resultQualifiedFilter'),
                latency: val('resultLatencyFilter'),
                loss: val('resultLossFilter'),
                fieldOrder: (resultFieldOrder || []).slice(),
                customFields: val('resultCustomFields'),
                separatorPreset: val('resultTextSeparatorPreset'),
                separatorCustom: val('resultTextSeparatorCustom'),
                ipv6Brackets: chk('resultIPv6Brackets'),
            };
        }

        function saveResultPanelSettings() {
            if (!resultPanelLoaded) return false;
            let localOK = true;
            try {
                localStorage.setItem(RESULT_PANEL_STORAGE_KEY, JSON.stringify(collectResultPanelSettings()));
            } catch (e) {
                localOK = false; // 本地缓存不可用时仍尝试同步服务器
            }
            if (!resultPanelApplying) scheduleResultPanelServerSync();
            return localOK;
        }

        // 改动后防抖同步服务器；显式保存走 flush 立即发送
        function scheduleResultPanelServerSync(delay = 500) {
            if (!resultPanelLoaded || resultPanelApplying) return;
            if (resultPanelSyncTimer) clearTimeout(resultPanelSyncTimer);
            resultPanelSyncTimer = setTimeout(() => {
                resultPanelSyncTimer = null;
                flushResultPanelServerSync(false);
            }, delay);
        }

        function flushResultPanelServerSync(manual) {
            if (resultPanelSyncTimer) {
                clearTimeout(resultPanelSyncTimer);
                resultPanelSyncTimer = null;
            }
            if (!resultPanelLoaded) return false;
            const sent = wsSend({ type: 'save_result_panel_config', data: collectResultPanelSettings() });
            if (!sent) {
                if (manual) {
                    showNoticeCard('与服务端的连接尚未就绪，结果面板设置暂存在本机浏览器');
                } else if (!resultPanelSyncOfflineNotified) {
                    // 自动同步失败只提示一次，避免连续改动反复弹窗
                    resultPanelSyncOfflineNotified = true;
                    showNoticeCard('与服务端的连接尚未就绪，结果面板设置暂存在本机浏览器；连接恢复后可再次保存');
                }
                return false;
            }
            resultPanelSyncOfflineNotified = false;
            if (manual) resultPanelManualSavePending = true;
            return true;
        }

        function handleResultPanelSaved(data) {
            const saved = !!(data && data.saved);
            if (saved) {
                if (resultPanelManualSavePending) {
                    resultPanelManualSavePending = false;
                    showNoticeCard('结果操作配置已保存');
                }
                return;
            }
            resultPanelManualSavePending = false;
            showNoticeCard((data && data.error) || '结果面板设置保存失败');
        }

        // 服务器尚无记录时，把浏览器里已有的旧设置上送，完成一次性迁移
        function migrateLocalResultPanelSettings() {
            if (resultPanelApplying || !resultPanelLoaded) return;
            flushResultPanelServerSync(false);
        }

        function readLocalResultPanelSettings() {
            let raw = null;
            try {
                raw = localStorage.getItem(RESULT_PANEL_STORAGE_KEY);
            } catch (e) {
                raw = null;
            }
            if (!raw) return null;
            try {
                const parsed = JSON.parse(raw);
                return parsed && typeof parsed === 'object' ? parsed : null;
            } catch (e) {
                return null; // 损坏 JSON：忽略并沿用默认值
            }
        }

        function restoreResultPanelSettings() {
            const s = readLocalResultPanelSettings();
            // 无论是否有已存记录，读取之后才允许回写，保证初始化渲染不覆盖存储
            resultPanelLoaded = true;
            if (!s) return;
            applyResultPanelSettings(s);
        }

        // 服务器设置是权威来源：应用后回写本地缓存（应用期间抑制回推服务器）
        function applyServerResultPanelSettings(settings) {
            if (!settings || typeof settings !== 'object') return false;
            if (!resultPanelLoaded) resultPanelLoaded = true;
            applyResultPanelSettings(settings);
            try {
                localStorage.setItem(RESULT_PANEL_STORAGE_KEY, JSON.stringify(collectResultPanelSettings()));
            } catch (e) { /* localStorage 不可用时仅保留服务器值 */ }
            return true;
        }

        function applyResultPanelSettings(s) {
            if (!s || typeof s !== 'object') return false;
            resultPanelApplying = true;
            try {
                applyResultPanelSettingsFields(s);
            } finally {
                resultPanelApplying = false;
            }
            return true;
        }

        function applyResultPanelSettingsFields(s) {
            const setVal = (id, v) => {
                const el = document.getElementById(id);
                if (el && typeof v === 'string' && v !== '') el.value = v;
            };
            const setChk = (id, v) => {
                const el = document.getElementById(id);
                if (el && typeof v === 'boolean') el.checked = v;
            };
            // 数据来源：仅在当前模式下可见的选项才恢复，避免隐藏选项导致模式错乱
            if (typeof s.source === 'string' && s.source) {
                const sourceEl = document.getElementById('resultSourceSelect');
                const opt = sourceEl ? Array.from(sourceEl.options).find((o) => o.value === s.source) : null;
                if (sourceEl && opt && !opt.hidden) {
                    sourceEl.value = s.source;
                    resultModalMode = s.source;
                }
            }
            setVal('resultFormat', s.format);
            // 先按恢复后的模式同步显隐（会把筛选重置为默认），再回填已存值
            applyResultModeFields();
            setVal('resultIPTypeFilter', s.ipType);
            setVal('resultQualifiedFilter', s.qualified);
            setVal('resultFileName', s.fileName);
            setVal('resultStartRow', s.startRow);
            setVal('resultEndRow', s.endRow);
            setVal('resultLatencyFilter', s.latency);
            setVal('resultLossFilter', s.loss);
            setVal('resultTextSeparatorPreset', s.separatorPreset);
            setVal('resultTextSeparatorCustom', s.separatorCustom);
            setChk('resultIPv6Brackets', s.ipv6Brackets);
            setVal('resultCustomFields', s.customFields);
            handleCustomResultFieldsChange();
            if (Array.isArray(s.fieldOrder)) {
                const known = new Set(RESULT_FIELDS.concat(resultCustomFields).map((f) => f.key));
                const restored = s.fieldOrder.filter((k) => typeof k === 'string' && known.has(k));
                if (restored.length) resultFieldOrder = restored;
            }
            renderResultFieldOptions();
        }

        // 按「字段与顺序」生成别名模板串：IP:PORT 已在每行地址中，别名只取其余字段；
        // 没有其他字段时生成 {none}，后端据此只输出地址。
        function buildR2AliasTemplate() {
            return buildR2AliasTemplateFromOrder(resultFieldOrder, getTextSeparator('result'));
        }

        // 把生成的模板串写入隐藏的 r2AliasFormat（保存/加载链路按 id 收集，协议不变），并刷新只读示例
        function syncR2AliasTemplate() {
            const template = buildR2AliasTemplate();
            const hidden = document.getElementById('r2AliasFormat');
            if (hidden) hidden.value = template;
            const preview = document.getElementById('r2AliasPreview');
            if (preview) {
                preview.value = template === R2_ALIAS_NONE_TEMPLATE
                    ? '（仅输出 IP:PORT，不加别名）'
                    : (template || '');
            }
        }

        function setGitHubUploadStatus(text, type = '') {
            const el = document.getElementById('githubUploadStatus');
            if (!el) return;
            el.textContent = text;
            el.classList.remove('error', 'success');
            if (type) el.classList.add(type);
        }

        function sendGitHubUpload(payload) {
            if (!payload.silent) setGitHubUploadStatus(`第 1/3 次上传中: ${payload.path}`, 'success');
            wsSend({ type: 'github_upload', data: payload });
        }

        function setEdgetunnelUploadStatus(text, type = '') {
            const el = document.getElementById('edgetunnelUploadStatus');
            if (!el) return;
            el.textContent = text;
            el.classList.remove('error', 'success');
            if (type) el.classList.add(type);
        }

        function setR2UploadStatus(text, type = '') {
            const el = document.getElementById('r2UploadStatus');
            if (!el) return;
            el.textContent = text;
            el.classList.remove('error', 'success');
            if (type) el.classList.add(type);
        }

        function uploadResultsToR2() {
            const enabled = document.getElementById('r2Enabled')?.checked;
            const workerUrl = (document.getElementById('r2WorkerUrl')?.value || '').trim();
            const tokenInput = document.getElementById('r2WorkerToken');
            const token = (tokenInput?.value || '').trim();
            const tokenSaved = !!tokenInput?.dataset.tokenSet;
            if (!enabled) {
                setR2UploadStatus('R2 上传未启用，请先在「R2 上传」分组开启并保存', 'error');
                return;
            }
            if (!workerUrl) {
                setR2UploadStatus('R2 上传未配置 Worker 地址，请填写并保存后重试', 'error');
                return;
            }
            if (!token && !tokenSaved) {
                setR2UploadStatus('R2 上传未配置 Worker token，请填写并保存后重试', 'error');
                return;
            }
            const format = document.getElementById('resultFormat').value;
            let content;
            try {
                content = buildSelectedResultContent(format);
            } catch (err) {
                setR2UploadStatus(err.message || '暂无可上传的结果', 'error');
                return;
            }
            if (!wsSend({ type: 'r2_upload', data: { content } })) {
                setR2UploadStatus('与服务端的连接尚未就绪，请稍后再试', 'error');
                return;
            }
            setR2UploadStatus('R2 上传中…', 'success');
            log('已提交 R2 上传请求');
        }

        function confirmEdgetunnelUpload(mode) {
            const msg = '使用 edgetunnel 上传前，请确认你的 edgetunnel 已将优选订阅模式设置为「自定义订阅」（支持汇聚订阅），否则 edgetunnel 将无法下发正确内容。\n\n另外，不建议上传过多的优选地址，节点并不是越多越好，建议精选延迟低、稳定性好的节点上传。';
            showConfirmCard(msg, function() {
                uploadSelectedResultsToEdgetunnel(mode);
            }, 'edgetunnel 上传提示');
        }

        async function uploadSelectedResultsToEdgetunnel(mode) {
            const el = document.getElementById('edgetunnelUploadStatus');
            el.textContent = '准备上传...';
            el.className = 'upload-status';
            const host = document.getElementById('edgetunnelHost').value.trim();
            const password = document.getElementById('edgetunnelPassword').value.trim();
            if (!host) return showNoticeCard('Host 不能为空');
            if (!password) return showNoticeCard('密码不能为空');
            await saveCurrentEdgetunnelAccount();
            const format = document.getElementById('resultFormat').value;
            let content;
            try {
                const quickFile = await readQuickUploadFile();
                content = quickFile ? quickFile.content : buildSelectedResultContent(format);
            } catch (err) {
                showNoticeCard(err.message || '暂无可操作结果');
                return;
            }
            setEdgetunnelUploadStatus('正在上传...', 'success');
            wsSend({ type: 'edgetunnel_upload', data: { host, password, content, mode } });
            log(`已提交 edgetunnel ${mode === 'append' ? '追加' : '覆盖'}上传请求`);
        }

        function renderRawURLCopy(rawURL) {
            if (!rawURL) return;
            const el = document.getElementById('githubUploadStatus');
            if (!el) return;
            const safeURL = escapeHTML(rawURL);
            const jsURL = escapeJSString(rawURL);
            el.innerHTML = `上传完成：<span class="copy-ip" onclick="copyToClipboard('${jsURL}', event)" title="点击复制 Raw 地址">${safeURL}</span>`;
        }

        function readQuickUploadFile() {
            const input = document.getElementById('githubQuickFile');
            const file = input && input.files ? input.files[0] : null;
            if (!file) return Promise.resolve(null);
            return new Promise((resolve, reject) => {
                const reader = new FileReader();
                reader.onload = () => resolve({ name: file.name, content: String(reader.result || '') });
                reader.onerror = () => reject(new Error('读取快速上传文件失败'));
                reader.readAsText(file);
            });
        }

        function downloadSelectedResults() {
            const format = document.getElementById('resultFormat').value;
            let content;
            try {
                content = buildSelectedResultContent(format);
            } catch (err) {
                showNoticeCard(err.message || '暂无可操作结果');
                return;
            }
            const name = document.getElementById('resultFileName').value.trim() || 'cfdata-results';
            downloadFile(content, name, format === 'substore' ? 'txt' : format);
        }

        async function uploadSelectedResultsToGitHub() {
            const format = document.getElementById('resultFormat').value;
            const quickFile = await readQuickUploadFile();
            let content;
            try {
                content = quickFile ? quickFile.content : buildSelectedResultContent(format);
            } catch (err) {
                showNoticeCard(err.message || '暂无可操作结果');
                return;
            }
            const repoParts = document.getElementById('githubRepo').value.trim().split('/');
            if (repoParts.length !== 2 || !repoParts[0] || !repoParts[1]) return showNoticeCard('仓库格式应为 owner/repo');
            const payload = {
                token: document.getElementById('githubToken').value.trim(),
                owner: repoParts[0], repo: repoParts[1],
                branch: document.getElementById('githubBranch').value.trim() || 'main',
                path: document.getElementById('githubPath').value.trim() || (quickFile ? `results/${quickFile.name}` : `results/ip.${format === 'substore' ? 'txt' : format}`),
                message: document.getElementById('githubMessage').value.trim() || 'update cfdata results',
                content,
            };
            if (!payload.token) return showNoticeCard('Token 不能为空');
            await saveCurrentGitHubAccount(payload);
            sendGitHubUpload(payload);
            log('已提交 GitHub 上传请求');
        }

        async function autoUploadNSBResults() {
            const value = document.getElementById('nsbAutoUploadAccount')?.value || '';
            await autoUploadResults(value, 'nsb', '自动上传已提交');
        }

        async function autoUploadOfficialResults() {
            const value = document.getElementById('officialAutoUploadAccount')?.value || '';
            await autoUploadResults(value, 'test', '官方自动上传已提交');
        }

        async function autoUploadResults(value, mode, logPrefix) {
            if (!value || value === 'r2') return;
            const [idxText, format] = value.split(':');
            const idx = parseInt(idxText, 10);
            const account = getGitHubAccounts()[idx];
            const accountKey = normalizeGitHubAccountKey(account);
            let token = githubSessionTokens.get(accountKey) || '';
            if (!token && account?.token) {
                token = await migrateLegacyGitHubToken(account, idx);
            } else if (!token && account?.tokenCipher) {
                token = await decryptStoredGitHubToken(account);
                if (token) githubSessionTokens.set(accountKey, token);
            }
            if (!account || !token || !account.repo) {
                log('自动上传失败: 已保存账户缺少 token 或仓库配置');
                return;
            }
            const fields = getDefaultResultFieldOrderFor(mode, format || 'txt');
            let rows = getCurrentResultRows(mode);
            if (!rows.length) {
                log('自动上传跳过: 没有可上传结果');
                return;
            }
            const content = buildResultContent(format || 'txt', rows, fields);
            const repoParts = account.repo.split('/');
            const path = (account.path || `results/ip.${format || 'txt'}`).replace(/\.(csv|txt)$/i, `.${format || 'txt'}`);
            const uploadKey = `${mode}:${idx}:${format || 'txt'}:${path}`;
            if (autoUploadKeys.has(uploadKey)) {
                log(`自动上传已跳过重复提交: ${path}`);
                return;
            }
            autoUploadKeys.add(uploadKey);
            const payload = {
                token,
                owner: repoParts[0], repo: repoParts[1],
                branch: account.branch || 'main',
                path,
                message: account.message || 'update cfdata results',
                content,
                silent: true,
            };
            sendGitHubUpload(payload);
            log(`${logPrefix}: ${account.name || account.repo} ${format || 'txt'}`);
        }

        function downloadFile(content, nameBase, ext) {
            const timestamp = new Date().toISOString().replace(/[-:T]/g, '').split('.')[0];
            const mime = ext === 'csv' ? 'text/csv;charset=utf-8;' : 'text/plain;charset=utf-8;';
            const blob = new Blob(["\uFEFF" + content], { type: mime });
            const link = document.createElement('a');
            link.href = URL.createObjectURL(blob);
            link.download = `${nameBase}_${timestamp}.${ext}`;
            link.click();
        }

        function handleMessage(msg) {
            // 任务恢复载荷：元信息 → 分批结果数据 → 完成消息，按序处理
            if (msg.type === 'task_restored') {
                applyTaskRestoredState(msg.data || {});
                return;
            }
            if (msg.type === 'task_restore_chunk') {
                applyTaskRestoreChunk(msg.data || {});
                return;
            }
            if (msg.type === 'task_restore_done') {
                applyTaskRestoreDone(msg.data || {});
                return;
            }
            // 带 seq 的任务消息：恢复期间先缓冲；恢复完成后丢弃已包含在恢复载荷里的旧消息
            if (typeof msg.seq === 'number' && msg.seq > 0) {
                if (taskRestorePending) {
                    taskRestoreBuffer.push(msg);
                    return;
                }
                if (taskRestoreLastSeq && msg.seq <= taskRestoreLastSeq) return;
            }
            switch (msg.type) {
                case 'init_config':
                    defaultSpeedTestUrl = msg.data.speedTestURL || defaultSpeedTestUrl;
                    webDebugMode = !!msg.data.debug;
                    cfCountry = normalizeProxyCountry(msg.data || {});
                    proxyWarningRequired = !!msg.data.proxyWarning;
                    proxyGeoCheckOK = msg.data.geoCheckOK !== false;
                    proxyWarningBypassed = false;
                    if (webDebugMode) log('调试模式已开启，错误将输出到程序 log');
                    if (proxyWarningRequired) log(`代理检测警告：当前网络标签 ${cfCountry}`);
                    const speedDefault = msg.data.speedTestDefault || msg.data.speedTestURL || AUTO_SPEED_URL_VALUE;
                    defaultSpeedTestUrl = speedDefault || defaultSpeedTestUrl;
                    applySpeedUrlConfig('officialSpeedUrl', 'officialSpeedUrlPreset', speedDefault);
                    applySpeedUrlConfig('nsbSpeedUrl', 'nsbSpeedUrlPreset', speedDefault);
                    applySpeedURLStatus(msg.data.speedURLStatus);
                    applyProxyStatus(msg.data.proxyStatus);
                    applyDcLibraryStatus(msg.data.dcLibraries);
                    renderVersionInfo(msg.data || {});
                    renderLastRun(msg.data.lastRun || null);
                    autoLoadConfig();
                    if (!scheduleConfigLoaded) {
                        scheduleConfigLoaded = true;
                        wsSend({ type: 'get_schedule_config' });
                    }
                    // 结果面板设置以服务器为准（localStorage 只是本地兜底）
                    wsSend({ type: 'get_result_panel_config' });
                    break;
                case 'result_panel_config':
                    {
                        const panelData = msg.data || {};
                        const serverSettings = panelData.settings;
                        if (serverSettings && typeof serverSettings === 'object' && Object.keys(serverSettings).length) {
                            applyServerResultPanelSettings(serverSettings);
                        } else if (panelData.error) {
                            log('读取服务器结果面板设置失败：' + panelData.error);
                        } else {
                            // 服务器尚无记录：把浏览器里已有的旧设置上送，完成迁移
                            migrateLocalResultPanelSettings();
                        }
                    }
                    break;
                case 'result_panel_config_saved':
                    handleResultPanelSaved(msg.data || {});
                    break;
                case 'last_run':
                    renderLastRun(msg.data || null);
                    break;
                case 'task_config_saved':
                    nzToast('参数已保存到服务器配置文件', 'success');
                    log('任务参数已保存到服务器配置文件');
                    break;
                case 'speed_url_status':
                    applySpeedURLStatus(msg.data || {});
                    break;
                case 'proxy_status':
                    applyProxyStatus(msg.data || {});
                    break;
                case 'history_list_data':
                    renderHistoryList((msg.data || {}).items || []);
                    break;
                case 'history_analysis':
                    {
                        const hp = msg.data || {};
                        window.nzAnalysisData = hp.record || null;
                        if (typeof window.nzRefreshAnalysis === 'function') window.nzRefreshAnalysis();
                        if (hp.fromHistory) log('已恢复最近一次任务分析（来自历史记录）');
                    }
                    break;
                case 'history_deleted':
                case 'history_cleared':
                    {
                        const hc = msg.data || {};
                        const text = msg.type === 'history_cleared' ? ('已清空 ' + (hc.removed || 0) + ' 条历史记录') : '已删除该历史记录';
                        if (typeof nzToast === 'function') nzToast(text, 'success');
                        const fb = document.getElementById('historyFeedback');
                        if (fb) { fb.textContent = text; fb.style.display = ''; }
                        if (msg.type === 'history_cleared') {
                            window.nzAnalysisData = null;
                            if (typeof window.nzRefreshAnalysis === 'function') window.nzRefreshAnalysis();
                        }
                        nzLoadHistoryList();
                    }
                    break;
                case 'notify_test_result':
                    {
                        const nr = msg.data || {};
                        const channelName = nr.channel === 'bark' ? 'Bark' : 'Telegram';
                        const text = nr.ok ? channelName + ' 测试通知已发送' : channelName + ' 测试失败：' + (nr.error || '未知错误');
                        if (typeof nzToast === 'function') nzToast(text, nr.ok ? 'success' : 'error');
                        const feedback = document.getElementById('notifyTestFeedback');
                        if (feedback) {
                            feedback.textContent = text;
                            feedback.style.display = '';
                        }
                    }
                    break;
                case 'schedule_config': {
                    applyScheduleConfig(msg.data || {});
                    const noticeSuppressed = suppressScheduleSavedNotice;
                    suppressScheduleSavedNotice = false;
                    if (msg.data && msg.data.saved && !noticeSuppressed) showNoticeCard('调度设置已保存');
                    break;
                }
                case 'proxy_updated':
                    applyProxyStatus(msg.data || {}); {
                        const input = document.getElementById('proxyInput');
                        if (input && (msg.data?.saved !== false)) {
                            input.value = '';
                        }
                    }
                    showNoticeCard((msg.data && msg.data.message) || '代理设置已保存并生效');
                    break;
                case 'version_info':
                    renderVersionInfo(msg.data || {});
                    break;
                case 'dc_library_status':
                    applyDcLibraryStatus(msg.data || {});
                    if (dcLibraryScanning) showNoticeCard('数据中心库已更新');
                    break;
                case 'dc_library_progress':
                    if (isStoppingTask) break;
                    updateProgress(msg.data.current || 0, msg.data.total || 0, '更新数据中心库');
                    break;
                case 'proxy_country_result':
                    handleProxyCountryResult(msg.data || {});
                    break;
                case 'config_data':
                    applyConfigValues(mapCLIConfigToForm(msg.data));
                    log('配置文件已从服务器加载');
                    break;
                case 'scan_progress':
                    if (isStoppingTask) break;
                    updateProgress(msg.data.current || 0, msg.data.total || 0, '扫描中', msg.data.etaSeconds);
                    break;
                case 'scan_result':
                    if (isStoppingTask) break;
                    window.nzAnalysisData = null; // 实时结果到达后退出历史回显
                    addScanResult(msg.data, currentMode === 'official' && runningTaskMode !== 'nsb');
                    break;
                case 'scan_complete_wait_dc':
                    const wasStoppingScan = isStoppingTask;
                    // 自动测速流水线由服务端在任务内继续执行，保持运行态等后续阶段
                    if (!officialAutoSpeedPending) {
                        finishRunningTask('✨ 重新开始扫描');
                    }
                    if (currentMode === 'official') {
                        sortAndRenderScanResults();
                        showDCSelection(msg.data);
                        if (!wasStoppingScan && !officialAutoSpeedPending) {
                            completeProgressWithCurrentCount('扫描完成，选择数据中心');
                        }
                    }
                    break;
                case 'test_progress':
                    if (isStoppingTask) break;
                    if (currentMode === 'official') {
                        updateProgress(msg.data.current || 0, msg.data.total || 0, '测试中');
                    }
                    break;
                case 'test_result':
                    if (isStoppingTask) break;
                    addTestResult(msg.data, currentMode === 'official' && document.getElementById('test').classList.contains('active'));
                    break;
                case 'test_complete':
                    if (isStoppingTask) {
                        resetStoppedTaskUI('已手动终止');
                        log('任务已手动终止');
                        break;
                    }
                    if (currentMode === 'official') {
                        testData = msg.data;
                        renderAllTestResults();
                        onTestComplete();
                        officialTestCompletePending = true;
                        updateProgress(msg.data.length, msg.data.length, '测试完成');
                        if (officialAutoSpeedPending) {
                            // 自动流水线：详细测试结果的上传延后到测速完成节点，避免重复上传
                            officialAutoStage = 'speed';
                        } else {
                            autoUploadOfficialResults();
                        }
                    }
                    break;
                case 'speed_test_result':
                    if (isStoppingTask) break;
                    updateSpeedResult(msg.data);
                    break;
                case 'official_speed_progress':
                    if (isStoppingTask) break;
                    {
                        const p = msg.data || {};
                        updateQualifiedSpeedProgress(p.qualified || 0, p.limit || 1);
                    }
                    break;
                case 'official_speed_complete':
                    if (isStoppingTask) {
                        resetStoppedTaskUI('已手动终止');
                        log('任务已手动终止');
                        break;
                    }
                    officialAutoSpeedPending = false;
                    officialAutoStage = '';
                    officialBatchSpeedRunning = false;
                    officialSpeedEndedByRateLimit = !!msg.data?.rateLimited;
                    officialSpeedEnabledForResults = ((msg.data?.totalQualified ?? msg.data?.qualified) || 0) > 0;
                    officialContinueSpeedUnlocked = true;
                    finishRunningTask('✨ 开始扫描与测试');
                    updateBatchSpeedButton();
                    setOfficialTestQualifiedFilter(officialSpeedEnabledForResults ? 'qualified' : 'all');
                    renderAllTestResults();
                    sortAndRenderScanResults();
                    if (officialSpeedEndedByRateLimit) {
                        progressBar.style.background = 'var(--warning-color)';
                        progressBar.style.width = '100%';
                        progressBar.style.animation = 'none';
                        progressBar.textContent = '检测到触发速率限制，请稍后重试或更换网络环境';
                    } else {
                        updateProgress(msg.data?.qualified || 0, msg.data?.limit || 1, '测速完成');
                    }
                    autoUploadOfficialResults();
                    break;
                case 'github_upload_result':
                    if (!msg.data.silent) {
                        setGitHubUploadStatus(`上传完成: ${msg.data.rawURL || msg.data.path || ''}`, 'success');
                        renderRawURLCopy(msg.data.rawURL || '');
                    }
                    log(`GitHub 上传完成: ${msg.data.rawURL || msg.data.path || ''}`);
                    showNoticeCard(`GitHub 上传完成\n${msg.data.path || ''}`);
                    break;

                case 'github_upload_status':
                    setGitHubUploadStatus(msg.data.message || 'GitHub 上传中', 'success');
                    break;

                case 'github_upload_error':
                    recordWebDebugError('github_upload_error', msg.data?.message || msg.data || 'GitHub 上传失败');
                    if (msg.data.silent) {
                        showNoticeCard(msg.data.message || 'GitHub 自动上传失败');
                    } else {
                        setGitHubUploadStatus(msg.data.message || 'GitHub 上传失败', 'error');
                    }
                    log(msg.data.message || 'GitHub 上传失败');
                    break;

                case 'edgetunnel_upload_result':
                    setEdgetunnelUploadStatus('上传成功', 'success');
                    log('edgetunnel 上传完成');
                    showNoticeCard('edgetunnel 上传完成');
                    break;

                case 'edgetunnel_upload_status':
                    setEdgetunnelUploadStatus(msg.data.message || 'edgetunnel 上传中', 'success');
                    break;

                case 'edgetunnel_upload_error':
                    recordWebDebugError('edgetunnel_upload_error', msg.data?.message || msg.data || 'edgetunnel 上传失败');
                    if (msg.data.silent) {
                        showNoticeCard(msg.data.message || 'edgetunnel 自动上传失败');
                    } else {
                        setEdgetunnelUploadStatus(msg.data.message || 'edgetunnel 上传失败', 'error');
                    }
                    log(msg.data.message || 'edgetunnel 上传失败');
                    break;

                case 'r2_upload_status':
                    setR2UploadStatus(msg.data.message || 'R2 上传中', 'success');
                    break;

                case 'r2_upload_result':
                    setR2UploadStatus(msg.data.message || 'R2 上传成功', 'success');
                    log(`R2 上传完成: ${msg.data.filename || ''}`);
                    showNoticeCard(`R2 上传完成\n${msg.data.filename || ''}`);
                    break;

                case 'r2_upload_error':
                    recordWebDebugError('r2_upload_error', msg.data?.message || msg.data || 'R2 上传失败');
                    setR2UploadStatus(msg.data.message || 'R2 上传失败', 'error');
                    log(msg.data.message || 'R2 上传失败');
                    break;

                case 'nsb_progress':
                    if (currentMode === 'nsb') {
                        const p = msg.data || {};
                        const percent = parseFloat(p.percent || 0);
                        const txt = p.text || '处理中';
                        if (p.phase === 'speed') {
                            updateQualifiedSpeedProgress(p.current || 0, p.total || 1);
                        } else {
                            updateProgress(p.current || 0, p.total || 1, `${txt}`, p.etaSeconds);
                            if (p.phase === 'reallink') progressBar.style.animation = 'none';
                        }
                    }
                    break;
                case 'nsb_scan_result':
                    window.nzAnalysisData = null; // 实时结果到达后退出历史回显
                    if (nsbBatchSpeedRunning) {
                        updateNSBDisplayedResult(msg.data || {});
                    } else {
                        addNSBLiveResult(msg.data || {}, currentMode === 'nsb');
                    }
                    break;
                case 'nsb_scan_sorted':
                    nsbLiveRows = [];
                    nsbLiveIndex = {};
                    nsbLiveRowElements = {};
                    (msg.data || []).forEach((row) => addNSBLiveResult(row || {}, false));
                    if (currentMode === 'nsb') renderAllNSBLiveRows();
                    break;
                case 'nsb_speed_complete':
                    if (isStoppingTask) {
                        resetStoppedTaskUI('已手动终止');
                        log('任务已手动终止');
                        break;
                    }
                    nsbBatchSpeedRunning = false;
                    nsbSpeedEnabledForResults = ((msg.data?.totalQualified ?? msg.data?.qualified) || 0) > 0;
                    nsbContinueSpeedUnlocked = true;
                    finishRunningTask('✨ 开始扫描与测试');
                    updateNSBSpeedButtons();
                    if (nsbSpeedEnabledForResults) setNSBTableQualifiedFilter('qualified');
                    updateProgress(msg.data?.qualified || 0, msg.data?.limit || 1, '测速完成');
                    autoUploadNSBResults();
                    break;

                case 'nsb_csv_ready':
                    finishRunningTask('✨ 开始扫描与测试');
                    nsbStopHandledByComplete = true;
                    progressBar.style.background = 'var(--warning-color)';
                    progressBar.style.width = '100%';
                    progressBar.style.animation = 'none';
                    progressBar.textContent = '任务已停止，结果已整理，正在回传表格';
                    log(`任务已停止: ${msg.data?.message || '已整理当前可用结果'}${msg.data?.file ? '，结果文件: ' + msg.data.file : ''}`);
                    break;

                case 'nsb_csv_complete':
                    const wasStoppingNSB = isStoppingTask;
                    finishRunningTask('✨ 开始扫描与测试');
                    nsbReallinkRunning = false;
                    nsbStopHandledByComplete = wasStoppingNSB;
                    nsbHeaders = msg.data.headers || [];
                    nsbRows = msg.data.rows || [];
                    nsbLiveRows = [];
                    nsbLiveIndex = {};
                    nsbLiveRowElements = {};
                    nsbContinueSpeedUnlocked = nsbSpeedEnabledForResults;
                    if (nsbSpeedEnabledForResults) {
                        setNSBTableQualifiedFilter((msg.data.qualifiedCount || 0) > 0 ? 'qualified' : 'all');
                    }
                    renderScanRowsCSV(nsbHeaders, nsbRows);
                    const nsbStatus = msg.data.status || 'complete';
                    const nsbMessage = msg.data.message || '测试完成';
                    updateProgress(nsbRows.length, Math.max(nsbRows.length, 1), nsbStatus === 'failed' ? '任务失败' : '完成');
                    progressBar.style.background = nsbStatus === 'failed' ? 'var(--error-color)' : (nsbStatus === 'partial' ? 'var(--warning-color)' : 'linear-gradient(to right, #10b981, #34d399)');
                    progressBar.style.animation = 'none';
                    if (nsbStatus === 'failed') {
                        progressBar.textContent = '任务失败';
                        log(`任务失败: 测试${nsbMessage}`);
                    } else if (nsbStatus === 'partial') {
                        progressBar.textContent = '任务结束';
                        log(`任务结束: ${nsbMessage}`);
                    } else {
                        log(`非标优选完成，已生成: ${msg.data.file}`);
                    }
                    autoUploadNSBResults();
                    break;

                case 'task_stopped':
                    if (nsbStopHandledByComplete) {
                        nsbStopHandledByComplete = false;
                        break;
                    }
                    resetStoppedTaskUI('已手动终止');
                    log('任务已手动终止');
                    break;

                case 'error':
                    recordWebDebugError('server_error', msg.data);
                    isStoppingTask = false;
                    if (String(msg.data || '').includes('已有任务正在运行')) {
                        // 自动流水线由服务端续跑，前端无需重试串联
                        if (officialBatchSpeedRunning) {
                    officialBatchSpeedRunning = false;
                    finishRunningTask('✨ 开始扫描与测试');
                } else if (nsbBatchSpeedRunning) {
                    nsbBatchSpeedRunning = false;
                    finishRunningTask('✨ 开始扫描与测试');
                } else if (nsbReallinkRunning) {
                    nsbReallinkRunning = false;
                    finishRunningTask('✨ 开始扫描与测试');
                }
                resetCurrentSpeedButton();
                updateBatchSpeedButton();
                updateNSBSpeedButtons();
                log('错误: ' + msg.data);
                showNoticeCard(String(msg.data || '服务端返回错误'));
                break;
            }
                    if (String(msg.data || '').includes('上传 GitHub')) {
                        setGitHubUploadStatus(String(msg.data), 'error');
                    }
                    if (msg.data && (msg.data.toLowerCase().includes('cancel') || msg.data.toLowerCase().includes('stop'))) {
                        resetStoppedTaskUI('已手动终止');
                    } else {
                        officialAutoSpeedPending = false;
                        officialAutoStage = '';
                        officialBatchSpeedRunning = false;
                        nsbBatchSpeedRunning = false;
                        officialTestCompletePending = false;
                        officialSpeedEndedByRateLimit = false;
                        if (currentSpeedTestingIP) {
                            resetCurrentSpeedButton();
                        }
                        finishRunningTask('✨ 开始扫描与测试');
                        updateBatchSpeedButton();
                        updateNSBSpeedButtons();
                        finishCompactIPv4();
                        progressBar.style.background = 'var(--error-color)';
                        progressBar.style.width = '100%';
                        progressBar.style.animation = 'none';
                        progressBar.textContent = '任务失败';
                    }
                    log('错误: ' + msg.data);
                    break;

                case 'reset_config_result':
                    handleResetConfigResult(msg.data);
                    break;
                case 'compact_ipv4_progress':
                    if (isStoppingTask) break;
                    {
                        const p = msg.data || {};
                        const percent = parseFloat(p.percent || 0);
                        updateProgress(p.current || 0, p.total || 1, `精简第 ${p.pass || 1} 轮`);
                        progressBar.textContent = `精简第 ${p.pass || 1} 轮 ${p.current || 0}/${p.total || 1} (${Math.round(percent)}%) 已保留 ${p.found || 0}`;
                    }
                    break;
                case 'compact_ipv4_hit':
                    if (isStoppingTask) break;
                    if (msg.data && msg.data.ip) {
                        compactHitBuffer.push(msg.data.ip);
                        if (!compactHitFlushTimer) {
                            compactHitFlushTimer = setTimeout(flushCompactHits, 500);
                        }
                    }
                    break;
                case 'compact_ipv4_done':
                    if (isStoppingTask) {
                        resetStoppedTaskUI('已手动终止');
                        log('任务已手动终止');
                        break;
                    }
                    flushCompactHits();
                    finishRunningTask('✨ 开始扫描与测试');
                    finishCompactIPv4('✅');
                    setTimeout(() => finishCompactIPv4('🧹'), 3000);
                    progressBar.style.background = 'linear-gradient(to right, #10b981, #34d399)';
                    progressBar.style.width = '100%';
                    progressBar.style.animation = 'none';
                    progressBar.textContent = `精简完成：保留 ${msg.data.count} 个子网`;
                    log(`IPv4 地址库精简完成：保留 ${msg.data.count} 个子网，已覆盖 ${msg.data.file}`);
                    break;
                case 'log':
                    log(msg.data);
                    break;
                case 'task_complete':
                    if (isStoppingTask) {
                        resetStoppedTaskUI('已手动终止');
                        log('任务已手动终止');
                        break;
                    }
                    if (officialBatchSpeedRunning) {
                        updateBatchSpeedButton();
                        break;
                    }
                    finishRunningTask('✨ 开始扫描与测试');
                    updateBatchSpeedButton();
                    if (officialTestCompletePending) {
                        officialTestCompletePending = false;
                        renderAllTestResults();
                        updateProgress(testData.length, Math.max(testData.length, 1), '测试完成');
                    } else if (officialSpeedEndedByRateLimit) {
                        officialSpeedEndedByRateLimit = false;
                    } else {
                        completeProgressWithCurrentCount('完成');
                    }
                    // 自动测速流水线由服务端在任务内续跑，前端不再串联发起后续任务
                    break;
            }
        }

        function renderVersionInfo(data) {
            const el = document.getElementById('versionLine');
            if (!el) return;
            const version = data.version || 'dev';
            const latest = data.latest || '';
            const releaseURL = data.releaseURL || 'https://github.com/PoemMisty/CFData-WEB/releases/latest';
            let html = escapeHTML(version);
            if (data.hasUpdate && latest) {
                html += ` <a href="${releaseURL}" target="_blank" rel="noopener noreferrer">发现新版本 ${escapeHTML(latest)}</a>`;
            } else if (data.error) {
                html += ` <span id="versionCheckError">更新检测失败</span>`;
            }
            el.innerHTML = html;
            if (data.error) {
                window.clearTimeout(renderVersionInfo.errorTimer);
                renderVersionInfo.errorTimer = window.setTimeout(() => {
                    const tip = document.getElementById('versionCheckError');
                    if (tip) tip.remove();
                }, 10000);
            }
        }

        function handleResetConfigResult(data) {
            const deleted = (data.deleted || []).join(', ') || '无';
            const missing = (data.missing || []).join(', ') || '无';
            const failed = (data.failed || []).join(', ') || '无';
            const reminder = data.reminder || '删掉之后，下次重新运行程序时会自动重新下载。';

            if (data.success) {
                showNoticeCard(`本地缓存已清理\n\n已删除: ${deleted}\n不存在: ${missing}\n\n提示: ${reminder}`);
                log(`本地缓存已清理，已删除: ${deleted}；不存在: ${missing}`);
            } else {
                showNoticeCard(`本地缓存清理失败\n\n已删除: ${deleted}\n不存在: ${missing}\n删除失败: ${failed}\n\n提示: ${reminder}`);
                log(`清理本地缓存时有文件没删掉: ${failed}`);
            }
        }

        window.addEventListener('DOMContentLoaded', () => {
            connect();
            initializeUI();
            bindScheduleCronControl();
            renderGitHubAccounts();
            renderEdgetunnelAccounts();
            switchMode('official');
            // 面板设置恢复必须晚于 switchMode 的初始化渲染，否则恢复值会被重置
            restoreResultPanelSettings();
            initThemeFromStorage();
        });

        function autoThemeBySystem() {
            const saved = localStorage.getItem('cfdata-theme') || 'auto';
            if (saved === 'auto') {
                applyTheme('auto');
            }
        }
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', autoThemeBySystem);