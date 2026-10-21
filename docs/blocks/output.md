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

本体が一行の場合や共通の安全条件を満たさない場合は、output ブロック全体を保持して警告します。共通条件は[共通方針](README.md#共通方針)を参照してください。

## 順位と保持

最終順位は[一覧](README.md#処理後の並び順)のとおりです。`precondition` の内部と未知ブロックの内部は整列しません。ラベル数・識別子・未知要素による妥当性検査は行いません。

## 設定

`output_order` で制御します。
