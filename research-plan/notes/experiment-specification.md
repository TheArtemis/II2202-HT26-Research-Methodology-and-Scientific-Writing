# Experiment specification

This note records concrete methodological decisions for the research plan. A
decision marked **fixed** should be reflected consistently in the report,
experiment configuration, and run metadata. Items marked **to be fixed** must
be decided before the main experiment begins.

Measurement terminology and its attribution to the main reference are recorded
in [`measurement-vocabulary.md`](measurement-vocabulary.md).

## Raft implementation

- **Status:** Fixed
- **Implementation:** HashiCorp Raft
- **Repository:** <https://github.com/hashicorp/raft>
- **Language/module:** Go, `github.com/hashicorp/raft`
- **Role in the study:** The same unmodified Raft implementation will be used
  in both the direct-communication and transparent multi-hop routing
  conditions. Routing mode is the treatment; the Raft implementation is a
  controlled variable.
- **Version:** `v1.7.3` (released 20 March 2025)
- **Commit:** `c0dc6a0b2c7e889f31e5ab2f7ed90ceb159acffe`
- **Release:** <https://github.com/hashicorp/raft/releases/tag/v1.7.3>

The research plan should describe HashiCorp Raft as the selected implementation
rather than referring generically to “a Raft implementation.” It should also
state the exact version or commit, the relevant Raft configuration parameters,
and whether measurements come from the client, implementation instrumentation,
or both. Any source-code modifications must be documented and applied equally
to both routing conditions unless the modification implements the routing
treatment itself.

### How to cite the implementation

The GitHub repository can be included in the bibliography as a software
reference. Use it to identify the implementation used in the experiment, for
example, “The experiments use HashiCorp Raft version 1.7.3.” Use the original
Raft paper by Ongaro and Ousterhout instead when making claims about the Raft
algorithm or its theoretical properties.

Cite the exact release and commit used rather than the changing `main` branch.
The following entry can be added to `II2202-research-plan.bib`:

```bibtex
@misc{hashicorp_raft,
  author       = {{HashiCorp}},
  title        = {{Raft}: Golang Implementation of the Raft Consensus Protocol},
  howpublished = {GitHub repository},
  year         = {2025},
  url          = {https://github.com/hashicorp/raft},
  note         = {Version 1.7.3, commit
                  c0dc6a0b2c7e889f31e5ab2f7ed90ceb159acffe,
                  accessed 10 September 2026}
}
```

If the exact experimental version is later archived in a repository that
assigns a DOI, prefer the archived software citation and DOI while retaining
the GitHub URL as supporting information.

## Network emulation and transparent forwarding

- **Status:** Fixed
- **Network emulator:** Mininet
- **Repository:** <https://github.com/mininet/mininet>
- **Version:** `2.3.0`, the latest official stable release available on
  10 September 2026.
- **Commit:** `d7f399d7a1200b602bd6a95ae88bae1009664d4a`
- **Release:** <https://github.com/mininet/mininet/releases/tag/2.3.0>
- **Link emulator:** Mininet traffic-controlled links (`TCLink`).
- **Forwarding mechanism:** Preconfigured Linux routes and IP forwarding inside
  the Mininet hosts.
- **Role in the study:** Mininet will provide the complete controlled network
  testbed. Each Mininet host will run one HashiCorp Raft replica. Mininet will
  create the predefined links, inject link failures, and apply controlled
  per-link delay.
- **Link control:** `TCLink` will define delay and any other selected link
  properties. The same link parameters and failure schedule will be used in
  paired direct-only and forwarding runs.
- **Node identity:** Each Raft replica will use a stable address that does not
  change when the path to that replica changes.
- **Direct-only condition:** A replica may communicate only over its surviving
  direct links. Intermediate Mininet hosts do not forward Raft traffic.
- **Forwarding condition:** Preconfigured Linux routes and IP forwarding inside
  the Mininet hosts will carry Raft traffic over surviving multi-hop paths.
  HashiCorp Raft will continue using the same peer addresses and will not be
  modified to select or manage routes.
- **Controlled comparison:** Routing mode is the treatment. The Raft
  implementation, topology, failed links, delay, workload, and fault timing
  remain fixed within each pair of runs.

NIFTY will remain part of the related work and motivation, but its implementation
will not be used in the experiment. This keeps the study focused on the causal
effect of transparent multi-hop forwarding rather than NIFTY's failure-detection
or route-convergence behaviour.

Before the main experiment, a pilot using the three-node chained topology must
verify that B cannot reach C in direct-only mode, that B can reach C through A
when forwarding is enabled, and that the configured per-link delay is reflected
in the end-to-end path.

## Execution host

- **Status:** Partially fixed
- **Environment:** A single standard Linux virtual machine will host Mininet,
  all Raft replicas, and the client.
- **Specifications:** The VM's vCPU count, memory, Linux distribution and
  version, kernel version, and underlying processor are not fixed yet. The
  selected values must be recorded before the experiment and included in the
  run metadata and final report.

## Workload

- **Status:** Partially fixed
- **Clients:** One client.
- **Model:** Open-loop; request submission is scheduled independently of the
  completion of earlier requests.
- **To be fixed later:** Request rate, entry size, request timeout, trial
  duration, warm-up duration, failure-injection and recovery schedule, and
  random seeds.

## Experimental matrix and repetitions

- **Status:** Fixed, subject to the pilot rule below
- **Topologies:** Five (`T0`--`T4`).
- **Routing modes:** Two (direct-only and transparent forwarding).
- **Per-link delays:** Three (0.5, 5, and 20 ms).
- **Target repetitions:** 20 independent repetitions per condition.
- **Target size:** `5 topologies x 2 routing modes x 3 delays x 20
  repetitions = 600 runs`.

Twenty repetitions per condition is a defensible starting target, not a
guarantee of statistical power. The experiment harness will first automate five
pilot runs per condition to estimate variability and total execution time. The
target of 20 repetitions will then be confirmed or revised before the remaining
runs are collected, and the decision and its rationale will be documented. The
pilot runs may be included among the 20 repetitions only if the experimental
protocol and implementation remain unchanged after the pilot.

## Quality assurance

- Run the three-node forwarding pilot described above before the main
  experiment.
- Validate connectivity and configured delay before every experimental batch.
- Start each run from a clean state or restore the required log state through a
  reproducible procedure.
- Record random seeds and complete run metadata.
- Monitor host load during every run.
- Define failed-run and data-exclusion rules before collecting the main
  experimental data.

## Decisions still to be fixed

- VM specifications.
- Request rate, entry size, request timeout, trial duration, warm-up duration,
  failure-injection and recovery schedule, and random seeds.
- Measurement sources, output schema, and operational definitions of liveness,
  recovery, and commit latency.

The research plan will not prescribe the low-level Mininet interface/address
configuration, individual HashiCorp Raft timeout and storage settings, or the
mapping between topology labels and server addresses. These implementation
details will be defined by the experiment harness and recorded with the
experiment artifacts rather than treated as experimental factors.
