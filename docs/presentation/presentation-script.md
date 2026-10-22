# Final presentation script

For the ten slides in `II2202-final-presentation.pptx`.
Lorenzo presents slides 1–5 and hands over to Juozas for slides 6–10.
Aim to finish in about 9 minutes 30 seconds, leaving 30 seconds within the presentation slot for pauses or a slower explanation. Opposition and questions follow separately.

The paragraphs below are the spoken script. Italic text gives presentation cues and is not spoken. The times are rehearsal targets.

| Slide | Topic | Speaker | Duration | Elapsed time |
| --- | --- | --- | --- | --- |
| 1 | Title | Lorenzo | 0:25 | 0:00–0:25 |
| 2 | Problem and research questions | Lorenzo | 1:05 | 0:25–1:30 |
| 3 | Background and related work | Lorenzo | 0:35 | 1:30–2:05 |
| 4 | Method | Lorenzo | 1:20 | 2:05–3:25 |
| 5 | Network scenarios and handover | Lorenzo | 1:20 | 3:25–4:45 |
| 6 | Progress and throughput results | Juozas | 1:55 | 4:45–6:40 |
| 7 | Latency results | Juozas | 1:20 | 6:40–8:00 |
| 8 | Conclusions and limitations | Juozas | 0:55 | 8:00–8:55 |
| 9 | Future work | Juozas | 0:30 | 8:55–9:25 |
| 10 | Close | Juozas | 0:05 | 9:25–9:30 |

## Slide 1 Title

**Lorenzo**

Hi everyone. I'm Lorenzo, and this is Juozas.

Today we’ll present our study of Raft under partial network partitions, and show how forwarding messages through intermediate nodes can restore progress and how that affects commit latency.

I'll explain the problem and our experiment. Then Juozas will present the results and conclusions.

## Slide 2 Partial partitions can interrupt Raft

**Lorenzo**

Raft is a leader-based consensus protocol that lets a cluster of replicas agree on a shared ordered log.

We study partial network partitions, where some direct links fail but nodes remain connected through peers.

Raft's normal communication does not forward messages. Under a partial partition, the system can struggle to commit even though an alternative path exists through other peers.

Our idea is to enable forwarding through connected nodes without touching the Raft implementation.

We ask two questions: when does forwarding restore progress, and how much extra time does a client wait for a commit?

## Slide 3 Raft and the routing gap

**Lorenzo**

Let's look at previous work and where we fit in.

Jensen and colleagues suggested the use of an overlay network to help Raft under partial partitions as an alternative to PreVote.

Nifty showed that transparent rerouting could help systems like Kafka with low overhead.

OmniPaxos takes another approach to partial connectivity by changing the consensus protocol and mentions overlay routing as a possible alternative.

Our focus is to measure the benefits and the delay of forwarding beneath an unchanged Raft implementation.

## Slide 4 Controlled network experiments

**Lorenzo**

We used HashiCorp Raft and Mininet, which lets us create virtual networks and control their connections and delays.

We compared direct communication with forwarding across five network layouts.

For each one, we tested delays of:

- half a millisecond
- 5 milliseconds
- 20 milliseconds

We repeated every combination 30 times, giving a total of 900 trials.

For each trial, the client sends 100 requests per second. We keep the heartbeat timeout at 500 milliseconds.

In each experiment, we wait 30 seconds for the cluster to warm up, then cut the selected links to form the desired network partition.

Then we start an observation period that lasts 30 seconds.

For T3 only, we skip the warm-up phase to test the configuration with different log histories.

We ran these trials on 16 EC2 instances on AWS, and each trial had all the nodes and the client on a single machine.

## Slide 5 Network configurations

**Lorenzo**

*Move from T0 to T4 as you explain each configuration.*

These are the five network configurations we tested.

T0 is the healthy baseline: all five nodes are directly connected. We expect stable progress in both modes.

T1 removes one link, but the leader still has a direct majority. We expect consensus to continue, so this is our control for a non-critical failure.

T2 leaves the leader connected only to A. We expect forwarding through A to restore its access to a majority.

