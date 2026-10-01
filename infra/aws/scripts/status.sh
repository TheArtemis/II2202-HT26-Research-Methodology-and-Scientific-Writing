#!/usr/bin/env bash
# Status: count finalized status.json / complete.json markers in S3.
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
lines = Path("${TMP}").read_text(encoding="utf-8").splitlines()
keys = []
for line in lines:
    parts = line.split()
    if len(parts) >= 4:
        keys.append(parts[-1])
complete = [k for k in keys if k.endswith("complete.json")]
status = [k for k in keys if k.endswith("status.json")]
workers = int("${WORKERS}")
total = int("${TOTAL_TRIALS}")
print(f"complete.json: {len(complete)} / {workers} workers")
for c in sorted(complete):
    print(f"  ✓ {c}")
print(f"status.json:   {len(status)} / {total} expected trials")
pct = (100.0 * len(status) / total) if total else 0.0
print(f"progress:      {len(status)}/{total} ({pct:.1f}%)")
if len(complete) >= workers and len(status) >= total:
    print("status: ALL COMPLETE")
elif len(complete) >= workers:
    print("status: workers finished (check validation for missing runs)")
else:
    print("status: IN PROGRESS")
PY
