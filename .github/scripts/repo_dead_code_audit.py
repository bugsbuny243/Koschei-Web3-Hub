#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import re
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
API = REPO / "koschei" / "api"
REPORT_DIR = REPO / "audit-output"
REPORT_DIR.mkdir(exist_ok=True)

TARGET_DIRS = [
    API / "internal" / "handlers",
    API / "internal" / "router",
    API / "internal" / "jobs",
    API / "internal" / "web3",
]
USER_FILES = {
    "rug_radar.go", "web3.go", "local_auth.go", "mev_shield.go",
    "liquidity_radar.go", "impact_metrics.go", "metadata.go",
    "owner_payment_health.go", "web3_jobs.go", "package_status.go",
    "jobs.go", "dao_guardian.go", "plans.go", "payments.go",
    "smart_money.go",
}
LEGACY_WORDS = (
    "legacy", "deprecated", "unused", "stub", "shopier", "paddle",
    "google_play", "local_auth", "rug_radar", "dao_guardian",
    "smart_money", "metadata", "liquidity_radar", "mev_shield",
    "web3_jobs", "package_status", "plans", "payments",
)
EXCLUDE_PARTS = {"vendor", ".git", "node_modules", "migrations"}

FUNC_RE = re.compile(r"(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(")
TYPE_RE = re.compile(r"(?m)^type\s+([A-Za-z_][A-Za-z0-9_]*)\b")
VAR_RE = re.compile(r"(?m)^(?:var|const)\s+([A-Za-z_][A-Za-z0-9_]*)\b")


def ignored(path: Path) -> bool:
    return any(part in EXCLUDE_PARTS for part in path.parts)


def source_files() -> list[Path]:
    return [p for p in API.rglob("*") if p.is_file() and not ignored(p)]


def declarations(text: str) -> list[str]:
    names = set(FUNC_RE.findall(text)) | set(TYPE_RE.findall(text)) | set(VAR_RE.findall(text))
    return sorted(name for name in names if len(name) >= 3)


def references(candidate: Path, names: list[str]) -> list[dict[str, object]]:
    out: list[dict[str, object]] = []
    if not names:
        return out
    patterns = [(name, re.compile(rf"\b{re.escape(name)}\b")) for name in names]
    for path in source_files():
        if path == candidate or path.suffix not in {".go", ".js", ".html", ".md", ".yml", ".yaml", ".json"}:
            continue
        lines = path.read_text(encoding="utf-8", errors="ignore").splitlines()
        for lineno, line in enumerate(lines, 1):
            matched = [name for name, pattern in patterns if pattern.search(line)]
            if matched:
                out.append({
                    "path": str(path.relative_to(REPO)), "line": lineno,
                    "symbols": matched, "text": line.strip()[:240],
                })
                if len(out) >= 120:
                    return out
    return out


def candidate_files() -> list[Path]:
    result: set[Path] = set()
    for directory in TARGET_DIRS:
        if not directory.exists():
            continue
        for path in directory.rglob("*.go"):
            if ignored(path):
                continue
            name = path.name.lower()
            text = path.read_text(encoding="utf-8", errors="ignore").lower()
            if (
                path.name in USER_FILES
                or name.endswith("_test.go")
                or any(word in name for word in LEGACY_WORDS)
                or "deprecated" in text[:800]
                or "legacy" in text[:800]
                or "not wired" in text[:800]
                or "unused" in text[:800]
            ):
                result.add(path)
    for path in API.rglob("*.disabled"):
        if path.is_file() and not ignored(path):
            result.add(path)
    for path in API.rglob("*.bak"):
        if path.is_file() and not ignored(path):
            result.add(path)
    return sorted(result)


def run(cmd: list[str], timeout: int = 420) -> tuple[int, str]:
    proc = subprocess.run(
        cmd, cwd=API, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        text=True, timeout=timeout, env={**os.environ, "CGO_ENABLED": "0"},
    )
    return proc.returncode, proc.stdout


def command_result(cmd: list[str], timeout: int = 420) -> dict[str, object]:
    try:
        code, output = run(cmd, timeout)
    except subprocess.TimeoutExpired:
        code, output = 124, "command timed out"
    return {"ok": code == 0, "exit": code, "tail": "\n".join(output.splitlines()[-35:])}


