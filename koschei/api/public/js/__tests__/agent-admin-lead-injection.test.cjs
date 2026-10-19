'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const html = fs.readFileSync(path.resolve(__dirname, '../../agents-admin.html'), 'utf8');
const script = html.match(/<script>([\\s\\S]*?)<\\/script>/)[1];
function renderLead(data) {
  const document = {getElementById(){return {addEventListener(){}};}};
  return vm.runInNewContext(script+'; lead('+JSON.stringify(data)+')', {document});
}
test('public IDs are HTML encoded and never emitted inside event-handler attributes',()=>{
  const payload=`alice' onfocus='saveKnowledge()' x='x`;
  const rendered=renderLead({external_id:payload,channel:'web',display_name:'Alice',stage:'open',score:5});
  assert.doesNotMatch(rendered, /on(?:click|focus|mouseover)\\s*=/i);
  assert.doesNotMatch(rendered, /saveKnowledge\\(\\)/);
  assert.match(rendered, /data-lead-action="assign"/);
  assert.match(rendered, /&#39;/);
  assert.doesNotMatch(rendered, /id="owner-/);
});
test('agent admin HTML has click delegation rather than external-ID inline JS',()=>{
  assert.match(script, /addEventListener\\('click',event/);
  assert.match(script, /button\\.dataset\\.externalId/);
  assert.doesNotMatch(script, /onclick=['"]assignLead/);
});
