output "campaign_id" {
  description = "Campaign identifier."
  value       = var.campaign_id
}

output "results_bucket_name" {
  description = "S3 bucket used for campaign shards."
  value       = local.results_bucket
}

output "worker_public_ips" {
  description = "Public IPs of worker-00 .. worker-N keyed by name."
  value       = { for w in local.workers : w.name => w.public_ip }
}

output "worker_instance_ids" {
  description = "Instance IDs keyed by worker name."
  value       = { for w in local.workers : w.name => w.instance_id }
}

output "security_group_id" {
  description = "Worker security group ID."
  value       = aws_security_group.workers.id
}

output "instance_profile_name" {
  description = "IAM instance profile attached to workers."
  value       = aws_iam_instance_profile.worker.name
}

output "inventory_path" {
  description = "Path to generated Ansible inventory."
  value       = abspath(local_file.inventory.filename)
}

output "fleet_json_path" {
  description = "Path to generated fleet.json."
  value       = abspath(local_file.fleet_json.filename)
}

output "fleet" {
  description = "Fleet metadata (same content as fleet.json)."
  value       = local.fleet
}

output "ansible_inventory" {
  description = "Ansible inventory.ini contents for worker-00..N."
  value       = local.inventory_ini
}
