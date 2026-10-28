const CONFIG = {
  keepOriginal: false,
  targets: [
//    'www.wto.org#wto',
    '123.cf.090227.xyz#CM',
//    'www.visa.cn#visa',
//    'www.shopify.com#shopify',
//    'staticdelivery.nexusmods.com#Mods',
    '172.67.66.139#139',
    '162.159.149.63#63'
  ],

  // 每行格式：host[:port]#alias；同时支持 WeTest HTML 表格。
  // 可直接填入本项目 R2 输出地址，例如：
  //   'https://<你的-R2-公网域名>/cfdata-targets.txt'
  urls: [
    'https://cf.090227.xyz/cu',
    'https://cf.090227.xyz/ct?ips=6',
//    'https://cf.090227.xyz/cmcc?ips=8',
    'https://www.wetest.vip/page/cloudflare/address_v4.html',
  ],

  urlTimeout: 5000,
  wetestOperators: ['电信', '联通', '移动'],
};

var SUPERSCRIPT_DIGITS = [
  '\u2070', '\u00B9', '\u00B2', '\u00B3', '\u2074',
  '\u2075', '\u2076', '\u2077', '\u2078', '\u2079'
];

var IPV6_RE = /^\[([^\]]+)\](?::(\d+))?$/;
var LF = String.fromCharCode(10);

var deepClone = typeof structuredClone === 'function'
  ? structuredClone
  : function (obj) { return JSON.parse(JSON.stringify(obj)); };

function warn(message) {
  if (typeof console !== 'undefined' && console && typeof console.log === 'function') {
    console.log('[targets] ' + message);
  }
}

function toSuperscript(n) {
  return String(n).split('').map(function (d) {
    return SUPERSCRIPT_DIGITS[Number(d)];
  }).join('');
}

