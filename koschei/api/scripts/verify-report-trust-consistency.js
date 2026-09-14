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

// Investigation/report evidence contract. Compatibility parsing may remain in
// legacy controllers, but the canonical customer surface must keep evidence
// maturity, authority and read-only boundaries explicit.
need('public/js/public-solana-scan.js', 'Pending evidence arms and monitoring windows');
need('public/js/public-solana-scan.js', 'Missing evidence = no safety decision');
need('public/js/public-solana-scan.js', '/api/public/transaction-simulate');
need('public/js/lp-control-evidence-card.js', 'Havuz hareket geçmişi bu taramada doğrulanamadı');
need('public/scan.html', 'INTELLIGENCE DESK');
need('public/scan.html', 'Investigate an address.');
need('public/scan.html', 'Every material state keeps its source boundary: verified, observed, unresolved, unavailable and not applicable are not collapsed into one score.');
need('public/scan.html', 'Unavailable or incomplete evidence cannot silently improve a risk outcome.');
need('public/scan.html', 'Transaction simulation never signs or broadcasts.');
need('public/scan.html', 'No wallet connection, signing or private keys.');
need('public/scan.html', 'data-scan-mode="token"');
need('public/scan.html', 'data-scan-mode="transaction"');
need('public/scan.html', 'data-scan-mode="deep"');
need('public/scan.html', '/css/koschei.css?v=1');
need('public/scan.html', 'Evidence first · Unknown stays unknown');
reject('public/scan.html', 'data-scan-mode="quick"');
reject('public/scan.html', 'Professional+');
reject('public/scan.html', 'Free Quick Check execution has been removed');

// Homepage trust contract follows the current evidence-led intelligence surface.
// Protect product truth and security boundaries instead of retired marketing copy.
need('public/index.html', 'Koschei Web3 | Security Intelligence');
need('public/index.html', 'Evidence-led Web3 security intelligence');
need('public/index.html', 'Security decisions,');
need('public/index.html', 'with proof attached.');
need('public/index.html', 'What is observed stays observed. What is unknown stays unknown.');
need('public/index.html', '<strong>Read-only</strong>');
need('public/index.html', '<strong>Fail-closed</strong>');
need('public/index.html', '<strong>No custody</strong>');
need('public/index.html', 'Illustrative structure · not live telemetry');
need('public/index.html', 'UNKNOWN ≠ SAFE');
need('public/index.html', 'Address Intelligence');
need('public/index.html', 'Authority Intelligence');
need('public/index.html', 'Approval Intelligence');
need('public/index.html', 'Transaction Preflight');
need('public/index.html', 'Trust Vector');
need('public/index.html', 'Observed is not verified');
need('public/index.html', 'Permission is not safety');
need('public/index.html', 'Absence is not proof');
need('public/index.html', 'Production where proven.');
need('public/index.html', 'Expansion where honest.');
need('public/index.html', 'Solana');
need('public/index.html', 'Production core');
need('public/index.html', 'PROBE READY');
need('public/index.html', '/css/koschei-home.css?v=2');
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
reject('public/index.html', '100% secure');

console.log('Canonical investigation, report and homepage trust consistency contract verified');
