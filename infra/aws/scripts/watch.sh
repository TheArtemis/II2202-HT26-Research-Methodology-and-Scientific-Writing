#!/usr/bin/env bash
# Watch loop: every 5 min collect → merge → summarize → copy into viz/public/data/.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cmd aws
require_cmd python3
ensure_aws_region

INTERVAL="${WATCH_INTERVAL_SEC}"
TOTAL_TRIALS="${TOTAL_TRIALS:-$DEFAULT_TOTAL_TRIALS}"
WORKERS="${WORKERS:-$DEFAULT_WORKERS}"
VIZ_DATA="${EXPERIMENTS_DIR}/viz/public/data"
ID="$(campaign_id)"

echo "watch: campaign=${ID} region=${AWS_REGION} interval=${INTERVAL}s (Ctrl-C to stop)"
echo "  viz data → ${VIZ_DATA}/dataset.jsonl"

sync_viz() {
  local merged dataset
  merged="$(campaign_local_dir)/merged"
  dataset="${merged}/dataset.jsonl"
  mkdir -p "${VIZ_DATA}"
  if [[ -f "${dataset}" ]]; then
    cp -f "${dataset}" "${VIZ_DATA}/dataset.jsonl"
    echo "watch: synced dataset.jsonl → viz ($(wc -l < "${dataset}" | tr -d ' ') rows)"
  else
    echo "watch: no dataset.jsonl yet"
  fi
}

all_complete() {
  local uri complete_count
  uri="$(s3_campaign_uri)"
  complete_count="$(aws s3 ls "${uri}/workers/" --recursive --region "${AWS_REGION}" 2>/dev/null | grep -c 'complete\.json$' || true)"
  [[ "${complete_count}" -ge "${WORKERS}" ]]
}

cycle=0
while true; do
  cycle=$((cycle + 1))
  echo
  echo "===== watch cycle ${cycle} @ $(date -u +%Y-%m-%dT%H:%M:%SZ) ====="
  "${SCRIPTS_DIR}/collect.sh"
  "${SCRIPTS_DIR}/merge.sh"
  sync_viz
  "${SCRIPTS_DIR}/status.sh" || true

  if all_complete; then
    LOCAL="$(campaign_local_dir)"
    PRESENT=0
    if [[ -f "${LOCAL}/merged/validation-report.json" ]]; then
      PRESENT="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["present"])' "${LOCAL}/merged/validation-report.json")"
    fi
    if [[ "${PRESENT}" -ge "${TOTAL_TRIALS}" ]]; then
      echo "watch: all ${WORKERS} workers complete and ${PRESENT}/${TOTAL_TRIALS} runs present — exiting"
      exit 0
    fi
    echo "watch: workers complete but present=${PRESENT}/${TOTAL_TRIALS}; continuing briefly…"
  fi

  echo "watch: sleeping ${INTERVAL}s…"
  sleep "${INTERVAL}"
done
