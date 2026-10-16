import type { ConditionAgg, ConditionKey, Filters, Rq1Verdict, RunRow } from "./types";

export function parseDatasetJSONL(text: string): RunRow[] {
  const rows: RunRow[] = [];
  for (const line of text.split(/\r?\n/)) {
    const t = line.trim();
    if (!t) continue;
    try {
      rows.push(JSON.parse(t) as RunRow);
    } catch {
      /* skip bad line */
    }
  }
  return rows;
}

export function uniqueSorted(vals: string[]): string[] {
  return [...new Set(vals)].sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
}

export function applyFilters(rows: RunRow[], f: Filters): RunRow[] {
  return rows.filter(
    (r) =>
      (f.topologies.length === 0 || f.topologies.includes(r.topology)) &&
      (f.delays.length === 0 || f.delays.includes(r.delay)) &&
      (f.heartbeats.length === 0 || f.heartbeats.includes(r.heartbeat)),
  );
}

function mean(nums: number[]): number | null {
  if (nums.length === 0) return null;
  return nums.reduce((a, b) => a + b, 0) / nums.length;
}

export function conditionKey(r: RunRow): string {
  return `${r.topology}|${r.mode}|${r.delay}|${r.heartbeat}`;
}

export function aggregateByCondition(rows: RunRow[]): ConditionAgg[] {
  const groups = new Map<string, RunRow[]>();
  for (const r of rows) {
    const k = conditionKey(r);
    const g = groups.get(k);
    if (g) g.push(r);
    else groups.set(k, [r]);
  }
  const out: ConditionAgg[] = [];
  for (const group of groups.values()) {
    const head = group[0];
    const key: ConditionKey = {
      topology: head.topology,
      mode: head.mode,
      delay: head.delay,
      heartbeat: head.heartbeat,
    };
    const recoveries = group
      .map((r) => r.recovery_time_ms)
      .filter((v): v is number => v != null && Number.isFinite(v));
    const latMed = group
      .map((r) => r.latency_median_ms)
      .filter((v): v is number => v != null && Number.isFinite(v));
    const latP95 = group
      .map((r) => r.latency_p95_ms)
      .filter((v): v is number => v != null && Number.isFinite(v));
    const fails = group.reduce((s, r) => s + (r.client_fail_observe || 0), 0);
    const oks = group.reduce((s, r) => s + (r.client_ok_observe || 0), 0);
    out.push({
      key,
      n: group.length,
      stableFrac: group.filter((r) => r.stable_progress).length / group.length,
      meanRecoveryMs: mean(recoveries),
      meanElections: mean(group.map((r) => r.elections_observe)) ?? 0,
      meanTermChanges: mean(group.map((r) => r.term_changes_observe)) ?? 0,
      meanThroughput: mean(group.map((r) => r.commit_throughput_observe_eps)) ?? 0,
      meanLatencyMedianMs: mean(latMed),
      meanLatencyP95Ms: mean(latP95),
      failRate: oks + fails > 0 ? fails / (oks + fails) : 0,
      meanOkObserve: mean(group.map((r) => r.client_ok_observe || 0)) ?? 0,
      meanFailObserve: mean(group.map((r) => r.client_fail_observe || 0)) ?? 0,
    });
  }
  return out.sort((a, b) => {
    const ta = `${a.key.topology}|${a.key.delay}|${a.key.heartbeat}|${a.key.mode}`;
    const tb = `${b.key.topology}|${b.key.delay}|${b.key.heartbeat}|${b.key.mode}`;
    return ta.localeCompare(tb, undefined, { numeric: true });
  });
}

export function rq1Pairs(aggs: ConditionAgg[]): {
  topology: string;
  delay: string;
  heartbeat: string;
  directStable: number | null;
  fwdStable: number | null;
  delta: number | null;
  directN: number;
  fwdN: number;
  directThr: number | null;
  fwdThr: number | null;
  directFailRate: number | null;
  fwdFailRate: number | null;
  directRecoveryMs: number | null;
  fwdRecoveryMs: number | null;
  verdict: Rq1Verdict;
}[] {
  const map = new Map<string, { direct?: ConditionAgg; fwd?: ConditionAgg }>();
  for (const a of aggs) {
    const k = `${a.key.topology}|${a.key.delay}|${a.key.heartbeat}`;
    const slot = map.get(k) ?? {};
    if (a.key.mode === "direct") slot.direct = a;
    if (a.key.mode === "forwarding") slot.fwd = a;
    map.set(k, slot);
  }
  const out = [];
  for (const [k, v] of map) {
    const [topology, delay, heartbeat] = k.split("|");
    const d = v.direct?.stableFrac ?? null;
    const f = v.fwd?.stableFrac ?? null;
    out.push({
      topology,
      delay,
      heartbeat,
      directStable: d,
      fwdStable: f,
      delta: d != null && f != null ? f - d : null,
      directN: v.direct?.n ?? 0,
      fwdN: v.fwd?.n ?? 0,
      directThr: v.direct?.meanThroughput ?? null,
      fwdThr: v.fwd?.meanThroughput ?? null,
      directFailRate: v.direct?.failRate ?? null,
      fwdFailRate: v.fwd?.failRate ?? null,
      directRecoveryMs: v.direct?.meanRecoveryMs ?? null,
      fwdRecoveryMs: v.fwd?.meanRecoveryMs ?? null,
      verdict: rq1Verdict(d, f),
    });
  }
  return out.sort((a, b) => a.topology.localeCompare(b.topology));
}

