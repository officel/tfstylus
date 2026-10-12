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

## 現行の警告条件

並べ替えが必要な本体が一行の場合は、provider ブロック全体を保持して警告します。

## 共通化後の差分（未実装）

一般属性 → `alias` → `region` → ネストブロックの固有順序を維持します。
provider 設定を属性名だけでメタ引数とみなさず、スキーマ取得や未知属性の妥当性検査は追加しません。
共通のチャンク・編集・保持検証へ移し、ネスト内部保持と常時有効の設定を維持します。

## 設定

provider の整列は常に有効です。現時点では、個別に無効にする設定はありません。
