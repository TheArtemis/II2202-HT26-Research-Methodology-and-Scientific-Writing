#!/usr/bin/env python3
"""Merge raw worker shards into a single harness-compatible results directory.

Rules (fleet plan):
  - Include only runs with finalized status.json
  - Concatenate per-worker manifest.jsonl (dedupe by run_id)
  - Write validation-report.json (expected, present, failed, missing)
  - Never requires EC2 SSH — S3-synced local cache is the source of truth
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import sys
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional


def load_json(path: Path) -> Any:
    with path.open(encoding="utf-8") as f:
        return json.load(f)


def iter_jsonl(path: Path) -> Iterable[Dict[str, Any]]:
    if not path.is_file():
        return
    with path.open(encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                yield json.loads(line)
            except json.JSONDecodeError:
                continue


def copy_run_dir(src: Path, dest: Path) -> None:
    if dest.exists():
        shutil.rmtree(dest)
    shutil.copytree(src, dest)


def merge(
    workers_dir: Path,
    merged_dir: Path,
    expected: int,
    config_src: Optional[Path] = None,
) -> Dict[str, Any]:
    merged_dir.mkdir(parents=True, exist_ok=True)

    present: Dict[str, Dict[str, Any]] = {}
    failed: List[str] = []
    manifests: Dict[str, Dict[str, Any]] = {}
    worker_complete: Dict[str, Any] = {}

    worker_dirs = sorted(
        p for p in workers_dir.iterdir() if p.is_dir() and p.name.startswith("worker-")
    )
    if not worker_dirs:
        # Accept flat layout or any subdirs.
        worker_dirs = sorted(p for p in workers_dir.iterdir() if p.is_dir())

    for wdir in worker_dirs:
        complete_path = wdir / "complete.json"
        if complete_path.is_file():
            try:
                worker_complete[wdir.name] = load_json(complete_path)
            except (OSError, json.JSONDecodeError):
                worker_complete[wdir.name] = {"ok": False, "error": "invalid complete.json"}

        for row in iter_jsonl(wdir / "manifest.jsonl"):
            rid = row.get("run_id")
            if not rid:
                continue
            # Prefer later / ok=true over earlier duplicates.
            prev = manifests.get(rid)
            if prev is None or (row.get("ok") and not prev.get("ok")):
                manifests[rid] = row

        for child in wdir.iterdir():
            if not child.is_dir():
                continue
            status_path = child / "status.json"
            if not status_path.is_file():
                continue
            try:
                st = load_json(status_path)
            except (OSError, json.JSONDecodeError):
                continue
            run_id = st.get("run_id") or child.name
            present[run_id] = st
            if not st.get("ok", False):
                failed.append(run_id)
            dest = merged_dir / run_id
            copy_run_dir(child, dest)

        # Propagate machine.json once (last writer wins; useful for debugging).
        machine = wdir / "machine.json"
        if machine.is_file():
            shutil.copy2(machine, merged_dir / f"machine-{wdir.name}.json")

    # Write concatenated deduped manifest.
    manifest_path = merged_dir / "manifest.jsonl"
    with manifest_path.open("w", encoding="utf-8") as mf:
        for rid in sorted(manifests.keys()):
            mf.write(json.dumps(manifests[rid], separators=(",", ":")) + "\n")
        # Ensure every present run appears even if worker manifest lagged.
        for rid, st in sorted(present.items()):
            if rid not in manifests:
                mf.write(json.dumps(st, separators=(",", ":"), default=str) + "\n")

    if config_src and config_src.is_file():
        shutil.copy2(config_src, merged_dir / "experiment.yaml")

    present_ids = set(present.keys())
    # Expected IDs unknown without expansion; report counts + missing vs expected N.
    missing_count = max(0, expected - len(present_ids))
    report = {
        "expected": expected,
        "present": len(present_ids),
        "failed": sorted(set(failed)),
        "failed_count": len(set(failed)),
        "missing": missing_count,
        "missing_count": missing_count,
        "ok_count": len(present_ids) - len(set(failed)),
        "workers": {
            name: {
                "complete": worker_complete.get(name),
            }
            for name in [w.name for w in worker_dirs]
        },
        "complete_workers": sum(
            1 for v in worker_complete.values() if isinstance(v, dict) and v.get("ok")
        ),
        "worker_count": len(worker_dirs),
        "run_ids": sorted(present_ids),
    }

    report_path = merged_dir / "validation-report.json"
    with report_path.open("w", encoding="utf-8") as f:
        json.dump(report, f, indent=2)
        f.write("\n")

    return report


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--workers-dir",
        type=Path,
        required=True,
        help="Local mirror of s3://…/campaigns/<id>/workers/",
    )
    ap.add_argument(
        "--merged-dir",
        type=Path,
        required=True,
        help="Output directory (harness-summarize compatible)",
    )
    ap.add_argument(
        "--expected",
        type=int,
        default=int(os.environ.get("TOTAL_TRIALS", "900")),
        help="Expected trial count (default 900 / TOTAL_TRIALS)",
    )
    ap.add_argument(
        "--config",
        type=Path,
        default=None,
        help="Optional experiment YAML to copy as experiment.yaml",
    )
    args = ap.parse_args()

    if not args.workers_dir.is_dir():
        print(f"error: workers dir not found: {args.workers_dir}", file=sys.stderr)
        return 1

    report = merge(args.workers_dir, args.merged_dir, args.expected, args.config)
    print(
        f"merge: present={report['present']}/{report['expected']} "
        f"failed={report['failed_count']} missing={report['missing_count']} "
        f"-> {args.merged_dir}"
    )
    print(f"merge: wrote {args.merged_dir / 'validation-report.json'}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
