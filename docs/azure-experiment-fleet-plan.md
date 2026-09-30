# Azure Experiment Fleet: Full Implementation Plan

## 1. Summary and fixed design

Build a reproducible Azure fleet that runs each Mininet/Raft trial entirely
inside one VM while distributing independent trials across 20 VMs.

- Freeze the main experiment matrix at:
  - 5 topologies: T0–T4
  - 2 modes: direct and forwarding
  - 3 delays: 0.5, 5, and 20 ms
  - 10 ms heartbeat
  - Pilot: 5 repetitions = 150 trials
  - Full: 20 repetitions = 600 trials
- Preserve the existing pending `pilot.yaml` and `full.yaml` corrections.
- Use 20 `Standard_D2s_v4` Ubuntu 22.04 VMs: two VMs in each of ten
  European regions.
- Keep every trial local to one VM; there is no cross-region experiment
  traffic.
- Use Terraform for Azure resources, Ansible for machine configuration,
  systemd for detached workers, and Azure Blob Storage for durable results.
- Provision the fleet only for a campaign. Destroy all VM, disk, NIC,
  public-IP, NSG, and VNet resources after verified collection.
- Retain the small storage/state foundation and raw experimental data
  indefinitely.
- Use €20 as the default campaign budget ceiling, despite the subscription
  having approximately €100 student credit.
- Treat region and worker as blocking variables in analysis.
- Pilot and full campaigns remain separate by default; pilot results are not
  silently reused as full-run repetitions.

Fleet distribution:

| Region | Workers |
|---|---:|
| Sweden Central | 2 |
| North Europe | 2 |
| West Europe | 2 |
| UK South | 2 |
| France Central | 2 |
| Germany West Central | 2 |
| Norway East | 2 |
| Switzerland North | 2 |
| Poland Central | 2 |
| Italy North | 2 |

Spain Central is the single-region reserve. If one primary region cannot
allocate both workers, replace that entire region with Spain Central. Do not
create an uneven or mixed-SKU fleet.

## 2. Harness and campaign model

Extend the Go harness from sequential range execution to immutable campaign
plans.

Public CLI:

```text
harness plan [flags] <config.yaml>
  --workers 20
  --inventory fleet.json
  --campaign-id ID
  --git-sha SHA
  --output campaign.json

harness run [flags] <config.yaml>
  --plan campaign.json
  --worker-id worker-00
  --output-dir PATH
  --resume
  --addr ADDRESS

harness merge [flags]
  --plan campaign.json
  --input PATH
  --output PATH

harness dry-run <config.yaml>
harness list <config.yaml>
harness summarize <results-dir>
```

Flags must precede the positional config argument consistently; documentation
and parsing tests will enforce this.

The versioned `campaign.json` will contain:

- Campaign ID, type, creation time, schema version, Git SHA, config checksum,
  and normalized embedded configuration.
- Worker ID, Azure region, VM name, and SKU.
- Every planned trial's global index, condition index, repetition, seed,
  worker assignment, and per-worker execution position.
- Expected trial and worker counts.
- A checksum for the immutable plan.

Sharding rules:

- Define 30 condition indices from topology × mode × delay × heartbeat.
- Full campaign assignment:
  `worker = (condition_index + repetition - 1) mod 20`.
- Each full condition therefore runs once on every worker and twice in every
  region.
- Pilot uses the same formula for repetitions 1–5, producing balanced loads
  of 7 or 8 trials per worker.
- Deterministically shuffle each worker's assigned execution order using the
  campaign plan checksum and worker ID. Trial seeds do not change with
  execution order.
- Replace index-dependent seeds with:
  `seed = base + condition_index × 1000 + (repetition - 1)`.
- Reject configurations with 1,000 or more repetitions. This makes pilot
  repetitions 1–5 identical to their corresponding full conditions even
  though the matrix sizes differ.

Resume and failure behavior:

- A trial with a finalized `status.json`, whether successful or failed, is
  never rerun by `--resume`.
- An incomplete trial directory is moved under
  `_attempts/<run-id>/<attempt-number>` before restarting.
- There is no automatic retry of finalized failures. Retrying one requires a
  new campaign.
- Exit code `0` means all assigned trials succeeded; `3` means all were
  finalized but one or more failed; `1` means the assignment was not fully
  finalized.
- The systemd worker considers exit codes 0 and 3 complete, uploads results,
  and records the failures.

Extend trial/dataset metadata with:

- `campaign_id`
- `worker_id`
- `region`
- `vm_id` and `vm_sku`
- `global_trial_index`
- `condition_index`
- `execution_position`
- Git, config, campaign-plan, and binary checksums
- CPU utilization, CPU steal, load, memory availability, and overload
  classification

