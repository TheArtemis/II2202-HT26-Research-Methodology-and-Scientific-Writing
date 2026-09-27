# Results visualizer (RQ1 / RQ2)

Frontend for exploring `dataset.jsonl` from `harness summarize`. Helps answer:

- **RQ1 (Liveness):** in which topologies forwarding restores stable progress vs direct
- **RQ2 (Latency):** how per-link delay and mode affect client commit latency

## Run

```bash
cd experiments
./bin/harness summarize results/smoke   # or results/pilot
cd viz
npm install
npm run sync-data                       # copies newest dataset into public/data/
npm run dev                             # http://localhost:5173
```

Or use **Open dataset.jsonl** in the UI to load any batch without restarting.

## Views

| Tab | Content |
|-----|---------|
| RQ1 · Liveness | Stable-progress bars (direct vs fwd), recovery & elections, throughput time series |
| RQ2 · Latency | Median / p95 latency by topology×delay, Δ(fwd−direct) |
| Runs | Raw per-run table with all DVs |

Filters: topology, delay, heartbeat.
