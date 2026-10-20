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

`count` の代わりに `for_each` も同じ位置に置きます。両方がある場合は同順位として元の相対順を保持します。
`id`、`name` は他のモジュール入力より前に置きます。その他の入力は `providers` の前に置き、入力同士の相対順を保持します。
`providers`、`for_each`、入力式、依存リストの内部は整列しません。
`version` は registry のモジュール用の任意属性です。source の種類や必須性は検証しません。

## 警告条件

`source`、`version`、`count` / `for_each`、`id`、`name`、その他の入力、`providers`、ネストブロック、`ignore_nested_deprecations`、`depends_on` の順に整列します。
モジュール入力は名前だけでメタ引数と判断せず、各式内部も保持します。未知ブロックは不透明なブロック群として相対順を保ち、内部は整列しません。ラベル数や未知要素の存在だけでは拒否しません。

移動時にコメント所属・要素境界を確定できない場合は、変更せず警告します。安全条件は[共通方針](README.md#共通方針)に従います。

## 設定

`module_order` で制御します。
