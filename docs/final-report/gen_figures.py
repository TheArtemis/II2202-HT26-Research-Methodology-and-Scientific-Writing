#!/usr/bin/env python3
"""Generate report figures from dataset.jsonl."""
import json
from collections import defaultdict
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np

ROOT = Path(__file__).resolve().parents[2]
DATA = ROOT / "results" / "campaigns" / "full-20261014-hb500-noprevote" / "merged" / "dataset.jsonl"
OUT = Path(__file__).resolve().parent / "figures"
OUT.mkdir(parents=True, exist_ok=True)

rows = [json.loads(l) for l in DATA.open(encoding="utf-8") if l.strip()]
assert len(rows) == 900 and len({r["run_id"] for r in rows}) == 900
assert all(r["trial_ok"] and r["heartbeat"] == "500ms" for r in rows)
DELAYS = ["500µs", "5ms", "20ms"]
DELAY_LABELS = {"500µs": "0.5 ms", "5ms": "5 ms", "20ms": "20 ms"}
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
    sd = float(np.std(xs, ddof=1)) if len(xs) > 1 else 0.0
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
fig, axes = plt.subplots(1, 3, figsize=(7.2, 3.0), sharey=True, layout="constrained")
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
    ax.set_title(f"Per-link delay {DELAY_LABELS[delay]}")
    ax.set_xlabel("Topology")
    ax.grid(axis="y", linestyle=":", alpha=0.5)
    ax.set_ylim(0, 150)

axes[0].set_ylabel("Successful client commits/s")
fig.legend(*axes[0].get_legend_handles_labels(), loc="outside lower center", ncol=2, frameon=False)
fig.suptitle("Throughput from injection to observation end (mean ± sample SD)")
fig.savefig(OUT / "rq1-throughput.pdf")
fig.savefig(OUT / "rq1-throughput.png")
plt.close(fig)

# --- Figure 2: RQ2 latency ---
fig, axes = plt.subplots(1, 3, figsize=(7.2, 3.0), sharey=False, layout="constrained")
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
    ax.set_title(f"Per-link delay {DELAY_LABELS[delay]}")
    ax.set_xlabel("Topology")
    ax.grid(axis="y", linestyle=":", alpha=0.5)

axes[0].set_ylabel("Median commit latency (ms)")
for ax in axes:
    ax.set_ylim(bottom=0)
    ax.annotate("—", (TOPOS.index("T3") - width / 2, 0), xytext=(0, 3),
                textcoords="offset points", ha="center", fontsize=9)
fig.legend(*axes[0].get_legend_handles_labels(), loc="outside lower center", ncol=2, frameon=False)
fig.suptitle("Successful-request median latency (mean ± sample SD over runs)")
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
    labels.append(DELAY_LABELS[delay])
    vals.append(m)
    errs.append(s)
ax.bar(labels, vals, yerr=errs, color=colors["forwarding"], capsize=3, width=0.5)
ax.set_ylabel("Injection to first client commit (s)")
ax.set_xlabel("Per-link delay")
ax.set_title("T3 forwarding (includes harness restart and setup)")
ax.grid(axis="y", linestyle=":", alpha=0.5)
fig.savefig(OUT / "rq1-t3-recovery.pdf")
fig.savefig(OUT / "rq1-t3-recovery.png")
plt.close(fig)

# Keep the report's numeric latency table tied to the same campaign as the plots.
table = [
    r"\begin{table}[H]",
    r"  \centering",
    r"  \caption{Mean of per-run median commit latencies (ms), with the number of runs having successful observe requests. Each condition has 30 validated trials.}",
    r"  \label{tab:latency}",
    r"  \footnotesize",
    r"  \begin{tabular}{llrrrr}",
    r"    \toprule",
    r"    Topology & Delay (ms) & Direct & $n$ & Forwarding & $n$ \\",
    r"    \midrule",
]
for topo in TOPOS:
    for delay in DELAYS:
        cells = []
        for mode in MODES:
            values = series(topo, mode, delay, "latency_median_ms")
            cells.extend([f"{np.mean(values):.2f}" if values else "--", str(len(values))])
        table.append("    " + " & ".join([topo, DELAY_LABELS[delay].split()[0], *cells]) + r" \\")
table.extend([r"    \bottomrule", r"  \end{tabular}", r"\end{table}"])
(OUT.parent / "latency-table.tex").write_text("\n".join(table) + "\n", encoding="utf-8")

print("Wrote figures to", OUT, f"({len(rows)} runs)")
for p in sorted(OUT.glob("*")):
    print(" ", p.name, p.stat().st_size)
