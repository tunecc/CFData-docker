const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

global.window = {};
// 用不同名变量接收，避免与 eval 声明的函数名冲突（直接 eval 声明进模块作用域）
eval(fs.readFileSync(path.join(__dirname, '..', 'common.js'), 'utf8'));
const fmtEtaText = global.window.formatEtaText;
const parseThreadCount = global.window.parseThreadsValue;

test('formatEtaText 对缺失/负值返回空串（自动隐藏）', () => {
    assert.equal(fmtEtaText(-1), '');
    assert.equal(fmtEtaText(undefined), '');
    assert.equal(fmtEtaText(null), '');
    assert.equal(fmtEtaText('x'), '');
});

test('formatEtaText 输出 mm:ss', () => {
    assert.equal(fmtEtaText(0), ' · 预计剩余 0:00');
    assert.equal(fmtEtaText(59), ' · 预计剩余 0:59');
    assert.equal(fmtEtaText(75), ' · 预计剩余 1:15');
    assert.equal(fmtEtaText(3600), ' · 预计剩余 60:00');
});

test('parseThreadsValue 保留 0=自动语义', () => {
    assert.equal(parseThreadCount('0'), 0);
});

test('parseThreadsValue 非法/负值回落默认 100', () => {
    assert.equal(parseThreadCount('abc'), 100);
    assert.equal(parseThreadCount('-5'), 100);
    assert.equal(parseThreadCount(''), 100);
    assert.equal(parseThreadCount(undefined), 100);
});

test('parseThreadsValue 正数原样', () => {
    assert.equal(parseThreadCount('30'), 30);
    assert.equal(parseThreadCount('400'), 400);
});
