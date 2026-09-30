#!/usr/bin/env bash
# Smoke qualify on all workers.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cmd ansible-playbook
INV="$(resolve_inventory)"

cd "${ANSIBLE_DIR}"
ansible-playbook \
  -i "${INV}" \
  playbooks/qualify.yml \
  ${ANSIBLE_ARGS:-}

echo "qualify: ok"
echo "Next: make launch CONFIG=experiments/configs/full.yaml"
