#!/usr/bin/env bash
# Launch equal shards: worker i → --from i*chunk --limit chunk (chunk=ceil(900/16)=57).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_ansible
INV="$(resolve_inventory)"
require_fleet_json

TOTAL_TRIALS="${TOTAL_TRIALS:-$DEFAULT_TOTAL_TRIALS}"
WORKERS="${WORKERS:-$DEFAULT_WORKERS}"
CHUNK="$(chunk_size)"
CONFIG_REL="$(config_rel_for_worker)"

echo "launch: ${TOTAL_TRIALS} trials / ${WORKERS} workers → chunk=${CHUNK}"
echo "  config (on worker): ${CONFIG_REL}"
python3 - <<PY
chunk = int("${CHUNK}")
workers = int("${WORKERS}")
for i in range(workers):
    print(f"  worker-{i:02d}: --from {i * chunk} --limit {chunk}")
PY

# Inventory from Terraform already embeds from_trial/limit_trials; verify they match.
python3 - <<PY
import re, sys
chunk = int("${CHUNK}")
text = open("${INV}", encoding="utf-8").read()
# INI lines: worker-00 ... from_trial=0 limit_trials=113
pat = re.compile(r"^(worker-(\d+))\s+.*\bfrom_trial=(\d+)\b.*\blimit_trials=(\d+)\b", re.M)
rows = pat.findall(text)
if not rows:
    # YAML inventory may encode differently — skip hard check.
    print("note: could not parse from_trial/limit_trials from inventory; trusting host vars", file=sys.stderr)
    sys.exit(0)
for name, idx, frm, lim in rows:
    i = int(idx)
    expect_from = i * chunk
    expect_lim = chunk
    if int(frm) != expect_from or int(lim) != expect_lim:
        print(f"error: {name} has from_trial={frm} limit_trials={lim}, expected {expect_from}/{expect_lim}", file=sys.stderr)
        sys.exit(1)
print(f"inventory shard check: ok ({len(rows)} workers)")
PY

cd "${ANSIBLE_DIR}"
ansible-playbook \
  -i "${INV}" \
  playbooks/launch.yml \
  -e "config=${CONFIG_REL}" \
  ${ANSIBLE_ARGS:-}

echo "launch: workers started"
echo "On laptop: make watch CAMPAIGN_ID=$(campaign_id)"
