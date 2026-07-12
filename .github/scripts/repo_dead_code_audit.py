#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
API = REPO / "koschei" / "api"
REPORT_DIR = REPO / "audit-output"
REPORT_DIR.mkdir(exist_ok=True)

EXACT_BASENAMES = {
    "rug_radar.go",
    "web3.go",
    "local_auth.go",
    "mev_shield.go",
    "liquidity_radar.go",
    "impact_metrics.go",
    "metadata.go",
    "owner_payment_health.go",
    "web3_jobs.go",
    "package_status.go",
    "jobs.go",
    "dao_guardian.go",
    "plans.go",
    "payments.go",
    "smart_money.go",
}
GLOBS = ["credits*.go", "*legacy*.go", "*_old.go", "*.disabled", "*.bak"]
EXCLUDE_PARTS = {"vendor", ".git", "node_modules", "migrations"}


def ignored(path: Path) -> bool:
    return any(part in EXCLUDE_PARTS for part in path.parts)


def all_source_files() -> list[Path]:
    return [p for p in API.rglob("*") if p.is_file() and not ignored(p)]


def candidate_files() -> list[Path]:
    found: set[Path] = set()
    for path in all_source_files():
        if path.name in EXACT_BASENAMES:
            found.add(path)
    for pattern in GLOBS:
        for path in API.rglob(pattern):
            if path.is_file() and not ignored(path):
                found.add(path)
    return sorted(found)


FUNC_RE = re.compile(r"(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(")
TYPE_RE = re.compile(r"(?m)^type\s+([A-Za-z_][A-Za-z0-9_]*)\b")
VAR_RE = re.compile(r"(?m)^(?:var|const)\s+([A-Za-z_][A-Za-z0-9_]*)\b")


def declarations(text: str) -> list[str]:
    names = set(FUNC_RE.findall(text)) | set(TYPE_RE.findall(text)) | set(VAR_RE.findall(text))
    return sorted(name for name in names if len(name) >= 3)


def reference_locations(candidate: Path, names: list[str]) -> list[dict[str, object]]:
    refs: list[dict[str, object]] = []
    if not names:
        return refs
    patterns = [(name, re.compile(rf"\b{re.escape(name)}\b")) for name in names]
    for path in all_source_files():
        if path == candidate:
            continue
        if path.suffix not in {".go", ".js", ".html", ".md", ".yml", ".yaml", ".json"}:
            continue
        try:
            lines = path.read_text(encoding="utf-8", errors="ignore").splitlines()
        except OSError:
            continue
        for lineno, line in enumerate(lines, 1):
            matched = [name for name, pattern in patterns if pattern.search(line)]
            if matched:
                refs.append({
                    "path": str(path.relative_to(REPO)),
                    "line": lineno,
                    "symbols": matched,
                    "text": line.strip()[:240],
                })
                if len(refs) >= 80:
                    return refs
    return refs


def run(cmd: list[str], timeout: int = 240) -> tuple[int, str]:
    proc = subprocess.run(
        cmd,
        cwd=API,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        timeout=timeout,
        env={**os.environ, "CGO_ENABLED": "0"},
    )
    return proc.returncode, proc.stdout


def simulate_removal(path: Path) -> dict[str, object]:
    disabled = path.with_name(path.name + ".audit-disabled")
    if disabled.exists():
        disabled.unlink()
    path.rename(disabled)
    try:
        build_code, build_output = run(["go", "build", "./..."], timeout=360)
    except subprocess.TimeoutExpired:
        build_code, build_output = 124, "go build timed out"
    finally:
        disabled.rename(path)
    return {
        "build_ok_without_file": build_code == 0,
        "build_exit": build_code,
        "build_tail": "\n".join(build_output.splitlines()[-25:]),
    }


def main() -> None:
    baseline_code, baseline_output = run(["go", "build", "./..."], timeout=360)
    results: list[dict[str, object]] = []
    for path in candidate_files():
        rel = str(path.relative_to(REPO))
        text = path.read_text(encoding="utf-8", errors="ignore") if path.suffix == ".go" else ""
        names = declarations(text)
        refs = reference_locations(path, names)
        simulation = simulate_removal(path) if path.suffix == ".go" else {
            "build_ok_without_file": True,
            "build_exit": 0,
            "build_tail": "non-Go artifact",
        }
        results.append({
            "path": rel,
            "size_bytes": path.stat().st_size,
            "lines": text.count("\n") + 1 if text else None,
            "declarations": names,
            "external_reference_count": len(refs),
            "references": refs,
            **simulation,
        })

    payload = {
        "baseline_build_ok": baseline_code == 0,
        "baseline_build_exit": baseline_code,
        "baseline_build_tail": "\n".join(baseline_output.splitlines()[-30:]),
        "candidate_count": len(results),
        "results": results,
    }
    (REPORT_DIR / "dead-code-audit.json").write_text(json.dumps(payload, indent=2), encoding="utf-8")

    lines = [
        "# Koschei dead-code audit",
        "",
        f"Baseline `go build ./...`: **{'PASS' if baseline_code == 0 else 'FAIL'}**",
        f"Candidates checked: **{len(results)}**",
        "",
        "| File | Build without file | External refs | Lines | Initial classification |",
        "|---|---:|---:|---:|---|",
    ]
    for item in results:
        build_ok = bool(item["build_ok_without_file"])
        refs = int(item["external_reference_count"])
        classification = "SAFE DELETE CANDIDATE" if build_ok and refs == 0 else ("NEEDS SURGERY" if build_ok else "KEEP / DEPENDENCY")
        lines.append(
            f"| `{item['path']}` | {'PASS' if build_ok else 'FAIL'} | {refs} | {item['lines'] or '-'} | **{classification}** |"
        )
    lines.extend(["", "## Reference and build details", ""])
    for item in results:
        lines.append(f"### `{item['path']}`")
        lines.append("")
        lines.append(f"- Build without file: **{'PASS' if item['build_ok_without_file'] else 'FAIL'}**")
        lines.append(f"- External references: **{item['external_reference_count']}**")
        if item["references"]:
            for ref in item["references"][:20]:
                lines.append(f"  - `{ref['path']}:{ref['line']}` — `{', '.join(ref['symbols'])}` — {ref['text']}")
        if not item["build_ok_without_file"]:
            lines.append("- Build tail:")
            lines.append("```text")
            lines.append(str(item["build_tail"]))
            lines.append("```")
        lines.append("")
    (REPORT_DIR / "dead-code-audit.md").write_text("\n".join(lines), encoding="utf-8")
    print("\n".join(lines[:40]))


if __name__ == "__main__":
    main()
