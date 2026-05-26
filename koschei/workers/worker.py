import json
import os
import subprocess
import time
import uuid
from pathlib import Path

import psycopg2

DB_URL = os.getenv("DATABASE_URL")
UE6_EDITOR = os.getenv("UE6_EDITOR_BIN", "/opt/unreal/Engine/Binaries/Linux/UnrealEditor")
UE6_PROJECT = os.getenv("UE6_PROJECT_PATH", "/workspace/ue6/KoscheiGame/KoscheiGame.uproject")

BASE_BUNDLE_LIMIT_MB = 200
ASSET_PACK_LIMIT_MB = 4096

if not DB_URL:
    raise RuntimeError("DATABASE_URL is required")


def log(cur, project_id, task_id, artifact_id, level, message, payload=None):
    payload = payload or {}
    log_id = str(uuid.uuid4())
    cur.execute(
        "INSERT INTO runtime_logs (id, project_id, task_id, level, message, metadata) VALUES (%s,%s,%s,%s,%s,%s::jsonb)",
        (log_id, project_id, task_id, level, message, json.dumps(payload)),
    )
    cur.execute(
        "INSERT INTO runtime_build_logs (artifact_id, runtime_log_id, level, message, payload) VALUES (%s,%s,%s,%s,%s::jsonb)",
        (artifact_id, log_id, level, message, json.dumps(payload)),
    )


def inject_gpad_config(project_path: str):
    cfg = Path(project_path).parent / "Config" / "DefaultEngine.ini"
    cfg.parent.mkdir(parents=True, exist_ok=True)
    content = cfg.read_text() if cfg.exists() else ""
    block = """
[/Script/AndroidRuntimeSettings.AndroidRuntimeSettings]
bEnableBundle=True
bPackageDataInsideApk=False
bUseExternalFilesDir=True
+GooglePADAssetPacks=(Name="install-time-pack",DeliveryType=InstallTime)
+GooglePADAssetPacks=(Name="fast-follow-pack",DeliveryType=FastFollow)
+GooglePADAssetPacks=(Name="on-demand-pack",DeliveryType=OnDemand)
""".strip()
    if "GooglePADAssetPacks" not in content:
        content = content + "\n\n" + block + "\n"
        cfg.write_text(content)


def run_headless_build():
    cmd = [UE6_EDITOR, UE6_PROJECT, "-run=Cook", "-TargetPlatform=Android", "-nullrhi", "-server", "-unattended"]
    return subprocess.run(cmd, capture_output=True, text=True, timeout=3600)


def mb(path):
    return Path(path).stat().st_size / (1024 * 1024)


def optimize_if_needed(aab_path, pack_path):
    base = mb(aab_path) if Path(aab_path).exists() else 0
    pack = mb(pack_path) if Path(pack_path).exists() else 0
    while base > BASE_BUNDLE_LIMIT_MB or pack > ASSET_PACK_LIMIT_MB:
        subprocess.run(["bash", "-lc", f"gzip -f {aab_path}"], check=False)
        subprocess.run(["bash", "-lc", f"tar -czf {pack_path}.tgz {pack_path}"], check=False)
        base = mb(aab_path + ".gz") if Path(aab_path + ".gz").exists() else base
        pack = mb(pack_path + ".tgz") if Path(pack_path + ".tgz").exists() else pack
        if base <= BASE_BUNDLE_LIMIT_MB and pack <= ASSET_PACK_LIMIT_MB:
            break
        break
    return base, pack


while True:
    conn = psycopg2.connect(DB_URL)
    conn.autocommit = False
    cur = conn.cursor()
    task_id = None
    try:
        cur.execute(
            """
            SELECT t.id, t.project_id, t.email, ga.id
            FROM runtime_tasks t
            JOIN generated_artifacts ga ON ga.runtime_project_id=t.project_id
            WHERE t.status='queued' AND t.task_type='android_build'
            ORDER BY t.created_at ASC
            LIMIT 1 FOR UPDATE SKIP LOCKED
            """
        )
        row = cur.fetchone()
        if not row:
            conn.commit(); cur.close(); conn.close(); time.sleep(5); continue

        task_id, project_id, email, artifact_id = row
        cur.execute("UPDATE runtime_tasks SET status='running', updated_at=NOW() WHERE id=%s", (task_id,))
        cur.execute("UPDATE generated_artifacts SET build_status='compiling', updated_at=NOW() WHERE id=%s", (artifact_id,))
        log(cur, project_id, task_id, artifact_id, "info", "Android build started", {"email": email})
        conn.commit()

        inject_gpad_config(UE6_PROJECT)
        result = run_headless_build()
        if result.returncode != 0:
            raise RuntimeError(result.stderr[-1000:])

        aab_path = os.getenv("AAB_OUTPUT", "/tmp/game.aab")
        pack_path = os.getenv("ASSET_PACK_OUTPUT", "/tmp/asset.pack")
        base_size, pack_size = optimize_if_needed(aab_path, pack_path)

        cur.execute("UPDATE generated_artifacts SET build_status='ready', base_bundle_size_mb=%s, asset_pack_size_mb=%s, status='completed', updated_at=NOW() WHERE id=%s", (base_size, pack_size, artifact_id))
        cur.execute("UPDATE runtime_tasks SET status='completed', output_json=%s::jsonb, updated_at=NOW() WHERE id=%s", (json.dumps({"aab": aab_path, "asset_pack": pack_path}), task_id))
        log(cur, project_id, task_id, artifact_id, "info", "Android build completed", {"base_bundle_size_mb": base_size, "asset_pack_size_mb": pack_size})
        conn.commit()
    except Exception as e:
        conn.rollback()
        if task_id:
            try:
                cur.execute("UPDATE runtime_tasks SET status='failed', error=%s, updated_at=NOW() WHERE id=%s", (str(e), task_id))
                cur.execute("UPDATE generated_artifacts SET build_status='failed', status='failed', updated_at=NOW() WHERE runtime_project_id=(SELECT project_id FROM runtime_tasks WHERE id=%s)", (task_id,))
                conn.commit()
            except Exception:
                conn.rollback()
    finally:
        cur.close(); conn.close(); time.sleep(5)
