const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

global.window = {};
// 用不同名变量接收，避免与 eval 声明的函数名冲突（直接 eval 声明进模块作用域）
eval(fs.readFileSync(path.join(__dirname, '..', 'validators.js'), 'utf8'));
const V = global.window.CFDataValidators;

test('validateConfigField 未注册字段返回 ok', () => {
    assert.deepEqual(V.validateConfigField('notAField', '123'), { level: 'ok', msg: '' });
});

test('并发数量：0=自动、过高提示限频、非法值 warn', () => {
    assert.deepEqual(V.validateConfigField('officialThreads', '0'), { level: 'info', msg: '0 = 按候选规模自动适配' });
    assert.equal(V.validateConfigField('nsbThreads', '100').level, 'ok');
    assert.equal(V.validateConfigField('nsbThreads', '1000').level, 'info');
    assert.equal(V.validateConfigField('officialThreads', '-5').level, 'warn');
    assert.equal(V.validateConfigField('officialThreads', 'abc').level, 'warn');
    assert.equal(V.validateConfigField('officialThreads', '').level, 'ok');
});

test('扫描合格延迟：过低/过高 info、非法 warn', () => {
    assert.equal(V.validateConfigField('officialDelay', '20').level, 'info');
    assert.equal(V.validateConfigField('officialDelay', '0').level, 'ok');
    assert.equal(V.validateConfigField('nsbDelay', '5000').level, 'info');
    assert.equal(V.validateConfigField('nsbDelay', '500').level, 'ok');
    assert.equal(V.validateConfigField('nsbDelay', '-1').level, 'warn');
});

test('测速下限：>10 提示过高、负值 warn', () => {
    assert.equal(V.validateConfigField('officialSpeedMin', '20').level, 'info');
    assert.equal(V.validateConfigField('nsbSpeedMin', '0.1').level, 'ok');
    assert.equal(V.validateConfigField('nsbSpeedMin', '-0.5').level, 'warn');
});

test('数量上限类：0 有语义说明、负/小数 warn', () => {
    const stop0 = V.validateConfigField('officialStopCount', '0');
    assert.equal(stop0.level, 'info');
    assert.ok(stop0.msg.includes('不提前结束'));
    const limit0 = V.validateConfigField('nsbResultLimit', '0');
    assert.ok(limit0.msg.includes('不限制'));
    assert.equal(V.validateConfigField('officialStopCount', '20').level, 'ok');
    assert.equal(V.validateConfigField('nsbResultLimit', '-3').level, 'warn');
    assert.equal(V.validateConfigField('officialStopCount', '2.5').level, 'warn');
});

test('检测间隔：<60 分钟建议 ≥2880、0 说明语义', () => {
    const short = V.validateConfigField('scheduleInterval', '30');
    assert.equal(short.level, 'info');
    assert.ok(short.msg.includes('2880'));
    assert.equal(V.validateConfigField('scheduleInterval', '2880').level, 'ok');
    assert.equal(V.validateConfigField('scheduleInterval', '0').level, 'info');
    assert.equal(V.validateConfigField('scheduleInterval', '-1').level, 'warn');
});

test('备用端口：非 1-65535 warn', () => {
    assert.equal(V.validateConfigField('nsbFallbackPort', '443').level, 'ok');
    assert.equal(V.validateConfigField('nsbFallbackPort', '0').level, 'warn');
    assert.equal(V.validateConfigField('nsbFallbackPort', '70000').level, 'warn');
    assert.equal(V.validateConfigField('nsbFallbackPort', '443.5').level, 'warn');
    assert.equal(V.validateConfigField('nsbFallbackPort', '').level, 'ok');
});

test('Worker URL：留空不提示、非 http(s) 前缀 warn', () => {
    assert.equal(V.validateConfigField('r2WorkerUrl', '').level, 'ok');
    assert.equal(V.validateConfigField('r2WorkerUrl', 'https://r2.example.com').level, 'ok');
    assert.equal(V.validateConfigField('r2WorkerUrl', 'ftp://x').level, 'warn');
    assert.equal(V.validateConfigField('r2WorkerUrl', 'r2.example.com').level, 'warn');
});
