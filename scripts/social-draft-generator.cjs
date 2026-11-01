#!/usr/bin/env node
'use strict';

// Human-reviewed, OFFLINE-only copy preparation for publicly released
// Koschei dossiers. Never sends HTTP, Discord webhooks or X API requests.
const fs = require('node:fs');

const PUBLIC_ORIGINS = new Set([
  'https://tradepigloball.co',
  'https://www.tradepigloball.co'
]);
const CASE_REF = /^KD1-[a-f0-9]{32}$/;
const SCHEMA = 'koschei-social-draft-v1';

function plainText(value, label, maxLength) {
  if (typeof value !== 'string') throw new Error(label + ' must be text');
  const text = value.normalize('NFC').trim();
  if (text.length < 8 || [...text].length > maxLength ||
      /[\u0000-\u001f\u007f<>@\u202a-\u202e\u2066-\u2069]/u.test(text) ||
      /@(everyone|here)\b/i.test(text) ||
      /\b(?:bearer\s+[a-z0-9]|api[_ -]?key\s*[:=]|private\s+key|seed\s+phrase|password\s*[:=])/i.test(text) ||
      /[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}/.test(text)) {
    throw new Error(label + ' contains unsafe or unreviewable content');
  }
  return text;
}

function validDossierURL(raw, caseRef) {
  if (typeof raw !== 'string') throw new Error('report_url required');
  let url;
  try { url = new URL(raw); } catch { throw new Error('invalid report_url'); }
  if (!PUBLIC_ORIGINS.has(url.origin) ||
      url.pathname !== '/dossier/' + caseRef ||
      url.search || url.hash || url.username || url.password ||
      url.href !== raw) {
    throw new Error('report_url must be an exact public Koschei dossier URL');
  }
  return url.href;
}

function fieldList(input, name, count, maxLength) {
  const items = input[name];
  if (!Array.isArray(items) || items.length < 1 || items.length > count) {
    throw new Error(name + ' must contain one to ' + count + ' reviewed statements');
  }
  return items.map((item, index) => plainText(item, name + '[' + index + ']', maxLength));
}

function generateDrafts(input) {
  if (!input || typeof input !== 'object' || Array.isArray(input) ||
      input.schema_version !== SCHEMA) {
    throw new Error('unsupported editorial input schema');
  }
  if (typeof input.case_ref !== 'string' || !CASE_REF.test(input.case_ref)) {
    throw new Error('canonical public dossier case_ref required');
  }
  if (input.publication_status !== 'public' ||
      input.owner_publication_checked !== true ||
      input.editorial_review !== 'approved_for_draft') {
    throw new Error('explicit public publication and human editorial review required');
  }
  const title = plainText(input.title, 'title', 72);
  const facts = fieldList(input, 'supported_observations', 3, 120);
  const limitations = fieldList(input, 'limitations', 3, 160);
  const url = validDossierURL(input.report_url, input.case_ref);

  // Avoid moral accusations and unverifiable offline classification:
  // only explicitly reviewed observations are carried into copy.
  const x = [
    'Koschei ARVIS | ' + title,
    facts[0],
    url,
    'On-chain signals only; see limitations in dossier.'
  ].join('\n');
  const discord = [
    '**Koschei ARVIS — Public Investigation Dossier**',
    '**' + title + '**',
    'Case reference: ' + input.case_ref,
    '',
    '**Documented observations (source: public dossier)**',
    ...facts.map(fact => '• ' + fact),
    '',
    '**Important limitations**',
    ...limitations.map(limit => '• ' + limit),
    '',
    'Evidence and verification: ' + url,
    '',
    'On-chain analysis does not establish real-world identity, intent or wrongdoing.'
  ].join('\n');

  if ([...x].length > 280) {
    throw new Error('X draft exceeds 280 characters; shorten the reviewed title/observation instead of truncating evidence');
  }
  if ([...discord].length > 2000) {
    throw new Error('Discord draft exceeds 2000 characters');
  }
  return {
    schema_version: SCHEMA,
    status: 'draft_only',
    case_ref: input.case_ref,
    requires_independent_publication_check: true,
    requires_human_posting_approval: true,
    publication_performed: false,
    platforms: { x, discord }
  };
}

function main(argv) {
  if (argv.length !== 3 || argv[0] !== '--input' || argv[2] !== '--dry-run') {
    throw new Error('usage: node scripts/social-draft-generator.cjs --input reviewed-case.json --dry-run');
  }
  const stat = fs.statSync(argv[1]);
  if (!stat.isFile() || stat.size > 16384) {
    throw new Error('input must be a local JSON file up to 16 KiB');
  }
  const input = JSON.parse(fs.readFileSync(argv[1], 'utf8'));
  process.stdout.write(JSON.stringify(generateDrafts(input), null, 2) + '\n');
}

if (require.main === module) {
  try {
    main(process.argv.slice(2));
  } catch (err) {
    process.stderr.write('social draft rejected: ' + err.message + '\n');
    process.exitCode = 1;
  }
}

module.exports = { generateDrafts, validDossierURL, SCHEMA };
