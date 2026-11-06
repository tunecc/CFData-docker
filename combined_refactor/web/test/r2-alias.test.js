const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

global.window = {};
eval(fs.readFileSync(path.join(__dirname, '..', 'common.js'), 'utf8'));
// 用不同名变量接收，避免与 eval 声明的函数名冲突
const aliasTemplate = global.window.buildR2AliasTemplateFromOrder;
const NONE = global.window.R2_ALIAS_NONE_TEMPLATE;

test('R2 别名排除 IP:PORT，只保留其余字段', () => {
    assert.equal(aliasTemplate(['ipport', 'dc'], '-'), '{dc}');
    assert.equal(aliasTemplate(['ipport', 'dc', 'dcCountry'], '-'), '{dc}-{dcCountry}');
    assert.equal(aliasTemplate(['dc', 'ipport', 'region'], '|'), '{dc}|{region}');
});

test('只选 IP:PORT 时返回无别名标记', () => {
    assert.equal(aliasTemplate(['ipport'], '-'), NONE);
    assert.equal(aliasTemplate([], '-'), NONE);
    assert.equal(aliasTemplate(['customOnly'], '-'), NONE);
});

test('无其他可识别字段时返回无别名标记，未知字段被忽略', () => {
    assert.equal(aliasTemplate(['ipport', 'custom', 'note'], '-'), NONE);
    assert.equal(aliasTemplate(['ipport', 'ip', 'port'], '-'), '{ip}-{port}');
});

test('非法输入按无别名处理，空分隔符回落 -', () => {
    assert.equal(aliasTemplate(undefined, '-'), NONE);
    assert.equal(aliasTemplate(null, '-'), NONE);
    assert.equal(aliasTemplate(['ipport', 'dc'], ''), '{dc}');
});
