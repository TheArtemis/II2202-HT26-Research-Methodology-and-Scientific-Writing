# Pilot Interview Exercise: Transparent Multi-Hop Routing for Raft

**Authors:** Lorenzo Deflorian and Juozas Skarbalius  
**LLM used for the pilot:** OpenAI Codex, 21 September 2026

## 1. Research aim and qualitative adaptation

Our research project concerns Raft clusters operating under partial network partitions. In such a partition, some direct communication links between replicas fail even though indirect paths through other replicas remain available. Raft normally communicates through direct point-to-point links and therefore cannot necessarily use this remaining connectivity. The original project aims to experimentally characterize whether transparent multi-hop routing can restore stable Raft progress in predefined partial-connectivity topologies without changing the consensus implementation. It also aims to measure the effect of the extra network hops and per-link delay on client-observed commit latency, especially when a remotely reached follower is needed to form a majority.

The planned project is a quantitative experiment rather than an interview study. For this exercise, we considered how interviews could complement that experiment. Suitable qualitative research questions would be:

1. How do practitioners experience, diagnose, and respond to partial-connectivity incidents in Raft-backed systems?
2. How do practitioners assess the operational benefits and risks of transparent multi-hop routing for maintaining Raft availability?

The relevant participants would be site reliability engineers, distributed-systems engineers, database engineers, or network engineers who have operated a Raft-backed production system, such as etcd, Consul, or a distributed database. Ideally, they would have investigated network failures, leader-election instability, or loss of commit progress. Such people may have practical knowledge that is not visible in controlled measurements: which symptoms appear first, how operators distinguish a partial partition from node failure or overload, which recovery actions they trust, and which trade-offs influence their willingness to add a routing layer.

## 2. Interview questions

The following questions form a short semi-structured interview guide. The labels identify the primary type of qualitative question represented by each one.

1. **Experience/behavior question:** Think of a specific incident in which a Raft-backed cluster lost some network paths while at least part of the cluster remained connected. Could you walk me through what happened, from the first symptoms through diagnosis and recovery, including what you observed about commits and leader elections? If you have not experienced exactly this situation, please describe the closest relevant incident.

2. **Opinion/value question:** When deciding whether to use transparent multi-hop routing to preserve connectivity between Raft replicas, what benefits and risks would matter most to you, and why?

3. **Knowledge question:** Based on your practical understanding of Raft deployments, under what network and operational conditions would you expect indirect routing to restore useful service, and under what conditions might it make the situation worse?

## 3. Prompts used with the LLM

The same question set was given to two fictional personas. Specifying different roles and constraints was intended to test whether the questions could elicit contrasting yet relevant answers.

### Prompt 1

> You are participating in a pilot interview for a university research project. Impersonate a fictional senior site reliability engineer with eight years of experience operating Kubernetes platforms. You have managed several production etcd clusters and have participated in incidents involving packet loss, asymmetric firewall rules, latency, and unstable leader elections. Answer in the first person from this practitioner's perspective. Give concrete operational detail, but do not claim certainty about facts the persona could not observe. Do not present the response as real evidence: this is a fictional simulation used only to test the interview questions. Answer each question separately in approximately 150--250 words.
>
> 1. Think of a specific incident in which a Raft-backed cluster lost some network paths while at least part of the cluster remained connected. Could you walk me through what happened, from the first symptoms through diagnosis and recovery, including what you observed about commits and leader elections? If you have not experienced exactly this situation, please describe the closest relevant incident.
>
> 2. When deciding whether to use transparent multi-hop routing to preserve connectivity between Raft replicas, what benefits and risks would matter most to you, and why?
>
> 3. Based on your practical understanding of Raft deployments, under what network and operational conditions would you expect indirect routing to restore useful service, and under what conditions might it make the situation worse?

### Prompt 2

> You are participating in a pilot interview for a university research project. Impersonate a fictional distributed-database engineer with eleven years of experience designing and operating Raft-replicated storage services across multiple availability zones. You have responsibility for performance, failure testing, and incident reviews. You are cautious about hiding network failures behind infrastructure mechanisms. Answer in the first person from this practitioner's perspective. Give concrete operational detail, explain your reasoning, and acknowledge uncertainty. Do not present the response as real evidence: this is a fictional simulation used only to test the interview questions. Answer each question separately in approximately 150--250 words.
>
> 1. Think of a specific incident in which a Raft-backed cluster lost some network paths while at least part of the cluster remained connected. Could you walk me through what happened, from the first symptoms through diagnosis and recovery, including what you observed about commits and leader elections? If you have not experienced exactly this situation, please describe the closest relevant incident.
>
> 2. When deciding whether to use transparent multi-hop routing to preserve connectivity between Raft replicas, what benefits and risks would matter most to you, and why?
>
> 3. Based on your practical understanding of Raft deployments, under what network and operational conditions would you expect indirect routing to restore useful service, and under what conditions might it make the situation worse?

