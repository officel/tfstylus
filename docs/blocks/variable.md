# variable ブロック

- [Variable block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/variable)
- [共通方針](README.md)

## 整列順

```hcl
variable "example" {
	description = "Example input."
	deprecated = "Use the replacement input."
	sensitive = false
	nullable = false
	ephemeral = false
	const = true
	type = string
	default = "example"
	validation {
		condition = var.example != ""
		error_message = "A value is required."
	}
}
```

重要度と情報量を重視した順序です。`validation` 内部は整列しません。
型と default の整合性、予約名、module をまたぐ重複や override の意味は検証しません。

## 現行の警告条件

次の場合は、variable ブロック全体を保持して警告します。

- 未知の属性・ブロックがある場合。
- ラベル数や識別子が不正、または `validation` にラベルがある場合。
- 本体が一行の場合。

## 共通化後の差分（未実装）

既知属性の固有順序と `validation` 内部保持は維持します。未知属性は `default` の後、ブロックの前の一般属性グループに保持します。
ラベル数・識別子・未知要素による妥当性検査は撤去し、未知ブロックの内部は整列しません。
移動時の安全条件は[共通方針](README.md#共通化後の方針未実装)に従います。

## 設定

`variable_order` で制御します。
