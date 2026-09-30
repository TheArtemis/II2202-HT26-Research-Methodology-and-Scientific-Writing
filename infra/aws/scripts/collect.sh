#!/usr/bin/env bash
# Collect raw worker shards from S3 → results/campaigns/<id>/workers/
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cmd aws

URI="$(s3_campaign_uri)"
LOCAL="$(campaign_local_dir)"
mkdir -p "${LOCAL}/workers"

echo "collect: ${URI}/workers/ → ${LOCAL}/workers/"
aws s3 sync "${URI}/workers/" "${LOCAL}/workers/"

# Also pull campaign-level config/fleet if present.
aws s3 cp "${URI}/config.yaml" "${LOCAL}/config.yaml" 2>/dev/null || true
aws s3 cp "${URI}/fleet.json" "${LOCAL}/fleet.json" 2>/dev/null || true

echo "collect: done → ${LOCAL}/workers/"
