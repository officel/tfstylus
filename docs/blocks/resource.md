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
	lifecycle {
		prevent_destroy = true
	}
	depends_on = [terraform_data.example]
}
```

`count` の代わりに `for_each` も同じ位置に置きます。
provider 固有のネストブロック、`dynamic`、`connection`、`provisioner` は同じグループです。
`lifecycle` 内部は整列しません。

## 現行の警告条件

ラベル数が2でない場合は、resource ブロック全体を保持して警告します。
provider 固有の未知の属性・ブロックは警告対象ではなく、グループ内に保持します。

## 共通化後の差分（未実装）

既存の順序を[共通順位](README.md#共通化後の方針未実装)の基本とします。lifecycle 内部は引き続き保持します。
ラベル数による妥当性検査は撤去し、未知要素を含むグループの安定順とコメント所属を検証します。
`dynamic`、`connection`、`provisioner` 等の相対順を変更せず、共通の編集・保持検証へ移します。

## 設定

`resource_order` で制御します。
