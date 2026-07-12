from pathlib import Path

path = Path("internal/services/pump_high_volume_radar.go")
text = path.read_text()
old = '''\t\t\t  AND COALESCE(signals->>'auto_scan_attempted','false')='true'
\t\t\t  AND created_at >= now()-($4 * interval '1 second')'''
new = '''\t\t\t  AND COALESCE(signals->>'auto_scan_attempted','false')='true'
\t\t\t  AND updated_at >= now()-($4 * interval '1 second')'''
if old not in text:
    raise SystemExit("attempt timestamp target missing")
path.write_text(text.replace(old, new, 1))
