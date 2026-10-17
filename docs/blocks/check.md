# check ブロック

- [check block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/check)
- [共通方針](README.md)

## 整列順

```hcl
check "example" {
	data "http" "example" {
		url = "https://example.com"
		retry {
			attempts = 2
		}
		depends_on = [terraform_data.example]
		provider = http.example
	}
	assert {
		condition = data.http.example.status_code == 200
		error_message = "Unexpected status code."
	}
}
```

`data` 内の provider 固有属性・ブロックは同じグループです。`tag` / `tags` 属性・ブロックはその後、`depends_on` の前に置きます。
[トップレベルの data](data.md)とは異なる順序で、固有ネストブロックの内部は整列しません。

## 警告条件

次の場合は、check ブロック全体を保持して警告します。

- 並べ替えが必要な本体が一行の場合。
- 移動時にコメント所属・要素境界を安全に確定できない場合。
- 候補検証で式・コメント所属・非対象ソースの保持を確認できない場合。

ラベル数や未知要素の存在だけでは拒否しません。外側の `data` → `assert` と内部 data / assert の固有順序を維持し、未知要素は一般属性・不透明ブロック群へ安定配置します。data の provider 固有ネスト内部は整列しません。

## 設定

`check_order` は check と内部の data / assert を制御します。
