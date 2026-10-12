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

`data` 内の provider 固有属性・ブロックは同じグループです。
[トップレベルの data](data.md)とは異なる順序で、固有ネストブロックの内部は整列しません。

## 現行の警告条件

次の場合は、check ブロック全体を保持して警告します。

- check のラベル数が1でない、data のラベル数が2でない、または assert にラベルがある場合。
- check / assert 本体に未知の属性・ブロックがある場合。
- 並べ替えが必要な本体が一行の場合。

## 共通化後の差分（未実装）

外側の `data` → `assert` と、内部 data / assert の固有順序は維持し、共通の順位・編集・保持検証へ移します。
ラベル数と未知要素による妥当性検査は撤去します。data の固有ネスト内部を新たに整列しません。
安全に移動できない場合の保持と警告は[共通方針](README.md#共通化後の方針未実装)に従います。

## 設定

`check_order` は check と内部の data / assert を制御します。
