#!/usr/bin/env bash
# Final collect/merge, upload merged/ to S3, destroy fleet.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cmd aws
ensure_aws_region

ID="$(campaign_id)"
export CAMPAIGN_ID="${ID}"
URI="$(s3_campaign_uri)"
LOCAL="$(campaign_local_dir)"

echo "finish: final collect + merge for ${ID} (region=${AWS_REGION})"
"${SCRIPTS_DIR}/collect.sh"
"${SCRIPTS_DIR}/merge.sh"

if [[ -d "${LOCAL}/merged" ]]; then
  echo "finish: uploading merged/ → ${URI}/merged/"
  aws s3 sync "${LOCAL}/merged/" "${URI}/merged/" \
    --region "${AWS_REGION}" \
    --exclude "*.tmp"
fi

# Also refresh viz once.
if [[ -f "${LOCAL}/merged/dataset.jsonl" ]]; then
  mkdir -p "${EXPERIMENTS_DIR}/viz/public/data"
  cp -f "${LOCAL}/merged/dataset.jsonl" "${EXPERIMENTS_DIR}/viz/public/data/dataset.jsonl"
fi

echo "finish: destroying fleet"
"${SCRIPTS_DIR}/infra-destroy.sh"

echo "finish: done — merged artifacts at ${LOCAL}/merged/ and ${URI}/merged/"
