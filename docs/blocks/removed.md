# removed ブロック

- [removed block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/removed)
- [共通方針](README.md)

## 整列順

```hcl
removed {
	from = aws_instance.old
	lifecycle {
		destroy = true
	}
	connection {
		host = self.public_ip
	}
	provisioner "local-exec" {
		when = destroy
		command = "echo goodbye"
	}
}
```

`from`、`lifecycle`、`connection` / `provisioner` の3グループだけを整列します。`connection` と `provisioner` は同じグループで、入力の相対順序を保持します。上の例と異なり `provisioner` が `connection` より前にあっても、その順序は変えません。複数の `provisioner` の順序も保持します。

`connection` と `provisioner` の内部は整列しません。`provisioner` 内の `connection`、`lifecycle` 内部も元のまま保持します。removed は移行・削除のために短期間使われるブロックであり、技術的に記述できることと通常使用することは別なので、内部の整列規則は追加しません。

属性・ブロック・コメント・式は元ソースの断片として移動します。トップレベルの removed block 同士は並べ替えません。必須性、参照先、`destroy` や `when` の値、provisioner の種類は検証せず、要素の追加・削除もしません。

## 警告条件

ラベル数や未知要素の存在だけでは拒否しません。未知属性は `from` の後、lifecycle より前の一般属性群に、未知ブロックは connection/provisioner と同じ不透明な末尾群に入力順で保持します。
整列が必要な場合にコメント所属やチャンク境界を確定できない場合、一行の body の場合は block を変更せず警告します。`connection` / `provisioner` の相対順と内部保持も維持します。

## 設定

`removed_order` で制御します。省略時は有効です。
