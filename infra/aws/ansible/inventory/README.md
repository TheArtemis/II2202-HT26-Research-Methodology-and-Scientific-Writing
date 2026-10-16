# Placeholder: Terraform `make infra-apply` writes inventory.ini next to ansible.cfg.
# Prefer that file. If you maintain a YAML inventory manually, place it here as hosts.yml
# and run: ansible-playbook -i inventory/hosts.yml …
#
# Expected host vars (per worker-00..worker-04):
#   ansible_host, ansible_user=ubuntu
#   worker_index, from_trial, limit_trials, instance_id
# Group vars on [workers:vars]:
#   campaign_id, results_bucket, s3_prefix, aws_region
