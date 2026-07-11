from pathlib import Path

p = Path('internal/handlers/security_radar_detail.go')
s = p.read_text()
old = '''\tif persisted == nil || !persisted.Signed {
\t\treturn out
\t}
'''
new = '''\tif !fresh.Signed {
\t\treturn out
\t}
\tif persisted == nil || !persisted.Signed {
\t\treturn out
\t}
'''
if old not in s:
    raise SystemExit('fresh verdict persistence guard insertion point not found')
p.write_text(s.replace(old, new, 1))

p = Path('public/js/owner-control-center.js')
s = p.read_text()
old = "level=String(f.risk_level||'unknown').toUpperCase(),danger=['HIGH','CRITICAL'].includes(level),accent=danger?'#ff526f':level==='MEDIUM'?'#ffc95c':'#18ffb2';"
new = "scored=f.signed!==false&&Number.isFinite(Number(f.risk_index)),level=scored?String(f.risk_level||'unknown').toUpperCase():'EVIDENCE PENDING',danger=['HIGH','CRITICAL'].includes(level),accent=danger?'#ff526f':level==='MEDIUM'?'#ffc95c':'#18ffb2';"
if old not in s:
    raise SystemExit('poster scored state insertion point not found')
s = s.replace(old, new, 1)
old = "c.fillText(`${level} WARNING · ${Number(f.risk_index||0)}/100`,70,245);"
new = "c.fillText(scored?`${level} WARNING · ${Number(f.risk_index)}/100`:'EVIDENCE PENDING · N/A',70,245);"
if old not in s:
    raise SystemExit('poster score label insertion point not found')
p.write_text(s.replace(old, new, 1))