In T3, C is isolated, and we skip the warm-up phase to preserve different log histories. A has an older log, so even though it can reach a majority of nodes, it cannot win enough votes to become leader. We expect forwarding to let another eligible candidate reach that majority.

T4 is a three-node chain. B and C cannot talk directly, but both connect to A. A direct majority can still exist. We expect forwarding to reduce disruption between the endpoints.

We measure successful client requests in every topology. In T1 and T4, we need to distinguish a client that cannot reach the leader from a cluster that cannot make progress.

I'll hand over to Juozas to show what actually happened.

## Slide 6 Forwarding sustains client progress

**Juozas**

Thanks, Lorenzo.

*Point to the three charts. Grey means direct communication, and teal means forwarding.*

Each chart shows a different link delay. The bars show successful client commits per second, averaged across thirty runs.

We also defined a simple stable-progress check. It requires successful commits and a recorded recovery, while limiting the number of election events. All four hundred and fifty forwarding trials passed it, compared with three hundred and nine direct trials. This check describes progress during our observation period.

T2 shows a clear throughput improvement. At the lowest delay, forwarding increased the average from about sixty-three to ninety-seven commits per second. Across the three delays, the improvement ranged from fifty-three to eighty-three percent.

T3 gives the clearest contrast. Forwarding produced commits in all ninety trials. Direct communication produced none.

The T3 forwarding bars are lower because the measurement includes restart and setup before the client starts. Once the client was running, throughput was close to one hundred commits per second. The roughly twenty to twenty-two seconds before the first commit include that setup, so we cannot treat it as election time alone.

For T1 and T4, forwarding also helped clients reach leaders that still had a majority. That is an availability benefit, but those cases alone do not show that consensus had stopped.

## Slide 7 A forwarded quorum adds commit latency

**Juozas**

Forwarding helped progress, but it also added delay.

Here, latency means how long the client waits for a successful commit. We took the median in each run, then averaged those medians. The charts use different vertical scales, so compare the two bars within each chart.

In the healthy network, T0, the difference between direct communication and forwarding was small.

T2 shows a larger cost because the leader needs forwarded paths to reach its majority. At the lowest link delay, latency rose from about two point four to four point one milliseconds. At the highest delay, it rose from about ninety-nine to one hundred and sixty-six milliseconds.

These numbers describe successful requests. Direct T3 has no latency value because no requests succeeded. Direct T1 and T4 also exclude runs with no successful requests.

So we need to read latency together with throughput and failures. A quick response among successful requests tells only part of the story when other requests cannot complete.

## Slide 8 Routing can improve resilience at a cost

**Juozas**

Our main finding is that forwarding improved progress in the partial partitions we tested. It restored commits in T3 and increased throughput in T2 across all three delays.

The cost was higher commit latency when reaching a majority required extra hops.

There are limits to that conclusion. Our trials used one Raft implementation, one workload, and one heartbeat timeout, with each cluster on a single machine.

We also need to separate client access from consensus progress, as we saw in T1 and T4, and separate setup from recovery time in T3.

The results support forwarding as a useful option for these conditions. Testing in other environments would tell us how far that benefit carries over.

## Slide 9 Future work

**Juozas**

Our next step would be to control where the client runs and measure recovery separately from setup.

We would also vary the heartbeat timeout to see how it interacts with network delay.

Finally, we would place replicas on separate machines or in different regions, where delays change over time. That would bring the experiment closer to a real deployment.

## Slide 10 Questions

**Juozas**

Thank you for listening. We're ready for the opposition and your questions.

## Rehearsal notes

Use the handover at about 4:45 as the main timing checkpoint. Read the script aloud together once and adjust the pace before adding any detail. Let the figures carry the full set of numbers; the spoken examples are enough.

If you are running late, shorten the related-work explanation on slide 3 and use only the lowest-delay latency example on slide 7. Keep the explanations of client reachability, successful-request latency, and T3 setup time because they affect the conclusions.

The script follows the [latest final report](../final-report/II2202-report.tex) and the [final presentation](II2202-final-presentation.pptx). The earlier script under `research-plan/slides/` belongs to the research-plan talk.
