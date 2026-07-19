(() => {
  'use strict';

  const clean = value => String(value ?? '').replace(/\s+/g, ' ').trim();
  const short = value => {
    const text = clean(value);
    return text.length > 28 ? `${text.slice(0, 12)}…${text.slice(-9)}` : text;
  };

  function publicResultURL(target, kind = 'token') {
    const value = clean(target);
    const normalizedKind = clean(kind).toLowerCase() || 'token';
    if (!value) return `${location.origin}/scan`;
    if (normalizedKind === 'token') return `${location.origin}/scan/${encodeURIComponent(value)}`;
    const query = new URLSearchParams({ target: value, kind: normalizedKind, source: 'x_share' });
    return `${location.origin}/scan?${query.toString()}`;
  }

  function resultLabel(payload = {}) {
    const finalVerdict = payload.final_verdict || payload.finalVerdict || {};
    const grade = clean(finalVerdict.grade || payload.grade).toUpperCase();
    if (grade) return `Birleşik not: ${grade}`;
    const decision = clean(payload.decision || finalVerdict.recommendation || payload.risk_level || payload.riskLevel).toUpperCase();
    if (decision) return `Sonuç: ${decision}`;
    const score = Number(payload.score);
    return Number.isFinite(score) ? `Ön kontrol: ${Math.max(0, Math.min(100, Math.round(score)))}/100` : 'Sonuç: kanıt dosyası hazır';
  }

  function evidenceLabel(payload = {}) {
    const finalVerdict = payload.final_verdict || payload.finalVerdict || {};
    const status = clean(payload.status).toLowerCase();
    if (status === 'evidence_pending' || finalVerdict.signed === false) return 'Durum: kanıt boşlukları açıkça işaretlendi';
    if (finalVerdict.signed === true || status === 'ready') return 'Durum: imzalı teknik hüküm';
    return 'Durum: teknik ön inceleme';
  }

  function buildText(payload = {}) {
    const target = short(payload.target);
    const signature = clean((payload.final_verdict || payload.finalVerdict || {}).signature || payload.signature);
    const rows = [
      `Koschei ARVIS ile ${target || 'Solana hedefi'} taraması`,
      resultLabel(payload),
      evidenceLabel(payload)
    ];
    if (signature) rows.push(`Doğrulama: ${short(signature)}`);
    rows.push('Eksik kanıt güvenli sayılmaz. #Koschei #SolanaSecurity');
    const text = rows.join('\n');
    return text.length > 260 ? `${text.slice(0, 257).trimEnd()}…` : text;
  }

  function buildIntent(payload = {}) {
    const url = payload.url || publicResultURL(payload.target, payload.kind);
    const query = new URLSearchParams({ text: buildText(payload), url });
    return `https://x.com/intent/tweet?${query.toString()}`;
  }

  function open(payload = {}) {
    const intent = buildIntent(payload);
    const popup = window.open(intent, '_blank', 'noopener,noreferrer,width=680,height=720');
    if (popup) popup.opener = null;
    return intent;
  }

  window.KoscheiInvestigationShare = Object.freeze({ publicResultURL, buildText, buildIntent, open });
})();
