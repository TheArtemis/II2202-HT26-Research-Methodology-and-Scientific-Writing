#!/usr/bin/env bash
# Create disposable 8-node fleet + write inventory.ini / fleet.json.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cmd terraform
require_cmd aws

: "${CAMPAIGN_ID:?set CAMPAIGN_ID=full-YYYYMMDD}"
: "${SSH_CIDR:?set SSH_CIDR=<your-public-ip>/32}"

AWS_REGION="${AWS_REGION:-$DEFAULT_REGION}"
export AWS_REGION AWS_DEFAULT_REGION="${AWS_REGION}"

SSH_PUBLIC_KEY="${SSH_PUBLIC_KEY:-}"
if [[ -z "${SSH_PUBLIC_KEY}" ]]; then
  for candidate in "${HOME}/.ssh/id_ed25519.pub" "${HOME}/.ssh/id_rsa.pub"; do
    if [[ -f "${candidate}" ]]; then
      SSH_PUBLIC_KEY="$(cat "${candidate}")"
      break
    fi
  done
fi
[[ -n "${SSH_PUBLIC_KEY}" ]] || die "set SSH_PUBLIC_KEY or place ~/.ssh/id_ed25519.pub"

WORKER_COUNT="${WORKERS:-$DEFAULT_WORKERS}"

cd "${TF_FLEET}"
if [[ ! -f backend.hcl ]]; then
  die "missing ${TF_FLEET}/backend.hcl — run make bootstrap (it writes this file)"
fi

terraform init -backend-config=backend.hcl
terraform apply -auto-approve \
  -var="aws_region=${AWS_REGION}" \
  -var="campaign_id=${CAMPAIGN_ID}" \
  -var="ssh_cidr=${SSH_CIDR}" \
  -var="ssh_public_key=${SSH_PUBLIC_KEY}" \
  -var="worker_count=${WORKER_COUNT}" \
  ${RESULTS_BUCKET:+-var="results_bucket_name=${RESULTS_BUCKET}"}

# Ensure outputs land where scripts expect (Terraform local_file defaults).
[[ -f "${INVENTORY_INI}" ]] || die "expected inventory at ${INVENTORY_INI}"
[[ -f "${FLEET_JSON}" ]] || die "expected fleet.json at ${FLEET_JSON}"

BUCKET="$(results_bucket)"
URI="s3://${BUCKET}/campaigns/${CAMPAIGN_ID}"

# Persist apply vars for finish/infra-destroy (terraform requires them even on destroy).
write_campaign_env "${CAMPAIGN_ID}" "${SSH_CIDR}" "${SSH_PUBLIC_KEY}" "${BUCKET}" "${AWS_REGION}"

# Seed campaign metadata on S3 (config + fleet.json).
aws s3 cp "${FLEET_JSON}" "${URI}/fleet.json"
CFG_SRC="${REPO_ROOT}/${CONFIG:-$DEFAULT_CONFIG}"
if [[ -f "${CFG_SRC}" ]]; then
  aws s3 cp "${CFG_SRC}" "${URI}/config.yaml"
fi

echo
echo "Fleet ready: ${CAMPAIGN_ID}"
echo "  inventory: ${INVENTORY_INI}"
echo "  fleet:     ${FLEET_JSON}"
echo "  campaign:  ${CAMPAIGN_ENV}"
echo "  s3:        ${URI}/"
echo "Next: make configure GIT_SHA=\$(git rev-parse HEAD)"
