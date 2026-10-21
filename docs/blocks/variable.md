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

## 警告条件

本体が一行の場合や共通の安全条件を満たさない場合は、variable ブロック全体を保持して警告します。共通条件は[共通方針](README.md#共通方針)を参照してください。

最終順位は[一覧](README.md#処理後の並び順)のとおりです。`validation` 内部は整列せず、未知ブロックは不透明な末尾群に入力相対順で保持します。ラベル数・識別子・未知要素の存在だけでは拒否しません。

## 設定

`variable_order` で制御します。
