# tfstylus test

- このファイルからコードフェンスを抽出して terraform のファイルを生成し、繰り返しテスト実行を可能にする

## テストファイル

- resource block の属性順をテストする
- ignore comment は `#` のみ有効
- 生成時は `//` にしておき、必要に応じて変更することでテスト可能にしている

### resource

- `count` / `for_each`、`id`、`name`、`display_name`、`provider`、provider 固有属性、provider 固有ブロック、`tags`、`lifecycle`、`depends_on` の順に整列する
- provider 固有ブロック、`dynamic`、`connection`、`provisioner` は相互の順序を保持する
- lifecycle 内の precondition / postcondition も整列する

```hcl:resource.tf
// tfstylus-ignore: resource_order
resource "aws_instance" "example" {
  depends_on = [terraform_data.example]
  provisioner "local-exec" {
    command = "echo done"
  }
  lifecycle {
    prevent_destroy = true
    postcondition {
      error_message = "The instance must be running."
      condition = self.instance_state == "running"
    }
    precondition {
      error_message = "An AMI is required."
      condition = var.ami != ""
    }
  }
  tags = {
    Name = "example"
  }
  ebs_block_device {
    device_name = "/dev/sdf"
    volume_size = 10
  }
  instance_type = "t3.micro"
  provider = aws.example
  ami = var.ami
  count = 1
}
```