`harness merge` must not silently omit failures. It produces one row per
planned trial, using nullable metric fields for failed or incomplete trials,
plus a validation report listing successes, failures, missing trials,
duplicates, checksum errors, and unexpected artifacts.

## 3. Infrastructure, configuration, and storage

Create three implementation areas: Terraform bootstrap/fleet modules, Ansible
roles/playbooks, and a root Makefile/operator scripts.

### Persistent foundation

A bootstrap Terraform module will create in Sweden Central:

- A dedicated foundation resource group.
- One private Azure Storage account with:
  - Anonymous/public Blob access disabled.
  - Entra ID authorization.
  - `tfstate` and `results` containers with separate access scopes.
  - Blob versioning.
  - 30-day soft delete.
  - No automatic lifecycle deletion of experiment data.
  - A deletion lock.
- Terraform bootstrap begins with local state, then automatically migrates
  that state into the new `tfstate` container. Local state and generated
  backend configuration are ignored by Git.

No storage keys or SAS tokens are placed in files. Operator access uses the
logged-in Azure CLI identity. VM system-assigned identities receive Blob Data
Contributor access only to the results container.

### Disposable fleet

A campaign Terraform module creates one resource group per region containing:

- One VNet and subnet.
- One NSG allowing SSH only from mandatory `SSH_CIDR`.
- Two `Standard_D2s_v4` VMs.
- Ubuntu Server 22.04 LTS Gen2.
- 64 GiB Standard SSD OS disks.
- Static Standard public IPs and NICs.
- System-assigned managed identities.
- Tags for project, campaign, region, owner, creation time, and expiration.
- Azure VM auto-shutdown as a cost-safety mechanism:
  - 45 minutes for infrastructure dry runs.
  - Four hours by default for real campaigns.

Terraform emits `fleet.json` and an Ansible inventory containing stable worker
IDs `worker-00` through `worker-19`.

No worker receives Azure Resource Manager permission to delete infrastructure.
Lifecycle control remains with the operator.

### Ansible configuration

Ansible will configure every worker identically:

- Dedicated unprivileged `experiment` account.
- Mininet 2.3.0 at commit
  `d7f399d7a1200b602bd6a95ae88bae1009664d4a`.
- Go 1.22.12.
- Open vSwitch, Python, networking/debugging packages, Git, and AzCopy.
- Exact clean, pushed experiment Git commit supplied through `GIT_SHA`.
- Reproducible Go builds with binary checksums.
- Root-owned `mininetd.service`.
- Unprivileged `experiment-worker.service`.
- Periodic result uploader and final upload service.
- Machine metadata containing CPU model/count, memory, Azure VM ID/SKU/region,
  image, kernel, package/tool versions, clock status, Git SHA, and checksums.

The launch preflight refuses a dirty, unpushed, or mismatched source revision.

### Results

Blob layout:

```text
campaigns/<campaign-id>/
  campaign.json
  config.yaml
  cost-estimate.json
  workers/<region>/<worker-id>/
    machine.json
    manifest.jsonl
    manifest.sha256
    runs/<run-id>/...
    complete.json
  merged/
    dataset.jsonl
    dataset.csv
    validation-report.json
    complete.json
```

The uploader synchronizes every 60 seconds without deleting remote objects. A
final sync writes hashes and uploads `complete.json` last. Fleet destruction
is permitted only after all worker markers exist, checksums validate, and
merged outputs have been uploaded.

## 4. Operator workflow and live infrastructure dry run

Expose the routine workflow through Make targets:

```text
make bootstrap
make infra-plan CAMPAIGN_ID=... MAX_RUNTIME_HOURS=4 MAX_BUDGET_EUR=20
make infra-dry-run SSH_CIDR=x.x.x.x/32 MAX_BUDGET_EUR=2
make infra-apply CAMPAIGN_ID=... SSH_CIDR=x.x.x.x/32
make configure GIT_SHA=<pushed-sha>
make qualify
make smoke
make launch CONFIG=experiments/configs/pilot.yaml
make status
make collect
make finish
make infra-destroy
make infra-dry-run-cleanup DRY_RUN_ID=...
```

`make finish` collects, merges, verifies Blob persistence, and destroys the
disposable fleet. `make infra-destroy` remains an idempotent recovery command.

Before any apply, preflight will:

- Verify Azure subscription and authentication.
- Query per-region total and Dsv4-family quotas.
- Confirm `Standard_D2s_v4` availability.
- Verify all 20 requested allocations fit quota.
- Query current Azure Retail Prices in EUR.
- Estimate compute, disk, and public-IP cost for the requested maximum
  duration.
