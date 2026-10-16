# Network infrastructure (Mininet + Go)

Go-controlled Mininet 2.3.0 testbed for topologies **T0–T4**. A thin Python
daemon owns the Mininet object; the experiment harness and `netctl` CLI talk to
it over localhost JSON-RPC. Raft binaries and measurement loops stay out of
Python.

## Layout

```
experiments/
  go.mod
  cmd/netctl/main.go          # operator / harness CLI
  cmd/raftd/main.go           # HashiCorp Raft replica
  cmd/raftclient/main.go      # open-loop client
  raftnode/                   # Raft node library
  raftclient/                 # open-loop client library
  netinfra/
    controller.go             # Controller interface
    client.go                 # JSON-RPC client
    spec.go                   # YAML load + validation
    validate.go               # reachability / delay gates
    mode.go / session.go
    specs/t0.yaml … t4.yaml
    mininetd/
      server.py               # JSON-RPC daemon
      topology_builder.py     # TCLink + identity /30 + routes
      requirements.txt
```

## Prerequisites (Linux VM)

- Root privileges (network namespaces)
- [Mininet 2.3.0](https://github.com/mininet/mininet/releases/tag/2.3.0)
  (`mn --version` → `2.3.0`)
- Go 1.22+
- `iproute2`, `iputils-ping`

This layer **cannot** be exercised on Windows without a Linux VM/WSL2 + root
Mininet setup.

## Addressing model

| Role | Example |
|------|---------|
| Identity (Raft bind/advertise, on `lo`) | A=`10.0.0.1` … from `identity_cidr` |
| Per-link /30 | `10.100.<i>.0/30` on each TCLink |

- **Direct:** `ip_forward=0`; routes only to on-link neighbors’ identity IPs.
- **Forwarding (simplified NIFTY overlay):** `ip_forward=1`; underlay is a
  **full mesh**. After inject brings down `failed: true` links, the daemon
  installs shortest-path `/32` detours so traffic to a peer is rewritten via an
  intermediate neighbour when the direct edge is down (same idea as NIFTY’s
  “reroute around the partition,” without OpenFlow/DV). Isolated components
  (e.g. T3’s C) stay unreachable — there is no physical path to repair.
  `rp_filter=0` and `accept_local=1` so multi-hop packets to identity addresses on `lo` work.
  Connectivity checks bind ping to the source identity (`ping -I`) so return traffic
  uses identity routes rather than the per-link `/30` (which remote hosts cannot route).
  Host routes also set `src=<identity>` so Raft TCP dials use the same returnable
  address (without this, spoke↔spoke overlay RPCs time out under T3).

`Start` brings **all** physical mesh links up at the chosen delay. Planned cuts
(`failed: true` in YAML) are applied later with `InjectPlannedFailures` /
`InjectAroundLeader` / `netctl inject` after warm-up; overlay routes refresh on
each link change.

## Run mininetd

```bash
cd experiments/netinfra/mininetd
sudo python3 server.py --socket /tmp/mininetd.sock
# or: sudo python3 server.py --tcp 127.0.0.1:17300
```

## Build and run netctl

```bash
cd experiments
go mod tidy
go build -o bin/netctl ./cmd/netctl
```

```bash
# Start T4, all links up, forwarding, 5 ms per-link delay
./bin/netctl up t4 --mode forwarding --delay 5ms

# Warm-up / start Raft on hosts (example)
./bin/netctl exec A -- ip addr show lo

# Inject planned failures (T4: B--C down), then validate QA gates
./bin/netctl inject
./bin/netctl validate

# Manual link / mode / delay
./bin/netctl link down B C
./bin/netctl mode direct
./bin/netctl delay 20ms

# Tear down (Stop + mn -c)
./bin/netctl down
```

Environment:

- `MININETD_ADDR` — unix path or `host:port` (default `/tmp/mininetd.sock`, TCP fallback `127.0.0.1:17300`)
- `NETINFRA_SESSION` — session JSON path (default `/tmp/netinfra-session.json`)

## Harness usage (Go)

```go
c := netinfra.NewClient(netinfra.ClientOptions{})
spec, _ := netinfra.LoadNamedSpec("t4")
ctx := context.Background()

// 1. Clean topology, all links up
_ = c.Start(ctx, spec, netinfra.Forwarding, 5*time.Millisecond)

// 2. Start Raft via ExecOn(node, "..."), warm-up...

// 3. Inject planned failures, measure
_ = c.InjectPlannedFailures()
_ = c.Validate(ctx)

// 4. Tear down
_ = c.Close()
```

`Controller` surface: `Start`, `SetLink`, `SetDelay`, `SetMode`, `ExecOn`,
`Validate`, `InjectPlannedFailures`, `Close`.

## Validation gates

After planned failures are injected:

1. **T4 pilot:** direct → B cannot ping C identity; forwarding → B reaches C via A; 2-hop RTT ≈ `4 ×` per-link delay.
2. Connectivity matrix matches Direct (neighbors only) vs Forwarding (BFS on surviving links).
3. Direct-link RTT ≈ `2 ×` per-link delay (symmetric TCLink ends).
4. `Close` / `Stop` runs `net.stop()` and best-effort `mn -c`.

## Unit tests (no Mininet)

```bash
cd experiments
go test ./netinfra/ -count=1
```

## Out of scope

Measurement schema aggregation and statistical analysis notebooks. The YAML
harness (`harness/`, `cmd/harness`, `configs/pilot.yaml`) runs the Stage 3/4
trial loop and archives per-run JSONL artifacts for offline analysis.
