from pathlib import Path

path = Path("internal/services/security_radar_store.go")
text = path.read_text()
old = '''\tvar id string
\terr := s.DB.QueryRowContext(ctx, `
\t\tINSERT INTO security_radar_seen_signatures (module_id, signature, source_address, source_target, network, seen_at, created_at)
\t\tVALUES ($1,$2,NULLIF($3,''),NULLIF($3,''),$4,now(),now())
\t\tON CONFLICT (signature, module_id, network) DO NOTHING
\t\tRETURNING id::text`, moduleID, signature, sourceAddress, network).Scan(&id)'''
new = '''\tvar id string
\t// Production carries both the legacy (module_id, signature) unique index
\t// and the newer network-scoped index. A targetless conflict handler makes
\t// duplicate PumpPortal deliveries idempotent against either constraint.
\terr := s.DB.QueryRowContext(ctx, `
\t\tINSERT INTO security_radar_seen_signatures (module_id, signature, source_address, source_target, network, seen_at, created_at)
\t\tVALUES ($1,$2,NULLIF($3,''),NULLIF($3,''),$4,now(),now())
\t\tON CONFLICT DO NOTHING
\t\tRETURNING id::text`, moduleID, signature, sourceAddress, network).Scan(&id)'''
if old not in text:
    raise SystemExit("MarkSignatureSeen target not found")
path.write_text(text.replace(old, new, 1))
