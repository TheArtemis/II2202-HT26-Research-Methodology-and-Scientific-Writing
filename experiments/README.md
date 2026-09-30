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
| `collect/` | Metrics derivation + dataset join |
| `viz/` | RQ1/RQ2 results frontend |
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
./bin/harness summarize results/smoke
./bin/harness run configs/pilot.yaml          # 150 trials
./bin/harness summarize results/pilot

# Explore RQ1 / RQ2
cd viz && npm install && npm run sync-data && npm run dev
```

## AWS fleet (full campaign)

Stage-4 `configs/full.yaml` is locked at **30 repetitions (900 trials)**. For the disposable
5-EC2 campaign (S3 shards, local merge, 5-minute live viz), see
[`infra/aws/README.md`](../infra/aws/README.md). During a run: `make watch` in `infra/aws` (or
`npm run watch-s3` under `viz/`) plus `?live=1` in the viz.

See `harness/README.md` and `raftnode/README.md` for details.
