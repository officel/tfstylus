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

## 現行の警告条件

次の場合は、output ブロック全体を保持して警告します。

- `postcondition` を含む未知・非対応の属性・ブロックがある場合。
- output のラベル数や識別子が不正、または `precondition` にラベルがある場合。
- 本体が一行の場合。

## 共通化後の差分（未実装）

既知属性と `precondition` の固有順序・内部保持は維持します。未知属性は `value` の後、ブロックの前の一般属性グループに保持します。
ラベル数・識別子・未知要素による妥当性検査は撤去し、未知ブロックの内部は整列しません。
移動時の安全条件は[共通方針](README.md#共通化後の方針未実装)に従います。

## 設定

`output_order` で制御します。
