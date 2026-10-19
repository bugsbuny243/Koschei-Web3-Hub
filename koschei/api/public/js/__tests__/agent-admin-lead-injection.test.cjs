'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const html = fs.readFileSync(path.resolve(__dirname, '../../agents-admin.html'), 'utf8');
const script = html.split('<script>')[1].split('</script>')[0];

function renderLead(data) {
  const document = { getElementById() { return { addEventListener() {} }; } };
  return vm.runInNewContext(script + '; lead(' + JSON.stringify(data) + ')', { document });
}

test('untrusted lead ID is escaped as data, not executable HTML', () => {
  const payload = "alice' onfocus='saveKnowledge()' x='x";
  const rendered = renderLead({ external_id: payload, channel: 'web', display_name: 'Alice', stage: 'open', score: 5 });
  assert.match(rendered, /data-lead-action="assign"/);
  assert.match(rendered, /data-external-id="alice&#39; onfocus=&#39;saveKnowledge\(\)&#39; x=&#39;x"/);
  assert.doesNotMatch(rendered, /onclick=/);
  assert.doesNotMatch(rendered, /id="owner-/);
});

test('lead actions use delegated listeners and typed dataset values', () => {
  assert.ok(script.includes("addEventListener('click',event"));
  assert.ok(script.includes('button.dataset.externalId'));
  assert.ok(!script.includes("onclick='assignLead("));
});
