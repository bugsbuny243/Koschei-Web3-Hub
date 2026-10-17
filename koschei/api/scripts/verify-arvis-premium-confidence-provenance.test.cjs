'use strict';
const {test}=require('node:test');
const assert=require('node:assert/strict');
const api=require('../public/js/arvis-premium-contract.js');

const signed={grade:'D',verdict:'hard_trigger',signed:true,
  signature:'fixture-ed25519-base64url-value',ruleset_version:'test-rules-v1'};
const baseline={investigation_report:{
  final_verdict:signed,
  holder_cluster:{confidence:'none',status:'insufficient_evidence'},
  investigation_coverage:{capability_total:14,completed:7,evidence_pending:6,not_applicable:1},
}};

test('PYUSD-shaped holder confidence none never becomes verdict confidence',()=>{
  const model=api.normalize(baseline),html=api.renderPremiumCard(baseline);
  assert.equal(model.grade,'D');
  assert.equal(model.signed,true);
  assert.equal(model.confidence,'unknown');
  assert.equal(model.confidenceSource,'no_decision_confidence_provided');
  assert.equal(model.clusterConfidence,'none');
  assert.match(html,/Confidence unknown/);
  assert.match(html,/Decision confidence: unknown/);
  assert.match(html,/Holder-cluster confidence none/);
  assert.doesNotMatch(html,/Decision confidence: none/);
  assert.match(html,/browser does not independently verify this verdict/);
  assert.match(html,/Holder-cluster confidence never substitutes/);
});

test('canonical analysis summary confidence overrides narrow holder evidence confidence',()=>{
  const report={...baseline.investigation_report,
    analysis_summary:{decision:{confidence:'low',readiness:'evidence_pending'}}};
  const model=api.normalize({investigation_report:report}),html=api.renderPremiumCard({investigation_report:report});
  assert.equal(model.confidence,'low');
  assert.equal(model.confidenceSource,'analysis_summary.decision.confidence');
  assert.equal(model.clusterConfidence,'none');
  assert.match(html,/Confidence low/);
  assert.match(html,/Decision confidence: low/);
  assert.match(html,/Holder-cluster confidence none/);
});

test('explicit verdict confidence is preserved without borrowing cluster confidence',()=>{
  const report={...baseline.investigation_report,
    final_verdict:{...signed,confidence:'medium'}};
  const model=api.normalize({investigation_report:report});
  assert.equal(model.confidence,'medium');
  assert.equal(model.confidenceSource,'final_verdict.confidence');
});

test('signature string alone cannot promote signed status',()=>{
  const payload={investigation_report:{final_verdict:{grade:'D',signed:false,signature:'orphan-signature'}}};
  const model=api.normalize(payload),html=api.renderPremiumCard(payload);
  assert.equal(model.signed,false);
  assert.match(html,/Signed proof unavailable/);
  assert.match(html,/No complete signed-verdict envelope/);
});

test('bare signed flag without signature cannot claim authenticated verdict',()=>{
  const payload={investigation_report:{final_verdict:{grade:'D',signed:true,signature:''}}};
  assert.equal(api.normalize(payload).signed,false);
  assert.match(api.renderPremiumCard(payload),/Signed proof unavailable/);
});

test('presentation does not turn supplied confidence text into HTML',()=>{
  const payload={investigation_report:{final_verdict:{...signed,confidence:'<img src=x onerror=alert(1)>'}}};
  const html=api.renderPremiumCard(payload);
  assert.ok(!html.includes('<img src=x onerror=alert(1)>'));
  assert.match(html,/&lt;img/);
});
