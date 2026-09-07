const fs = require('fs');

function read(file) {
  return fs.readFileSync(file, 'utf8');
}

function need(file, text) {
  const body = read(file);
  if (!body.includes(text)) throw new Error(`${file} missing ${text}`);
}

function reject(file, text) {
  const body = read(file);
  if (body.includes(text)) throw new Error(`${file} contains retired ${text}`);
}

// Evidence renderer assets are retained until their remaining compatibility
// consumers are audited, but the classic scan page itself is no longer a
// customer product surface.
need('public/js/public-solana-scan.js', 'Pending evidence arms and monitoring windows');
need('public/js/public-solana-scan.js', 'Missing evidence = no safety decision');
need('public/js/public-solana-scan.js', '/api/public/transaction-simulate');
need('public/js/lp-control-evidence-card.js', 'Havuz hareket geçmişi bu taramada doğrulanamadı');
if (fs.existsSync('public/scan.html')) throw new Error('retired classic scan page returned');
for (const retired of [
  'public/js/customer-transaction-preflight-v1.js',
  'public/js/customer-investigation-ux-v2.js',
  'scripts/verify-customer-transaction-preflight-ui-v1.js',
  'scripts/verify-customer-investigation-ux-v2.js',
]) {
  if (fs.existsSync(retired)) throw new Error(`retired scan-only artifact returned: ${retired}`);
}
need('internal/http/static_aliases.go', '"/scan.html"');
need('internal/http/static_aliases.go', '"/dashboard#transaction-preflight"');

// The public product homepage is one clean surface and points customers to the
// single operational customer panel.
need('public/index.html', 'See the blind spot.');
need('public/index.html', 'missing evidence stays unknown');
need('public/index.html', 'Solana live production core');
need('public/index.html', 'Material conclusions stay traceable to evidence');
need('public/index.html', 'STRUCTURE ONLY · NOT LIVE TELEMETRY');
need('public/index.html', 'Customer Panel');
need('public/index.html', '/css/koschei-home.css?v=1');
reject('public/index.html', '/css/koschei.css?v=1');
reject('public/index.html', '/js/koschei-global-shell.js');
reject('public/index.html', '/js/koschei-home-universe-v2.js');
reject('public/index.html', '/js/homepage-preflight-v2.js');
reject('public/index.html', '/js/koschei-security-world.js');
reject('public/index.html', 'Ethereum</b><small>LIVE');
reject('public/index.html', 'TRON</b><small>LIVE');

// Persistence-backed dashboard controls must stay absent while production is
// intentionally stateless. Read-only Solana transaction simulation is a live,
// stateless capability and belongs to the Customer Panel.
need('public/dashboard.html', 'PERSISTENCE OFF');
need('public/dashboard.html', 'Durable history');
need('public/dashboard.html', 'Feedback storage');
need('public/dashboard.html', 'id="transaction-preflight"');
need('public/dashboard.html', 'LIVE · SOLANA MAINNET · READ ONLY');
need('public/dashboard.html', 'NO SIGNING · NO BROADCAST');
reject('public/dashboard.html', 'id="feedbackForm"');
reject('public/dashboard.html', 'id="exposureForm"');
need('public/js/koschei-dashboard.js', '/api/public/transaction-simulate');
need('public/js/koschei-dashboard.js', 'renderPreflightResult');
reject('public/js/koschei-dashboard.js', '/api/analytics/event');
reject('public/js/koschei-dashboard.js', '/api/v1/radar/exposure');
need('public/js/customer-workspace-v2.js', "read('/api/me')");
for (const forbiddenRoute of [
  '/api/auth/premium-access',
  '/api/v1/radar/jobs/',
  '/api/watchlist',
  '/api/watchlist/alerts',
  '/api/v1/radar/exposure',
]) {
  reject('public/js/customer-workspace-v2.js', forbiddenRoute);
}
for (const retired of ['public/exposure-report.html','public/feedback.html','public/security-ecosystem.html','public/token-vesting.html']) {
  if (fs.existsSync(retired)) throw new Error(`retired standalone surface returned: ${retired}`);
}

const homepage = read('public/index.html');
if ((homepage.match(/<link rel="stylesheet"/g) || []).length !== 1) {
  throw new Error('public/index.html must load exactly one stylesheet');
}
const dashboard = read('public/dashboard.html');
if ((dashboard.match(/<link rel="stylesheet"/g) || []).length !== 1) {
  throw new Error('public/dashboard.html must load exactly one stylesheet');
}

console.log('Two-surface product trust and canonical dashboard preflight contract verified');
