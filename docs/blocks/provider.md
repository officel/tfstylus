# provider ブロック

- [Provider block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/provider)
- [共通方針](README.md)

## 整列順

```hcl
provider "aws" {
	profile = "example"
	alias = "example"
	region = "ap-northeast-1"
	default_tags {
		tags = { Environment = "example" }
	}
}
```

`alias` / `region` 以外の属性を先頭に置きます。
`region` は名前による特例で、すべての provider に存在するとは限りません。
`default_tags` も他のネストブロックと同じグループです。

## 変更せず警告する条件

並べ替えが必要な本体が一行の場合は、provider ブロック全体を保持して警告します。

## 設定

provider の整列は常に有効です。現時点では、個別に無効にする設定はありません。