- Fail closed if the estimate exceeds `MAX_BUDGET_EUR`.

Current expected fleet cost is approximately €2.17 per provisioned hour:

- Compute: €1.9602/hour.
- Disks: approximately €0.1186/hour.
- Public IPs: approximately €0.086/hour.

Expected campaign ranges:

- Infrastructure dry run: €0.55–€1.10.
- Smoke and pilot: approximately €1.60–€2.17.
- Full campaign: approximately €2.17–€3.25.
- Four-hour hard ceiling: approximately €8.66.

### Live infrastructure dry run

`make infra-dry-run` is a real, destructive rehearsal, distinct from
`harness dry-run`.

It will:

1. Create or verify the persistent foundation.
2. Run price, quota, SKU, budget, and SSH-CIDR preflight.
3. Create all ten regional resource groups and all 20 VMs.
4. Verify VM, disk, NIC, IP, VNet, subnet, NSG, identity, tag, and region
   counts.
5. Wait for SSH on every VM.
6. Run lightweight Ansible checks for Python, sudo, DNS, outbound HTTPS,
   Ubuntu repositories, GitHub access, Azure Instance Metadata Service, and
   synchronized system time.
7. Use every VM's managed identity to upload, download, checksum, and delete a
   unique Blob probe.
8. Upload `infra-dry-run-report.json`.
9. Always run Terraform destroy for all disposable resources, including on
   failed verification.
10. Query Azure by dry-run tags and fail if any VM, disk, NIC, public IP, or
    disposable resource group remains.

The persistent storage/state foundation is intentionally retained. An
auto-shutdown scheduled 45 minutes after creation limits compute exposure if
the operator process is interrupted. `KEEP_ON_FAILURE=1` is the only explicit
override that may retain a failed rehearsal fleet.

The dry-run report records timestamps, subscription, regions, resource counts,
connection results, identity/Blob results, estimated and elapsed cost, destroy
result, and remaining-resource query.

## 5. Qualification, campaigns, tests, and acceptance

### Worker qualification

Before pilot or full launch, every worker runs both smoke conditions, producing
40 smoke trials total.

Qualification passes only when:

- Both smoke conditions finalize successfully on all workers.
- CPU utilization p95 is below 75%.
- CPU steal p95 is below 5%.
- Load-1 p95 is below 1.5.
- Available memory never falls below 1.5 GiB.
- No OOM, Mininet, clock, or timing failures occur.
- Direct and forwarding connectivity checks behave as specified.
- Configured link delay is visible in measured RTT.

If qualification fails due to capacity, do not mix VM sizes. Destroy the fleet
and regenerate the campaign for ten `Standard_D4s_v4` workers, one per region.
With ten workers, each full condition runs twice per worker and twice per
region.

### Campaign gates

- Infrastructure dry run must pass before the first pilot.
- Pilot launch requires all 20 workers to qualify.
- After the 150-trial pilot, review variability, runtime, overload metrics,
  failures, and cost.
- Freeze any protocol changes before the full campaign.
- Any protocol, workload, timing, harness, or infrastructure change after the
  pilot requires a new campaign ID and prevents automatic pilot reuse.
- Full launch requires exactly 600 planned trials and 30 conditions × 20
  repetitions.

### Automated tests

- Go unit tests for matrix counts, canonical seeds, deterministic sharding,
  balanced pilot allocation, full all-worker coverage, shuffle stability,
  resume, finalized failures, incomplete attempts, duplicate detection, and
  merge completeness.
- Regression tests for 150 pilot and 600 full expansions.
- Dataset tests proving failures remain represented as rows.
- Terraform formatting and validation for bootstrap and fleet modules.
- Ansible syntax/idempotence checks and inventory validation.
- ShellCheck for lifecycle scripts.
- Static tests confirming no secrets, storage keys, SAS tokens, state files,
  generated inventories, or private keys can be committed.
- A mocked Azure lifecycle test for cleanup traps and orphan detection.
- The real `infra-dry-run` is the final infrastructure acceptance test.

### Documentation acceptance

Update the experiment READMEs and research-plan sources so they consistently
describe:

- 150 pilot and 600 full trials.
- Azure-hosted single-VM-per-trial execution.
- Parallel trials across a multi-region fleet.
- Region and worker blocking.
- Fixed VM SKU and qualification criteria.
- Blob result retention and reproducibility metadata.
- The distinction between the static harness dry run and live infrastructure
  dry run.
- Current cost estimates and mandatory teardown.

Success means a clean operator can run the documented Make workflow, create
and validate all 20 workers, execute a campaign in parallel, recover safely
after interruption, obtain a complete verified dataset in Blob Storage, and
leave no billable fleet resources after finishing.
