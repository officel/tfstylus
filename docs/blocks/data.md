# data ブロック

- [data block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/data)
- [共通方針](README.md)

## 整列順

```hcl
data "aws_instance" "example" {
	count = 1
	instance_id = var.instance_id
	provider = aws.example
	filter {
		name = "instance-state-name"
		values = ["running"]
	}
	lifecycle {
		precondition {
			condition = var.instance_id != ""
			error_message = "An instance ID is required."
		}
		postcondition {
			condition = self.instance_state == "running"
			error_message = "The instance must be running."
		}
	}
	depends_on = [terraform_data.example]
}
```

`count` の代わりに `for_each` も同じ位置に置きます。
provider 固有のネストブロックと `dynamic` は同じグループです。
`lifecycle` 内の条件ブロックは並べ替えますが、各条件内部は整列しません。
check 内の data は [check ブロック](check.md)の順序を使います。

## 現行の警告条件

- 並べ替えが必要な本体が一行の場合は、data ブロック全体を保持して警告します。
- 並べ替えが必要で、未知要素、`dynamic`、順序依存の可能性がある要素を安全に保てない場合も、data ブロック全体を保持して警告します。

## 共通化後の差分（未実装）

トップレベルでは `provider` を一般属性より前へ変更し、[共通順位](README.md#共通化後の方針未実装)に揃えます。
未知属性は一般属性として保持し、lifecycle 内の未知要素も存在だけでは拒否しません。
条件ブロックの配置と内部保持、反復ブロックの相対順、コメントの安全条件は維持します。
check 内の data は引き続き check 固有の順序です。

## 設定

`data_order` はトップレベルの data を制御します。check 内の data は `check_order` の対象です。
