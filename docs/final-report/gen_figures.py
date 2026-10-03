#!/usr/bin/env python3
"""Generate report figures from dataset.jsonl."""
import json
from collections import defaultdict
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np

ROOT = Path(r"f:\Projects\II2202-HT26-Research-Methodology-and-Scientific-Writing")
DATA = ROOT / "experiments" / "viz" / "public" / "data" / "dataset.jsonl"
OUT = ROOT / "docs" / "final-report" / "figures"
OUT.mkdir(parents=True, exist_ok=True)

rows = [json.loads(l) for l in DATA.open(encoding="utf-8") if l.strip()]
# Prefer 500us and 5ms; skip broken 20ms except for narrative
DELAYS = ["500µs", "5ms"]
TOPOS = ["T0", "T1", "T2", "T3", "T4"]
MODES = ["direct", "forwarding"]

g = defaultdict(list)
for r in rows:
    g[(r["topology"], r["mode"], r["delay"])].append(r)


def series(topo, mode, delay, field):
    xs = []
    for r in g.get((topo, mode, delay), []):
        v = r.get(field)
        if v is None:
            continue
        if field.startswith("latency") and r.get("latency_count", 0) == 0:
            continue
        xs.append(float(v))
    return xs


def mean_sd(xs):
    if not xs:
        return 0.0, 0.0
    m = float(np.mean(xs))
    sd = float(np.std(xs, ddof=0)) if len(xs) > 1 else 0.0
    return m, sd


# Style
plt.rcParams.update(
    {
        "font.family": "sans-serif",
        "font.size": 9,
        "axes.titlesize": 10,
        "axes.labelsize": 9,
        "figure.dpi": 150,
        "savefig.dpi": 200,
        "savefig.bbox": "tight",
    }
)

# --- Figure 1: RQ1 throughput ---
fig, axes = plt.subplots(1, 2, figsize=(7.2, 2.8), sharey=True)
width = 0.35
x = np.arange(len(TOPOS))
colors = {"direct": "#4C78A8", "forwarding": "#F58518"}

for ax, delay in zip(axes, DELAYS):
    for i, mode in enumerate(MODES):
        means, sds = [], []
        for topo in TOPOS:
            m, s = mean_sd(series(topo, mode, delay, "commit_throughput_observe_eps"))
            means.append(m)
            sds.append(s)
        offset = -width / 2 if mode == "direct" else width / 2
        ax.bar(
            x + offset,
            means,
            width,
            yerr=sds,
            label=mode.capitalize(),
            color=colors[mode],
            capsize=2,
            error_kw={"linewidth": 0.8},
        )
    ax.set_xticks(x)
    ax.set_xticklabels(TOPOS)
    ax.set_title(f"Per-link delay {delay}")
    ax.set_xlabel("Topology")
    ax.grid(axis="y", linestyle=":", alpha=0.5)
    ax.set_ylim(0, 115)

axes[0].set_ylabel("Commit throughput (entries/s)")
axes[1].legend(frameon=False, loc="upper right")
fig.suptitle("Observe-window commit throughput (mean ± SD)", y=1.02)
fig.savefig(OUT / "rq1-throughput.pdf")
fig.savefig(OUT / "rq1-throughput.png")
plt.close(fig)

# --- Figure 2: RQ2 latency ---
fig, axes = plt.subplots(1, 2, figsize=(7.2, 2.8), sharey=False)
for ax, delay in zip(axes, DELAYS):
    for i, mode in enumerate(MODES):
        means, sds = [], []
        for topo in TOPOS:
            m, s = mean_sd(series(topo, mode, delay, "latency_median_ms"))
            means.append(m)
            sds.append(s)
        offset = -width / 2 if mode == "direct" else width / 2
        ax.bar(
            x + offset,
            means,
            width,
            yerr=sds,
            label=mode.capitalize(),
            color=colors[mode],
            capsize=2,
            error_kw={"linewidth": 0.8},
        )
    ax.set_xticks(x)
    ax.set_xticklabels(TOPOS)
    ax.set_title(f"Per-link delay {delay}")
    ax.set_xlabel("Topology")
    ax.grid(axis="y", linestyle=":", alpha=0.5)

axes[0].set_ylabel("Median commit latency (ms)")
axes[1].legend(frameon=False, loc="upper left")
fig.suptitle("Client-observed median commit latency (mean ± SD over runs)", y=1.02)
fig.savefig(OUT / "rq2-latency.pdf")
fig.savefig(OUT / "rq2-latency.png")
plt.close(fig)

# --- Figure 3: T3 recovery time (forwarding only where recovered) ---
fig, ax = plt.subplots(figsize=(4.5, 2.6))
labels = []
vals = []
errs = []
for delay in DELAYS:
    xs = series("T3", "forwarding", delay, "recovery_time_ms")
    # convert to seconds
    xs = [v / 1000.0 for v in xs]
    m, s = mean_sd(xs)
    labels.append(delay)
    vals.append(m)
    errs.append(s)
ax.bar(labels, vals, yerr=errs, color=colors["forwarding"], capsize=3, width=0.5)
ax.set_ylabel("Recovery time (s)")
ax.set_xlabel("Per-link delay")
ax.set_title("T3 forwarding: inject → first post-failure commit")
ax.grid(axis="y", linestyle=":", alpha=0.5)
fig.savefig(OUT / "rq1-t3-recovery.pdf")
fig.savefig(OUT / "rq1-t3-recovery.png")
plt.close(fig)

print("Wrote figures to", OUT)
for p in sorted(OUT.glob("*")):
    print(" ", p.name, p.stat().st_size)
