# Results visualizer (RQ1 / RQ2)

Frontend for exploring `dataset.jsonl` from `harness summarize`. Helps answer:

- **RQ1 (Liveness):** in which topologies forwarding restores stable progress vs direct (H1 verdicts, OK throughput, fail rates, recovery, elections, time series)
- **RQ2 (Latency):** how per-link delay and mode affect client commit latency (successful commits only)

## Run

```bash
cd experiments
./bin/harness summarize results/pilot   # refreshes metrics.json + dataset.jsonl
cd viz
npm install
npm run sync-data                       # copies newest dataset into public/data/
npm run dev                             # http://localhost:5173
```

Or use **Open dataset.jsonl** in the UI to load any batch without restarting.

### Live refresh (AWS campaign)

During a fleet run, the laptop bridges private S3 → local `dataset.jsonl` every 5 minutes; the
browser never talks to AWS:

```bash
# Terminal A
npm run sync-data && npm run dev
# open http://localhost:5173/?live=1  (or click Live in the UI)

# Terminal B — shells out to infra/aws `make watch` when present
CAMPAIGN_ID=full-YYYYMMDD npm run watch-s3
```

Live mode re-fetches `/data/dataset.jsonl` and `/data/campaign-status.json` every **5 minutes**
with a cache-bust query param and shows campaign progress (ok / failed / not started, per-repetition
grid, worker `complete.json` count). `make watch` / `npm run sync-data` copy both files into
`public/data/`. Full lifecycle and S3 layout: [`infra/aws/README.md`](../../infra/aws/README.md).

## Views

| Tab | Content |
|-----|---------|
| RQ1 · Liveness | H1 verdict cards, stable rate, OK throughput, fail rate, recovery/elections, OK vs fail time series |
| RQ2 · Latency | Median / p95 latency by topology×delay, Δ(fwd−direct) |
| Runs | Per-run OK/fail counts and fail% |

**Throughput:** `commit_throughput_observe_eps` and `throughput_series[].eps` count **successful** commits only. Failures appear as `client_fail_observe` and `throughput_series[].fail_eps`.

Filters: topology, delay, heartbeat.
