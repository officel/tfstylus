# list block

- [list block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/tfquery/list)
- [共通方針](README.md)

## 対象

`.tfquery.hcl` ファイルの `list` block を整列します。通常の `.tf` ファイルへ list 整列を適用しません。

## 整列順

```hcl
list "<TYPE>" "<LABEL>" {
	count = <number>
	for_each = <map or set>
	provider = <provider>.<alias>
	include_resource = <boolean>
	limit = <number>
	config {
		<provider-specific arguments>
	}
}
```

最終順位は[一覧](README.md#処理後の並び順)のとおりです。`count` と `for_each` の相互排他は検証・修復しません。各属性式と同順位要素の相対順は保持します。
`config` 内部は整列しません。`tag` / `tags` は他のブロックと異なり、一般属性・ブロックとして扱います。

未知ブロックの内部は provider 固有または不明なため整列しません。同順位 block と一般属性は入力相対順を保持します。

## 警告条件

コメント所属・要素境界が曖昧な場合は block 全体を保持して警告します。要素の移動が不要なら、曖昧なコメントだけを理由に警告しません。共通の保持方針は[共通方針](README.md#共通方針)を参照してください。

必須属性、型・値、Terraform の意味は検証せず、属性や block の追加・削除も行いません。

## 設定

`list_order` で制御します。省略時は有効です。`false` にすると list block の変更・警告を行いません。
