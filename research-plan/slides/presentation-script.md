# Presentation script — Transparent Multi-Hop Routing for Raft

Two speakers: **L = Lorenzo**, **J = Juozas**.
Target: ~8 minutes. Keep a steady pace, pause between slides.

---

## Slide 1 — Title (~40 s) — L

**L:** Hi everyone. We are Lorenzo and Juozas. Our project is about keeping the Raft consensus protocol working when a network breaks in a partial way.

---

## Slide 2 — Research topic and problem (~2 min) — L

**L:** Raft is a leader-based consensus protocol. To make progress, the leader must talk to a majority of nodes.

The problem is a *partial* partition. That is not a clean split into two halves.
Some direct links fail, but the nodes are still reachable through another node.
Raft only uses direct links, so the leader can lose its majority even though a path still exists.

*(point to the figure)* On the right are the cases we plan to test, T0 to T4.
In each row, the left side is normal Raft with direct links only, and the right side adds forwarding through a middle node.
T0 is the healthy baseline. T1 is a control where the leader still has a direct majority.
T2 to T4 are the interesting failures, where direct-only Raft gets stuck.

So we ask two questions.
**RQ1:** in which of these partitions does forwarding bring back stable progress?
**RQ2:** when we add delay to those extra hops, how much does it slow down commits?

I'll hand over to Juozas for how we test this.

---



## Slide 3 — Research method and work plan (~2 min 30 s) — J

**J:** Thanks. This is a quantitative experiment. We compare the two modes under identical conditions.

*(point to the bullets)* We use the HashiCorp Raft implementation, so we don't change the protocol itself.
We build the network in Mininet and pre-configure the forwarding routes in advance.
A client sends requests continuously, and we repeat every run many times to average out timing noise.

*(point to the table)* We vary three things: the communication mode, direct or multi-hop;
the topology, T0 to T4; and the per-link delay, from half a millisecond up to twenty.
We measure whether the cluster keeps committing, how fast it recovers, how many elections happen, and the commit latency.

*(point to the work plan)* The plan is five steps: build the environment, run small pilots to check it works,
then run the full set with twenty repetitions, analyse the data, and write the report.
Lorenzo leads the Raft and client side and RQ1; I lead the network, delays, and RQ2.

Now back to Lorenzo for the ethics.

---



## Slide 4 — Key ethical issues (~1 min 30 s) — L

**L:** On ethics, the direct risks are low, but we still want to be clear.

There are no human participants and no personal or sensitive data. Everything we use is open source.

On integrity: our setup is deterministic and we record seeds, so results can be reproduced.
If forwarding does *not* help in some case, we report that as a real finding, not something to hide.
And we use AI tools only for assistance, like editing text or helper scripts, not for the actual research thinking.

---



## Slide 5 — Key sustainability issues + close (~1 min 30 s) — J

**J:** For sustainability we separate our own experiment from the bigger picture.

*(point to environmental)* In our experiment we use local VMs, not a cloud fleet, and we run pilots first so we don't waste compute.
At a larger scale it cuts both ways: an overlay adds some extra network and CPU cost,
but if it prevents outages, the system uses its resources more efficiently overall.

*(point to economic)* Same idea for cost. We only use our own hardware.
Broadly, extra infrastructure can cost more, but proven resilience can save on redundancy and downtime.

*(point to relevance)* Overall: knowing *when* multi-hop routing restores progress
can help avoid long stalls and unnecessary failovers under partial partitions.

That's our plan. Thank you — happy to take questions.

---



### Timing cheat-sheet


| Slide            | Speaker | Time      |
| ---------------- | ------- | --------- |
| 1 Title          | L       | 0:40      |
| 2 Problem        | L       | 2:00      |
| 3 Method         | J       | 2:30      |
| 4 Ethics         | L       | 1:30      |
| 5 Sustainability | J       | 1:30      |
| **Total**        |         | **~8:10** |


