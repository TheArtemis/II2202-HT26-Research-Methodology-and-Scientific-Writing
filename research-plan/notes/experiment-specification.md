# Experiment specification

This note records concrete methodological decisions for the research plan. A
decision marked **fixed** should be reflected consistently in the report,
experiment configuration, and run metadata. Items marked **to be fixed** must
be decided before the main experiment begins.

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

## Decisions still to be fixed

- Network emulation and transparent forwarding mechanism.
- Raft configuration, including heartbeat, election, commit, and lease
  timeouts; snapshot and log-store settings; and transport configuration.
- Mapping between topology node labels and Raft server IDs/addresses.
- Workload, trial duration, failure-injection schedule, repetitions, and random
  seeds.
- Measurement sources, output schema, and operational definitions of liveness,
  recovery, and commit latency.
