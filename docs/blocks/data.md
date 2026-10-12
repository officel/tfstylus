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

## 変更せず警告する条件

- 並べ替えが必要な本体が一行の場合は、data ブロック全体を保持して警告します。
- 並べ替えが必要で、未知要素、`dynamic`、順序依存の可能性がある要素を安全に保てない場合も、data ブロック全体を保持して警告します。

## 設定

`data_order` はトップレベルの data を制御します。check 内の data は `check_order` の対象です。