def simulate(path: Path) -> dict[str, object]:
    disabled = path.with_name(path.name + ".audit-disabled")
    path.rename(disabled)
    try:
        build = command_result(["go", "build", "./..."])
        tests = command_result(["go", "test", "./..."], timeout=600) if build["ok"] else {"ok": False, "exit": -1, "tail": "skipped because build failed"}
    finally:
        disabled.rename(path)
    return {"build_without": build, "tests_without": tests}


def route_reference_count(refs: list[dict[str, object]]) -> int:
    route_markers = ("server.go", "main.go", "routes", "router", "public/js", "public/")
    return sum(1 for ref in refs if any(marker in str(ref["path"]) for marker in route_markers))


def main() -> None:
    baseline_build = command_result(["go", "build", "./..."])
    baseline_tests = command_result(["go", "test", "./..."], timeout=600) if baseline_build["ok"] else {"ok": False, "exit": -1, "tail": "skipped because build failed"}
    results: list[dict[str, object]] = []
    for path in candidate_files():
        text = path.read_text(encoding="utf-8", errors="ignore") if path.suffix == ".go" else ""
        names = declarations(text)
        refs = references(path, names)
        sim = simulate(path) if path.suffix == ".go" else {
            "build_without": {"ok": True, "exit": 0, "tail": "non-Go artifact"},
            "tests_without": {"ok": True, "exit": 0, "tail": "non-Go artifact"},
        }
        safe = bool(sim["build_without"]["ok"] and sim["tests_without"]["ok"] and not refs)
        results.append({
            "path": str(path.relative_to(REPO)), "lines": text.count("\n") + 1 if text else None,
            "size_bytes": path.stat().st_size, "declarations": names,
            "reference_count": len(refs), "route_reference_count": route_reference_count(refs),
            "references": refs, "safe_delete_candidate": safe, **sim,
        })

    payload = {
        "baseline_build": baseline_build, "baseline_tests": baseline_tests,
        "candidate_count": len(results), "results": results,
    }
    (REPORT_DIR / "dead-code-audit.json").write_text(json.dumps(payload, indent=2), encoding="utf-8")

    lines = [
        "# Koschei repository cleanup audit", "",
        f"Baseline build: **{'PASS' if baseline_build['ok'] else 'FAIL'}**",
        f"Baseline tests: **{'PASS' if baseline_tests['ok'] else 'FAIL'}**",
        f"Candidates: **{len(results)}**", "",
        "| File | Build -file | Test -file | Refs | Route refs | Classification |",
        "|---|---:|---:|---:|---:|---|",
    ]
    for item in results:
        if item["safe_delete_candidate"]:
            cls = "SAFE DELETE"
        elif item["build_without"]["ok"] and item["tests_without"]["ok"]:
            cls = "REMOVE WITH REFERENCES"
        else:
            cls = "KEEP / REFACTOR FIRST"
        lines.append(
            f"| `{item['path']}` | {'PASS' if item['build_without']['ok'] else 'FAIL'} | "
            f"{'PASS' if item['tests_without']['ok'] else 'FAIL'} | {item['reference_count']} | "
            f"{item['route_reference_count']} | **{cls}** |"
        )
    lines += ["", "## Details", ""]
    for item in results:
        lines += [f"### `{item['path']}`", "", f"- Safe delete: **{item['safe_delete_candidate']}**", f"- References: **{item['reference_count']}**", f"- Route/frontend references: **{item['route_reference_count']}**"]
        for ref in item["references"][:25]:
            lines.append(f"  - `{ref['path']}:{ref['line']}` — `{', '.join(ref['symbols'])}` — {ref['text']}")
        if not item["build_without"]["ok"]:
            lines += ["- Build failure:", "```text", str(item["build_without"]["tail"]), "```"]
        elif not item["tests_without"]["ok"]:
            lines += ["- Test failure:", "```text", str(item["tests_without"]["tail"]), "```"]
        lines.append("")
    (REPORT_DIR / "dead-code-audit.md").write_text("\n".join(lines), encoding="utf-8")
    print("\n".join(lines[:80]))


if __name__ == "__main__":
    main()