function sanitizeAlias(value) {
  return String(value == null ? '' : value)
    .replace(/#/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

function parsePort(raw, ctx) {
  var port = Number(raw);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error('invalid port: ' + ctx);
  }
  return port;
}

function isLikelyIPv6(host) {
  if (!/^[0-9a-fA-F:.]+$/.test(host)) return false;
  if (host.indexOf('::') !== -1) return true;
  return host.split(':').length === 8;
}

function splitHostPort(endpoint) {
  if (endpoint.charCodeAt(0) === 0x5b) {
    var m = endpoint.match(IPV6_RE);
    if (!m) throw new Error('invalid IPv6: ' + endpoint);
    return { host: m[1], port: m[2] ? parsePort(m[2], endpoint) : undefined };
  }

  var i = endpoint.indexOf(':');
  if (i === -1) return { host: endpoint, port: undefined };

  if (endpoint.indexOf(':', i + 1) !== -1) {
    if (isLikelyIPv6(endpoint)) return { host: endpoint, port: undefined };
    throw new Error('invalid endpoint: ' + endpoint);
  }

  return {
    host: endpoint.slice(0, i).trim(),
    port: parsePort(endpoint.slice(i + 1), endpoint),
  };
}

function parseTargetLine(line) {
  var i = line.indexOf('#');
  if (i === -1) throw new Error('bad format: ' + line + ' (expect host[:port]#alias)');

  var endpoint = line.slice(0, i).trim();
  var alias = sanitizeAlias(line.slice(i + 1));

  if (!endpoint) throw new Error('missing host: ' + line);
  if (!alias) throw new Error('missing alias: ' + line);

  var hp = splitHostPort(endpoint);
  var host = String(hp.host || '').trim();
  if (!host || /\s/.test(host)) {
    throw new Error('invalid host: ' + line);
  }

  return { host: host, port: hp.port, alias: alias };
}

function parseStaticTargets(list) {
  var out = [];
  if (!Array.isArray(list)) return out;

  for (var i = 0; i < list.length; i++) {
    var line = String(list[i] == null ? '' : list[i]).trim();
    if (!line || line.charAt(0) === '#') continue;
    try {
      out.push(parseTargetLine(line));
    } catch (e) {
      warn('skip invalid static target "' + line + '": ' + ((e && e.message) || e));
    }
  }

  return out;
}

function stripHtml(s) {
  return String(s || '').replace(/<.*?>/g, '').trim();
}

function parseWetestCloudflareText(text) {
  var out = [];
  var html = String(text || '');
  var rowRe = /<tr[\s\S]*?<\/tr>/g;
  var cellRe = /<td data-label="线路名称">([\s\S]*?)<\/td>[\s\S]*?<td data-label="优选地址">([\d.:a-fA-F]+)<\/td>[\s\S]*?<td data-label="数据中心">([\s\S]*?)<\/td>/;

  var operators = Array.isArray(CONFIG.wetestOperators)
    ? CONFIG.wetestOperators
    : ['电信', '联通', '移动'];

  var m;
  while ((m = rowRe.exec(html)) !== null) {
    var row = m[0];
    var cells = row.match(cellRe);
    if (!cells) continue;

    var isp = stripHtml(cells[1]);
    var ip = String(cells[2] || '').trim();
    var colo = stripHtml(cells[3]);

    if (!ip) continue;

    var matched = false;
    for (var i = 0; i < operators.length; i++) {
      if (isp.indexOf(operators[i]) !== -1) {
        matched = true;
        break;
      }
    }
    if (!matched) continue;

    out.push({
      host: ip,
      port: undefined,
      alias: sanitizeAlias('CF优选-' + isp + (colo ? '-' + colo : '')),
    });
  }

  return out;
}

function parseTargetText(text) {
  var raw = String(text || '');

  if (raw.indexOf('data-label="线路名称"') !== -1 &&
      raw.indexOf('data-label="优选地址"') !== -1) {
    return parseWetestCloudflareText(raw);
  }

  var out = [];
  var lines = raw.split(LF);

  for (var k = 0; k < lines.length; k++) {
    var line = lines[k].trim();
    if (!line) continue;
    if (line.charAt(0) === '#') continue;
    if (line.indexOf('#') === -1) continue;

    try {
      out.push(parseTargetLine(line));
    } catch (e) {
      warn('skip invalid target line "' + line + '": ' + ((e && e.message) || e));
    }
  }

  return out;
}

function targetKey(target) {
  return String(target.host || '').toLowerCase() + '\u0000' +
    (target.port === undefined ? '' : String(target.port)) + '\u0000' +
    String(target.alias || '');
}

function dedupeTargets(targets) {
  var seen = Object.create(null);
  var out = [];

  for (var i = 0; i < targets.length; i++) {
    var key = targetKey(targets[i]);
    if (seen[key]) continue;
    seen[key] = true;
    out.push(targets[i]);
  }

  return out;
}

var HAS_URLS = Array.isArray(CONFIG.urls) && CONFIG.urls.some(function (u) {
  return String(u || '').trim();
});

if (!parseStaticTargets(CONFIG.targets).length && !HAS_URLS) {
  throw new Error('CONFIG.targets and CONFIG.urls cannot both be empty');
}

function injectAlias(name, alias) {
  if (!name) return alias;

  var i = name.indexOf('_');
  return i === -1
    ? name + '-' + alias
    : name.slice(0, i) + '-' + alias + name.slice(i);
}

function cloneWithTarget(proxy, target) {
  var c = deepClone(proxy);

  c.server = target.host;
  if (target.port !== undefined) c.port = target.port;
  c.name = injectAlias(proxy.name, target.alias);

  return c;
}

function dedupAliases(targets) {
  var counts = new Map();

  for (var i = 0; i < targets.length; i++) {
    var a = targets[i].alias;
    counts.set(a, (counts.get(a) || 0) + 1);
  }

  // 已有别名全部保留，后缀只选未被占用的值，避免与静态 "X⁰" 之类撞名
  var used = new Set(targets.map(function (t) { return t.alias; }));
  var idx = new Map();

  return targets.map(function (t) {
    if ((counts.get(t.alias) || 0) <= 1) return t;

    var j = idx.get(t.alias) || 0;
    var alias;
    do {
      alias = t.alias + toSuperscript(j);
      j++;
    } while (used.has(alias));
    idx.set(t.alias, j);
    used.add(alias);

    return {
      host: t.host,
      port: t.port,
      alias: alias,
    };
  });
}

function getSubstoreHTTP() {
  return (typeof $substore !== 'undefined' && $substore) ? $substore :
    (typeof $ !== 'undefined' && $) ? $ : null;
}

function hasHTTPClient() {
  var api = getSubstoreHTTP();
  if (api && api.http && typeof api.http.get === 'function') return true;
  return typeof fetch === 'function';
}

async function httpGetText(url) {
  var $api = getSubstoreHTTP();

  if ($api && $api.http && typeof $api.http.get === 'function') {
    var res = await $api.http.get({ url: url, timeout: CONFIG.urlTimeout });
    if (!res) return '';
    if (res.body !== undefined) return res.body;
    if (res.data !== undefined) return res.data;
    if (res.rawBody !== undefined) return res.rawBody;
    return '';
  }

  if (typeof fetch === 'function') {
    var controller = typeof AbortController === 'function' ? new AbortController() : null;
    var timer = null;
    if (controller && CONFIG.urlTimeout > 0) {
      timer = setTimeout(function () {
        controller.abort();
      }, CONFIG.urlTimeout);
    }

    try {
      var r = await fetch(url, controller ? { signal: controller.signal } : undefined);
      if (r && r.ok === false) {
        throw new Error('HTTP ' + (r.status || 'error'));
      }
      return await r.text();
    } finally {
      if (timer) clearTimeout(timer);
    }
  }

  throw new Error('no HTTP client available in this runtime');
}

async function fetchTargetsFromUrls(urls) {
  if (!hasHTTPClient()) {
    throw new Error('no HTTP client available in this runtime (need $substore.http.get or fetch)');
  }

  var list = await Promise.all(urls.map(async function (u) {
    try {
      var text = await httpGetText(u);
      return parseTargetText(text);
    } catch (e) {
      warn('fetch failed: ' + u + ' - ' + ((e && e.message) || e));
      return [];
    }
  }));

  var result = [];

  for (var i = 0; i < list.length; i++) {
    for (var j = 0; j < list[i].length; j++) {
      result.push(list[i][j]);
    }
  }

  return result;
}

async function operator(proxies, targetPlatform, context) {
  if (!proxies) proxies = [];
  if (!Array.isArray(proxies)) return proxies;

  var targets = parseStaticTargets(CONFIG.targets);

  var urls = (Array.isArray(CONFIG.urls) ? CONFIG.urls : [])
    .map(function (u) { return String(u || '').trim(); })
    .filter(Boolean);

  if (urls.length) {
    var fetched = await fetchTargetsFromUrls(urls);
    for (var i = 0; i < fetched.length; i++) {
      targets.push(fetched[i]);
    }
  }

  if (!targets.length) return proxies;

  targets = dedupAliases(dedupeTargets(targets));

  var keep = Boolean(CONFIG.keepOriginal);
  var perProxy = targets.length + (keep ? 1 : 0);
  var output = new Array(proxies.length * perProxy);
  var n = 0;

  for (var p = 0; p < proxies.length; p++) {
    var proxy = proxies[p];

    if (!proxy || !proxy.server) {
      output[n++] = proxy;
      continue;
    }

    if (keep) output[n++] = proxy;

    for (var t = 0; t < targets.length; t++) {
      output[n++] = cloneWithTarget(proxy, targets[t]);
    }
  }

  output.length = n;
  return output;
}

// 仅 Node 测试环境导出，Sub-Store 运行时没有 module。
if (typeof module !== 'undefined' && module && module.exports) {
  module.exports = {
    CONFIG: CONFIG,
    sanitizeAlias: sanitizeAlias,
    parseTargetLine: parseTargetLine,
    parseStaticTargets: parseStaticTargets,
    parseWetestCloudflareText: parseWetestCloudflareText,
    parseTargetText: parseTargetText,
    dedupeTargets: dedupeTargets,
    dedupAliases: dedupAliases,
    httpGetText: httpGetText,
    fetchTargetsFromUrls: fetchTargetsFromUrls,
    operator: operator,
  };
}
