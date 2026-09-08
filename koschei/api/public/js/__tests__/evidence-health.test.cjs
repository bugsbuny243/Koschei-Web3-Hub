'use strict';

const assert = require('node:assert/strict');
const { readFileSync, readdirSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');

const NOW = Date.parse('2026-09-08T00:00:00Z');
const fresh = (pipeline_status = 'healthy') => ({
  status: 'ok',
  arvis: { pipeline_status, cached: true, cache_expires_at: new Date(NOW + 15000).toISOString() },
});

function element() {
  const classes = new Set();
  return {
    dataset: {}, textContent: '',
    classList: {
      contains: value => classes.has(value),
      add: value => classes.add(value),
      remove: value => classes.delete(value),
      toggle(value, enabled) { if (enabled) classes.add(value); else classes.delete(value); },
    },
    setAttribute(name, value) { this[name] = value; },
    addEventListener() {}, appendChild() {}, querySelector() { return null; },
  };
}

async function mount(script, payload, failure) {
  const indicator = element(), pipeline = element(), row = element(), top = element(), label = element();
  pipeline.closest = () => row;
  top.querySelector = () => label;
  const timers = new Map();
  const requests = [];
  let timerID = 0;
  const context = {
    AbortController, URL,
    Date: class extends Date { static now() { return NOW; } },
    MutationObserver: class { observe() {} },
    location: { pathname: '/test', origin: 'https://example.test' },
    document: {
      readyState: 'complete', body: element(), head: element(),
      getElementById: id => ({ commandPipelineState: pipeline, topStatus: top })[id] || null,
      querySelector: selector => selector.startsWith('link[') || selector === '.customer-mobile-nav-v3' ? element() : null,
      querySelectorAll: selector => selector === '[data-koschei-live]' ? [indicator] : [],
      addEventListener() {}, createElement: element,
    },
    setTimeout(callback, delay) { const id = ++timerID; timers.set(id, { callback, delay }); return id; },
    clearTimeout: id => timers.delete(id),
    fetch: async (url, options) => {
      requests.push({ url, options });
      if (failure === 'network') throw new Error('network unavailable');
      return {
        ok: failure !== 'http', status: failure === 'http' ? 503 : 200,
        json: async () => { if (failure === 'json') throw new Error('invalid JSON'); return typeof payload === 'function' ? payload() : payload; },
      };
    },
  };
  context.window = context;
  vm.runInNewContext(readFileSync(path.join(__dirname, '..', script), 'utf8'), context, { filename: script });
  await new Promise(resolve => setImmediate(resolve));
  return {
    isReady: () => script === 'koschei-dashboard.js' ? top.dataset.state === 'live' : indicator.dataset.koscheiDependencyState === 'ready',
    visibleText: () => script === 'koschei-dashboard.js' ? label.textContent : indicator.textContent,
    timers,
    requests,
  };
}

for (const script of ['koschei-dashboard.js', 'koschei-product-v2.js']) {
  for (const status of ['not_ready', 'unhealthy', 'disconnected', 'live_provider_mode', 'ok', 'ready', 'live', 'manual', 'processing', 'stale', 'degraded', 'unknown']) {
    test(`${script}: ${status} is not verified evidence health`, async () => {
      assert.equal((await mount(script, fresh(status))).isReady(), false);
    });
  }
  for (const [name, payload] of [
    ['liveness only', { status: 'ok' }],
    ['empty response', {}],
    ['null response', null],
    ['missing expiry', { arvis: { pipeline_status: 'healthy', cached: true } }],
    ['invalid expiry', { arvis: { ...fresh().arvis, cache_expires_at: 'invalid' } }],
    ['expired', { arvis: { ...fresh().arvis, cache_expires_at: new Date(NOW - 1).toISOString() } }],
    ['expires now', { arvis: { ...fresh().arvis, cache_expires_at: new Date(NOW).toISOString() } }],
    ['not collected', { arvis: { ...fresh().arvis, cached: false } }],
    ['string boolean', { arvis: { ...fresh().arvis, cached: 'true' } }],
  ]) {
    test(`${script}: ${name} remains unverified`, async () => {
      assert.equal((await mount(script, payload)).isReady(), false);
    });
  }
  for (const status of ['healthy', 'operational']) {
    test(`${script}: fresh ${status} expires without a page reload`, async () => {
      const view = await mount(script, fresh(status));
      assert.equal(view.isReady(), true);
      assert.match(view.visibleText(), /evidence pipeline operational/i);
      assert.doesNotMatch(view.visibleText(), /production.*ready/i);
      assert.equal(view.timers.size, 2, 'only expiry and the next refresh should remain');
      const expiry = [...view.timers.values()][0];
      assert.equal(expiry.delay, 15000);
      expiry.callback();
      assert.equal(view.isReady(), false, 'cached evidence must not stay green indefinitely');
    });
  }
  for (const failure of ['network', 'http', 'json']) {
    test(`${script}: ${failure} failure cannot report readiness`, async () => {
      const view = await mount(script, fresh(), failure);
      assert.equal(view.isReady(), false);
      assert.equal(view.timers.size, 1, 'failed observations should retry without retaining the request timeout');
      assert.equal([...view.timers.values()][0].delay, 15000);
    });
  }
  test(`${script}: requests measured evidence and recovers on refresh`, async () => {
    let response = { status: 'ok' };
    const view = await mount(script, () => response);
    assert.equal(view.isReady(), false);
    assert.equal(view.requests[0].url, '/health?evidence=refresh');
    assert.equal(view.requests[0].options.cache, 'no-store');
    const [id, retry] = [...view.timers.entries()][0];
    assert.equal(retry.delay, 15000);
    view.timers.delete(id);
    response = fresh();
    await retry.callback();
    assert.equal(view.isReady(), true);
    assert.equal(view.requests.length, 2);
  });
}

test('all customer pages use new cache keys for both health scripts', () => {
  const publicDir = path.join(__dirname, '..', '..');
  const assets = new Set();
  for (const filename of readdirSync(publicDir).filter(name => name.endsWith('.html'))) {
    const html = readFileSync(path.join(publicDir, filename), 'utf8');
    for (const match of html.matchAll(/\/js\/(koschei-(?:dashboard|product-v2)\.js)([^"'\s>]*)/g)) {
      assets.add(match[1]);
      assert.equal(match[2], '?v=3', `${filename} still serves an old health script cache key`);
    }
  }
  assert.deepEqual([...assets].sort(), ['koschei-dashboard.js', 'koschei-product-v2.js']);
});
