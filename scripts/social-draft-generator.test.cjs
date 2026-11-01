'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync, spawnSync } = require('node:child_process');
const { generateDrafts } = require('./social-draft-generator.cjs');

const caseRef = 'KD1-' + 'a'.repeat(32);
function reviewedInput() {
  return {
    schema_version: 'koschei-social-draft-v1',
    case_ref: caseRef,
    publication_status: 'public',
    owner_publication_checked: true,
    editorial_review: 'approved_for_draft',
    title: 'Public on-chain evidence review',
    supported_observations: [
      'The reviewed dossier contains an on-chain event timeline.'
    ],
    limitations: [
      'This analysis does not attribute an event to a real-world person.'
    ],
    report_url: 'https://tradepigloball.co/dossier/' + caseRef
  };
}

test('generates bounded disclosure-first drafts and never claims publication', () => {
  const draft = generateDrafts(reviewedInput());
  assert.equal(draft.status, 'draft_only');
  assert.equal(draft.publication_performed, false);
  assert.equal(draft.requires_human_posting_approval, true);
  assert.equal(draft.requires_independent_publication_check, true);
  assert.ok(draft.platforms.x.includes('/dossier/' + caseRef));
  assert.ok(draft.platforms.discord.includes('Important limitations'));
  assert.ok([...draft.platforms.x].length <= 280);
  assert.ok([...draft.platforms.discord].length <= 2000);
  assert.ok(!/free tier|checkout|stripe|shopier/i.test(draft.platforms.x));
});

test('requires real public publication attestation and human review', () => {
  for (const mutation of [
    v => { v.publication_status = 'draft'; },
    v => { v.publication_status = 'hidden'; },
    v => { v.owner_publication_checked = false; },
    v => { v.editorial_review = 'pending'; },
    v => { v.schema_version = 'unrecognized'; }
  ]) {
    const input = reviewedInput();
    mutation(input);
    assert.throws(() => generateDrafts(input));
  }
});

test('only exact canonical public dossier URLs are accepted', () => {
  for (const url of [
    'http://tradepigloball.co/dossier/' + caseRef,
    'https://evil.example/dossier/' + caseRef,
    'https://tradepigloball.co.evil.example/dossier/' + caseRef,
    'https://tradepigloball.co/dossier/' + 'b'.repeat(32),
    'https://tradepigloball.co/dossier/' + caseRef + '?private=1',
    'https://tradepigloball.co/dossier/' + caseRef + '#hash',
    'https://user:secret@tradepigloball.co/dossier/' + caseRef,
    'https://tradepigloball.co/dossier/' + caseRef + '/more'
  ]) {
    const input = reviewedInput();
    input.report_url = url;
    assert.throws(() => generateDrafts(input), /report_url/);
  }
});

test('rejects mentions, HTML/control characters, secrets and personal email data', () => {
  for (const unsafe of [
    '@everyone urgent action',
    '<script>alert(1)</script>',
    'Send to person@example.com immediately',
    'API_KEY=abcdefgh123456',
    'A public note\nwith injection',
    'seed phrase: one two three'
  ]) {
    const input = reviewedInput();
    input.supported_observations = [unsafe];
    assert.throws(() => generateDrafts(input));
  }
});

test('fails rather than silently truncating X content', () => {
  const input = reviewedInput();
  input.title = 'T'.repeat(72);
  input.supported_observations = ['F'.repeat(120)];
  assert.throws(() => generateDrafts(input), /280 characters/);
});

test('CLI only reads a local file on explicit --dry-run', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'koschei-social-draft-'));
  try {
    const p = path.join(dir, 'reviewed.json');
    fs.writeFileSync(p, JSON.stringify(reviewedInput()), { mode: 0o600 });
    const cli = path.join(__dirname, 'social-draft-generator.cjs');
    const output = JSON.parse(execFileSync(process.execPath, [
      cli, '--input', p, '--dry-run'
    ], { encoding: 'utf8', timeout: 5000 }));
    assert.equal(output.publication_performed, false);
    const attemptedPost = spawnSync(process.execPath, [
      cli, '--input', p, '--post'
    ], { encoding: 'utf8', timeout: 5000 });
    assert.notEqual(attemptedPost.status, 0);
    assert.equal(attemptedPost.stdout, '');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
