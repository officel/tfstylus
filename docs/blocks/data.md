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
	tags = {
		Name = "example"
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
`tag` / `tags` は属性・ブロックを問わず、一般ブロックの後、`lifecycle` の前に置きます。
`lifecycle` 内の条件ブロックは並べ替えますが、各条件内部は整列しません。
check 内の data は [check ブロック](check.md)の順序を使います。

## 警告条件

- 一行の本体、または移動時にコメント所属・要素境界を安全に確定できない場合は、data ブロックを変更せず警告します。
- `dynamic`、未知 block など順序依存の可能性がある要素は、不透明な block 群として入力相対順を保持します。

## 順位と保持

トップレベルでは `provider` を一般属性より前へ置きます。未知属性は一般属性群に保持します。
`lifecycle` 内は precondition → postcondition → 不透明な未知要素の順で整列します。条件内部は整列しません。
コメントの安全条件と反復要素の相対順を維持します。
check 内の data は引き続き check 固有の順序です。

## 設定

`data_order` はトップレベルの data を制御します。check 内の data は `check_order` の対象です。
