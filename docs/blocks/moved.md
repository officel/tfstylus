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

最終順位は[一覧](README.md#処理後の並び順)のとおりです。`from` は移動元、`to` は移動先なので、移動元から移動先へ読む順にしています。Terraform では両方の属性が必須ですが、tfstylus は必須性、参照先の妥当性、式の評価を検証せず、属性の追加・削除もしません。

未知属性と不透明なネストブロックは既知属性の後に入力相対順を保ちます。トップレベルの moved block 同士は並べ替えません。

ラベル数や未知要素の存在だけでは拒否しません。移動時のコメント所属・要素境界など共通の安全条件は[共通方針](README.md#共通方針)に従います。

## 設定

`moved_order` で制御します。省略時は有効です。
