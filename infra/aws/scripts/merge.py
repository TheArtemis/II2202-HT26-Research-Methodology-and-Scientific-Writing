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
import re
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Tuple

_REP_RE = re.compile(r"_r(\d+)$")


def load_json(path: Path) -> Any:
    with path.open(encoding="utf-8") as f:
        return json.load(f)


def parse_repetition(run_id: str, status: Dict[str, Any]) -> Optional[int]:
    if isinstance(status.get("repetition"), int):
        return int(status["repetition"])
    m = _REP_RE.search(run_id)
    return int(m.group(1)) if m else None


def matrix_shape(config_src: Optional[Path]) -> Tuple[int, int]:
    """Return (repetitions, conditions_per_rep) from experiment YAML if present."""
    reps = int(os.environ.get("REPETITIONS", "30"))
    per_rep = max(1, int(os.environ.get("TOTAL_TRIALS", "900")) // max(1, reps))
    if not config_src or not config_src.is_file():
        return reps, per_rep
    try:
        text = config_src.read_text(encoding="utf-8")
    except OSError:
        return reps, per_rep
    # Minimal parse — avoid requiring PyYAML on the laptop.
    m_reps = re.search(r"^\s*repetitions:\s*(\d+)\s*$", text, re.M)
    if m_reps:
        reps = int(m_reps.group(1))

    def _list_len(key: str) -> Optional[int]:
        m = re.search(rf"^\s*{key}:\s*\[([^\]]*)\]", text, re.M)
        if not m:
            return None
        items = [x.strip() for x in m.group(1).split(",") if x.strip()]
        return len(items) if items else None

    n_topo = _list_len("topologies") or 5
    n_mode = _list_len("modes") or 2
    n_delay = _list_len("delays") or 3
    n_hb = _list_len("heartbeats") or 1
    per_rep = n_topo * n_mode * n_delay * n_hb
    return reps, per_rep


def build_campaign_status(
    report: Dict[str, Any],
    present: Dict[str, Dict[str, Any]],
    failed_ids: set,
    reps_planned: int,
    per_rep: int,
    campaign_id: str,
) -> Dict[str, Any]:
    by_rep: Dict[str, Dict[str, int]] = {}
    for rep in range(1, reps_planned + 1):
        by_rep[str(rep)] = {
            "expected": per_rep,
            "present": 0,
            "ok": 0,
            "failed": 0,
            "not_started": per_rep,
        }

    for rid, st in present.items():
        rep = parse_repetition(rid, st)
        if rep is None:
            continue
        key = str(rep)
        if key not in by_rep:
            by_rep[key] = {
                "expected": per_rep,
                "present": 0,
                "ok": 0,
                "failed": 0,
                "not_started": per_rep,
            }
        slot = by_rep[key]
        slot["present"] += 1
        if rid in failed_ids:
            slot["failed"] += 1
        else:
            slot["ok"] += 1
        slot["not_started"] = max(0, slot["expected"] - slot["present"])

    complete_workers = int(report.get("complete_workers") or 0)
    worker_count = int(report.get("worker_count") or 0)
    present_n = int(report["present"])
    expected_n = int(report["expected"])
    if complete_workers >= worker_count > 0 and present_n >= expected_n:
        phase = "complete"
    elif complete_workers >= worker_count > 0:
        phase = "workers_done_incomplete"
    elif present_n > 0:
        phase = "in_progress"
    else:
        phase = "not_started"

    return {
        "campaign_id": campaign_id,
        "phase": phase,
        "expected": expected_n,
        "present": present_n,
        "ok_count": int(report["ok_count"]),
        "failed_count": int(report["failed_count"]),
        "missing_count": int(report["missing_count"]),
        "not_started": int(report["missing_count"]),
        "complete_workers": complete_workers,
        "worker_count": worker_count,
        "repetitions_planned": reps_planned,
        "conditions_per_repetition": per_rep,
        "by_repetition": dict(sorted(by_rep.items(), key=lambda kv: int(kv[0]))),
    }


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
    failed_ids = set(failed)
    # Expected IDs unknown without expansion; report counts + missing vs expected N.
    missing_count = max(0, expected - len(present_ids))
    campaign_id = os.environ.get("CAMPAIGN_ID", "")
    fleet_path = workers_dir.parent / "fleet.json"
    if not campaign_id and fleet_path.is_file():
        try:
            campaign_id = str(load_json(fleet_path).get("campaign_id") or "")
        except (OSError, json.JSONDecodeError, TypeError):
            campaign_id = ""

    reps_planned, per_rep = matrix_shape(config_src)
    report = {
        "expected": expected,
        "present": len(present_ids),
        "failed": sorted(failed_ids),
        "failed_count": len(failed_ids),
        "missing": missing_count,
        "missing_count": missing_count,
        "ok_count": len(present_ids) - len(failed_ids),
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
        "campaign_id": campaign_id,
        "repetitions_planned": reps_planned,
        "conditions_per_repetition": per_rep,
    }

    status = build_campaign_status(
        report, present, failed_ids, reps_planned, per_rep, campaign_id
    )
    report["by_repetition"] = status["by_repetition"]
    report["phase"] = status["phase"]

    report_path = merged_dir / "validation-report.json"
    with report_path.open("w", encoding="utf-8") as f:
        json.dump(report, f, indent=2)
        f.write("\n")

    status_path = merged_dir / "campaign-status.json"
    with status_path.open("w", encoding="utf-8") as f:
        json.dump(status, f, indent=2)
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
        f"phase={report.get('phase')} -> {args.merged_dir}"
    )
    print(f"merge: wrote {args.merged_dir / 'validation-report.json'}")
    print(f"merge: wrote {args.merged_dir / 'campaign-status.json'}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
