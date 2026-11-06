const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

global.window = {};
eval(fs.readFileSync(path.join(__dirname, '..', 'common.js'), 'utf8'));
const buildChoices = global.window.buildResultFieldChoices;
const choiceKeys = global.window.resultFieldChoiceKeys;

const BUILTIN = [
    { key: 'ipport', label: 'IP:PORT' },
    { key: 'ip', label: 'IP地址' },
    { key: 'dc', label: '数据中心' },
];
const CUSTOM = [{ key: 'note', label: '备注', value: '测试版' }];

test('字段清单始终包含全部内置字段与自定义字段', () => {
    const fields = buildChoices(BUILTIN, CUSTOM);
    assert.deepEqual(fields.map((f) => f.key), ['ipport', 'ip', 'dc', 'note']);
});

test('字段清单不随结果数据变化：空自定义字段仍返回全部内置字段', () => {
    assert.deepEqual(choiceKeys(BUILTIN, []), ['ipport', 'ip', 'dc']);
    assert.deepEqual(choiceKeys([], []), []);
});

test('自定义字段追加在内置字段之后', () => {
    const fields = buildChoices(BUILTIN, CUSTOM);
    assert.equal(fields[fields.length - 1].key, 'note');
    assert.equal(fields[0].key, 'ipport');
});

test('字段 key 去重且保留首次出现顺序', () => {
    const duplicated = [{ key: 'ipport', label: 'IP:PORT' }, { key: 'dc', label: '数据中心' }];
    const custom = [{ key: 'dc', label: '重复自定义' }, { key: 'note', label: '备注' }];
    assert.deepEqual(choiceKeys(duplicated, custom), ['ipport', 'dc', 'note']);
    // 清单本身仍按输入顺序保留重复项，交由 key 列表去重
    assert.equal(buildChoices(duplicated, custom).length, 4);
});

test('非法输入按空清单处理，不抛异常', () => {
    assert.deepEqual(choiceKeys(undefined, null), []);
    assert.deepEqual(choiceKeys([null, { label: '无 key' }, { key: '' }], []), []);
});
