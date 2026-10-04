/* Persisted operator evidence only. No wallet, signing or response authority. */
(() => {
  'use strict';
  const status = document.getElementById('campaign-status');
  const runtime = document.getElementById('campaign-runtime');
  const list = document.getElementById('campaign-list');
  const refresh = document.getElementById('campaign-refresh');
  function field(parent, label, value) {
    const term = document.createElement('dt'); term.textContent = label;
    const detail = document.createElement('dd'); detail.textContent = String(value ?? 'Unknown');
    parent.append(term, detail);
  }
  async function load() {
    refresh.disabled = true;
    status.textContent = 'Loading persisted campaigns…';
    runtime.replaceChildren(); list.replaceChildren();
    try {
      const response = await fetch('/api/owner/campaigns', {credentials: 'same-origin', cache: 'no-store', signal: AbortSignal.timeout(7000)});
      if (!response.ok) throw new Error(response.status === 404 || response.status === 401 ? 'Sign in through the owner console.' : 'Campaign storage is unavailable. Evidence remains unknown.');
      const data = await response.json();
      if (data.version !== 'koschei.global-campaign-runtime.v1' || !Array.isArray(data.campaigns) || !data.runtime) throw new Error('Campaign response cannot be verified.');
      field(runtime, 'Materializer', data.runtime.enabled ? 'Enabled' : 'Disabled');
      field(runtime, 'Pending / failed', `${data.runtime.pending} / ${data.runtime.failed}`);
      field(runtime, 'Processed sources', data.runtime.processed);
      field(runtime, 'Latest processed source', data.runtime.latest_processed_at);
      field(runtime, 'Oldest pending source', data.runtime.oldest_pending_at);
      field(runtime, 'Automatic response', 'Disabled');
      status.textContent = data.campaigns.length ? `${data.campaigns.length} current campaign projections.` : 'No verified linked evidence has produced a campaign projection.';
      for (const snapshot of data.campaigns) {
        const c = snapshot.campaign, command = snapshot.command;
        if (!c || !command || command.containment_verified || command.response_state !== 'monitoring') throw new Error('Unexpected response authority; projection hidden.');
        const article = document.createElement('article');
        const title = document.createElement('h2'); title.textContent = c.campaign_ref; article.append(title);
        const facts = document.createElement('dl');
        field(facts, 'Revision / state', `${c.revision} / ${c.state}`);
        field(facts, 'Networks', (c.networks || []).join(', '));
        field(facts, 'Observed evidence', c.last_observed_at);
        field(facts, 'Response', command.response_state);
        field(facts, 'Containment', 'Unverified');
        field(facts, 'Missing evidence', (command.missing_evidence || []).join(', '));
        field(facts, 'Evidence hash', c.evidence_hash_sha256);
        article.append(facts);
        const details = document.createElement('details');
        const summary = document.createElement('summary'); summary.textContent = 'Source references and temporal / bridge evidence';
        const source = document.createElement('pre'); source.textContent = JSON.stringify(snapshot, null, 2);
        details.append(summary, source); article.append(details); list.append(article);
      }
    } catch (error) {
      runtime.replaceChildren(); list.replaceChildren();
      status.textContent = error.message || 'Campaign evidence is unavailable.';
    } finally { refresh.disabled = false; }
  }
  refresh.addEventListener('click', load);
  load();
})();
