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

`config` の内部は provider 固有のため整列しません。未知属性は相対順を保持し、`config` より前に配置します。

## 現行の警告条件

list block のラベル数が2でない場合は、その block を保持して警告します。整列が必要な状態で未知のネストブロックがある場合や、コメント所属・要素境界が曖昧な場合も block 全体を保持して警告します。要素の移動が不要なら、曖昧なコメントだけを理由に警告しません。

必須属性、型・値、Terraform の意味は検証せず、属性や block の追加・削除も行いません。

## 共通化後の差分（未実装）

`count` / `for_each` は同順位へ変更し、両方がある入力でも相対順を維持して排他検証・修復はしません。
`include_resource`、`limit` の固有順序、未知属性の位置、`config` 内部保持と `.tfquery.hcl` 専用の適用範囲は維持します。
ラベル数や未知ネストブロックの存在だけによる拒否は撤去し、安全条件は[共通方針](README.md#共通化後の方針未実装)に従います。

## 設定

`list_order` で制御します。省略時は有効です。`false` にすると list block の変更・警告を行いません。
