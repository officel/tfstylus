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
`lifecycle` 内の条件ブロックだけでなく、各条件内部も整列します。

## 現行の警告条件

- ラベル数が2でない場合は、ephemeral ブロック全体を保持して警告します。ラベルの命名制約などの意味検証は行いません。
- `lifecycle`、`precondition`、`postcondition` にラベルがある場合も、ephemeral ブロック全体を保持して警告します。
- 並べ替えが必要な本体が一行、または条件の内部整列が必要なのに複数の lifecycle がある場合は、ephemeral ブロック全体を保持して警告します。
- lifecycle 内の未知要素・不正構造や、安全に分類できない要素があり、移動が必要な場合は、ephemeral ブロック全体を保持して警告します。

## 共通化後の差分（未実装）

`provider` を一般属性より前へ変更し、[共通順位](README.md#共通化後の方針未実装)に揃えます。
ラベル数や未知要素による妥当性検査は撤去し、属性・内部ブロックの存在を検証しません。
既存の lifecycle と条件内部の整列範囲は維持します。複数の lifecycle などは存在だけで拒否せず、個々の元断片と相対順を安全に保持できるかで判断します。

## 設定

`ephemeral_order` で制御します。
