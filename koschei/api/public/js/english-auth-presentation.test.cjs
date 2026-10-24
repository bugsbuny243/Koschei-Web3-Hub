'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const authSource = fs.readFileSync(path.join(__dirname, 'koschei-auth.js'), 'utf8');
const overlaySource = fs.readFileSync(path.join(__dirname, 'english-auth-presentation.js'), 'utf8');
// Test fixture token; the backend endpoints below are mocked.
const token = ['head', Buffer.from(JSON.stringify({ exp: 4102444800 })).toString('base64url'), 'signature'].join('.');
const SIGNED_OUT_KEY = 'koschei_explicitly_signed_out';

function page({ store, search = '', meOK = true, withOverlay = true }) {
  const location = {
    origin: 'https://koschei.example',
    pathname: '/login.html',
    search,
    hash: '',
    href: '',
    replace() {},
  };
  const response = (body, ok = true, status = 200) => ({
    ok, status,
    headers: { get: () => null },
    json: async () => body,
    text: async () => JSON.stringify(body),
  });
  async function fakeFetch(input) {
    const url = typeof input === 'string' ? input : input.url;
    if (url === '/api/config') return response({ neonAuthUrl: '' });
    if (url === '/api/me') return meOK ? response({ email: 'member@example.org' }) : response({ error: 'unauthorized' }, false, 401);
    if (url === '/api/auth/login') return response({ token });
    return response({}, false, 404);
  }
  const context = {
    localStorage: {
      getItem: key => store.get(key) ?? null,
      setItem: (key, value) => store.set(key, String(value)),
      removeItem: key => store.delete(key),
    },
    location,
    document: {
      title: 'Test',
      readyState: 'complete',
      body: null,
      documentElement: {},
      querySelectorAll: () => [],
      addEventListener() {},
    },
    history: { replaceState() {} },
    MutationObserver: class { observe() {} },
    Node: { TEXT_NODE: 3, ELEMENT_NODE: 1 },
    Headers,
    URL,
    URLSearchParams,
    Response,
    atob,
    fetch: fakeFetch,
    queueMicrotask,
    setTimeout,
    clearTimeout,
  };
  context.window = context;
  vm.runInNewContext(authSource, context, { filename: 'koschei-auth.js' });
  if (withOverlay) vm.runInNewContext(overlaySource, context, { filename: 'english-auth-presentation.js' });
  return context.KoscheiAuth;
}

test('a verified sign-in after logout survives the next page load', async () => {
  const store = new Map([[SIGNED_OUT_KEY, '1']]);
  const login = page({ store, search: '?signed_out=1' });
  await login.signIn('member@example.org', 'fixture-password');
  assert.equal(store.has(SIGNED_OUT_KEY), false);

  const dashboard = page({ store, withOverlay: false });
  assert.equal(await dashboard.init(), true);
  assert.equal(dashboard.isLoggedIn(), true);
});

test('an unverified sign-in keeps the earlier logout in force', async () => {
  const store = new Map([[SIGNED_OUT_KEY, '1']]);
  const login = page({ store, meOK: false });
  await assert.rejects(login.signIn('member@example.org', 'fixture-password'));
  assert.equal(store.get(SIGNED_OUT_KEY), '1');
  assert.equal(store.has('koschei_jwt'), false);
});
