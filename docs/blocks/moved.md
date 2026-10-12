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

## 警告条件

ラベル数や未知属性・ブロックの存在だけでは拒否しません。未知属性と不透明なネストブロックは既知属性の後に入力相対順を保ちます。
属性の移動が必要な場合にコメント所属やチャンク境界を確定できない場合、一行の body の場合は block を変更せず警告します。

## 設定

`moved_order` で制御します。省略時は有効です。
