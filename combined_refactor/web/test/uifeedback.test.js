const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

global.window = {};
// 用不同名变量接收，避免与 eval 声明的函数名冲突（直接 eval 声明进模块作用域）
eval(fs.readFileSync(path.join(__dirname, '..', 'common.js'), 'utf8'));
const tier = global.window.qualifiedRateTier;
const trunc = global.window.truncateMiddle;
const BROADCAST_KEY = global.window.TASK_DONE_BROADCAST_KEY;
const buildBroadcast = global.window.buildTaskDoneBroadcast;
const parseBroadcast = global.window.parseTaskDoneBroadcast;
const createBuffer = global.window.createLogBuffer;

test('qualifiedRateTier 三态：0 红 / <5% 橙 / ≥5% 绿', () => {
    assert.equal(tier(0, 100), 'bad');       // 0%
});

test('qualifiedRateTier 边界：5/100=5% 为 good，4/100=4% 为 warn', () => {
    assert.equal(tier(4, 100), 'warn');
    assert.equal(tier(5, 100), 'good');
    assert.equal(tier(1, 1000), 'warn');     // 0.1%
    assert.equal(tier(100, 100), 'good');    // 100%
});

test('qualifiedRateTier 非法输入按 bad 处理', () => {
    assert.equal(tier(10, 0), 'bad');        // 无结果
    assert.equal(tier(0, 0), 'bad');
    assert.equal(tier(-1, 100), 'bad');
    assert.equal(tier('x', 100), 'bad');
});

test('truncateMiddle 保头去中', () => {
    assert.equal(trunc('short', 10), 'short');            // 不超长原样
    assert.equal(trunc('', 5), '');                        // 空串
    const out = trunc('abcdefghijklmnop', 9);
    assert.equal(out.length, 9);
    assert.ok(out.startsWith('abcd'));
    assert.ok(out.endsWith('nop'));
    assert.ok(out.includes('…'));
    assert.equal(trunc('abcdef', 1), 'a');                 // 上限 1 只留头
    assert.equal(trunc(null, 5), '');                      // 非字符串
    assert.equal(trunc('abcdef', 0), 'abcdef');            // 非法上限原样
});

test('任务完成广播：键固定、格式带时间戳与模式', () => {
    assert.equal(BROADCAST_KEY, 'cfdata_task_done');
    const v = JSON.parse(buildBroadcast('nsb', 1234));
    assert.deepEqual(v, { ts: 1234, mode: 'nsb' });
    assert.equal(JSON.parse(buildBroadcast('official', 5)).mode, 'official');
    assert.equal(JSON.parse(buildBroadcast('unknown', 5)).mode, 'official');
    // ts 缺省取当前时间（正数）
    assert.ok(JSON.parse(buildBroadcast('official')).ts > 0);
});

test('广播解析：只接受更新时间戳、坏载荷忽略', () => {
    assert.deepEqual(parseBroadcast('{"ts":2000,"mode":"nsb"}', 1000), { ts: 2000, mode: 'nsb' });
    assert.equal(parseBroadcast('{"ts":2000,"mode":"nsb"}', 2000), null);  // 相等不算更新
    assert.equal(parseBroadcast('{"ts":999,"mode":"nsb"}', 1000), null);
    assert.equal(parseBroadcast('not-json', 0), null);
    assert.equal(parseBroadcast('{"ts":"x"}', 0), null);
    assert.equal(parseBroadcast(null, 0), null);
    assert.equal(parseBroadcast('{"ts":3000,"mode":"weird"}', 0).mode, 'official');
});

test('日志框选缓冲：缓冲/回灌/省略计数', () => {
    const buf = createBuffer(5);
    for (let i = 1; i <= 7; i++) buf.push('line-' + i);
    assert.equal(buf.size, 5);          // 上限截断
    assert.equal(buf.omitted, 2);       // 丢最旧 2 行
    const out = buf.drain();
    assert.deepEqual(out.lines, ['line-3', 'line-4', 'line-5', 'line-6', 'line-7']);
    assert.equal(out.omitted, 2);
    assert.equal(buf.size, 0);          // drain 后复位
    assert.equal(buf.omitted, 0);
    buf.push('x');
    buf.clear();
    assert.equal(buf.size, 0);
});

test('日志框选缓冲：默认上限 500', () => {
    const buf = createBuffer();
    for (let i = 0; i < 600; i++) buf.push('l' + i);
    assert.equal(buf.size, 500);
    assert.equal(buf.omitted, 100);
});
