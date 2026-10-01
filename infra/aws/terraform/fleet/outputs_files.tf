locals {
  workers = [
    for i, instance in aws_instance.worker : {
      name               = local.worker_names[i]
      index              = i
      instance_id        = instance.id
      public_ip          = instance.public_ip
      private_ip         = instance.private_ip
      availability_zone  = instance.availability_zone
      from_trial         = i * local.trials_per_worker
      limit_trials       = local.trials_per_worker
    }
  ]

  fleet = {
    campaign_id        = var.campaign_id
    project            = var.project
    region             = var.aws_region
    results_bucket     = local.results_bucket
    s3_prefix          = "campaigns/${var.campaign_id}/"
    instance_type  = var.instance_type
    worker_count   = var.worker_count
    workers        = local.workers
  }

  inventory_ini = join("\n", concat(
    [
      "[workers]",
    ],
    [
      for w in local.workers :
      format(
        "%s ansible_host=%s ansible_user=ubuntu worker_index=%d from_trial=%d limit_trials=%d instance_id=%s",
        w.name,
        w.public_ip,
        w.index,
        w.from_trial,
        w.limit_trials,
        w.instance_id,
      )
    ],
    [
      "",
      "[workers:vars]",
      "ansible_python_interpreter=/usr/bin/python3",
      "campaign_id=${var.campaign_id}",
      "results_bucket=${local.results_bucket}",
      "s3_prefix=campaigns/${var.campaign_id}/",
      "aws_region=${var.aws_region}",
      "",
    ],
  ))
}

resource "local_file" "inventory" {
  filename = "${path.module}/${var.inventory_path}"
  content  = local.inventory_ini
}

resource "local_file" "fleet_json" {
  filename = "${path.module}/${var.fleet_json_path}"
  content  = jsonencode(local.fleet)
}
