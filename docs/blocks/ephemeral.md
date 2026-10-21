# ephemeral ブロック

- [Ephemeral block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/ephemeral)
- [共通方針](README.md)

## 整列順

```hcl
ephemeral "example_session" "example" {
	count = 1
	name = var.name
	provider = example.alias
	options {
		timeout = 30
	}
	tags = {
		Name = "example"
	}
	lifecycle {
		precondition {
			condition = var.name != ""
			error_message = "A name is required."
		}
		postcondition {
			condition = self.token != ""
			error_message = "A token is required."
		}
	}
	depends_on = [terraform_data.example]
}
```

`example_session` は配置を示す仮の型です。トップレベルの最終順位と共通順位の適用範囲は[一覧](README.md#処理後の並び順)を参照してください。provider 固有のネストブロックと `dynamic` は同じグループです。
`lifecycle` 内の precondition / postcondition に加え、各条件内部も整列します。

## 警告条件

トップレベルの並べ替えが必要な本体が一行の場合、または内部の条件整列が必要なのに複数の `lifecycle` がある場合は、ephemeral ブロック全体を保持して警告します。コメント所属などの共通安全条件は[共通方針](README.md#共通方針)に従います。ラベル数やラベル名だけを理由とする拒否は行いません。

条件内は `condition` → `error_message` → その他の属性 → ブロックの順にし、未知属性・未知ブロックは各グループ内で入力相対順を保ちます。

## 設定

`ephemeral_order` で制御します。
