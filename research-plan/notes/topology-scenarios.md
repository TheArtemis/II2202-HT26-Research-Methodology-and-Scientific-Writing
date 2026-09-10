# Planned topology scenarios

The experiments compare direct communication with transparent multi-hop
forwarding under the same network conditions. All links shown are bidirectional.
Solid links remain available after fault injection; dashed links have failed.
Blue paths show logical multi-hop communication provided by Linux routing and
IP forwarding over those surviving links; they do not represent repaired or
additional physical links.

The five-node scenarios use the same pentagonal node arrangement as
Omni-Paxos: A is on the left, B at the top, C on the right, and D and E at the
bottom.

## T0 — Full connectivity

- **Nodes:** Five: A, B, C, D, and E.
- **Initial leader:** C.
- **Connectivity:** Every pair of nodes has a working direct link.
- **Purpose:** Healthy baseline for commit latency, throughput, and leader
  stability.
- **Expected behaviour:** Raft should remain stable without using forwarded
  paths.

## T1 — Non-critical partial cut

- **Nodes:** Five: A, B, C, D, and E.
- **Initial leader:** C.
- **Failed link:** C--E.
- **Connectivity:** All other direct links remain available. C can still reach
  A, B, and D directly and therefore retains a majority.
- **Purpose:** Control case in which a link fails without removing the leader's
  direct quorum.
- **Expected behaviour:** Direct communication should remain live, with little
  or no effect on elections or commit latency.

## T2 — Quorum-loss

- **Nodes:** Five: A, B, C, D, and E.
- **Initial leader:** C.
- **Working links:** A--B, A--C, A--D, and A--E.
- **Failed links:** B--C, B--D, B--E, C--D, C--E, and D--E.
- **Connectivity:** A is the only quorum-connected node. C remains reachable
  from A but cannot communicate directly with a majority.
- **Purpose:** Test whether forwarding restores progress when the existing
  leader loses its direct quorum despite the graph remaining connected.
- **Expected behaviour:** Direct-only Raft may stall; transparent forwarding
  should restore paths from the leader to a majority.
- **Origin:** Omni-Paxos quorum-loss scenario.

## T3 — Constrained election

- **Nodes:** Five: A, B, C, D, and E.
- **Initial leader:** C.
- **Working links:** A--B, A--D, and A--E.
- **Failed links:** A--C, B--C, B--D, B--E, C--D, C--E, and D--E.
- **Initial logs:** A=`[1,1]`, B=`[1,1,2]`, C=`[1,1,2,2]`,
  D=`[1,1,2]`, and E=`[1,1]`.
- **Connectivity:** C is isolated. A is the only quorum-connected candidate,
  but its log is older than the logs of B and D.
- **Purpose:** Test whether forwarding enables a viable election when
  connectivity and Raft's log-freshness voting rule favour different nodes.
- **Expected behaviour:** Direct-only Raft may fail to elect a leader that can
  make progress; transparent forwarding should restore stable progress.
- **Origin:** Omni-Paxos outdated-log/constrained-election scenario.

## T4 — Chained connectivity

- **Nodes:** Three: A, B, and C.
- **Initial leader:** B.
- **Working links:** B--A and A--C.
- **Failed link:** B--C.
- **Connectivity:** A bridges B and C, but direct communication does not forward
  Raft messages through A.
- **Purpose:** Test whether forwarding prevents repeated elections caused by
  the two endpoint nodes being unable to hear one another directly.
- **Expected behaviour:** Direct-only Raft may alternate elections and terms;
  transparent forwarding should allow B and C to communicate through A and
  stabilize the cluster.
- **Origin:** Omni-Paxos chained scenario. The original paper and experiment use
  three nodes for this case.

## Figure

See the [topology figure](../figures/topology-scenarios.pdf). T0 and T1 are
project controls; T2--T4 reproduce the corresponding Omni-Paxos topology
shapes. Each row compares direct-only communication with transparent forwarding
while keeping the failed-link graph unchanged.
