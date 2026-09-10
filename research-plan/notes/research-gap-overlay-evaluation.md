# Note: Scope the research-gap claim carefully

## Purpose

Preserve the literature-based reasoning behind the project's research gap for
use when the research plan is written. Do not claim broadly that no experimental
evaluation exists. The missing evaluation is narrower: a controlled comparison
of the same Raft implementation with and without transparent multi-hop routing
under the same partial-connectivity conditions.

## What the existing literature establishes

### Jensen et al. (2021)

Jensen, Howard, and Mortier experimentally study etcd/Raft during partial
network failures. They evaluate etcd behaviour and mechanisms such as PreVote.
In Section 5 (p. 16), they identify overlay networks as another potential fix
that could reroute messages through an intermediate node. They do not implement
or measure that alternative.

- Local paper: [`../../data/jensen-2021-raft-partial-network-failures.pdf`](../../data/jensen-2021-raft-partial-network-failures.pdf)
- Relevant location: Section 5, p. 16

### Alfatafta et al. (2020): Nifty

Alfatafta et al. implement Nifty, a transparent communication layer that routes
traffic around partial partitions. Their experimental evaluation uses HDFS,
Kafka, RabbitMQ, ActiveMQ, MongoDB, and VoltDB, with results averaged over 30
runs. It demonstrates that transparent routing can mask partial partitions in
several systems, but does not evaluate Nifty beneath Raft.

The paper does discuss and experiment with LogCabin, an implementation of Raft,
when analyzing existing partial-partition fault-tolerance mechanisms. That work
belongs to the design analysis in Section 5; LogCabin is not one of the six
systems used to evaluate Nifty in Section 7. This distinction must be retained.

- Local paper: [`../../data/alfatafta-2020-nifty-partial-network-partitioning.pdf`](../../data/alfatafta-2020-nifty-partial-network-partitioning.pdf)
- Relevant locations: Section 5, pp. 356--359; Section 7, pp. 360--363

### Ng et al. (2023): Omni-Paxos

The Omni-Paxos paper does contain an experimental evaluation of partial
connectivity. Section 7.2 compares Omni-Paxos, Raft, Raft with PreVote and
CheckQuorum, Multi-Paxos, and Viewstamped Replication under partial-connectivity
scenarios. It would therefore be inaccurate to say that Omni-Paxos provides no
formal or experimental evaluation of Raft under partial connectivity.

What it does not evaluate is Raft assisted by a transparent overlay. Section 8
discusses Nifty as an alternative and argues that overlay routing could reduce
fault tolerance by introducing dependencies on intermediate paths and could
increase commit latency through additional network hops. These are reasoned
design arguments, but the paper does not experimentally quantify them and does
not include a `Raft + overlay` condition.

- Local paper: [`../../data/ng-2023-omni-paxos.pdf`](../../data/ng-2023-omni-paxos.pdf)
- Relevant locations: Section 7.2, pp. 124--126; Section 8, p. 127

## Wording to avoid

Avoid unrestricted statements such as:

> There is no experimental evaluation of transparent multi-hop routing for
> Raft under partial network partitions.

Also avoid saying that Omni-Paxos makes "just assumptions." That understates its
analysis and could incorrectly imply that the paper does not evaluate Raft at
all. Prefer "reasoned arguments that are not empirically quantified."

## Recommended concise gap statement

> Among the studies reviewed, none experimentally compares the same Raft
> implementation with and without transparent multi-hop forwarding under
> controlled partial-connectivity topologies.

The phrase "among the studies reviewed" is important. It reports the result of
the literature review without making an unprovable universal claim about every
publication.

## Recommended expanded paragraph

> Existing studies provide complementary but incomplete evidence concerning
> transparent routing for Raft. Jensen et al. experimentally examine etcd/Raft
> under partial network failures and identify overlay routing as a potential
> remedy, but do not implement or evaluate it. Alfatafta et al. implement and
> evaluate Nifty, a transparent communication layer that reroutes traffic
> around partial partitions, across six data-centric systems. Although their
> broader design analysis includes LogCabin, an implementation of Raft, their
> experimental evaluation does not apply Nifty to Raft. Ng et al.
> experimentally compare Raft and other consensus protocols under partial
> connectivity, but do not include an overlay-assisted Raft condition. Instead,
> they identify possible fault-tolerance and commit-latency costs of an overlay
> without experimentally measuring them. Consequently, among the studies
> reviewed, there is no controlled comparison that isolates how transparent
> multi-hop routing affects Raft's liveness and commit latency under the same
> predefined partial-connectivity conditions.

## How to apply this note later

1. Use the expanded paragraph in **Theory and Related Work**, after separately
   explaining Raft, partial connectivity, Nifty, and Omni-Paxos.
2. End that subsection with the concise gap statement or the final sentence of
   the expanded paragraph.
3. In **Research Problem, Aim, and Research Questions**, convert the gap into
   the planned comparison: same Raft implementation, same topologies and delay,
   with routing mode as the independent variable.
4. In **Research Method**, make `direct` and `overlay` explicit experimental
   conditions. Hold the Raft implementation, workload, topology, timing, and
   configuration constant within each comparison.
5. Cite the claims at their precise locations rather than repeatedly using bare
   paper-level citations. With `natbib`, suitable forms are:

   ```tex
   \cite[Sec.~5, p.~16]{jensen2021raft}
   \cite[Sec.~5, pp.~356--359]{alfatafta2020nifty}
   \cite[Sec.~7, pp.~360--363]{alfatafta2020nifty}
   \cite[Sec.~7.2, pp.~124--126]{ng2023omnipaxos}
   \cite[Sec.~8, p.~127]{ng2023omnipaxos}
   ```

6. Recheck the page and section references against the saved PDFs before final
   submission, and update the note if additional directly relevant literature
   is found.

## Claim-to-method consistency check

Before submission, confirm that the plan actually fills the stated gap:

- The experiment uses a real or clearly identified Raft implementation.
- Direct and overlay modes differ only in routing treatment.
- Both modes use the same predefined partial-connectivity topology and delay.
- Liveness and client-observed commit latency have operational definitions.
- The analysis reports run-level comparisons rather than treating every request
  from one run as an independent sample.

