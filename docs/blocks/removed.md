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

最終順位は[一覧](README.md#処理後の並び順)のとおりです。`connection` と `provisioner` は同じグループで、入力の相対順序を保持します。上の例と異なり `provisioner` が `connection` より前にあっても、その順序は変えません。複数の `provisioner` の順序も保持します。

`connection` と `provisioner` の内部は整列しません。`provisioner` 内の `connection`、未知の lifecycle 要素の内部は元のまま保持します。`lifecycle` の要素は[共通順位](../../design.md#lifecycle-ブロック内の順序)で整列します。removed は移行・削除のために短期間使われるブロックであり、技術的に記述できることと通常使用することは別なので、その他の内部整列規則は追加しません。

トップレベルの removed block 同士は並べ替えません。必須性、参照先、`destroy` や `when` の値、provisioner の種類は検証せず、要素の追加・削除もしません。未知属性は `from` の後、lifecycle より前に、未知ブロックは connection / provisioner と同じ不透明な末尾群に入力順で保持します。
共通の安全条件は[共通方針](README.md#共通方針)に従い、`connection` / `provisioner` の相対順と内部保持も維持します。

## 設定

`removed_order` で制御します。省略時は有効です。
