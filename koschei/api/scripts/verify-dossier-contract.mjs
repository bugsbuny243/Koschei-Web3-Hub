import { createHash } from 'node:crypto';
import { DOSSIER_ROW_IDS, dossierCaseRef, verifyDossierObject } from '../../../oss/verifier/typescript/verify-dossier.mjs';

const mint = 'Mint111';
const signature = 'VerdictSignature111';
const caseRef = dossierCaseRef(mint, signature);
const rows = DOSSIER_ROW_IDS.map(id => ({
  id,
  label: id,
  state: id === 'claim' || id === 'mev' ? 'not_applicable' : 'verified',
  value: { observed: true },
  refs: {
    wallets: id === 'concentration' ? ['Owner111'] : [],
    accounts: ['Mint111'],
    signatures: id === 'signed' ? [signature] : [],
    slots: id === 'launch' ? [100] : [],
    evidence_keys: [`row:${id}`]
  }
}));

const body = {
  dossier_version: 'koschei-dossier-v1',
  case_ref: caseRef,
  produced_at: '2026-07-17T08:00:00Z',
  source_snapshot_hash: `sha256:${'a'.repeat(64)}`,
  token: { mint, network: 'solana-mainnet' },
  verdict: { grade: 'F', signed: true, signature },
  verdict_card: { mapper_id: 'koschei-verdict-card', mapper_version: 'test', signal_rows: rows },
  threat_anticipation: { pathways: [] },
  evidence_arms: [],
  transaction_evidence: [],
  evidence_references: {},
  actor_dossier: {},
  holder_concentration_context: { available: true, sample_count: 50000 },
  technical_report: { target: mint },
  verification: { verdict_signature: signature },
  limitations: ['Capability-not-intent', 'Identity boundary', 'Evidence-window boundary']
};
const hash = `sha256:${createHash('sha256').update(Buffer.from(JSON.stringify(body), 'utf8')).digest('hex')}`;
const bundle = { ...body, bundle_hash: hash };
const valid = verifyDossierObject(bundle);
if (!valid.ok) throw new Error(`valid dossier rejected: ${JSON.stringify(valid)}`);

const tampered = JSON.parse(JSON.stringify(bundle));
tampered.verdict.grade = 'A';
const invalid = verifyDossierObject(tampered);
if (invalid.ok || !invalid.errors.includes('bundle_hash_mismatch')) throw new Error(`tampered dossier accepted: ${JSON.stringify(invalid)}`);

const missingRefs = JSON.parse(JSON.stringify(bundle));
missingRefs.verdict_card.signal_rows.find(row => row.id === 'concentration').refs = { wallets: [], accounts: [], signatures: [], slots: [], evidence_keys: [] };
const missingResult = verifyDossierObject(missingRefs);
if (missingResult.ok || !missingResult.errors.includes('populated_signal_missing_refs:concentration')) throw new Error(`missing refs accepted: ${JSON.stringify(missingResult)}`);

console.log('dossier verifier contract: ok');
