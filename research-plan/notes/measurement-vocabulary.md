# Measurement vocabulary and attribution

## Purpose

Use vocabulary that remains comparable with the Omni-Paxos paper while giving
every measured concept an operational definition appropriate for the HashiCorp
Raft experiment. The conceptual source and the project's measurement rule must
be distinguishable.

Using established terminology is not plagiarism. Paraphrase the source, cite it
where its concepts or findings are used, and introduce project-specific rules
with wording such as “In this study, we define...” Use quotation marks and a
page reference if exact wording is ever retained.

Primary source: [`../../data/ng-2023-omni-paxos.pdf`](../../data/ng-2023-omni-paxos.pdf).
The most relevant locations are Section 2 (p. 117), Section 5.1 (p. 121),
Section 7.2 (pp. 124--126), and Section 8 (p. 127).

## Concept-to-measurement mapping

### Stable progress

- **Status in Omni-Paxos:** Used as a concept, but not assigned a numerical
  threshold in the evaluation.
- **Paper-aligned meaning:** Client proposals continue to be decided without
  the protocol remaining in repeated leader changes or failing to elect a
  capable leader. The formal basis is eventual agreement on a
  quorum-connected leader.
- **Project operational definition:** After a bounded recovery interval, the
  cluster continuously commits client requests for the remainder of the
  partial-partition observation period without persistent election churn.
- **Still to specify:** The minimum acceptable commit activity, measurement
  window, maximum gap between commits, and election-churn threshold.

Attribute the conceptual basis to Ng et al., then introduce the numerical rule
as this project's definition.

### Commit success

- **Status in Omni-Paxos:** Not a defined term. The paper speaks of client
  requests, proposals, and log entries being *decided*.
- **Project operational definition:** A submitted request is successful if the
  client receives confirmation that its HashiCorp Raft entry was committed
  within the configured timeout.
- **Run-level metric:**

  \[
  \text{commit success ratio} =
  \frac{\text{requests confirmed committed within the timeout}}
       {\text{requests submitted}}.
  \]

State that a committed Raft entry is treated as corresponding to a decided
entry in the Omni-Paxos evaluation. Do not claim that “commit success” is an
Omni-Paxos metric.

### Recovery

- **Status in Omni-Paxos:** Used and experimentally evaluated. In Section 7.2,
  down-time is the duration during which the client receives no decided
  replies. Recovery is discussed in heartbeat or election-timeout rounds.
- **Project operational definition:** Recovery time is the elapsed time from
  fault injection until stable progress is re-established.
- **Measurement safeguard:** Require sustained progress after the first
  post-fault commit so that a single temporary commit is not classified as
  recovery.
- **Still to specify:** The sustained-progress window and treatment of trials
  that do not recover before the observation period ends.

Cite `\cite[Sec.~7.2, pp.~124--126]{ng2023omnipaxos}` for the paper's
down-time and recovery approach.

### Livelock

- **Status in Omni-Paxos:** Explicitly described in the chained scenario.
- **Paper-aligned meaning:** Higher terms are repeatedly propagated, producing
  repeated leader changes without stable log-replication progress. The system
  remains active, unlike a deadlock in which no leader can be elected.
- **Project operational definition:** A run is classified as livelocked when
  leaders or terms continue changing during the observation window while the
  cluster fails to maintain stable commit progress.
- **Still to specify:** The minimum number or rate of term/leader changes and
  the observation window used for classification.

Cite `\cite[Sec.~2, p.~117]{ng2023omnipaxos}` for the chained scenario.

### Client-observed commit latency

- **Status in Omni-Paxos:** “Commit latency” is mentioned in Section 8 as a
  potential overlay cost caused by additional network hops, but it is not
  operationally defined or measured there.
- **Project operational definition:** The elapsed time from client submission
  of a request until the client receives confirmation that its entry has been
  committed.
- **Reporting:** Summarize latency within each independent run before comparing
  conditions; do not treat every request from one run as an independent
  experimental repetition.

Cite `\cite[Sec.~8, p.~127]{ng2023omnipaxos}` only for the claim that an
overlay may increase commit latency through additional hops. Introduce the
measurement definition as this project's own.

### Decision-determining follower

- **Status in Omni-Paxos:** Project-specific term. “Quorum-critical follower”
  does not occur in the paper and must not be confused with
  *quorum-connected server*.
- **Project operational definition:** For a particular entry, the follower
  whose acknowledgement completes the first majority required for commitment.
  The role can change between entries and is not a permanent property of a
  follower.
- **Preferred name:** *Decision-determining follower* or *follower in the
  deciding majority*.

The paper formally defines a quorum-connected server as a correct server with
a direct link to at least a majority of correct servers, including itself
(`\cite[Sec.~5.1, p.~121]{ng2023omnipaxos}`). That cluster-level connectivity
property is different from the request-level role defined above.

## Suggested introductory paragraph

> We adopt the paper's general vocabulary of progress, decided requests,
> recovery, and livelock, while defining project-specific operational criteria
> suitable for HashiCorp Raft. Following Ng et al., progress is observable when
> client requests continue to receive decided replies, and down-time is the
> interval during which no such replies are received. Because HashiCorp Raft
> uses commit terminology, we treat a successfully committed client entry as
> corresponding to a decided request in the Omni-Paxos evaluation.

Paraphrase or adapt this paragraph when integrating it into the research plan
and attach the specific citations listed above.

## Attribution checklist

- Cite Omni-Paxos for its partial-connectivity concepts, scenarios, definitions,
  evaluation design, and findings.
- Use “Following Ng et al....” for a concept adapted from the paper.
- Use “In this study, we define...” for every project-specific threshold or
  measurement rule.
- Do not write “Omni-Paxos defines...” unless the paper actually presents a
  definition.
- Use quotation marks and a page reference for any exact quotation.
- Keep HashiCorp Raft terminology (“committed”) distinct from Omni-Paxos
  terminology (“decided”), and state the mapping once.

