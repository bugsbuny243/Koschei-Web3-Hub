#!/usr/bin/env node
'use strict';

// CI contract for the isolated, stateless developer Compose profile.
// Deliberately fails if a production database, billing credential or
// privileged container is introduced into the local smoke service.
const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');

const rendered = execFileSync('docker', [
  'compose', '-f', 'compose.yaml', 'config', '--format', 'json'
], { encoding: 'utf8', timeout: 20000 });
const cfg = JSON.parse(rendered);
assert.deepEqual(Object.keys(cfg.services || {}).sort(), ['api']);
const api = cfg.services.api;
assert.equal(api.environment.APP_ENV, 'development');
assert.equal(api.environment.PORT, '8080');
assert.equal(api.environment.KOSCHEI_RUNTIME_ROLE, 'api');
assert.equal(api.environment.CACHE_ENABLED, 'false');
assert.equal(api.environment.WEB3_WORKER_ENABLED, 'false');
assert.equal(api.environment.KOSCHEI_X402_ENABLED, 'false');
assert.equal(api.environment.KOSCHEI_AUTOMATIC_SCANNING_ENABLED, 'false');
assert.equal(api.read_only, true);
assert.equal(api.privileged || false, false);
assert.equal(api.network_mode || '', '');
assert.ok((api.cap_drop || []).includes('ALL'));
assert.ok((api.security_opt || []).includes('no-new-privileges:true'));

for (const field of ['env_file', 'volumes', 'secrets', 'configs', 'devices']) {
  assert.ok(!api[field] || api[field].length === 0,
    'local API must not configure ' + field);
}
for (const name of Object.keys(api.environment || {})) {
  assert.ok(!/^(APP_DATABASE_URL|APP_DATABASE_READ_URL|ENTITLEMENT_DATABASE_URL|DATABASE_URL|DATABASE_READ_URL|REDIS_URL|NATS_URL|POLAR_|STRIPE_|SHOPIER_|PAYMENT_|API_KEY_PEPPER|USER_SESSION_SECRET|OWNER_SECRET|NEON_AUTH_)/.test(name),
    'local smoke must not pass production or billing environment: ' + name);
}
assert.equal(api.ports.length, 1);
assert.equal(api.ports[0].host_ip, '127.0.0.1');
assert.equal(Number(api.ports[0].target), 8080);
assert.equal(api.build.dockerfile, 'Dockerfile');
console.log('PASS local Compose: one localhost-only stateless API, no billing or persisted DB');
