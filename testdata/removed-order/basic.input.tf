# file header
locals { keep = "same" }
removed {
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
  # source address
  from = aws_instance.old
  // shared connection
  connection {
    user = "root"
    host = self.public_ip
    type = "ssh"
  }
  /* removal policy */
  lifecycle {
    destroy = true
  }
  provisioner "remote-exec" {
    inline = ["echo goodbye"]
    when   = destroy
  }
}
# file footer
