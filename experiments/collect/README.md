# Data collection (`collect/`)

Implements research-plan measurement artifacts and dependent-variable derivation.

## Raw artifacts (per run, written by harness)

| File | Research-plan mapping |
|------|----------------------|
| `client.jsonl` | Request id, submit + commit timestamps, latency |
| `client_enriched.jsonl` | Same + `phase` (`warmup` / `observe`) |
| `events-*.jsonl` | Leader, term, election, commit with `ts` + `ts_ns` |
| `timeline.json` | Phase markers (`inject_at_ns`, warmup/observe bounds) |
| `hostload.jsonl` | Controller load1/5/15 + MemAvailable (threat mitigation) |
| `status.json` / `meta.yaml` | IVs, seed, inject time |
| `metrics.json` | Derived DVs for this run |

## Dependent variables (`metrics.json`)

| Field | DV |
|-------|-----|
| `commit_throughput_observe_eps` | Commit throughput (RQ1) |
| `recovery_time_ms` | Failure → first post-inject commit (RQ1) |
| `elections_observe` / `term_changes_observe` | Election / term counts (RQ1) |
| `stable_progress` | Heuristic liveness flag (RQ1) |
| `latency_median_ms` / `latency_p95_ms` | Commit latency (RQ2) |
| `throughput_series` | Commits/sec over observe window |

## Join dataset

```bash
./bin/harness summarize results/pilot
# → results/pilot/dataset.jsonl
# → results/pilot/dataset.csv
```

Re-analyzes each run directory (writes/refreshes `metrics.json`) and concatenates
rows joinable by `topology,mode,delay,heartbeat,repetition`.
