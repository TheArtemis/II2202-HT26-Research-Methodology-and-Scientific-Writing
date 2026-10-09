#!/usr/bin/env bash
# Status: unique finalized status.json / complete.json markers in S3.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cmd aws
require_cmd python3
ensure_aws_region

URI="$(s3_campaign_uri)"
TOTAL_TRIALS="${TOTAL_TRIALS:-$DEFAULT_TOTAL_TRIALS}"
WORKERS="${WORKERS:-$DEFAULT_WORKERS}"
TMP="$(mktemp)"
trap 'rm -f "${TMP}"' EXIT

echo "campaign: $(campaign_id)"
echo "s3:       ${URI}/ (region=${AWS_REGION})"

aws s3 ls "${URI}/workers/" --recursive --region "${AWS_REGION}" 2>/dev/null >"${TMP}" || true

python3 - <<PY
from pathlib import Path
import re

lines = Path("${TMP}").read_text(encoding="utf-8").splitlines()
keys = []
for line in lines:
    parts = line.split()
    if len(parts) >= 4:
        keys.append(parts[-1])

complete = sorted(k for k in keys if k.endswith("complete.json"))
# Count UNIQUE run_ids — workers historically duplicated the same shard contents
# when --from/--limit were ignored, so raw status.json object counts inflate.
status_keys = [k for k in keys if k.endswith("status.json")]
run_ids = set()
for k in status_keys:
    # …/workers/worker-NN/<RUN_ID>/status.json
    m = re.search(r"/workers/[^/]+/([^/]+)/status\.json$", k)
    if m:
        run_ids.add(m.group(1))
    else:
        # fallback: parent dir name
        run_ids.add(Path(k).parent.name)

workers = int("${WORKERS}")
total = int("${TOTAL_TRIALS}")
unique = len(run_ids)
raw = len(status_keys)
print(f"complete.json: {len(complete)} / {workers} workers")
for c in complete:
    print(f"  ✓ {c}")
print(f"status.json:   {unique} unique run_ids / {total} expected  (raw objects={raw})")
pct = (100.0 * unique / total) if total else 0.0
print(f"progress:      {unique}/{total} ({pct:.1f}%)")
if len(complete) >= workers and unique >= total:
    print("status: ALL COMPLETE")
elif len(complete) >= workers:
    print("status: workers finished (check validation for missing runs)")
elif unique >= total and len(complete) == 0:
    print("status: IN PROGRESS (enough unique runs but no complete.json — workers may still be running or were torn down early)")
else:
    print("status: IN PROGRESS")
PY
