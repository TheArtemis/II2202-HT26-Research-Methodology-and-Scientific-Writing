#!/usr/bin/env bash
# Idempotent fleet teardown (EC2 + networking leftovers). Keeps S3 results.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cmd terraform
load_campaign_env

AWS_REGION="${AWS_REGION:-$DEFAULT_REGION}"
export AWS_REGION AWS_DEFAULT_REGION="${AWS_REGION}"

: "${CAMPAIGN_ID:?set CAMPAIGN_ID=… (or run make infra-apply so ${CAMPAIGN_ENV} exists)}"
: "${SSH_CIDR:?set SSH_CIDR=<ip>/32 (or run make infra-apply so ${CAMPAIGN_ENV} exists)}"

if [[ -z "${SSH_PUBLIC_KEY:-}" ]]; then
  for candidate in "${HOME}/.ssh/id_ed25519.pub" "${HOME}/.ssh/id_rsa.pub"; do
    if [[ -f "${candidate}" ]]; then
      SSH_PUBLIC_KEY="$(cat "${candidate}")"
      break
    fi
  done
fi
[[ -n "${SSH_PUBLIC_KEY:-}" ]] || die "set SSH_PUBLIC_KEY or place ~/.ssh/id_ed25519.pub"

cd "${TF_FLEET}"
if [[ ! -f backend.hcl ]]; then
  die "missing ${TF_FLEET}/backend.hcl"
fi

terraform init -backend-config=backend.hcl >/dev/null
terraform destroy -auto-approve \
  -var="aws_region=${AWS_REGION}" \
  -var="campaign_id=${CAMPAIGN_ID}" \
  -var="ssh_cidr=${SSH_CIDR}" \
  -var="ssh_public_key=${SSH_PUBLIC_KEY}"

# Clean generated inventory/fleet pointers (optional; regeneratable).
rm -f "${INVENTORY_INI}" "${FLEET_JSON}" "${CAMPAIGN_ENV}"

echo "infra-destroy: fleet torn down (S3 campaigns/ retained)"
