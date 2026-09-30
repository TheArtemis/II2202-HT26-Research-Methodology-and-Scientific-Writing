#!/usr/bin/env bash
# Ansible configure: Mininet, Go, build harness, systemd units; checkout GIT_SHA.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cmd ansible-playbook

: "${GIT_SHA:?set GIT_SHA=<commit> (push a clean commit first)}"
REPO_URL="${REPO_URL:-https://github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing.git}"

INV="$(resolve_inventory)"
export GIT_SHA REPO_URL

cd "${ANSIBLE_DIR}"
ansible-playbook \
  -i "${INV}" \
  site.yml \
  -e "git_sha=${GIT_SHA}" \
  -e "repo_url=${REPO_URL}" \
  ${ANSIBLE_ARGS:-}

echo "configure: ok (GIT_SHA=${GIT_SHA})"
echo "Next: make qualify"
