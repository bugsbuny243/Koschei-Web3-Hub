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
// no longer exposes free Quick Check execution.
need('public/js/public-solana-scan.js', 'Pending evidence arms and monitoring windows');
need('public/js/public-solana-scan.js', 'Missing evidence = no safety decision');
need('public/js/public-solana-scan.js', '/api/public/transaction-simulate');
need('public/js/lp-control-evidence-card.js', 'Havuz hareket geçmişi bu taramada doğrulanamadı');
need('public/scan.html', 'PROFESSIONAL · CLASSIC INVESTIGATION CONSOLE');
need('public/scan.html', 'Missing evidence is shown as a limitation, not converted into a safety claim.');
need('public/scan.html', 'Transaction simulation never signs or broadcasts.');
need('public/scan.html', 'Free Quick Check execution has been removed');
need('public/scan.html', '/css/koschei.css?v=1');
reject('public/scan.html', 'data-scan-mode="quick"');
reject('public/scan.html', 'Professional+');

// Homepage trust contract follows the current clean homepage introduced by the
// 2026-09-07 surface refactor. The verifier protects product truth and security
// boundaries rather than pinning retired Universe Gateway copy/assets.
need('public/index.html', 'See the blind spot.');
need('public/index.html', 'Before it becomes the attack.');
need('public/index.html', 'Solana live production core');
need('public/index.html', 'missing evidence stays unknown');
need('public/index.html', 'No custody · no private keys');
need('public/index.html', 'One reasoning path.');
need('public/index.html', 'Every claim accountable.');
need('public/index.html', 'ARVIS is not a generic risk-score machine.');
need('public/index.html', 'Attack Path');
need('public/index.html', 'Evidence provenance');
need('public/index.html', 'Protocol Defense Validation');
need('public/index.html', '<span class="state building">Building</span>');
need('public/index.html', 'Cross-chain Intelligence');
need('public/index.html', '<span class="state building">Expansion</span>');
need('public/index.html', 'Solana is the live core; additional chain adapters remain expansion work until their evidence paths are production-ready.');
need('public/index.html', 'STRUCTURE ONLY · NOT LIVE TELEMETRY');
need('public/index.html', '/css/koschei-home.css?v=1');
need('public/index.html', 'Enter Customer Panel');
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

console.log('Professional investigation, report and current homepage trust consistency contract verified');
