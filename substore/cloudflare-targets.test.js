'use strict';

const assert = require('node:assert/strict');
const path = require('node:path');

const targetPath = path.join(__dirname, 'cloudflare-targets.js');

function captureLogs() {
  const logs = [];
  const original = console.log;
  console.log = function (...args) {
    logs.push(args.join(' '));
  };
  return {
    logs,
    restore() {
      console.log = original;
    },
  };
}

function withFetch(fetchImpl, fn) {
  const original = globalThis.fetch;
  globalThis.fetch = fetchImpl;
  const promise = Promise.resolve().then(fn);
  return promise.finally(() => {
    globalThis.fetch = original;
  });
}

function withoutFetch(fn) {
  const original = globalThis.fetch;
  globalThis.fetch = undefined;
  const promise = Promise.resolve().then(fn);
  return promise.finally(() => {
    globalThis.fetch = original;
  });
}

async function main() {
  const lib = require(targetPath);
  assert.equal(typeof lib.operator, 'function', 'operator export');

  assert.equal(lib.sanitizeAlias('  LA#X\n Y\tZ  '), 'LAX Y Z');

  const parsed = lib.parseTargetText(
    [
      '# comment',
      '1.2.3.4:443#LAX',
      '[2606:4700::1]:8443#HKG',
      '[2606:4700::2]#NRT',
      '5.6.7.8#  ',
      '9.9.9.9:70000#badport',
      'no-separator',
    ].join('\n')
  );
  assert.equal(parsed.length, 3, 'invalid lines skipped');
  assert.deepEqual(parsed[0], { host: '1.2.3.4', port: 443, alias: 'LAX' });
  assert.deepEqual(parsed[1], { host: '2606:4700::1', port: 8443, alias: 'HKG' });
  assert.deepEqual(parsed[2], { host: '2606:4700::2', port: undefined, alias: 'NRT' });

  assert.deepEqual(
    lib.parseTargetLine('2606:4700::1#v6'),
    { host: '2606:4700::1', port: undefined, alias: 'v6' },
    'unbracketed IPv6 accepted as host'
  );
  assert.deepEqual(
    lib.parseTargetLine('2001:db8:0:0:0:0:0:1#full'),
    { host: '2001:db8:0:0:0:0:0:1', port: undefined, alias: 'full' },
    'full IPv6 accepted as host'
  );
  for (const bad of ['example.com:80:90#bad', 'abc:def#bad', '1.2.3.4:443:80#bad']) {
    assert.throws(() => lib.parseTargetLine(bad), /invalid/i, 'multi-colon host rejected: ' + bad);
  }

  const staticLogs = captureLogs();
  let statics;
  try {
    statics = lib.parseStaticTargets(['1.2.3.4#A', 'bad-line', '# comment', '5.6.7.8:0#B']);
  } finally {
    staticLogs.restore();
  }
  assert.equal(statics.length, 1);
  assert.equal(staticLogs.logs.length, 2, 'two invalid static targets reported');

  const deduped = lib.dedupeTargets([
    { host: 'A.example.com', port: 443, alias: 'X' },
    { host: 'a.example.com', port: 443, alias: 'X' },
    { host: 'a.example.com', port: 8443, alias: 'X' },
    { host: 'a.example.com', port: 443, alias: 'Y' },
  ]);
  assert.equal(deduped.length, 3, 'identical targets removed');

  const aliased = lib.dedupAliases([
    { host: 'a', alias: 'X' },
    { host: 'b', alias: 'X' },
    { host: 'c', alias: 'Y' },
  ]);
  assert.deepEqual(
    aliased.map((t) => t.alias),
    ['X\u2070', 'X\u00B9', 'Y']
  );

  const collision = lib.dedupAliases([
    { host: 'a', alias: 'X' },
    { host: 'b', alias: 'X' },
    { host: 'c', alias: 'X\u2070' },
  ]);
  assert.deepEqual(
    collision.map((t) => t.alias),
    ['X\u00B9', 'X\u00B2', 'X\u2070'],
    'suffix skips alias already in use'
  );

  const wetest = lib.parseTargetText(
    '<table><tr><td data-label="线路名称">电信</td><td data-label="优选地址">1.2.3.4</td><td data-label="数据中心">LAX</td></tr></table>'
  );
  assert.equal(wetest.length, 1);
  assert.equal(wetest[0].alias, 'CF优选-电信-LAX');

  lib.CONFIG.urlTimeout = 50;
  let seenOptions;
  await withFetch(
    async (url, options) => {
      seenOptions = options;
      return { ok: true, text: async () => '1.2.3.4#A\n[2606:4700::1]#B\n' };
    },
    async () => {
      const targets = await lib.fetchTargetsFromUrls(['https://example.com/list.txt']);
      assert.equal(targets.length, 2);
    }
  );
  assert.ok(seenOptions && seenOptions.signal, 'fetch receives AbortSignal');

  lib.CONFIG.urlTimeout = 20;
  const timeoutLogs = captureLogs();
  let timedOut;
  try {
    await withFetch(
      (url, options) =>
        new Promise((resolve, reject) => {
          options.signal.addEventListener('abort', () => {
            const err = new Error('aborted');
            err.name = 'AbortError';
            reject(err);
          });
        }),
      async () => {
        timedOut = await lib.fetchTargetsFromUrls(['https://slow.example/list.txt']);
      }
    );
  } finally {
    timeoutLogs.restore();
  }
  assert.deepEqual(timedOut, [], 'timed-out URL contributes no targets');
  assert.ok(
    timeoutLogs.logs.some((line) => line.indexOf('fetch failed') !== -1),
    'timeout failure logged'
  );

  await withoutFetch(async () => {
    await assert.rejects(
      () => lib.fetchTargetsFromUrls(['https://example.com/list.txt']),
      /no HTTP client/i
    );
  });

  lib.CONFIG.targets = ['1.2.3.4:443#LAX', '[2606:4700::1]#HKG', 'bad-line'];
  lib.CONFIG.urls = [];
  const output = await lib.operator([{ name: 'node', server: 'old.example.com', port: 1 }], 'clash', {});
  assert.equal(output.length, 2, 'invalid static target skipped');
  assert.equal(output[0].name, 'node-LAX');
  assert.equal(output[0].server, '1.2.3.4');
  assert.equal(output[0].port, 443);
  assert.equal(output[1].name, 'node-HKG');
  assert.equal(output[1].server, '2606:4700::1');
  assert.equal(output[1].port, 1, 'missing target port keeps proxy port');

  lib.CONFIG.targets = ['1.2.3.4:443#LAX'];
  lib.CONFIG.urls = ['https://example.com/list.txt'];
  await withFetch(
    async () => ({ ok: true, text: async () => '1.2.3.4:443#LAX\n5.6.7.8:443#LAX\n' }),
    async () => {
      const merged = await lib.operator([{ name: 'p', server: 'old', port: 1 }], 'clash', {});
      assert.equal(merged.length, 2, 'static and fetched duplicates merged');
      assert.deepEqual(
        merged.map((p) => p.name),
        ['p-LAX\u2070', 'p-LAX\u00B9']
      );
    }
  );

  await withoutFetch(async () => {
    await assert.rejects(
      () => lib.operator([{ name: 'p', server: 'old' }], 'clash', {}),
      /no HTTP client/i
    );
  });

  lib.CONFIG.targets = [];
  lib.CONFIG.urls = [];
  const empty = await lib.operator([{ name: 'p', server: 'old' }], 'clash', {});
  assert.deepEqual(empty, [{ name: 'p', server: 'old' }], 'no targets returns input');

  console.log('cloudflare-targets.test.js: all assertions passed');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
