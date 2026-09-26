# Experiments

Go + Mininet 2.3.0 testbed for the II2202 research plan:

**Transparent multi-hop routing for Raft under partial network partitions.**

## Packages

| Path | Role |
|------|------|
| `netinfra/` | Mininet control plane (T0–T4 YAML topologies) |
| `raftnode/` + `cmd/raftd` | HashiCorp Raft replicas |
| `raftclient/` + `cmd/raftclient` | Open-loop workload client |
| `harness/` + `cmd/harness` | YAML-driven trial runner (pilot / full) |
| `configs/` | `smoke.yaml`, `pilot.yaml`, `full.yaml`, … |

## Quick start (Linux VM)

```bash
cd experiments
go build -o bin/raftd ./cmd/raftd
go build -o bin/raftclient ./cmd/raftclient
go build -o bin/harness ./cmd/harness
go build -o bin/netctl ./cmd/netctl

sudo python3 netinfra/mininetd/server.py --socket /tmp/mininetd.sock &

./bin/harness dry-run configs/pilot.yaml
./bin/harness run configs/smoke.yaml          # 2 trials
./bin/harness run configs/pilot.yaml          # 150 trials
```

See `harness/README.md` and `raftnode/README.md` for details.
