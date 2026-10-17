# ephemeral ブロック

- [Ephemeral block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/ephemeral)
- [共通方針](README.md)

## 整列順

```hcl
ephemeral "example_session" "example" {
	count = 1
	name = var.name
	provider = example.alias
	options {
		timeout = 30
	}
	tags = {
		Name = "example"
	}
	lifecycle {
		precondition {
			condition = var.name != ""
			error_message = "A name is required."
		}
		postcondition {
			condition = self.token != ""
			error_message = "A token is required."
		}
	}
	depends_on = [terraform_data.example]
}
```

`example_session` は配置を示す仮の型です。`count` の代わりに `for_each` も同じ位置に置きます。
provider 固有のネストブロックと `dynamic` は同じグループです。
`tag` / `tags` は属性・ブロックを問わず、一般ブロックの後、`lifecycle` の前に置きます。
`lifecycle` 内の条件ブロックだけでなく、各条件内部も整列します。

## 警告条件

- 並べ替えが必要な本体が一行、または条件の内部整列が必要なのに複数の lifecycle がある場合は、ephemeral ブロック全体を保持して警告します。
ラベル数やラベル名だけを理由とする拒否は行いません。コメント所属・要素境界を確定できない場合は、変更せず警告します。

`provider` は一般属性より前に配置します。既存の lifecycle と条件内部の整列範囲を維持し、複数の lifecycle などは移動時の曖昧さがある場合に安全のためスキップします。
既知 condition/error_message 属性と pre/postcondition の順序を先に整列し、未知属性・未知 block はそれぞれの内部で不透明な末尾群として保持します。

## 設定

`ephemeral_order` で制御します。
