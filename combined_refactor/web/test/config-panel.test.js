const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

// 配置面板结构契约（A1/A2 证据）：右栏五页统一 cfg-field 行结构，无旧标记混用、
// 无 readonly 输入框提示牌、无 emoji；表单控件 id 保持稳定（保存链路按 id 收集）。
const html = fs.readFileSync(path.join(__dirname, '..', '..', 'index.html'), 'utf8');

function cfgPanelSlice() {
    const start = html.indexOf('<aside class="config-col">');
    const end = html.indexOf('</aside>', start);
    assert.ok(start > 0 && end > start, '配置面板 aside 区块存在');
    return html.slice(start, end);
}

const panel = cfgPanelSlice();

test('配置面板：统一 cfg-field 行结构且数量充足', () => {
    const count = (panel.match(/class="cfg-field[ "]/g) || []).length;
    assert.ok(count >= 40, `cfg-field 行数量应 ≥40，实际 ${count}`);
});

test('配置面板：不再混用旧表单标记', () => {
    for (const legacy of ['option-item', 'modal-field', 'form-group', 'inline-check',
        'control-title', 'section-box', 'settings-title', 'settings-panel',
        'field-with-action', 'github-field']) {
        assert.equal(panel.includes(legacy), false, `配置面板内不应出现旧标记 ${legacy}`);
    }
});

test('配置面板：readonly 输入框提示牌清零', () => {
    const readonlyInputs = (panel.match(/<input[^>]*readonly[^>]*>/g) || []);
    assert.equal(readonlyInputs.length, 0, `不应有 readonly input，实际 ${readonlyInputs.length}`);
});

test('配置面板：emoji 清零', () => {
    const emoji = panel.match(/[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}\u{2B00}-\u{2BFF}\u{FE0F}]/gu) || [];
    assert.equal(emoji.length, 0, `配置面板不应有 emoji，实际 ${emoji.join(' ')}`);
});

test('配置面板：保存链路依赖的关键控件 id 保持不变', () => {
    const ids = ['officialIpType', 'officialTestPort', 'officialThreads', 'officialDelay',
        'officialScanMode', 'officialScanModeHint', 'officialDcSelect', 'officialTargetDC',
        'officialDcLibraryStatus', 'officialStopCount', 'officialAutoSpeed', 'officialSpeedMin',
        'officialSpeedLimit', 'speedBandwidth', 'officialSpeedUrlPreset', 'officialSpeedUrl',
        'officialAutoUploadAccount', 'nsbFileInput', 'nsbSourceUrl', 'nsbFallbackPort',
        'nsbThreads', 'nsbDelay', 'nsbResultLimit', 'nsbTls', 'nsbScanMode', 'nsbScanModeHint',
        'nsbTargetDC', 'nsbEnableSpeed', 'nsbSpeedMin', 'nsbSpeedLimit', 'nsbSpeedTest',
        'nsbSpeedUrlPreset', 'nsbSpeedUrl', 'nsbRelinkPreset', 'nsbRelinkHost',
        'nsbCompactView', 'nsbOutFile', 'nsbAutoUploadAccount', 'scheduleEnabled',
        'scheduleCron', 'scheduleCronDisplay', 'scheduleInterval', 'notifyBarkServer',
        'notifyBarkKey', 'notifyTelegramToken', 'notifyTelegramChatID', 'proxyInput',
        'proxyStatusText', 'proxySourceText', 'configAutoLoad', 'resultSourceSelect',
        'resultFormat', 'resultFileName', 'resultStartRow', 'resultEndRow',
        'resultIPTypeFilter', 'resultQualifiedFilter', 'resultLatencyFilter', 'resultLossFilter',
        'resultPreview', 'resultCustomFields', 'resultTextSeparatorPreset', 'resultIPv6Brackets',
        'resultOrderList', 'r2Enabled', 'r2WorkerUrl', 'r2WorkerToken', 'r2Filename',
        'r2AliasPreview', 'r2AliasFormat', 'r2UploadStatus', 'githubAccountSelect',
        'githubAccountName', 'githubToken', 'githubRepo', 'githubBranch', 'githubPath',
        'githubMessage', 'githubSaveToken', 'githubQuickFile', 'githubUploadStatus',
        'uploadProxyHint', 'historyList'];
    for (const id of ids) {
        assert.ok(panel.includes(`id="${id}"`), `关键控件 id 缺失: ${id}`);
    }
});

test('配置面板：长说明以可见 hint 呈现（抽查）', () => {
    // 原 title 悬浮说明抽样：应出现在 cfg-label-hint 文本中
    for (const text of ['同时扫描的线程数', '按 /24 子网探测', '真实链路验证',
        '总带宽上限', '自动适配']) {
        assert.ok(/class="cfg-label-hint"[^>]*>[^<]*|<\/span>/.test(panel) &&
            panel.includes(text), `hint 文本缺失: ${text}`);
    }
});

test('配置面板：保存入口与面板切换函数保持不变', () => {
    for (const fn of ['saveTaskConfigToServer(\'official\')', 'saveTaskConfigToServer(\'nsb\')',
        'saveAutomationPage()', 'saveSettingsPage()', 'saveResultsPage()',
        'nzShowConfigPanel(\'mode\')', 'nzShowConfigPanel(\'automation\')',
        'nzShowConfigPanel(\'results\')', 'nzShowConfigPanel(\'settings\')']) {
        assert.ok(panel.includes(fn), `入口函数缺失: ${fn}`);
    }
});
