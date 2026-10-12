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

## 現行の警告条件

ラベル付きの removed block は全体を保持して警告します。整列が必要な場合に、未知の属性・ブロック、所属を確定できないコメント、曖昧なチャンク境界があれば、その block を変更せず警告します。一行の body は並べ替えず警告します。整列が不要な場合は、未知要素があっても変更・警告を行いません。

## 共通化後の差分（未実装）

`from` → `lifecycle` → `connection` / `provisioner` の固有順序と内部保持は維持します。
未知属性は `from` の後の一般属性として保持し、未知ブロックは不透明なまま扱います。
ラベルと未知要素の存在だけを理由とする拒否は撤去します。provisioner 等の相対順と移動時の安全条件は[共通方針](README.md#共通化後の方針未実装)に従います。

## 設定

`removed_order` で制御します。省略時は有効です。
