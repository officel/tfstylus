# moved ブロック

- [moved block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/moved)
- [共通方針](README.md)

## 整列順

```hcl
moved {
	from = aws_instance.old
	to = aws_instance.new
}
```

`from` は移動元、`to` は移動先なので、移動元から移動先へ読む順にしています。Terraform では両方の属性が必須ですが、tfstylus は必須性、参照先の妥当性、式の評価を検証せず、属性の追加・削除もしません。

既知の属性以外は末尾のグループに入力順で保持します。属性、コメント、式は元ソースの断片として移動します。トップレベルの moved block 同士は並べ替えません。

## 現行の警告条件

ラベル付きの moved block は全体を保持して警告します。属性の移動が必要な場合に、ネストブロックや所属を確定できないコメント、曖昧なチャンク境界があれば、その block を変更せず警告します。一行の body は並べ替えず警告します。

## 共通化後の差分（未実装）

`from` → `to` と一般属性グループの相対順は維持します。
ラベルとネストブロックの存在だけを理由とする拒否は撤去し、内部は不透明なまま保持します。
移動時の安全条件は[共通方針](README.md#共通化後の方針未実装)に従います。

## 設定

`moved_order` で制御します。省略時は有効です。
