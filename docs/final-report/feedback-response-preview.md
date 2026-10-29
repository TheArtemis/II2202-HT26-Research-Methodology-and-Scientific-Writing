# Response to opposition feedback

**Status:** Implemented on 29 October 2026 and ready for review. Nothing has been submitted.

This response covers the three prioritised issues in Matyas Kozar's written opposition dated 19 October 2026. No separate teacher comments, seminar notes, or annotated report were provided, so it does not claim to address those sources.

## Decisions and implemented changes

| Comment | Decision | Change in the report |
| --- | --- | --- |
| 1. Explain the per-link delays | Accepted | Section 2.2 explains that 0.5, 5, and 20 ms are illustrative experimental settings, describes their role in comparing forwarding costs at a fixed heartbeat timeout, and avoids unsupported claims about regional or production latency. |
| 2. Improve the results structure | Accepted | Sections 3.1 and 3.2 now emphasise the principal comparisons, with T3 first. Detailed values are in Appendix A, Tables 3-8. The throughput, latency, and T3 recovery figures remain in the main report. |
| 3. Explain limited generalisability | Broader experiments reserved for future studies | Section 4.2 explains how shared host resources, static delays, and a fixed timeout limit the conclusions. Section 4.4 reserves separate-host/region deployments, variable delays, and timeout variation for a new experimental campaign. |

The first two comments improve the explanation and presentation without changing the experiment. The third identifies a valid boundary on the conclusions. Its request for clarification is addressed now; broader experimental validation remains future work.

## Results presentation

The revised results lead with the T3 contrast: successful commits in all 90 forwarding trials and none in direct mode. They report the T2 throughput and latency differences as approximate percentages, while directing readers to detailed tables for absolute values, variability, and sample sizes. No percentage improvement is claimed for T3 because its direct throughput is zero.

Appendix A contains:

- Table 3: throughput means and sample standard deviations for all conditions.
- Table 4: stable-progress counts and counts of trials without successful client commits.
- Table 5: recovery means, sample standard deviations, and contributing trial counts.
- Table 6: mean recorded election-event and term-change counts.
- Table 7: T3 client-start timing and throughput measured from client start.
- Table 8: the existing successful-request latency summary and contributing sample sizes, moved from the main results.

The new RQ1 and T3 timing tables are generated from the existing 900-trial dataset and run timelines by `gen_tables.py`. No experiments were rerun. The distinction between injection-based and client-start-based timing is retained.

The discussion now distinguishes the T1/T4 client-reachability cases from the T2 quorum-loss case. The previous wording attributed lower throughput too broadly to election instability. Validation wording also distinguishes successful experiment execution from successful client commits.

## Submission comment

The following text describes the implemented changes. Add any separate teacher or seminar feedback before submission if applicable.

> We accepted the first two prioritised comments in the written opposition. First, we clarified that the per-link delays of 0.5, 5, and 20 ms are illustrative experimental settings and explained their role in comparing communication modes with a fixed heartbeat timeout. We avoided attributing these settings to a particular deployment without supporting evidence. Second, we restructured the results by moving detailed numerical summaries into Appendix A, Tables 3-8, retaining the main comparisons and figures in the text, and giving greater prominence to T3, where forwarding produced successful commits in all 90 trials while direct communication produced none. We also clarified the distinction between client reachability and consensus progress in the discussion.
>
> For the third comment, we clarified how hosting replicas on one machine, using static emulated delays, and fixing the heartbeat timeout limit the conclusions. Broader validation remains future work: we did not extend the experiments to separate hosts or regions, variable network delays, or additional heartbeat timeouts because doing so would require a new experimental campaign beyond the present study. These evaluations are identified in the future-work section, and the conclusions remain bounded to the tested conditions. The concern about generalisability is therefore acknowledged, while the additional experimental work is deferred.

## Verification

The revised PDF compiled successfully with Tectonic. It contains 16 pages: one title/abstract page, ten main-report pages, two reference pages, and three appendix pages. The main report meets the stated ten-page limit. The 11 pt document setting is retained, the smaller latency-table font override has been removed, and no table of contents has been added.

The template layout is retained. Full compliance with any additional Canvas requirements has not been independently verified because the linked assignment could not be accessed; the checks above use the requirements quoted in the request.
