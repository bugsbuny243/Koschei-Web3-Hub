(() => {
  'use strict';

  const clean = value => String(value ?? '').replace(/\s+/g, ' ').trim();
  const short = value => {
    const text = clean(value);
    return text.length > 28 ? `${text.slice(0, 12)}…${text.slice(-9)}` : text;
  };
  const networkNames = Object.freeze({
    'solana-mainnet': 'Solana',
    'ethereum-mainnet': 'Ethereum',
    'base-mainnet': 'Base',
    'arbitrum-mainnet': 'Arbitrum',
    'optimism-mainnet': 'Optimism',
    'bitcoin-mainnet': 'Bitcoin'
  });
  const networkTags = Object.freeze({
    'solana-mainnet': '#Solana',
    'ethereum-mainnet': '#Ethereum',
    'base-mainnet': '#Base',
    'arbitrum-mainnet': '#Arbitrum',
    'optimism-mainnet': '#Optimism',
    'bitcoin-mainnet': '#Bitcoin'
  });

  function publicResultURL(target, kind = 'token', network = '') {
    const value = clean(target);
    const normalizedKind = clean(kind).toLowerCase() || 'token';
    const normalizedNetwork = clean(network).toLowerCase();
    if (!value) return `${location.origin}/scan`;
    if (normalizedKind === 'token') return `${location.origin}/scan/${encodeURIComponent(value)}`;
    const query = new URLSearchParams({ mode: 'address', target: value, source: 'x_share' });
    if (normalizedNetwork) query.set('network', normalizedNetwork);
    return `${location.origin}/scan?${query.toString()}`;
  }

  function networkLabel(payload = {}) {
    return clean(payload.networkLabel) || networkNames[clean(payload.network).toLowerCase()] || (clean(payload.kind).toLowerCase() === 'token' ? 'Solana' : 'Web3');
  }

  function resultLabel(payload = {}) {
    const finalVerdict = payload.final_verdict || payload.finalVerdict || {};
    const evidence = clean(payload.evidence_status || payload.evidenceStatus).toUpperCase();
    const verdict = clean(payload.verdict || payload.decision || finalVerdict.recommendation || payload.risk_level || payload.riskLevel).toUpperCase();
    const grade = clean(finalVerdict.grade || payload.grade).toUpperCase();
    if (evidence) return `Evidence: ${evidence}`;
    if (grade) return `Grade: ${grade}`;
    if (verdict) return `Verdict: ${verdict}`;
    const score = Number(payload.score);
    return Number.isFinite(score) ? `Pre-check: ${Math.max(0, Math.min(100, Math.round(score)))}/100` : 'Evidence: REVIEW';
  }

  function evidenceLabel(payload = {}) {
    const finalVerdict = payload.final_verdict || payload.finalVerdict || {};
    const status = clean(payload.status).toLowerCase();
    const reasons = Array.isArray(payload.reasons) ? payload.reasons.map(clean) : [];
    if (reasons.includes('PARTIAL_EVIDENCE_EVM_AUTHORITY_UNAVAILABLE')) return 'Coverage: PARTIAL — authority evidence unavailable';
    if (status === 'evidence_pending' || finalVerdict.signed === false) return 'Coverage: PARTIAL — evidence gaps remain';
    if (status === 'insufficient_evidence' || status === 'needs_context') return 'Coverage: UNKNOWN — evidence incomplete';
    if (finalVerdict.signed === true || status === 'ready') return 'Coverage: signed technical verdict';
    if (status === 'observed' || status === 'evidence_ready') return `Verdict: ${clean(payload.verdict).toUpperCase() || 'REVIEW'}`;
    return 'Coverage: technical observation';
  }

  function buildText(payload = {}) {
    const target = short(payload.target);
    const network = networkLabel(payload);
    const signature = clean((payload.final_verdict || payload.finalVerdict || {}).signature || payload.signature);
    const rows = [
      `Koschei Web3 · ${network}`,
      `Target: ${target || 'public on-chain target'}`,
      resultLabel(payload),
      evidenceLabel(payload)
    ];
    if (signature && signature !== '—') rows.push(`Proof: ${short(signature)}`);
    const tag = networkTags[clean(payload.network).toLowerCase()] || (network === 'Solana' ? '#Solana' : '');
    rows.push(`Missing evidence is not proof of safety. #Koschei #Web3Security${tag ? ` ${tag}` : ''}`);
    const text = rows.join('\n');
    return text.length > 260 ? `${text.slice(0, 257).trimEnd()}…` : text;
  }

  function buildIntent(payload = {}) {
    const url = payload.url || publicResultURL(payload.target, payload.kind, payload.network);
    const query = new URLSearchParams({ text: buildText(payload), url });
    return `https://x.com/intent/tweet?${query.toString()}`;
  }

  function open(payload = {}) {
    const intent = buildIntent(payload);
    const popup = window.open(intent, '_blank', 'noopener,noreferrer,width=680,height=720');
    if (popup) popup.opener = null;
    return intent;
  }

  function installRadarShare() {
    if (!/^\/security-radar(?:\.html)?\/?$/.test(location.pathname)) return;
    const reportBody = document.getElementById('reportBody');
    const reportBox = reportBody?.closest('.reportbox');
    if (!reportBody || !reportBox || document.getElementById('shareRadarResult')) return;

    const button = document.createElement('button');
    button.id = 'shareRadarResult';
    button.type = 'button';
    button.className = 'btn';
    button.textContent = 'Share on X';
    button.hidden = true;
    const actions = document.createElement('div');
    actions.className = 'actions';
    actions.style.marginBottom = '14px';
    actions.appendChild(button);
    reportBox.insertBefore(actions, reportBody);

    let payload = {};
    const refresh = () => {
      const target = clean(reportBody.querySelector('.target-full')?.textContent);
      if (!target) {
        button.hidden = true;
        payload = {};
        return;
      }
      const grade = clean(reportBody.querySelector('.scorebox strong')?.textContent);
      const signature = clean(reportBody.querySelector('.signature')?.textContent).replace(/^İmza:\s*/i, '');
      const statusText = clean(reportBody.querySelector('.creator-warning .pill')?.textContent || reportBody.querySelector('.verdict-head .pill')?.textContent).toLowerCase();
      const status = statusText.includes('pending') || statusText.includes('eksik') ? 'evidence_pending' : 'ready';
      payload = { target, kind: 'token', network: 'solana-mainnet', grade, signature, status, url: publicResultURL(target, 'token', 'solana-mainnet') };
      button.hidden = false;
    };

    button.addEventListener('click', () => open(payload));
    new MutationObserver(refresh).observe(reportBody, { childList: true, subtree: true, characterData: true });
    refresh();
  }

  window.KoscheiInvestigationShare = Object.freeze({ publicResultURL, buildText, buildIntent, open, installRadarShare });
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', installRadarShare, { once: true });
  else installRadarShare();
})();
