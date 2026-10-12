# output ブロック

- [Output block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/output)
- [共通方針](README.md)

## 整列順

```hcl
output "example" {
	description = "Example output."
	deprecated = "Use the replacement output."
	sensitive = false
	ephemeral = false
	type = string
	value = var.example
	precondition {
		condition = var.example != ""
		error_message = "A value is required."
	}
	depends_on = [terraform_data.example]
}
```

`precondition` 内部は整列しません。
root / child module の文脈による属性の制約は検証しません。

## 警告条件

次の場合は、output ブロック全体を保持して警告します。

- 本体が一行の場合。
- 移動時にコメント所属・要素境界を安全に確定できない場合。
- 候補検証で式・コメント所属・非対象ソースの保持を確認できない場合。

## 順位と保持

既知属性と `precondition` の固有順序・内部保持は維持します。未知属性は `value` の後の一般属性グループへ、未知ブロックは不透明なブロック群へ安定配置します。
ラベル数・識別子・未知要素による妥当性検査は行いません。未知ブロックの内部は整列しません。
移動時の安全条件は[共通方針](README.md#共通方針)に従います。

## 設定

`output_order` で制御します。
