#!/usr/bin/env bash
# Persistent bootstrap: S3 results/tfstate + DynamoDB lock (once per account/region).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cmd terraform
require_cmd aws

AWS_REGION="${AWS_REGION:-$DEFAULT_REGION}"
export AWS_REGION AWS_DEFAULT_REGION="${AWS_REGION}"

cd "${TF_BOOTSTRAP}"
terraform init
terraform apply -auto-approve \
  -var="aws_region=${AWS_REGION}"

# Write fleet remote-backend config so infra-apply can init without a manual copy.
mkdir -p "${TF_FLEET}"
BACKEND_HCL="${TF_FLEET}/backend.hcl"
terraform output -raw backend_hcl_snippet >"${BACKEND_HCL}"
echo
echo "Bootstrap complete."
echo "  Wrote ${BACKEND_HCL}"
echo "Next: make infra-apply CAMPAIGN_ID=full-YYYYMMDD SSH_CIDR=<you>/32"
