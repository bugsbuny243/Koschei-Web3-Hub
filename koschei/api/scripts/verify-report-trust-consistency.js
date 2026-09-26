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

// Investigation/report evidence contract. The internal controller may retain
// compatibility parsing for old quick-preflight responses, but the customer UI
// no longer exposes free Quick Check execution. Pin the current Intelligence
// Desk evidence and read-only boundaries instead of retired visual copy.
need('public/js/public-solana-scan.js', 'Pending evidence arms and monitoring windows');
need('public/js/public-solana-scan.js', 'Missing evidence = no safety decision');
need('public/js/public-solana-scan.js', '/api/public/transaction-simulate');
need('public/js/lp-control-evidence-card.js', 'Havuz hareket geçmişi bu taramada doğrulanamadı');
need('public/scan.html', 'INTELLIGENCE DESK');
need('public/scan.html', 'Evidence provenance');
need('public/scan.html', 'Unavailable or incomplete evidence cannot silently improve a risk outcome.');
need('public/scan.html', 'Transaction simulation never signs or broadcasts.');
need('public/scan.html', 'data-scan-mode="token"');
need('public/scan.html', 'data-scan-mode="transaction"');
need('public/scan.html', 'data-scan-mode="deep"');
need('public/scan.html', 'data-customer-arvis-result');
need('public/scan.html', '/css/koschei.css?v=1');
reject('public/scan.html', 'data-scan-mode="quick"');
reject('public/scan.html', 'Professional+');

// Homepage trust contract follows the current Security Intelligence surface.
// Protect evidence boundaries and no-custody truth instead of retired visual copy.
need('public/index.html', 'Security decisions,');
need('public/index.html', 'with proof attached.');
need('public/index.html', 'What is observed stays observed. What is unknown stays unknown.');
need('public/index.html', '<strong>Read-only</strong>');
need('public/index.html', '<strong>Fail-closed</strong>');
need('public/index.html', '<strong>No custody</strong><span>no private keys</span>');
need('public/index.html', 'Illustrative structure · not live telemetry');
need('public/index.html', 'UNKNOWN ≠ SAFE');
need('public/index.html', 'Evidence contract');
need('public/index.html', 'Every conclusion keeps its boundary.');
need('public/index.html', 'Observed is not verified');
need('public/index.html', 'Permission is not safety');
need('public/index.html', 'Absence is not proof');
need('public/index.html', 'Coverage without theatre');
need('public/index.html', 'Production core');
need('public/index.html', 'LIVE CORE');
need('public/index.html', 'PROBE READY');
need('public/index.html', '/css/koschei-home.css?v=4');
need('public/index.html', 'Enter Intelligence Console');
need('public/index.html', 'Evidence first · Unknown stays unknown');

// Retired homepage implementations and unsupported production claims stay out.
reject('public/index.html', '/css/koschei-universe-v1.css?v=1');
reject('public/index.html', '/css/koschei-home-universe-v2.css?v=1');
reject('public/index.html', '/js/koschei-home-universe-v2.js?v=1');
reject('public/index.html', '/js/homepage-preflight-v2.js?v=1');
reject('public/index.html', '/js/koschei-security-world.js?v=1');
reject('public/index.html', 'STATIC HTML + VANILLA JS');
reject('public/index.html', 'Ethereum</b><small>LIVE');
reject('public/index.html', 'TRON</b><small>LIVE');

console.log('Intelligence Desk investigation, report and current homepage trust consistency contract verified');
