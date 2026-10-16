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

## 警告条件

並べ替えが必要な本体が一行、またはコメント所属・要素境界を安全に確定できない場合は provider ブロック全体を保持して警告します。

## 順位と保持

一般属性 → `alias` → `region` → ネストブロックの固有順序を維持します。同順位の要素は入力相対順を保ちます。
provider 設定を属性名だけでメタ引数とみなさず、未知属性・ブロックも一般グループで保持します。provider スキーマの取得や妥当性検査は行いません。
ネスト内部の順序を維持します。

## 設定

`provider_order` で有効・無効を切り替えます。省略時は有効、`false` で無効になります。
