# module ブロック

- [module block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/module)
- [共通方針](README.md)

## 整列順

```hcl
module "example" {
	source = "example/network/aws"
	version = "~> 1.0"
	count = 1
	id = var.id
	name = var.name
	cidr = var.cidr
	providers = {
		aws = aws.example
	}
	ignore_nested_deprecations = true
	depends_on = [terraform_data.example]
}
```

最終順位は[一覧](README.md#処理後の並び順)のとおりです。`id` / `name` は他のモジュール入力より前に置き、その他の入力は `providers` の前に置きます。
`providers`、`for_each`、入力式、依存リストの内部は整列しません。
`version` は registry のモジュール用の任意属性です。source の種類や必須性は検証しません。

## 警告条件

モジュール入力は名前だけでメタ引数と判断せず、各式内部も保持します。未知ブロックは不透明なブロック群として相対順を保ち、内部は整列しません。ラベル数や未知要素の存在だけでは拒否しません。

安全に移動できない場合の共通条件は[共通方針](README.md#共通方針)を参照してください。

## 設定

`module_order` で制御します。
