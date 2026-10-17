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

`count` の代わりに `for_each` も同じ位置に置きます。
provider 固有のネストブロック、`dynamic`、`connection`、`provisioner` は同じグループです。
`tag` / `tags` は属性・ブロックを問わず、一般ブロックの後、`lifecycle` の前に置きます。
`lifecycle` 内部は整列しません。

## 警告条件

既存の順序を[共通順位](README.md#共通方針)の基本とします。lifecycle 内部は引き続き保持します。
provider 固有の未知属性・ブロックは一般属性・ブロック群に安定順で保持します。ラベル数や未知要素の存在だけでは拒否しません。
`dynamic`、`connection`、`provisioner` 等の相対順を変更せず、lifecycle 内部も整列しません。
コメント所属や要素境界を確定できず安全に移動できない場合は、block を変更せず警告します。

## 設定

`resource_order` で制御します。
