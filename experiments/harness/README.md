# Experiment harness (YAML-driven pilot / full runs)

Orchestrates the research-plan trial loop against Mininet + HashiCorp Raft:

1. Start topology (all links up) at configured per-link delay and mode  
2. Start `raftd` on each host (`-prefer-leader` on YAML `initial_leader`)  
3. Start open-loop `raftclient` inside the client node (default: initial leader)  
4. Warm up and require successful commits  
5. Inject planned link failures; optional connectivity validate  
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
  experiment.yaml          # frozen batch config
  manifest.jsonl           # one status line per trial
  T4_forwarding_d5ms_hb10ms_r01/
    meta.yaml
    status.json
    client.jsonl           # RQ2 latency
    events-A.jsonl …       # RQ1 leader/term/commit
    raftd-A.log …
    client.log
```

## Unit tests (no Mininet)

```bash
go test ./harness/ ./raftnode/ ./raftclient/ ./netinfra/ -count=1
```
