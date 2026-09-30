resource "aws_instance" "worker" {
  count = var.worker_count

  ami                         = data.aws_ami.ubuntu_2204.id
  instance_type               = var.instance_type
  subnet_id                   = element(local.subnet_ids, count.index)
  vpc_security_group_ids      = [aws_security_group.workers.id]
  key_name                    = aws_key_pair.fleet.key_name
  iam_instance_profile        = aws_iam_instance_profile.worker.name
  associate_public_ip_address = true

  root_block_device {
    volume_type           = "gp3"
    volume_size           = var.root_volume_gb
    delete_on_termination = true
    encrypted             = true
  }

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }

  tags = {
    Name        = local.worker_names[count.index]
    Worker      = local.worker_names[count.index]
    WorkerIndex = tostring(count.index)
    ExpiresAt   = local.kill_at
  }

  lifecycle {
    ignore_changes = [
      tags["ExpiresAt"],
      ami,
    ]
  }
}
