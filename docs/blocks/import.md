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

`id` の代わりに `identity` も使用できます。Terraform では `id` と `identity` は同時に指定できません。tfstylus はこの排他性や `to` の必須性、式の評価を検証せず、属性の追加・削除もしません。

`id` / `identity` は取り込む対象、`to` はその配置先なので、対象から配置先へ読む順にしています。

既知の属性以外は末尾のグループに入力順で保持します。属性、コメント、式は元ソースの断片として移動します。トップレベルの import block 同士は並べ替えません。

## 現行の警告条件

ラベル付きの import block は全体を保持して警告します。属性の移動が必要な場合に、ネストブロックや所属を確定できないコメント、曖昧なチャンク境界があれば、その block を変更せず警告します。一行の body は並べ替えず警告します。

## 共通化後の差分（未実装）

`id` / `identity` → `to` の固有順序と、一般属性を末尾に保持する規則は維持します。
ラベルとネストブロックの存在だけを理由とする拒否は撤去し、内部は不透明なまま保持します。
コメント・境界・順序保持の安全条件は[共通方針](README.md#共通化後の方針未実装)に従います。

## 設定

`import_order` で制御します。省略時は有効です。