## 4. LLM-generated answers

The following answers are synthetic. They represent the LLM's performance of the requested personas and are not accounts from human participants.

### Simulated participant 1: senior site reliability engineer

**Answer to Question 1.**  
The closest incident involved a three-member etcd cluster spread across three failure domains. A firewall policy update was applied unevenly: the first member could communicate with the second, and the second with the third, but traffic between the first and third was rejected in one direction. Our first symptom was not a clean “network partition” alert. Kubernetes API writes became slow and then intermittently timed out. The etcd dashboards showed repeated leader changes and a growing difference between successful reads and writes. CPU and disk latency looked normal, so overload was initially a plausible but weak explanation.

We compared peer-health checks from every member rather than testing from a single monitoring host. That exposed the asymmetric reachability. Packet captures then showed that heartbeat and replication traffic was not following the same successful paths we had assumed from ordinary host-level checks. During the unstable period, some read requests succeeded, but commits were irregular because the leader could not consistently maintain communication with a majority. We recovered by rolling back the firewall policy, after which one leader remained stable and write latency returned to its baseline range. We did not try to route peer traffic through the middle member during the incident because that behavior had not been tested. The incident review led us to add pairwise, bidirectional peer checks and alerts on term changes rather than relying only on endpoint availability.

**Answer to Question 2.**  
The main benefit would be preserving a quorum when the underlying network still contains a usable path. That could turn an outage into a period of higher latency, and it could buy operators time to repair the direct path without forcing a leadership transition. Transparency is also attractive because we would not need to modify etcd or maintain a custom consensus implementation.

My strongest concern is that the routing layer could conceal the actual failure and create behavior that is harder to diagnose. I would want to know how quickly it detects a failed path, whether routes can oscillate, and whether both directions of a peer connection take compatible paths. A route that is technically available but suffers heavy loss or unpredictable delay may be worse than a clearly failed link because it can provoke election churn and long request tails. I would also worry about correlated failure: an intermediate node could become a forwarding bottleneck while also participating in the quorum.

I would only enable such routing with strict observability. We would need per-peer path information, hop count, loss, delay, route-change events, and a clear indication that traffic is being detoured. I would also want bounded failover behavior and a way to disable forwarding quickly. For me, restored availability is valuable, but not if the mechanism makes the cluster's effective topology invisible during an incident.

**Answer to Question 3.**  
I would expect indirect routing to help when the surviving links are bidirectional and reasonably stable, the detour lets a leader reach a majority, and the added round-trip time remains well below the configured election timeout. It is especially plausible when one direct link has failed but an otherwise healthy intermediate node and path are available. The workload must also fit within the reduced bandwidth, since forwarded replication traffic can concentrate on fewer links. In that situation, the likely trade-off is higher commit latency rather than complete loss of writes.

It could make things worse when reachability is asymmetric, routes change faster than the failure detector can settle, or the detour delay approaches the heartbeat or election thresholds. Forwarding several peers through the same bridge could overload that bridge and turn one link or node into a shared failure point. I would also be cautious if the overlay duplicates, reorders, or tunnels traffic in ways that interact badly with connection timeouts, although Raft's safety should not depend on low latency. The important distinction is safety versus availability: delayed messages should not permit incorrect commits in a correct implementation, but they can prevent useful progress. Finally, routing cannot help if no leader candidate can reach a majority over stable paths. It only exploits surviving connectivity; it cannot create capacity or connectivity that is not there.

### Simulated participant 2: distributed-database engineer

**Answer to Question 1.**  
One incident started with maintenance on a top-of-rack switch. We had five replicas across three availability zones. Two replicas could still reach a third through the normal routed network, but a stale access-control entry blocked selected replica-to-replica flows after addresses changed. Client metrics showed a sharp increase in write latency, followed by failed writes. The confusing part was that every machine responded to our management probes and some replica pairs communicated normally. The cluster therefore looked healthier from outside than it was from the consensus transport's point of view.

