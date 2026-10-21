# import ブロック

- [import block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/import)
- [共通方針](README.md)

## 整列順

```hcl
import {
	for_each = local.buckets
	provider = aws.west
	id = each.value
	to = aws_s3_bucket.this[each.key]
}
```

最終順位は[一覧](README.md#処理後の並び順)のとおりです。`id` の代わりに `identity` も使用できます。Terraform では `id` と `identity` は同時に指定できません。tfstylus はこの排他性や `to` の必須性、式の評価を検証せず、属性の追加・削除もしません。

`id` / `identity` は取り込む対象、`to` はその配置先なので、対象から配置先へ読む順にしています。未知属性・未知ブロックは末尾の各グループに入力順で保持し、トップレベルの import block 同士は並べ替えません。
移動時のコメント所属・要素境界など共通の安全条件は[共通方針](README.md#共通方針)に従います。

## 設定

`import_order` で制御します。省略時は有効です。
