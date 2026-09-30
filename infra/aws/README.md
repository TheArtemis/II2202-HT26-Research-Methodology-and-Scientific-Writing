# AWS EC2 experiment fleet

Disposable **5-node** fleet in **eu-north-1** for the Stage-4 full campaign
([`experiments/configs/full.yaml`](../../experiments/configs/full.yaml)): **30 repetitions → 900 trials**,
sharded equally (**180 trials / worker**). Workers upload **raw** shards to S3; your laptop merges,
summarizes, and refreshes the local viz. Compute is disposable; S3 (results + tfstate) is persistent.

Run all Make targets from this directory (`cd infra/aws`). Terraform writes Ansible inventory to
`ansible/inventory.ini` and `ansible/fleet.json` (see `ansible/inventory/hosts.yml.example` only if you
maintain a YAML inventory by hand).

## Defaults (locked)

| Item | Value |
|------|--------|
| Region | `eu-north-1` |
| Workers | 5 × on-demand `c5a.xlarge` (Ubuntu 22.04) |
| Campaign | `full.yaml`, `repetitions: 30` → **900** trials (`5 × 2 × 3 × 30`) |
| Sharding | harness `--from` / `--limit` (equal chunks of 180) |
| Merge | **local laptop only** (workers never merge) |
| Live viz | laptop polls S3 every **5 min**; browser auto-refreshes with `?live=1` |
| Hard ceiling | **4 hours** (EventBridge auto-terminate; clock starts at `infra-apply`) |
| SSH | restrict with `SSH_CIDR=<you>/32` |

## Timing math (why 4h)

Per-trial wall time with `observe: 30s` is roughly **45–55s**. Across 5 workers:

- 180 × 50s ≈ **150 min** of pure trial time
- Plus ~15–25 min for provision / Ansible / smoke qualify before `launch`

The kill switch is armed at **terraform apply**, not at launch, so a 3h ceiling
is too tight. Default `max_runtime_hours=4`. Cost stays small
(~4h × 5 × `c5a.xlarge`).

Equal shard math (N=900, W=5):

- `chunk = ceil(900 / 5) = 180`
- worker `i` → `--from $((i * 180)) --limit 180`
- Ranges: `[0,180)`, `[180,360)`, `[360,540)`, `[540,720)`, `[720,900)`
- The harness expands with **repetition outermost**, so each 180-trial shard
  covers **all** topology×mode×delay conditions (6 reps each). That avoids
  confounding worker identity with topology.
- Identical frozen YAML + `seeds.base` on every host so `seed = base + global_index`
  stays correct
## Operator lifecycle

1. Push a clean commit → note `GIT_SHA`
2. `make bootstrap` (once — persistent S3 + DynamoDB lock; writes `terraform/fleet/backend.hcl`)
3. `make infra-apply CAMPAIGN_ID=full-YYYYMMDD SSH_CIDR=<you>/32`
   (writes `ansible/inventory.ini`, `ansible/fleet.json`, and `.campaign.env` for finish/destroy)
4. `make configure GIT_SHA=…`
5. `make qualify` (smoke on all 5)
6. `make launch CONFIG=experiments/configs/full.yaml`
7. On the laptop (two terminals):
   - `cd experiments/viz && npm run sync-data && npm run dev`
   - `make watch CAMPAIGN_ID=…` (from `infra/aws`) **or** `cd experiments/viz && CAMPAIGN_ID=… npm run watch-s3`
   - Open [http://localhost:5173/?live=1](http://localhost:5173/?live=1) (or use the **Live** toggle)
8. `make finish` → final local merge + upload `merged/` + **destroy EC2**
9. If abandoned: EventBridge kills instances at 4h; still run `make infra-destroy` for leftover networking/disks

### `SSH_CIDR`

Pass your public IP as `/32` so the security group allows SSH **only** from you, e.g.:

```bash
make infra-apply CAMPAIGN_ID=full-20260930 SSH_CIDR=203.0.113.10/32
```

Do not use `0.0.0.0/0`. Egress on workers is HTTPS/DNS for apt/git/S3.

## Make targets (operator laptop)

| Target | Action |
|--------|--------|
| `bootstrap` | Persistent S3 bucket + tfstate lock (once); writes `fleet/backend.hcl` |
| `infra-apply` | Create 5 EC2 + `ansible/inventory.ini` / `fleet.json` / `.campaign.env` |
| `configure` | Ansible + pin `GIT_SHA` |
| `qualify` | Smoke on all 5 |
| `launch` | Equal shards, start workers |
| `status` | Count finalized runs / `complete.json` in S3 |
| `collect` | `aws s3 sync` workers → `results/campaigns/<id>/workers/` |
| `merge` | Union run dirs + manifests → `merged/`; run `./bin/harness summarize` **locally** |
| `watch` | Loop every **5 min**: collect → merge → summarize → `viz` `sync-data` |
| `finish` | Final collect/merge → upload `merged/` to S3 → destroy fleet |
| `infra-destroy` | Idempotent teardown (uses `.campaign.env` from last apply) |

### Local merge rules

- Include only runs with finalized `status.json`
- Concatenate per-worker `manifest.jsonl` (dedupe by `run_id`)
- Write `validation-report.json` (expected 900, present, failed, missing)
- Never require EC2 SSH for merge — **S3 is the source of truth**

### Local watch + viz

```bash
# Terminal A — frontend
cd experiments/viz
npm run sync-data && npm run dev
# http://localhost:5173/?live=1

# Terminal B — S3 → merge → summarize → public/data/dataset.jsonl
cd infra/aws
make watch CAMPAIGN_ID=full-YYYYMMDD
# or: cd experiments/viz && CAMPAIGN_ID=full-YYYYMMDD npm run watch-s3
```

`npm run watch-s3` shells out to `make watch` when `infra/aws/Makefile` exists; otherwise it prints
this flow. The viz polls `/data/dataset.jsonl` every 5 minutes with a cache-bust query param when
live mode is on. **No public S3 bucket and no browser AWS credentials.**

## S3 layout

Bucket (bootstrap): `ii2202-experiments-<account_id>` with prefixes `tfstate/` and `campaigns/`.

```text
campaigns/<campaign-id>/
  config.yaml
  fleet.json
  workers/worker-0N/
    machine.json
    manifest.jsonl
    <RUN_ID>/...
    complete.json
  merged/                    # written by laptop on finish (and optionally during watch)
    dataset.jsonl
    dataset.csv
    validation-report.json
```

Workers sync only their raw shard under `workers/<worker-id>/` (timer ~every 60s). The laptop
writes `merged/` on `finish` (and during `watch` if configured).

## Architecture (summary)

```text
Operator laptop --terraform--> fleet (5× c5a.xlarge)
Operator laptop --ansible----> configure / launch shards
Workers --------s3 sync------> campaigns/<id>/workers/...
Laptop ---------s3 sync------> local cache → merge → summarize → viz/public/data/
Viz (?live=1) --fetch--------> /data/dataset.jsonl every 5 min
finish -----------------------> upload merged/ + destroy EC2
```

## Out of scope (v1)

- Multi-region fleet, Spot instances
- Browser-direct S3 / Cognito
- New Go `harness plan|merge` CLI (shell/Python local merge is enough)
- Changing topology/mode/delay matrix (only reps → 30)
