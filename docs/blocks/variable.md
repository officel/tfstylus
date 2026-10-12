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

次の場合は、variable ブロック全体を保持して警告します。

- 本体が一行の場合。
- 移動時にコメント所属・要素境界を安全に確定できない場合。
- 候補検証で式・コメント所属・非対象ソースの保持を確認できない場合。

既知属性の固有順序と `validation` 内部保持は維持します。未知属性は `default` 後の一般属性群、validation はその後、未知ブロックは不透明な末尾群に入力相対順で保持します。
ラベル数・識別子・未知要素の存在だけでは拒否しません。移動時の安全条件は[共通方針](README.md#共通方針)に従います。

## 設定

`variable_order` で制御します。
