#!/usr/bin/env bash
# Local merge of collected worker shards + harness summarize.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cmd python3

LOCAL="$(campaign_local_dir)"
WORKERS_DIR="${LOCAL}/workers"
MERGED_DIR="${LOCAL}/merged"
TOTAL_TRIALS="${TOTAL_TRIALS:-$DEFAULT_TOTAL_TRIALS}"

[[ -d "${WORKERS_DIR}" ]] || die "missing ${WORKERS_DIR} — run make collect first"

CFG_CANDIDATES=(
  "${LOCAL}/config.yaml"
  "${REPO_ROOT}/${CONFIG:-$DEFAULT_CONFIG}"
)
CFG=""
for c in "${CFG_CANDIDATES[@]}"; do
  if [[ -f "${c}" ]]; then
    CFG="${c}"
    break
  fi
done

python3 "${SCRIPTS_DIR}/merge.py" \
  --workers-dir "${WORKERS_DIR}" \
  --merged-dir "${MERGED_DIR}" \
  --expected "${TOTAL_TRIALS}" \
  ${CFG:+--config "${CFG}"}

ensure_harness_bin
(
  cd "${EXPERIMENTS_DIR}"
  ./bin/harness summarize "${MERGED_DIR}"
)

echo "merge: dataset → ${MERGED_DIR}/dataset.jsonl"
