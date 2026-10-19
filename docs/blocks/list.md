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

`count` と `for_each` は相互排他ですが、tfstylus は同時指定を検証・修復せず、存在する属性を指定順に整列します。各属性式と、同じ順位の属性同士の相対順は保持します。

属性 `id` → `name` → `display_name` は、存在するものだけをこの固定順で `count` / `for_each` と `provider` の間に置きます。`*_id` とその他の `*_name` は一般属性のままです。同名ブロックや `config` 内部は対象外です。
`provider` 以降は `include_resource` → `limit` → 一般属性 → ブロック群の固有順位を維持します。

`config` と未知ブロックの内部は provider 固有または不明なため整列しません。同順位 block は入力相対順を保持します。未知属性は相対順を保持し、block 群より前に配置します。

## 警告条件

コメント所属・要素境界が曖昧な場合は block 全体を保持して警告します。要素の移動が不要なら、曖昧なコメントだけを理由に警告しません。ラベル数や未知 block の存在だけでは拒否しません。

必須属性、型・値、Terraform の意味は検証せず、属性や block の追加・削除も行いません。

## 設定

`list_order` で制御します。省略時は有効です。`false` にすると list block の変更・警告を行いません。
