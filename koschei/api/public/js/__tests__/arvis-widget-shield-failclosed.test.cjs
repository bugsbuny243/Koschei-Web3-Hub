'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const widgetSource = fs.readFileSync(path.resolve(__dirname, '../../widget.js'), 'utf8');
const shieldSource = fs.readFileSync(path.resolve(__dirname, '../../sdk/koschei-shield.js'), 'utf8');
const tick = () => new Promise(resolve => setImmediate(resolve));

async function widgetScenario(response, scriptAttributes) {
  let html = '';
  let fetched = '';
  const root = {
    setAttribute() {},
    style: {},
    set innerHTML(value) { html = value; },
    get innerHTML() { return html; }
  };
  const script = {
    src: 'https://api.koschei.example/widget.js',
    getAttribute(name) {
      return Object.hasOwn(scriptAttributes || {}, name) ? scriptAttributes[name] : name === 'data-token' ? '11111111111111111111111111111111' : null;
    },
    parentNode: { insertBefore() {} },
    nextSibling: null
  };
  const document = {
    currentScript: script,
    baseURI: 'https://customer.example/page',
    createElement() { return root; },
    getElementsByTagName() { return [script]; }
  };
  vm.runInNewContext(widgetSource, {
    document, URL,
    fetch: async url => {
      fetched = String(url);
      if (response instanceof Error) throw response;
      return { ok: Boolean(response), json: async () => response };
    }
  }, { filename: 'widget.js' });
  await tick(); await tick();
  return { html, fetched };
}

test('widget never fabricates risk grade on network failure', async () => {
  for (const response of [new Error('offline'), null, { ok: false }, {ok:true, signed:false, grade:'A', risk_index:9,risk_level:'low'}]) {
    const { html, fetched } = await widgetScenario(response);
    assert.match(fetched, /^https:\/\/api\.koschei\.example\/api\/v1\/risk\/badge\?/);
    assert.match(html, /Unknown/);
    assert.doesNotMatch(html, /Risk Index [0-9]+\/100/);
    assert.doesNotMatch(html, />Low</);
    assert.doesNotMatch(html, />A</);
  }
});

test('widget keeps authenticated verdict and never calls embedder origin', async () => {
  const { html, fetched } = await widgetScenario({ok:true, signed:true, signature:'sample-signature',grade:'D',risk_index:74,risk_level:'high',verdict:'Rule triggered'});
  assert.match(fetched, /api\.koschei\.example/);
  assert.doesNotMatch(fetched, /customer\.example/);
  assert.match(html, /Risk Index 74\/100/);
  assert.match(html, />D</);
});
test('widget permits explicit HTTPS API origin override', async () => {
  const { fetched } = await widgetScenario(null, {'data-token':'mint', 'data-api-base':'https://custom.koschei.example'});
  assert.match(fetched, /^https:\/\/custom\.koschei\.example\//);
});

async function shieldScenario(response) {
  const browser = {
    location: { origin: 'https://api.koschei.example' },
    fetch: async () => ({ ok: true, json: async () => response })
  };
  const context = { window: browser, fetch: browser.fetch, globalThis: browser };
  vm.runInNewContext(shieldSource, context, {filename:'koschei-shield.js'});
  const client = new browser.KoscheiShield();
  return client.assertSafe({target:'mint'});
}

test('Shield assertSafe is allow-only and refuses unknown and unsigned', async () => {
  const permitted = {ok:true, action:'allow',signed:true,signature:'signature',risk_level:'low',final_policy:'publish'};
  assert.equal((await shieldScenario(permitted)).action, 'allow');
  const untrusted = [
    {ok:true,action:'withhold',signed:false,risk_level:'unknown'},
    {ok:true,action:'block',signed:true,signature:'signature',risk_level:'critical'},
    {ok:true,action:'warn',signed:true,signature:'signature',risk_level:'high'},
    {ok:true,action:'allow',signed:false,signature:'signature',risk_level:'low'},
    {ok:true,action:'allow',signed:true,signature:'',risk_level:'low'},
    {ok:true,action:'allow',signed:true,signature:'signature',risk_level:'unknown'},
    {ok:true,action:'allow',signed:true,signature:'signature',risk_level:'low',final_policy:'withhold'},
    {action:'allow',signed:true,signature:'signature',risk_level:'low'},
    {}
  ];
  for (const verdict of untrusted) await assert.rejects(shieldScenario(verdict), /not authorized/);
});
