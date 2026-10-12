# module ブロック

- [module block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/module)
- [共通方針](README.md)

## 整列順

```hcl
module "example" {
	count = 1
	source = "example/network/aws"
	version = "~> 1.0"
	name = var.name
	cidr = var.cidr
	providers = {
		aws = aws.example
	}
	ignore_nested_deprecations = true
	depends_on = [terraform_data.example]
}
```

`count` の代わりに `for_each` も同じ位置に置きます。
モジュール固有入力は `version` と `providers` の間に置き、入力同士の相対順を保持します。
`providers`、`for_each`、入力式、依存リストの内部は整列しません。
`version` は registry のモジュール用の任意属性です。source の種類や必須性は検証しません。

## 変更せず警告する条件

ラベル数が1でない場合は、module ブロック全体を保持して警告します。
未知のネストブロックがあり、属性の整列が必要な場合も全体を保持して警告します。
未知の属性はモジュール固有入力として扱い、警告対象にはしません。

## 設定

`module_order` で制御します。
