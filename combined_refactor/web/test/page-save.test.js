const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

global.window = {};
eval(fs.readFileSync(path.join(__dirname, '..', 'common.js'), 'utf8'));
// 用不同名变量接收，避免与 eval 声明的函数名冲突（直接 eval 声明进模块作用域）
const anyFilled = global.window.hasAnyFieldValue;

test('页面级保存：空白表单视为未填写，应跳过', () => {
    assert.equal(anyFilled(['', '   ', null, undefined]), false);
});

test('页面级保存：任一字段填写即视为需要保存', () => {
    assert.equal(anyFilled(['', 'myname/myrepo', '']), true);
    assert.equal(anyFilled(['', '', 'ghp_token']), true);
    assert.equal(anyFilled(['备注', '', '']), true);
});

test('页面级保存：非数组输入按空表单处理', () => {
    assert.equal(anyFilled(undefined), false);
    assert.equal(anyFilled(null), false);
    assert.equal(anyFilled('token'), false);
});
