'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'koschei-auth.js'), 'utf8');
const token = ['head', Buffer.from(JSON.stringify({ exp: 4102444800 })).toString('base64url'), 'signature'].join('.');

function setup({ next = '', remoteOK = true, remoteThrows = false } = {}) {
  const store = new Map();
  const requests = [];
  const location = {
    origin: 'https://koschei.example',
    pathname: '/dashboard',
    search: next ? '?next=' + encodeURIComponent(next) : '',
    hash: '',
    href: '',
    replaced: '',
    replace(value) { this.replaced = value; },
  };
  const storage = {
    getItem: key => store.get(key) ?? null,
    setItem: (key, value) => store.set(key, String(value)),
    removeItem: key => store.delete(key),
  };
  const response = (body, ok = true, status = 200) => ({
    ok, status,
    headers: { get: () => null },
    json: async () => body,
    text: async () => JSON.stringify(body),
  });
  async function fakeFetch(input, init = {}) {
    const url = typeof input === 'string' ? input : input.url;
    requests.push({ url, init });
    if (url === '/api/config') return response({ neonAuthUrl: 'https://neon.example' });
    if (url === '/api/me') return response({ email: 'member@example.org' });
    if (url.endsWith('/sign-out')) {
      if (remoteThrows) throw new Error('network unavailable');
      return response({}, remoteOK, remoteOK ? 200 : 503);
    }
    if (url.endsWith('/sign-in/email')) return response({ token });
    if (url.endsWith('/token') || url.endsWith('/get-session')) return response({ token });
    return response({}, false, 404);
  }
  const context = {
    localStorage: storage,
    location,
    document: { title: 'Test' },
    Headers,
    URL,
    URLSearchParams,
    atob,
    fetch: fakeFetch,
  };
  context.window = context;
  vm.runInNewContext(source, context, { filename: 'koschei-auth.js' });
  return { auth: context.KoscheiAuth, store, requests, location };
}

test('nextPath rejects URL authority tricks and accepts a local route', () => {
  for (const value of [
    '/\\outside.example.invalid',
    '//outside.example.invalid',
    '/%5coutside.example.invalid',
    '/%2foutside.example.invalid',
    'https://outside.example.invalid/',
    '/login?next=/dashboard',
  ]) {
    const { auth } = setup({ next: value });
    assert.equal(auth.nextPath('/'), '/', value);
  }
  const { auth } = setup({ next: '/dashboard?view=alerts#open' });
  assert.equal(auth.nextPath('/'), '/dashboard?view=alerts#open');
});

test('signOut attempts Neon cookie termination before redirect and blocks silent restore', async () => {
  const { auth, store, requests, location } = setup();
  store.set('koschei_jwt', token);
  await auth.signOut();
  const signout = requests.find(x => x.url === 'https://neon.example/sign-out');
  assert.ok(signout, 'Neon sign-out must be called');
  assert.equal(signout.init.method, 'POST');
  assert.equal(signout.init.credentials, 'include');
  assert.equal(store.has('koschei_jwt'), false);
  assert.equal(store.get('koschei_explicitly_signed_out'), '1');
  assert.equal(location.replaced, '/login.html?signed_out=1');
  const count = requests.length;
  assert.equal(await auth.init(), false);
  assert.equal(await auth.restoreNeonSession(), false);
  assert.equal(requests.length, count, 'signed-out client must never fetch a new session');
});

test('failed remote sign-out remains fail closed rather than quietly restoring', async () => {
  const { auth, store, requests, location } = setup({ remoteOK: false });
  await auth.signOut();
  assert.equal(store.get('koschei_explicitly_signed_out'), '1');
  assert.equal(location.replaced, '/login.html?signed_out=local_only');
  const count = requests.length;
  assert.equal(await auth.init(), false);
  assert.equal(requests.length, count);
});

test('explicit successful sign-in clears local logout state', async () => {
  const { auth, store } = setup({ remoteThrows: true });
  await auth.signOut();
  assert.equal(store.get('koschei_explicitly_signed_out'), '1');
  await auth.signIn('member@example.org', 'not-a-real-password');
  assert.equal(store.has('koschei_explicitly_signed_out'), false);
  assert.equal(await auth.init(), true);
});