/** RQ1 H1-style classification from mean stable fractions. */
export function rq1Verdict(directStable: number | null, fwdStable: number | null): Rq1Verdict {
  if (directStable == null || fwdStable == null) return "incomplete";
  const dLive = directStable >= 0.5;
  const fLive = fwdStable >= 0.5;
  if (!dLive && fLive) return "fwd_restores";
  if (dLive && fLive) return "both_live";
  if (dLive && !fLive) return "direct_only";
  return "both_dead";
}

export function verdictLabel(v: Rq1Verdict): string {
  switch (v) {
    case "fwd_restores":
      return "Forwarding restores";
    case "both_live":
      return "Both live";
    case "direct_only":
      return "Direct only";
    case "both_dead":
      return "Neither stable";
    default:
      return "Incomplete pair";
  }
}

/** RQ2: latency by delay × mode (optionally within topology). */
export function rq2LatencyByDelay(aggs: ConditionAgg[]): {
  delay: string;
  topology: string;
  directMed: number | null;
  fwdMed: number | null;
  directP95: number | null;
  fwdP95: number | null;
}[] {
  const map = new Map<string, { direct?: ConditionAgg; fwd?: ConditionAgg }>();
  for (const a of aggs) {
    const k = `${a.key.topology}|${a.key.delay}|${a.key.heartbeat}`;
    const slot = map.get(k) ?? {};
    if (a.key.mode === "direct") slot.direct = a;
    if (a.key.mode === "forwarding") slot.fwd = a;
    map.set(k, slot);
  }
  const out = [];
  for (const [k, v] of map) {
    const [topology, delay] = k.split("|");
    out.push({
      delay,
      topology,
      directMed: v.direct?.meanLatencyMedianMs ?? null,
      fwdMed: v.fwd?.meanLatencyMedianMs ?? null,
      directP95: v.direct?.meanLatencyP95Ms ?? null,
      fwdP95: v.fwd?.meanLatencyP95Ms ?? null,
    });
  }
  return out.sort((a, b) =>
    `${a.topology}|${a.delay}`.localeCompare(`${b.topology}|${b.delay}`, undefined, { numeric: true }),
  );
}

export function avgThroughputSeries(rows: RunRow[]): {
  t: number;
  directOk: number | null;
  fwdOk: number | null;
  directFail: number | null;
  fwdFail: number | null;
}[] {
  const byMode = {
    direct: rows.filter((r) => r.mode === "direct"),
    forwarding: rows.filter((r) => r.mode === "forwarding"),
  };
  const maxT = Math.max(
    0,
    ...rows.flatMap((r) => (r.throughput_series ?? []).map((b) => b.offset_s)),
  );
  const out = [];
  for (let t = 0; t <= maxT; t++) {
    const avgAt = (mode: "direct" | "forwarding", field: "eps" | "fail_eps") => {
      const vals = byMode[mode]
        .map((r) => {
          const b = r.throughput_series?.find((x) => x.offset_s === t);
          if (!b) return undefined;
          return field === "eps" ? b.eps : (b.fail_eps ?? 0);
        })
        .filter((v): v is number => v != null);
      if (vals.length === 0) return null;
      return vals.reduce((a, b) => a + b, 0) / vals.length;
    };
    out.push({
      t,
      directOk: avgAt("direct", "eps"),
      fwdOk: avgAt("forwarding", "eps"),
      directFail: avgAt("direct", "fail_eps"),
      fwdFail: avgAt("forwarding", "fail_eps"),
    });
  }
  return out;
}

export function fmtNum(v: number | null | undefined, digits = 2): string {
  if (v == null || Number.isNaN(v)) return "—";
  return v.toFixed(digits);
}

export function fmtPct(v: number | null | undefined): string {
  if (v == null || Number.isNaN(v)) return "—";
  return `${(v * 100).toFixed(0)}%`;
}
