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

## 変更せず警告する条件

次の場合は、output ブロック全体を保持して警告します。

- `postcondition` を含む未知・非対応の属性・ブロックがある場合。
- output のラベル数や識別子が不正、または `precondition` にラベルがある場合。
- 本体が一行の場合。

## 設定

`output_order` で制御します。
