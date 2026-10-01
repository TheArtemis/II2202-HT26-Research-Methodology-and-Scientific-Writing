# Fleet Terraform — disposable 8-node EC2 workers
#
# Prerequisites: bootstrap applied (`make bootstrap` writes backend.hcl here).
#
#   terraform init -backend-config=backend.hcl
#   terraform apply -var=...   # or: make infra-apply from infra/aws
#
# Writes ../../ansible/inventory.ini and ../../ansible/fleet.json for Ansible/ops.
