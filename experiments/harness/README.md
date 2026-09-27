# Experiment harness (YAML-driven pilot / full runs)

Orchestrates the research-plan trial loop against Mininet + HashiCorp Raft:

1. Start topology with **all links up**
2. Start `raftd` (natural election; no forced preferred leader)
3. Wait until healthy and **any** leader exists; record it in `status.leader`
4. Optional warmup to confirm the cluster is initially stable under that leader
5. Inject cuts **remapped around the actual leader** (YAML pattern relative to `initial_leader`); for T3+`seed_t3`, reseed divergent logs under the partition with the tip on that leader
6. Observe for configured duration  
7. Stop processes, archive JSONL/logs under `output_dir/<run_id>/`, tear down  

## Configs

| File | Purpose | Trials |
|------|---------|--------|
| `configs/smoke.yaml` | End-to-end sanity (T4 × 2 modes) | 2 |
| `configs/pilot.yaml` | Stage 3 pilot (5 reps) | **150** |
| `configs/full.yaml` | Stage 4 main (20 reps) | **600** |
| `configs/heartbeat-sweep.yaml` | Optional heartbeat IV | 60 |

Edit YAML to change rate, observe window, topologies, etc. Heartbeat defaults to
`10ms` so the main matrix matches the plan’s 600-run target
(`5 × 2 × 3 × 20`). Cross `[5ms, 10ms, 500ms]` via `heartbeat-sweep.yaml`
(HashiCorp clamps values below 5ms).

## Build

```bash
cd experiments
go build -o bin/raftd ./cmd/raftd
go build -o bin/raftclient ./cmd/raftclient
go build -o bin/harness ./cmd/harness
go build -o bin/netctl ./cmd/netctl
```

Binaries must be on a filesystem path visible inside Mininet hosts (usual for
local Mininet).

## Pilot sequence (Linux VM + root)

```bash
# 1. Daemon
sudo python3 netinfra/mininetd/server.py --socket /tmp/mininetd.sock &

# 2. Validate expansion (no Mininet needed)
./bin/harness dry-run configs/smoke.yaml
./bin/harness list configs/pilot.yaml | head

# 3. Smoke
./bin/harness run configs/smoke.yaml

# 4. Pilot (continue_on_error: true — failed trials stay in manifest)
./bin/harness run configs/pilot.yaml

# Resume / subset
./bin/harness run configs/pilot.yaml --from 10 --limit 5
```

## Per-run archive layout

```
results/pilot/
  experiment.yaml
  manifest.jsonl
  dataset.jsonl / dataset.csv   # after: harness summarize
  T4_forwarding_d5ms_hb10ms_r01/
    meta.yaml
    status.json
    timeline.json               # phase markers (inject_at_ns, …)
    hostload.jsonl              # controller load samples
    client.jsonl                # RQ2 raw
    client_enriched.jsonl       # + phase tags
    events-A.jsonl …            # RQ1 raw (leader/term/election/commit)
    metrics.json                # derived DVs
    raftd-*.log / client.log
```

After a batch:

```bash
./bin/harness summarize results/pilot
```

## Unit tests (no Mininet)

```bash
go test ./harness/ ./raftnode/ ./raftclient/ ./netinfra/ -count=1
```
