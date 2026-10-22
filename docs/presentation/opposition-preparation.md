# Opposition questions and answers

Preparation for the discussion of **Transparent Multi-Hop Routing for Raft Under Partial Network Partitions**. These are likely questions, with short answers you can practise saying aloud. They follow the latest final report and the 900-trial campaign.

Practise questions 1–9 first. Start with the short answer and add detail only if asked. Lorenzo can lead on the problem, method, and topologies. Juozas can lead on the results and their interpretation. Both should be comfortable explaining the limitations.

## Questions to practise first

### 1 What is the main contribution of your project

We measured how transparent forwarding affects progress and client commit latency beneath an unchanged Raft implementation. Forwarding restored commits in T3 and improved throughput in T2 across all three delays. It also increased latency when reaching a majority required forwarded paths.

*Report: Research Problem, Aim, and Research Questions; Conclusion.*

### 2 Did you show that consensus fails in T1 and T4

In some of those trials, the leader still had a majority, but the client could not reach it because its direct link had failed. Forwarding restored client access. We therefore describe those cases as a benefit to client availability. They do not establish that Raft itself had stopped making progress.

**If they challenge the discussion's explanation about elections:** That wording is too broad for T1 and T4. Our detailed results identify client reachability as the issue in the zero-throughput cases. We would make that distinction clearer in the discussion.

*Report: RQ1: Client progress and recovery; Validity, Limitations, and Generalizability.*

### 3 Does Raft really need twenty seconds to recover in T3

Our measurement starts at failure injection and includes restarting the replicas, setting up their logs, and checking the system before starting the client. That takes much of the twenty to twenty-two seconds. We cannot use this number as a measurement of Raft's election time alone.

**If they ask why T3 throughput is only about 59 commits per second:** The throughput denominator includes that setup time too. When we measure from client start, forwarding achieves about 99.5 to 100 commits per second.

*Report: Work Procedure and Data Collection; RQ1: Client progress and recovery.*

### 4 Is the latency comparison biased because failed requests are excluded

Yes, it has that limitation. The latency figures describe successful requests. Direct T3 has no latency value because no requests succeeded, and the direct T1 and T4 summaries leave out runs with no successful requests. We need to interpret those figures together with throughput, failures, and the number of contributing runs.

**If they ask what “mean median latency” means:** We calculate the median latency within each run, then average those medians across runs with successful requests. It is not the median of all requests pooled together.

*Report: Data Analysis and Quality Assurance; RQ2: Commit latency; latency table.*

### 5 Why did you disable PreVote

We kept PreVote disabled in both communication modes, so both used the same election configuration. Our results describe that configuration. We did not compare it with PreVote enabled, so we cannot say forwarding is better than PreVote or that the same gains would occur with it enabled.

*Report: Experimental Platform and Conditions.*

### 6 Does preconfiguring the forwarding routes make the experiment unrealistic

It simplifies the experiment. We wanted to measure what happens when an alternative path is available, without mixing that with the time needed to discover and install a route. A deployed system would also need failure detection and route discovery. Our measurements do not include those costs.

*Report: Research Design and Methodological Rationale.*

### 7 What exactly does stable progress mean

It is a descriptive check for our trials. It requires a successful client commit, a recorded recovery, and an election-event count at or below a defined threshold. Passing it describes behaviour during our observation period. It does not prove that Raft will always make progress.

**If they question the threshold:** The threshold is `max(5, floor(N_ok / 10) + 3)`, where `N_ok` is the number of successful client commits. It allows more election events when there are more commits, so it is not an independent measure of election stability. We would support our answer with the separate throughput and event measurements, rather than relying on that label alone.

*Report: Data Analysis and Quality Assurance.*

### 8 How can all 900 trials be valid if many direct trials failed to make progress

Validation means the experiment ran correctly and produced the required measurements. A correctly executed trial can still produce no successful client requests. All 900 trials passed validation, while 450 forwarding trials and 309 direct trials passed our stable-progress check.

*Report: Data Analysis and Quality Assurance; Results; Conclusion.*

### 9 Are the differences statistically significant

We did not perform formal significance tests. We repeated each condition thirty times and reported means and sample standard deviations. We can describe the differences we observed, including the consistent T3 contrast, but we should not call them statistically significant on the basis of this analysis.

*Report: Data Analysis and Quality Assurance; Results.*

