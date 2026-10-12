# file header
locals { keep = "same" }
removed {
  # source address
  from = aws_instance.old
  /* removal policy */
  lifecycle {
    destroy = true
  }
  # first provisioner
  provisioner "local-exec" {
    command = <<-EOT
      echo "# keep this text"
    EOT
    when    = destroy
    connection {
      user = "root"
      host = self.public_ip
      type = "ssh"
    }
  }
  // shared connection
  connection {
    user = "root"
    host = self.public_ip
    type = "ssh"
  }
  provisioner "remote-exec" {
    inline = ["echo goodbye"]
    when   = destroy
  }
}
# file footer