The logs showed that the existing leader could replicate to one follower but not to enough followers to commit. Other replicas increased their terms, and leadership moved more than once, but a newly elected leader did not remain useful for long. We reconstructed a directed connectivity matrix using probes on the actual peer ports and correlated it with term and match-index logs. That was more informative than host reachability. We first stopped the network change, then removed the stale rule and restarted only the affected peer connections. Commit throughput recovered without changing membership.

The lesson was that “all nodes are alive” and “the cluster has a path between nodes” do not prove that Raft has the direct, timely communication it needs. We later added a fault-injection test for pairwise partial partitions and required the database and network teams to use the same definition of reachability during incident response.

**Answer to Question 2.**  
I would compare the availability gained with the additional state and failure modes introduced by the routing system. The positive case is straightforward: if a direct link failure prevents a leader from reaching a majority, a stable detour may restore commits without reconfiguration or data movement. That is valuable for short-lived infrastructure failures, and keeping the mechanism below Raft makes it reusable across implementations.

However, transparent recovery can undermine deliberate failure boundaries. We place replicas in different zones partly to avoid one component carrying several supposedly independent paths. If two quorum paths are silently forwarded through one switch, host, or zone, the cluster may appear redundant while depending on a single resource. Added latency also affects more than the median: it can change which replicas form the fastest majority and produce large tail latency when a routed follower is quorum-critical. Route discovery and failure detection could interact with Raft's own timeouts, producing two control loops that continually react to each other.

Before adoption I would require explicit bounds: maximum route length, delay, and duration; loop prevention; path-diversity reporting; and tests for asymmetric loss and route flapping. I would probably use forwarding as a temporary degraded mode rather than a permanent default. I also want operators and automation to see that degraded mode, because a system that keeps serving traffic can otherwise postpone repair until a second failure removes the remaining path.

**Answer to Question 3.**  
The best case is a sparse but stable surviving topology with enough bidirectional edges to connect one eligible leader to a majority. The chosen leader must have an up-to-date log, and the routed connections must remain within the deployment's timing and bandwidth envelope. Indirect routing is most useful when path failure is the limiting factor and the replicas themselves, including storage and CPU, are healthy. I would expect success when one extra hop restores a quorum and the election timeout is comfortably larger than the new delay and its variation.

I would not expect routing alone to solve a constrained election in every implementation. If no quorum-reachable candidate can win because of log freshness, the overlay must restore paths that let an eligible candidate obtain votes or let another replica catch up; merely giving one old candidate more neighbors may not be enough. Forwarding may worsen service when detours share a congested link, cross a high-latency region, or create correlated paths that fail together. It may also prolong an ambiguous degraded state in which the cluster sometimes commits and sometimes starts elections.

I would therefore evaluate topology and timing together. A graph-theoretic path to a majority is necessary, but it is not sufficient operationally. The paths must be bidirectional, stable for longer than the relevant election and replication intervals, and capable of carrying the workload. I would also test recovery when direct connectivity returns, because route convergence back to the preferred path can itself cause disruption.

## 5. Concluding observations

Formulating the experience question was the easiest part because it could be anchored in a concrete incident and a chronological account. That wording produced useful descriptions of symptoms, diagnostic actions, recovery, and later preventive measures. The opinion and knowledge questions were more challenging because technical wording can easily imply that multi-hop routing is desirable or that it will restore progress. Using “whether,” and explicitly asking when routing might make matters worse, reduced that bias and invited discussion of both benefits and risks. Both simulated participants interpreted partial connectivity as intended and distinguished ordinary host availability from the peer-to-peer connectivity required by Raft. Their answers also surfaced issues that the quantitative plan should consider, including asymmetric reachability, route flapping, shared bottlenecks, observability, path diversity, and interactions between routing convergence and Raft election timeouts.

Before interviewing human participants, we would make two modifications. First, we would simplify or define terms such as “quorum-critical” and “transparent multi-hop routing,” because relevant network or operations engineers may not use the same vocabulary. Second, Question 1 currently contains several prompts in one sentence. We would ask for the incident first and use symptoms, diagnosis, commits, elections, and recovery as optional follow-up probes. This would reduce the risk of overwhelming participants or directing their stories too strongly. We would retain the “closest relevant incident” option because exact partial partitions may be rare. Finally, the LLM answers were coherent and useful for testing wording, but they were shaped by the detailed personas and common technical narratives. They cannot establish what real practitioners experience; actual interviews could reveal different terminology, organizational constraints, and unexpected themes.