## Method and design questions

### 10 Why did you choose these five topologies

They let us examine specific situations: a healthy network, a cut where a direct majority remains, loss of a leader's direct majority, a difficult election with different log histories, and a three-node chain. Some come from scenarios in the OmniPaxos paper. They are selected test cases, so our conclusions apply to those cases rather than every possible partial partition.

*Report: Research Design and Methodological Rationale; topology figure.*

### 11 Why can node A not simply become leader in T3

A can communicate with enough nodes, but its log is older. Other nodes can reject it because Raft checks how up to date a candidate's log is before voting. In this scenario, the candidates that can win those votes lack enough direct connections. Forwarding gives an eligible candidate a path to a majority.

**If they ask whether T3 is artificial:** We deliberately created different log histories to test this particular election problem. The experiment demonstrates behaviour under that constructed condition. It does not tell us how often that condition occurs in production.

*Report: Work Procedure and Data Collection; Interpretation and Answers to the Research Questions.*

### 12 Why use Mininet and run a whole cluster on one machine

Mininet lets us control the links and their delays while running the Raft implementation. Keeping one trial on one machine makes the network conditions easier to control. It also limits realism because the replicas share machine resources. Tests across separate hosts would be an important next step.

**If they ask what the sixteen AWS machines did:** They ran independent trials in parallel. The replicas of a single trial were not spread across those sixteen machines.

*Report: Experimental Platform and Conditions; Validity, Limitations, and Generalizability.*

### 13 Why use a 500 millisecond heartbeat timeout

We fixed one timeout so that both communication modes and all three link delays shared the same setting. This lets us compare them within that configuration. We did not establish that 500 milliseconds is optimal, and testing other timeouts would show how sensitive the results are to that choice.

*Report: Research Design and Methodological Rationale; Implications and Future Work.*

### 14 Are thirty repetitions and a thirty second observation period enough

Thirty repetitions let us show variation between runs. Thirty seconds after setup gives us a fixed window for comparing progress. Those choices still limit the evidence: they do not establish long-term reliability or show whether a stalled direct trial might recover later. Longer observations and more repetitions would strengthen the evaluation.

*Report: Research Design and Methodological Rationale; Work Procedure and Data Collection; Data Analysis and Quality Assurance.*

### 15 Why only 100 requests per second and 64 byte payloads

We used one fixed workload to compare the communication modes under the same demand. Throughput near 100 commits per second means the system kept up with that workload. It does not establish its maximum capacity. Higher request rates and larger payloads could change the trade-off and would need separate experiments.

*Report: Research Design and Methodological Rationale; Experimental Platform and Conditions.*

### 16 Do ordinary networks not already route messages through intermediate devices

Yes. Our direct mode and forwarding mode represent two specific reachability configurations. Direct mode uses only the surviving point-to-point links in the experiment. Forwarding enables paths through intermediate peers. We are measuring that difference, rather than claiming that ordinary IP networks never use multiple hops.

*Report: Research Design and Methodological Rationale; Experimental Platform and Conditions.*

## Results and interpretation questions

### 17 Can you claim that forwarding has no overhead in a healthy network

We observed small differences in T0. At the lowest delay, mean median latency was 2.42 milliseconds with direct communication and 2.53 with forwarding. That supports a small observed difference under this workload. It does not show that the overhead is exactly zero or that it stays small at higher load.

*Report: RQ2: Commit latency.*

### 18 How can direct communication have lower latency but lower throughput

They measure different things. Latency describes the requests that succeeded, while throughput counts successful commits over the observation interval. Direct mode can serve some requests quickly and spend other periods unable to complete them. Forwarding can complete more requests overall while taking longer for each successful request.

*Report: Data Analysis and Quality Assurance; RQ1: Client progress and recovery; RQ2: Commit latency.*

### 19 Did the experiment support both hypotheses

The observations support them within our tested conditions. Forwarding restored commits in T3 and increased throughput in T2. T2 also showed higher latency when the majority required forwarded paths, while T0 showed small latency differences. The T1 and T4 findings need the separate explanation about client reachability.

*Report: Research Problem, Aim, and Research Questions; Interpretation and Answers to the Research Questions.*

### 20 Does forwarding preserve safety and solve every partition

