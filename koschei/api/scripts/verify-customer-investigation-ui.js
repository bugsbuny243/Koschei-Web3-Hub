const fs = require('fs');
const source = fs.readFileSync('public/js/security-radar-detail.js', 'utf8');
const required = [
  'normalizeCustomerInvestigation',
  'envelope?.investigation_report',
  'renderDetail(directReport',
  'lpControlPanel(data.lp_control)',
  'liveEvidencePanel(data.full_scan_live_evidence)',
  'behaviorPanel(data.behavior_signals)',
  'UNIFIED GRADE',
  'EVIDENCE PENDING'
];
for (const marker of required) {
  if (!source.includes(marker)) throw new Error(`missing customer investigation UI marker: ${marker}`);
}
const postBlock = source.slice(source.indexOf("api('/api/v1/radar/check'"), source.indexOf('async function boot'));
if (!postBlock.includes('data.investigation_report') && !postBlock.includes('normalizeCustomerInvestigation(data, target)')) {
  throw new Error('POST response is not consumed as an investigation report');
}
if (postBlock.indexOf('renderDetail(directReport') > postBlock.indexOf('await openDetail(target, item)')) {
  throw new Error('direct investigation rendering must precede legacy detail fallback');
}
console.log('customer investigation UI contract verified');
