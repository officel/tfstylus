# resource ブロック

- [resource block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/resource)
- [共通方針](README.md)

## 整列順

```hcl
resource "aws_instance" "example" {
	count = 1
	provider = aws.example
	ami = var.ami
	instance_type = "t3.micro"
	ebs_block_device {
		device_name = "/dev/sdf"
		volume_size = 10
	}
	tags = {
		Name = "example"
	}
	lifecycle {
		prevent_destroy = true
	}
	depends_on = [terraform_data.example]
}
```

共通順位の適用範囲と `tag` / `tags` の位置は[共通方針](README.md#共通方針)を参照してください。provider 固有のネストブロック、`dynamic`、`connection`、`provisioner` は同じグループです。`lifecycle` 内部は整列しません。

## 警告条件

provider 固有の未知属性・ブロックは一般属性・ブロック群に安定順で保持します。`dynamic`、`connection`、`provisioner` 等の相対順も変更しません。安全に移動できない場合の共通条件は[共通方針](README.md#共通方針)を参照してください。

## 設定

`resource_order` で制御します。
