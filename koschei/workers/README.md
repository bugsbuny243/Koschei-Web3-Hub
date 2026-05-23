# Koschei Runtime Worker

Mock-safe runtime worker for Phase 2.

## Run

```bash
pip install -r requirements.txt
DATABASE_URL=postgres://... python worker.py
```

The worker polls queued `runtime_tasks`, processes one task at a time, writes logs, results, and credit events.
