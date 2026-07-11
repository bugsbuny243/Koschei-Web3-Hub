from pathlib import Path

p = Path('public/js/owner-control-center.js')
s = p.read_text()
old = '''function radarReportHTML(d){const f=obj(d.final_verdict),dist=obj(d.holder_distribution),src=obj(d.source_context),struct=obj(d.structural_memory),mods=arr(d.modules),evidence=arr(d.evidence),accounts=arr(dist.top_accounts),level=f.risk_level||'unknown';return`'''
new = '''function radarReportHTML(d){const f=obj(d.final_verdict),dist=obj(d.holder_distribution),src=obj(d.source_context),struct=obj(d.structural_memory),mods=arr(d.modules),evidence=arr(d.evidence),accounts=arr(dist.top_accounts),scored=f.signed!==false&&Number.isFinite(Number(f.risk_index)),level=scored?(f.risk_level||'unknown'):'evidence_pending',scoreText=scored?`${num(f.risk_index)}/100`:'N/A';return`'''
if old not in s:
    raise SystemExit('radarReportHTML header not found')
s = s.replace(old, new, 1)
old = '''<h2>${esc(String(level).toUpperCase())} · ${num(f.risk_index)}/100 · Grade ${esc(f.grade||'—')}</h2>'''
new = '''<h2>${esc(String(level).toUpperCase())} · ${esc(scoreText)} · Grade ${esc(scored?(f.grade||'—'):'—')}</h2>'''
if old not in s:
    raise SystemExit('report score heading not found')
s = s.replace(old, new, 1)
old = '''<details class="owner-details section-gap"><summary><span><b>İlk 20 token hesabı</b><small>Token account; owner wallet eşlemesi ayrı kanıt gerektirir.</small></span><span>⌄</span></summary>${accounts.length?`<div class="table-wrap section-gap"><table class="table"><thead><tr><th>#</th><th>Token account</th><th>Bakiye</th><th>Pay</th><th>Kümülatif</th></tr></thead><tbody>${accounts.map(a=>`<tr><td>${a.rank}</td><td class="mono">${esc(a.token_account)}</td><td>${num(a.balance)}</td><td>${num(a.percentage)}%</td><td>${num(a.cumulative_percentage)}%</td></tr>`).join('')}</tbody></table></div>`:'<div class="empty section-gap">Holder hesabı verisi alınamadı.</div>'}</details>'''
new = '''<details class="owner-details section-gap"><summary><span><b>İlk 20 holder rolü</b><small>Token account → owner wallet/PDA → owner program → ekonomik rol.</small></span><span>⌄</span></summary>${accounts.length?`<div class="table-wrap section-gap"><table class="table"><thead><tr><th>#</th><th>Token account</th><th>Owner wallet / PDA</th><th>Rol</th><th>Bakiye</th><th>Ham pay</th><th>Dolaşım payı</th></tr></thead><tbody>${accounts.map(a=>`<tr><td>${a.rank}</td><td class="mono">${esc(short(a.token_account,30))}</td><td class="mono">${esc(short(a.owner_wallet,30))}</td><td>${badge(a.role||'unknown')}<div class="muted small">${esc(a.excluded_from_holder_risk?'Holder riskinden çıkarıldı':'Holder riskine dahil')}</div></td><td>${num(a.balance)}</td><td>${num(a.raw_percentage)}%</td><td>${a.excluded_from_holder_risk?'—':num(a.circulating_percentage)+'%'}</td></tr>`).join('')}</tbody></table></div>`:'<div class="empty section-gap">Holder hesabı verisi alınamadı.</div>'}</details>'''
if old not in s:
    raise SystemExit('legacy holder table not found')
s = s.replace(old, new, 1)
p.write_text(s)
