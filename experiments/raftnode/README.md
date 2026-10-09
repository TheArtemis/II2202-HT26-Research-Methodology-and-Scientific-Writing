# Raft replicas + open-loop client

HashiCorp [raft](https://github.com/hashicorp/raft) replicas and a single open-loop
client, aligned with the research plan (HashiCorp Raft, Mininet identity IPs,
client-side commit latency, Raft event logs).

## Layout

```
experiments/
  cmd/raftd/          # replica binary (one per Mininet host)
  cmd/raftclient/     # open-loop workload client
  raftnode/           # Raft + FSM + HTTP Apply API + events
  raftclient/         # open-loop sender + JSONL metrics
```

## Ports (per identity IP)

| Port | Role |
|------|------|
| 7000 | HashiCorp Raft TCP transport |
| 7001 | HTTP Apply / stats API |

## Build

```bash
cd experiments
go build -o bin/raftd ./cmd/raftd
go build -o bin/raftclient ./cmd/raftclient
go build -o bin/netctl ./cmd/netctl
```

Copy `bin/raftd` into a path visible inside Mininet hosts (e.g. shared mount).

## Start replicas (after `netctl up`)

```bash
PEERS=$(./bin/netctl peers t4)          # e.g. A=10.0.0.1,B=10.0.0.2,C=10.0.0.3
# Prefer the YAML initial_leader so warm-up commits succeed before inject.

# On host B (T4 initial_leader), then A and C — example via netctl exec:
./bin/netctl exec B -- /path/raftd -id B -bind 10.0.0.2 -peers "$PEERS" \
  -heartbeat 10ms -prefer-leader -events /tmp/raft-B.jsonl &
./bin/netctl exec A -- /path/raftd -id A -bind 10.0.0.1 -peers "$PEERS" \
  -heartbeat 10ms -events /tmp/raft-A.jsonl &
./bin/netctl exec C -- /path/raftd -id C -bind 10.0.0.3 -peers "$PEERS" \
  -heartbeat 10ms -events /tmp/raft-C.jsonl &
```

Flags of note:

- `-heartbeat` — research IV (1 / 10 / 500 ms). HashiCorp enforces a **5 ms floor**.
- `-prefer-leader` — shorter election timeout; use on the topology `initial_leader`.
- `-seed-log 1,1,2,2` — T3 constrained-election log terms (per node; see plan).
- `-bootstrap` — default true; identical full-voter config on every replica.
- Empty `-data-dir` → in-memory stores (clean per run).

### T3 seed logs

| Node | `-seed-log` |
|------|-------------|
| A | `1,1` |
| B | `1,1,2` |
| C | `1,1,2,2` |
| D | `1,1,2` |
| E | `1,1` |

## Open-loop client

Submits at a fixed rate **without waiting** for prior Apply responses (research plan).

```bash
./bin/raftclient -peers "$PEERS" -leader B -rate 100 -duration 30s \
  -size 64 -out /tmp/client.jsonl
```

Each JSONL line:

```json
{"id":"1","submit_ns":...,"commit_ns":...,"latency_ns":...,"ok":true,"index":5,"term":2,"leader_id":"B"}
```

## Raft event log (JSONL)

```json
{"ts":"...","type":"leader","node":"B","leader":"B","term":2}
{"ts":"...","type":"commit","node":"B","index":5,"term":2}
{"ts":"...","type":"election","node":"A","term":3}
```

## HTTP API (on each replica)

- `POST /apply` `{"id":"...","payload":"..."}` — leader only; others return 503 + `X-Raft-Leader-ID`
- `GET /stats` — state, term, leader, commit/applied indexes
- `GET /health`

## Unit tests (no Mininet)

```bash
cd experiments
go test ./raftnode/ ./raftclient/ ./netinfra/ -count=1
```
