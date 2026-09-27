import { useCallback, useEffect, useMemo, useState } from "react";
import {
  aggregateByCondition,
  applyFilters,
  avgThroughputSeries,
  fmtNum,
  fmtPct,
  parseDatasetJSONL,
  rq1Pairs,
  rq2LatencyByDelay,
  uniqueSorted,
} from "./aggregate";
import { GroupedBarChart, HeatCell, LineChart, SimpleBars } from "./charts";
import type { Filters, RunRow } from "./types";

const DIRECT = "#1d4ed8";
const FWD = "#b45309";

const emptyFilters: Filters = { topologies: [], delays: [], heartbeats: [] };

export default function App() {
  const [rows, setRows] = useState<RunRow[]>([]);
  const [source, setSource] = useState("none");
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [tab, setTab] = useState<"rq1" | "rq2" | "runs">("rq1");

  const loadText = useCallback((text: string, label: string) => {
    const parsed = parseDatasetJSONL(text);
    if (parsed.length === 0) {
      setError("No rows parsed from JSONL.");
      return;
    }
    setRows(parsed);
    setSource(label);
    setError(null);
    setFilters({
      topologies: uniqueSorted(parsed.map((r) => r.topology)),
      delays: uniqueSorted(parsed.map((r) => r.delay)),
      heartbeats: uniqueSorted(parsed.map((r) => r.heartbeat)),
    });
  }, []);

  useEffect(() => {
    fetch("/data/dataset.jsonl")
      .then((r) => (r.ok ? r.text() : Promise.reject(new Error(`${r.status}`))))
      .then((t) => loadText(t, "public/data/dataset.jsonl"))
      .catch(() => {
        /* wait for upload */
      });
  }, [loadText]);

  const onFile = (file: File | null) => {
    if (!file) return;
    file.text().then((t) => loadText(t, file.name));
  };

  const allTopos = useMemo(() => uniqueSorted(rows.map((r) => r.topology)), [rows]);
  const allDelays = useMemo(() => uniqueSorted(rows.map((r) => r.delay)), [rows]);
  const allHBs = useMemo(() => uniqueSorted(rows.map((r) => r.heartbeat)), [rows]);

  const filtered = useMemo(() => applyFilters(rows, filters), [rows, filters]);
  const aggs = useMemo(() => aggregateByCondition(filtered), [filtered]);
  const pairs = useMemo(() => rq1Pairs(aggs), [aggs]);
  const latRows = useMemo(() => rq2LatencyByDelay(aggs), [aggs]);
  const thrSeries = useMemo(() => avgThroughputSeries(filtered), [filtered]);

  const stableDirect = pairs.filter((p) => p.directStable != null);
  const restored = pairs.filter(
    (p) => p.directStable != null && p.fwdStable != null && p.directStable < 0.5 && p.fwdStable >= 0.5,
  );
  const meanLatDelta = (() => {
    const deltas = latRows
      .filter((r) => r.directMed != null && r.fwdMed != null)
      .map((r) => (r.fwdMed as number) - (r.directMed as number));
    if (deltas.length === 0) return null;
    return deltas.reduce((a, b) => a + b, 0) / deltas.length;
  })();

  const toggle = (key: keyof Filters, value: string) => {
    setFilters((prev) => {
      const set = new Set(prev[key]);
      if (set.has(value)) set.delete(value);
      else set.add(value);
      return { ...prev, [key]: [...set].sort() };
    });
  };

  return (
    <div className="app">
      <header className="hero">
        <p className="brand">II2202 · Raft under partial partitions</p>
        <h1>Results explorer</h1>
        <p className="lede">
          Load a <code>dataset.jsonl</code> from <code>harness summarize</code> to compare direct vs
          forwarding for RQ1 (liveness) and RQ2 (commit latency).
        </p>
        <div className="load-row">
          <label className="file-btn">
            Open dataset.jsonl
            <input
              type="file"
              accept=".jsonl,.json,text/plain"
              onChange={(e) => onFile(e.target.files?.[0] ?? null)}
            />
          </label>
          <span className="source">
            {rows.length > 0 ? `${rows.length} runs · ${source}` : "No data loaded yet"}
          </span>
        </div>
        {error && <p className="error">{error}</p>}
      </header>

      {rows.length > 0 && (
        <>
          <section className="filters">
            <FilterGroup
              title="Topology"
              options={allTopos}
              selected={filters.topologies}
              onToggle={(v) => toggle("topologies", v)}
            />
            <FilterGroup
              title="Delay"
              options={allDelays}
              selected={filters.delays}
              onToggle={(v) => toggle("delays", v)}
            />
            <FilterGroup
              title="Heartbeat"
              options={allHBs}
              selected={filters.heartbeats}
              onToggle={(v) => toggle("heartbeats", v)}
            />
          </section>

          <nav className="tabs" aria-label="Research questions">
            <button type="button" className={tab === "rq1" ? "active" : ""} onClick={() => setTab("rq1")}>
              RQ1 · Liveness
            </button>
            <button type="button" className={tab === "rq2" ? "active" : ""} onClick={() => setTab("rq2")}>
              RQ2 · Latency
            </button>
            <button type="button" className={tab === "runs" ? "active" : ""} onClick={() => setTab("runs")}>
              Runs
            </button>
          </nav>

          {tab === "rq1" && (
            <section className="panel">
              <h2>RQ1 — When does forwarding restore stable progress?</h2>
              <p className="hint">
                Stable progress = recovered after inject, observe-window commits, limited election churn
                (see <code>metrics.stable_progress</code>).
              </p>

              <div className="stat-row">
                <div className="stat">
                  <span className="stat-label">Conditions compared</span>
                  <strong>{pairs.length}</strong>
                </div>
                <div className="stat">
                  <span className="stat-label">Forwarding restores (direct &lt;50%, fwd ≥50%)</span>
                  <strong>{restored.length === 0 ? "—" : restored.map((r) => r.topology).join(", ")}</strong>
                </div>
                <div className="stat">
                  <span className="stat-label">Filtered runs</span>
                  <strong>{filtered.length}</strong>
                </div>
              </div>

              <GroupedBarChart
                title="Stable-progress rate by topology (mean over reps)"
                yLabel="Fraction stable"
                groups={pairs.map((p) => ({
                  label: p.topology,
                  series: [
                    { name: "direct", value: p.directStable, color: DIRECT },
                    { name: "forwarding", value: p.fwdStable, color: FWD },
                  ],
                }))}
              />

              <div className="table-wrap">
                <table>
                  <caption>Direct vs forwarding — stable fraction and recovery</caption>
                  <thead>
                    <tr>
                      <th>Topology</th>
                      <th>Delay</th>
                      <th>HB</th>
                      <th>Direct stable</th>
                      <th>Fwd stable</th>
                      <th>Δ (fwd−dir)</th>
                      <th>n</th>
                    </tr>
                  </thead>
                  <tbody>
                    {pairs.map((p) => (
                      <tr key={`${p.topology}|${p.delay}|${p.heartbeat}`}>
                        <td>{p.topology}</td>
                        <td>{p.delay}</td>
                        <td>{p.heartbeat}</td>
                        <HeatCell value={p.directStable} label="direct" />
                        <HeatCell value={p.fwdStable} label="forwarding" />
                        <td className={p.delta != null && p.delta > 0 ? "pos" : p.delta != null && p.delta < 0 ? "neg" : ""}>
                          {fmtPct(p.delta)}
                        </td>
                        <td>
                          {p.directN}/{p.fwdN}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <div className="split">
                <SimpleBars
                  title="Mean recovery time after inject (ms)"
                  unit=" ms"
                  items={aggs
                    .filter((a) => a.meanRecoveryMs != null)
                    .map((a) => ({
                      label: `${a.key.topology} ${a.key.mode}`,
                      value: a.meanRecoveryMs as number,
                      color: a.key.mode === "direct" ? DIRECT : FWD,
                    }))}
                />
                <SimpleBars
                  title="Mean elections in observe window"
                  unit=""
                  items={aggs.map((a) => ({
                    label: `${a.key.topology} ${a.key.mode}`,
                    value: a.meanElections,
                    color: a.key.mode === "direct" ? DIRECT : FWD,
                  }))}
                />
              </div>

              {thrSeries.length > 1 && (
                <LineChart
                  title="Commit throughput over observe window (avg commits/s)"
                  yLabel="commits/s"
                  xLabel="Seconds since inject"
                  series={[
                    {
                      name: "direct",
                      color: DIRECT,
                      points: thrSeries.map((p) => ({ x: p.t, y: p.direct })),
                    },
                    {
                      name: "forwarding",
                      color: FWD,
                      points: thrSeries.map((p) => ({ x: p.t, y: p.fwd })),
                    },
                  ]}
                />
              )}

              {stableDirect.length > 0 && (
                <p className="caption">
                  Source: harness <code>dataset.jsonl</code> · filtered subset · bar height = fraction of
                  repetitions with <code>stable_progress</code>.
                </p>
              )}
            </section>
          )}

          {tab === "rq2" && (
            <section className="panel">
              <h2>RQ2 — How does path delay affect commit latency?</h2>
              <p className="hint">
                Client-side median and p95 latency in the post-inject observe window. T1 is a control
                (leader keeps a direct quorum); T2–T4 may require a forwarded follower for majority.
              </p>

              <div className="stat-row">
                <div className="stat">
                  <span className="stat-label">Mean Δ median latency (fwd − direct)</span>
                  <strong>{meanLatDelta == null ? "—" : `${meanLatDelta >= 0 ? "+" : ""}${fmtNum(meanLatDelta)} ms`}</strong>
                </div>
                <div className="stat">
                  <span className="stat-label">Latency samples (observe OK)</span>
                  <strong>{filtered.reduce((s, r) => s + r.latency_count, 0)}</strong>
                </div>
              </div>

              <GroupedBarChart
                title="Median commit latency by topology (ms)"
                yLabel="Latency (ms)"
                groups={latRows.map((r) => ({
                  label: `${r.topology}\n${r.delay}`,
                  series: [
                    { name: "direct median", value: r.directMed, color: DIRECT },
                    { name: "forwarding median", value: r.fwdMed, color: FWD },
                  ],
                }))}
              />

              <GroupedBarChart
                title="p95 commit latency by topology (ms)"
                yLabel="Latency (ms)"
                groups={latRows.map((r) => ({
                  label: `${r.topology} ${r.delay}`,
                  series: [
                    { name: "direct p95", value: r.directP95, color: DIRECT },
                    { name: "forwarding p95", value: r.fwdP95, color: FWD },
                  ],
                }))}
              />

              <div className="table-wrap">
                <table>
                  <caption>Latency summary (mean of per-run medians / p95s)</caption>
                  <thead>
                    <tr>
                      <th>Topology</th>
                      <th>Delay</th>
                      <th>Direct med</th>
                      <th>Fwd med</th>
                      <th>Δ med</th>
                      <th>Direct p95</th>
                      <th>Fwd p95</th>
                    </tr>
                  </thead>
                  <tbody>
                    {latRows.map((r) => {
                      const dMed =
                        r.directMed != null && r.fwdMed != null ? r.fwdMed - r.directMed : null;
                      return (
                        <tr key={`${r.topology}|${r.delay}`}>
                          <td>{r.topology}</td>
                          <td>{r.delay}</td>
                          <td>{fmtNum(r.directMed)}</td>
                          <td>{fmtNum(r.fwdMed)}</td>
                          <td className={dMed != null && dMed > 0.5 ? "neg" : ""}>
                            {dMed == null ? "—" : `${dMed >= 0 ? "+" : ""}${fmtNum(dMed)}`}
                          </td>
                          <td>{fmtNum(r.directP95)}</td>
                          <td>{fmtNum(r.fwdP95)}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
              <p className="caption">
                Values in ms · client timestamps only (RQ2) · compare forwarding cost when delay and
                topology force multi-hop quorum members.
              </p>
            </section>
          )}

          {tab === "runs" && (
            <section className="panel">
              <h2>Per-run table</h2>
              <div className="table-wrap wide">
                <table>
                  <thead>
                    <tr>
                      <th>Run</th>
                      <th>Topo</th>
                      <th>Mode</th>
                      <th>Delay</th>
                      <th>Stable</th>
                      <th>Recovery ms</th>
                      <th>Elect</th>
                      <th>Terms</th>
                      <th>Thr put</th>
                      <th>Lat med</th>
                      <th>Lat p95</th>
                      <th>OK/Fail</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filtered.map((r) => (
                      <tr key={r.run_id}>
                        <td className="mono">{r.run_id}</td>
                        <td>{r.topology}</td>
                        <td>{r.mode}</td>
                        <td>{r.delay}</td>
                        <td>{r.stable_progress ? "yes" : "no"}</td>
                        <td>{fmtNum(r.recovery_time_ms)}</td>
                        <td>{r.elections_observe}</td>
                        <td>{r.term_changes_observe}</td>
                        <td>{fmtNum(r.commit_throughput_observe_eps, 1)}</td>
                        <td>{fmtNum(r.latency_median_ms)}</td>
                        <td>{fmtNum(r.latency_p95_ms)}</td>
                        <td>
                          {r.client_ok_observe}/{r.client_fail_observe}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}
        </>
      )}
    </div>
  );
}

function FilterGroup({
  title,
  options,
  selected,
  onToggle,
}: {
  title: string;
  options: string[];
  selected: string[];
  onToggle: (v: string) => void;
}) {
  return (
    <div className="filter-group">
      <span className="filter-title">{title}</span>
      <div className="chips">
        {options.map((o) => (
          <button
            key={o}
            type="button"
            className={selected.includes(o) ? "chip on" : "chip"}
            onClick={() => onToggle(o)}
          >
            {o}
          </button>
        ))}
      </div>
    </div>
  );
}
