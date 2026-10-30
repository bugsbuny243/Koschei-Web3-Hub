/* Owner-only SOC workspace. Server data is always rendered as text. */
(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const node = (tag, text, cls) => { const n = document.createElement(tag); if (text != null) n.textContent = String(text); if (cls) n.className = cls; return n; };
  const date = value => value ? new Date(value).toLocaleString() : 'Unknown';
  let offset = 0, selected = '', detailSequence = 0, queueSequence = 0, queueBusy = false, writing = false, filters = new URLSearchParams({status:'active'});
  const limit = 50;
  async function api(path, body) {
    const response = await fetch(path, {credentials:'same-origin',cache:'no-store',headers:body ? {'Content-Type':'application/json'} : {},method:body ? 'POST' : 'GET',body:body ? JSON.stringify(body) : undefined,signal:AbortSignal.timeout(12000)});
    const data = await response.json().catch(() => ({}));
    if (!response.ok) {
      const message = response.status === 404 || response.status === 401 || response.status === 403 ? 'Owner session required, or case unavailable. Sign in through the owner control center.' : response.status === 409 ? 'Case changed in another session, or is no longer active. Reload it before trying again.' : (data.message || data.error || 'Incident store unavailable.');
      throw new Error(typeof message === 'string' ? message : 'Request failed.');
    }
    return data;
  }
  function notice(message, error = false) { $('caseNotice').textContent = message; $('caseNotice').classList.toggle('error',error); }
  async function queue(force = false) {
    if (queueBusy && !force) return;
    const seq = ++queueSequence; queueBusy = true;
    try {
      const q = new URLSearchParams(filters); q.set('limit',limit); q.set('offset',offset);
      const data = await api('/api/owner/soc/queue?' + q);
      if (seq !== queueSequence) return;
      $('queue').replaceChildren();
      for (const item of data.incidents) {
        const button = node('button',null,'case-row' + (item.id === selected ? ' selected' : '')); button.type = 'button';
        const top = node('div',null,'row-top'); top.append(node('span',item.incident_ref),node('span',item.severity,'badge ' + item.severity));
        const meta = node('div',null,'row-meta'); meta.append(node('span',item.operations.tier + ' · ' + (item.operations.assignee || 'Unassigned')),node('span',item.sla.state,'badge ' + item.sla.state));
        button.append(top,node('strong',item.title),meta,node('span',item.network + ' · ' + item.status,'small muted'));
        button.addEventListener('click',() => { if (writing) return; selected = item.id; document.querySelectorAll('.case-row').forEach(n => n.classList.remove('selected')); button.classList.add('selected'); loadCase(); });
        $('queue').append(button);
      }
      if (!data.incidents.length) $('queue').append(node('p','No incidents match these filters. Create a case from the existing Security page.','empty'));
      $('count').textContent = data.telemetry.cases; $('unassignedCount').textContent = data.telemetry.unassigned;
      $('overdue').textContent = data.telemetry.first_response.overdue; $('breached').textContent = data.telemetry.first_response.breached;
      $('pageLabel').textContent = 'Page ' + (offset / limit + 1); $('previous').disabled = offset === 0; $('next').disabled = !data.has_more;
      $('connection').textContent = 'Updated ' + date(data.as_of) + ' · 15-second queue refresh · displayed-page metrics'; $('connection').classList.remove('error');
    } catch (error) {
      if (seq !== queueSequence) return;
      $('connection').textContent = 'Unavailable / stale — ' + error.message; $('connection').classList.add('error');
      for (const id of ['count','unassignedCount','overdue','breached']) $(id).textContent = '—';
      $('previous').disabled = true; $('next').disabled = true;
    } finally { if (seq === queueSequence) queueBusy = false; }
  }
  function field(label, input) { const e = node('label',label); e.append(input); return e; }
  function select(options) { const e = node('select'); for (const [value,label] of options) { const o = node('option',label); o.value = value; e.append(o); } return e; }
  function fact(label,value) { const e = node('div'); e.append(node('small',label),node('span',value)); return e; }
  function timeline(events, soc) {
    const list = node('ol',null,'timeline');
    for (const event of events) {
      const li = node('li'); const time = node('time',date(event.created_at)); time.dateTime = event.created_at;
      li.append(time,node('p',(event.action || event.action_type) + ' · ' + event.actor),node('p',event.summary || 'No note'));
      if (soc) li.append(node('small','Revision ' + event.version + ' · ' + event.before.tier + ' → ' + event.after.tier + ' · assignee: ' + (event.before.assignee || 'unassigned') + ' → ' + (event.after.assignee || 'unassigned')));
      list.append(li);
    }
    if (!events.length) list.append(node('li','No recorded actions.','muted'));
    return list;
  }
  async function submit(form, path, body) {
    if (writing) return;
    writing = true; form.querySelector('button[type="submit"]').disabled = true;
    try { await api(path,body); await loadCase(); await queue(true); notice('Action saved to the case timeline.'); }
    catch (error) { notice(error.message,true); }
    finally { writing = false; form.querySelector('button[type="submit"]').disabled = false; }
  }
  async function loadCase() {
    if (!selected) return;
    const id = selected, seq = ++detailSequence; notice('Loading case…'); $('reloadCase').hidden = false;
    try {
      const [data, legacy] = await Promise.all([api('/api/owner/soc/case?id=' + encodeURIComponent(id)),api('/api/owner/incidents/' + encodeURIComponent(id))]);
      if (seq !== detailSequence || id !== selected) return;
      const item = data.incident, op = item.operations, root = $('case'); root.replaceChildren();
      root.append(node('h3',item.title,'case-title'),node('p',legacy.incident.summary,'muted'));
      const facts = node('div',null,'facts');
      facts.append(fact('Case / status',item.incident_ref + ' / ' + item.status),fact('Level / assignee',op.tier + ' / ' + (op.assignee || 'Unassigned')),fact('First-response SLA',item.sla.state.toUpperCase()),fact('Response due',date(item.sla.due_at)),fact('First response',date(item.sla.responded_at)),fact('Initial / current severity',(item.initial_severity || 'Unknown') + ' / ' + item.severity)); root.append(facts);
      for (const [label,refs] of [['Evidence',legacy.incident.evidence_refs],['Alerts',legacy.incident.alert_refs],['Dossiers',legacy.incident.dossier_refs]]) {
        const details = node('details'); details.append(node('summary',label + ' (' + (refs || []).length + ')')); const list = node('ul',null,'refs');
        for (const ref of refs || []) { const li = node('li'); if (label === 'Dossiers' && /^KD1-[a-f0-9]{32}$/.test(ref)) { const a = node('a',ref); a.href = '/dossier/' + encodeURIComponent(ref); li.append(a); } else li.textContent = ref; list.append(li); } details.append(list); root.append(details);
      }
      const form = node('form',null,'case-form'); form.append(node('h2','Queue action'));
      const action = select([['assign','Assign / unassign'],['escalate','Escalate level'],['acknowledge','Acknowledge response']]);
      const assignee = node('input'); assignee.maxLength = 120; assignee.value = op.assignee; assignee.placeholder = 'Label; empty clears assignment';
      const tier = select([['L2','L2 · Investigation'],['L3','L3 · Specialist']]); if (op.tier === 'L2') tier.value = 'L3';
      const reason = node('textarea'); reason.required = true; reason.maxLength = 4000;
      const assigneeField = field('Assignee label',assignee), tierField = field('Escalate to',tier);
      const toggle = () => { assigneeField.hidden = action.value !== 'assign'; tierField.hidden = action.value !== 'escalate'; }; action.addEventListener('change',toggle); toggle();
      const save = node('button','Save queue action','primary'); save.type = 'submit';
      form.append(field('Action',action),assigneeField,tierField,field('Reason / handoff note',reason),save);
      if (item.status === 'resolved' || item.status === 'closed') { save.disabled = true; form.append(node('p','Reopen through the response workflow before changing queue metadata.','muted')); }
      form.addEventListener('submit',e => {e.preventDefault(); const body = {action:action.value,summary:reason.value,expected_version:op.version}; if(action.value === 'assign') body.assignee = assignee.value; if(action.value === 'escalate') body.tier = tier.value; submit(form,'/api/owner/soc/case?id=' + encodeURIComponent(id),body);}); root.append(form);
      const workflow = node('form',null,'case-form'); workflow.append(node('h2','Response workflow'));
      const transitions = {open:['investigate','contain'],investigating:['reopen','contain','resolve'],contained:['reopen','investigate','resolve'],resolved:['reopen','close'],closed:['reopen']};
      const responseAction = select(['note',...(transitions[item.status] || [])].map(v => [v,v])); const note = node('textarea'); note.required = true; note.maxLength = 4000;
      const record = node('button','Record response'); record.type = 'submit'; workflow.append(field('Action',responseAction),field('Response note',note),record);
      workflow.addEventListener('submit',e => { e.preventDefault(); submit(workflow,'/api/owner/incidents/' + encodeURIComponent(id),{action:responseAction.value,summary:note.value}); }); root.append(workflow);
      const handoff = node('a','Open complete handoff JSON (v2)'); handoff.href = '/api/owner/incidents/' + encodeURIComponent(id) + '?handoff=2';
      root.append(handoff,node('p','Includes incident and SOC histories, assignment and first-response SLA. Owner labels and evidence references are not independently verified.','muted small'));
      root.append(node('h3','SOC handoff history'));
      const socHistory = timeline(data.events,true); root.append(socHistory);
      const more = node('button','Load earlier handoffs'); more.type = 'button'; more.hidden = !data.has_more_events; let cursor = data.events.at(-1)?.id;
      more.addEventListener('click',async () => { more.disabled = true; try {const older = await api('/api/owner/soc/case?id=' + encodeURIComponent(id) + '&before_event=' + cursor); if(selected !== id || seq !== detailSequence) return; const extra = timeline(older.events,true); socHistory.append(...extra.children); cursor = older.events.at(-1)?.id; more.hidden = !older.has_more_events;} catch(error){notice(error.message,true);} finally{more.disabled = false;} }); root.append(more);
      root.append(node('h3','Original incident timeline'),node('p','Existing workflow history; up to the first 500 recorded actions.','muted small'),timeline(legacy.actions || [],false));
      notice('Revision ' + op.version + ' · Reload the case to see changes from other sessions.');
    } catch(error) { if(seq === detailSequence) { $('case').replaceChildren(); notice(error.message,true); } }
  }
  $('filters').addEventListener('submit',e => {e.preventDefault();filters = new URLSearchParams(new FormData(e.currentTarget));offset = 0;queue(true);});
  $('refresh').addEventListener('click',() => queue(true)); $('reloadCase').addEventListener('click',() => {if(!writing) loadCase();});
  $('previous').addEventListener('click',() => {offset = Math.max(0,offset-limit);queue(true);}); $('next').addEventListener('click',() => {offset += limit;queue(true);});
  setInterval(() => {if(!document.hidden) queue();},15000);
  queue();
})();