Raft's voting and commit rules stay unchanged in our setup. Our experiment measured progress and latency; it did not provide a new safety proof. Forwarding also needs a surviving path. It cannot reconnect groups that have no path between them or remove Raft's requirement for a majority.

*Report: Background and Rationale; Research Problem, Aim, and Research Questions; Research Method.*

### 21 How reproducible are the results

We used fixed software versions and predefined seeds, recorded the actual initial leader, and collected client logs, Raft events, and timing information. We also checked connectivity after injection and monitored host load. Those measures support repetition and inspection, but scheduling and runtime variation mean identical inputs can still produce different timings.

*Report: Work Procedure and Data Collection; Data Analysis and Quality Assurance; Validity, Limitations, and Generalizability.*

### 22 What would you change first in a follow up experiment

First, we would control client-to-leader reachability so that we could separate client access from consensus progress. We would also measure recovery separately from restart and setup. Those changes address the main interpretation problems in T1, T4, and T3 before expanding to more workloads or deployments.

*Report: Implications and Future Work; Conclusion.*

## Wider discussion questions

### 23 Is this a new routing algorithm and how does it differ from earlier work

Our contribution is the experimental comparison beneath an unchanged Raft implementation. We used shortest surviving paths, rather than introducing a new routing algorithm. Earlier work motivated transparent rerouting or changed the consensus protocol. We measured progress and client latency for our selected Raft scenarios and configuration.

*Report: Theory and Related Work; Research Design and Methodological Rationale; Experimental Platform and Conditions.*

### 24 Would you recommend deploying this in a production database

The results give a reason to test forwarding further. A production decision would also need measurements with the actual workload, changing network delays, separate hosts, and the intended election settings. It would also need to include the time and cost of detecting failures and installing routes.

*Report: Validity, Limitations, and Generalizability; Ethics, Societal Aspects, and Sustainability; Implications and Future Work.*

### 25 What changed from the research plan

The main design stayed the same: compare direct communication and forwarding in Mininet across selected topologies and delays. We moved execution to AWS so we could complete the experiments within the course deadline, fixed the heartbeat timeout at 500 milliseconds, and increased repetitions to thirty per condition.

*Report: Research Design and Methodological Rationale; Work Procedure and Data Collection.*

### 26 Were there ethical or sustainability concerns and how did you use AI

We used artificial workloads and no personal data or human participants. The experiments used sixteen AWS instances, but we did not measure energy use or emissions, so we cannot claim an environmental benefit. AI assisted with implementation, figures, summaries, and writing. We remain responsible for checking the work and interpreting the results.

*Report: Use of AI Tools; Ethics, Societal Aspects, and Sustainability.*

## Numbers to remember

| Item | Result or setting |
| --- | --- |
| Design | 5 topologies × 2 modes × 3 delays × 30 repetitions = 900 trials |
| Validation | All 900 trials passed setup and artifact checks |
| Stable-progress check | Forwarding 450/450; direct 309/450 |
| T3 successful client commits | Present in 90/90 forwarding trials; absent in all 90 direct trials |
| T2 throughput at 0.5 ms per link | Direct 63.34; forwarding 96.93 commits/s, about 53% higher |
| T2 throughput improvements | About 53%, 83%, and 69% at 0.5, 5, and 20 ms per link |
| T2 mean median latency at 0.5 ms per link | Direct 2.41; forwarding 4.08 ms |
| T2 mean median latency at 20 ms per link | Direct 99.47; forwarding 166.21 ms |
| T3 forwarding injection to first commit | About 20.44–22.12 seconds, including restart and setup |
| T3 forwarding throughput from client start | About 99.48–99.97 commits/s |
| Fixed settings | 500 ms heartbeat timeout, PreVote disabled, 100 requests/s, 64 byte payloads |
| Execution | 16 AWS workers; each individual trial's nodes and client share one host |

If you forget a decimal, give the rounded value and explain what it measures. The interpretation matters more than reciting every number.

## Source material

The section references above refer to the [final report](../final-report/II2202-report.tex). Latency values and contributing sample sizes are in the [latency table](../final-report/latency-table.tex). The underlying measurements are in the [900-trial campaign dataset](../../results/campaigns/full-20261014-hb500-noprevote/merged/dataset.jsonl).

Use this alongside the [presentation script](presentation-script.md). Keep the first answer brief enough to leave room for the opponent's follow-up.
