const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

global.window = {};
eval(fs.readFileSync(path.join(__dirname, '..', 'common.js'), 'utf8'));
// 用不同名变量接收，避免与 eval 声明的函数名冲突
const takeRows = global.window.takePreviewRows;

test('预览最多取前 N 行', () => {
    const rows = [1, 2, 3, 4, 5, 6, 7];
    assert.deepEqual(takeRows(rows, 5), [1, 2, 3, 4, 5]);
    assert.deepEqual(takeRows(rows, 3), [1, 2, 3]);
});

test('预览行数不超过实际结果数', () => {
    assert.deepEqual(takeRows([{ a: 1 }], 5), [{ a: 1 }]);
    assert.deepEqual(takeRows([], 5), []);
});

test('非法输入与非法 limit 按安全值处理', () => {
    assert.deepEqual(takeRows(undefined, 5), []);
    assert.deepEqual(takeRows(null, 5), []);
    assert.deepEqual(takeRows('x', 5), []);
    const rows = [1, 2, 3];
    assert.deepEqual(takeRows(rows, 0), rows);
    assert.deepEqual(takeRows(rows, -1), rows);
    assert.deepEqual(takeRows(rows, undefined), rows);
});

test('limit 为小数时向下取整', () => {
    assert.deepEqual(takeRows([1, 2, 3], 2.8), [1, 2]);
});
