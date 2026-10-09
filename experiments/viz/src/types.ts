export type Mode = "direct" | "forwarding" | string;

export interface ThroughputBucket {
  offset_s: number;
  /** Successful commits per second (failures never included). */
  eps: number;
  /** Failed client submits per second. */
  fail_eps?: number;
}

export interface RunRow {
  run_id: string;
  topology: string;
  mode: Mode;
  delay: string;
  heartbeat: string;
  repetition: number;
  seed: number;
  trial_ok: boolean;
  commit_throughput_observe_eps: number;
  commit_throughput_warmup_eps: number;
  recovery_time_ns: number | null;
  recovery_time_ms: number | null;
  elections_observe: number;
  term_changes_observe: number;
  elections_warmup: number;
  term_changes_warmup: number;
  stable_progress: boolean;
  raft_commits_observe: number;
  client_ok_observe: number;
  client_fail_observe: number;
  client_ok_warmup: number;
  latency_count: number;
  latency_median_ns: number | null;
  latency_p95_ns: number | null;
  latency_mean_ns: number | null;
  latency_median_ms: number | null;
  latency_p95_ms: number | null;
  throughput_series?: ThroughputBucket[];
  host_load_max_1m?: number | null;
  host_load_mean_1m?: number | null;
  host_overloaded?: boolean;
}

export interface Filters {
  topologies: string[];
  delays: string[];
  heartbeats: string[];
}

export interface ConditionKey {
  topology: string;
  mode: string;
  delay: string;
  heartbeat: string;
}

export interface ConditionAgg {
  key: ConditionKey;
  n: number;
  stableFrac: number;
  meanRecoveryMs: number | null;
  meanElections: number;
  meanTermChanges: number;
  /** Mean successful-commit throughput (eps); failures excluded upstream. */
  meanThroughput: number;
  meanLatencyMedianMs: number | null;
  meanLatencyP95Ms: number | null;
  /** client_fail / (ok + fail) in observe window. */
  failRate: number;
  meanOkObserve: number;
  meanFailObserve: number;
}

export type Rq1Verdict =
  | "both_live"
  | "fwd_restores"
  | "direct_only"
  | "both_dead"
  | "incomplete";

export interface RepetitionStatus {
  expected: number;
  present: number;
  ok: number;
  failed: number;
  not_started: number;
}

export interface CampaignStatus {
  campaign_id: string;
  phase: "not_started" | "in_progress" | "workers_done_incomplete" | "complete" | string;
  expected: number;
  present: number;
  ok_count: number;
  failed_count: number;
  missing_count: number;
  not_started: number;
  complete_workers: number;
  worker_count: number;
  repetitions_planned: number;
  conditions_per_repetition: number;
  by_repetition: Record<string, RepetitionStatus>;
}
