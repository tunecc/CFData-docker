const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

global.window = {};
eval(fs.readFileSync(path.join(__dirname, '..', 'common.js'), 'utf8'));
const fmtProxyTestResult = global.window.formatProxyTestResult;
test('代理测试结果：成功 payload 包含出口 IP、地区、延迟与来源', () => {
    const { text, level } = fmtProxyTestResult({
        ok: true,
        exitIP: '104.16.1.1',
        country: 'US',
        latencyMs: 235,
        proxyLabel: 'socks5://127.0.0.1:1080',
        source: '程序配置',
    });
    assert.equal(level, 'lvl-ok');
    assert.ok(text.includes('测试成功'), text);
    assert.ok(text.includes('socks5://127.0.0.1:1080'), text);
    assert.ok(text.includes('104.16.1.1'), text);
    assert.ok(text.includes('US'), text);
    assert.ok(text.includes('235'), text);
    assert.ok(text.includes('程序配置'), text);
});

test('代理测试结果：成功但缺延迟时不显示延迟片段', () => {
    const { text, level } = fmtProxyTestResult({
        ok: true,
        exitIP: '104.16.1.1',
        country: 'US',
        source: '直连',
    });
    assert.equal(level, 'lvl-ok');
    assert.ok(!text.includes('延迟'), text);
    assert.ok(text.includes('直连'), text);
});

test('代理测试结果：失败 payload 显示原因并归为 warn', () => {
    const { text, level } = fmtProxyTestResult({ ok: false, error: 'connection refused' });
    assert.equal(level, 'lvl-warn');
    assert.ok(text.includes('测试失败'), text);
    assert.ok(text.includes('connection refused'), text);
});

test('代理测试结果：失败且无 error 时显示未知原因', () => {
    const { text, level } = fmtProxyTestResult({ ok: false });
    assert.equal(level, 'lvl-warn');
    assert.ok(text.includes('未知原因'), text);
});

test('代理测试结果：空 payload 按失败处理', () => {
    const { text, level } = fmtProxyTestResult(null);
    assert.equal(level, 'lvl-warn');
    assert.ok(text.includes('未知原因'), text);
});
