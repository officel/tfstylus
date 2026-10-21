# check ブロック

- [check block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/check)
- [共通方針](README.md)

## 整列順

```hcl
check "example" {
	data "http" "example" {
		provider = http.example
		url = "https://example.com"
		retry {
			attempts = 2
		}
		depends_on = [terraform_data.example]
	}
	assert {
		condition = data.http.example.status_code == 200
		error_message = "Unexpected status code."
	}
}
```

外側と内側の最終順位は[一覧](README.md#処理後の並び順)のとおりです。data 本体の順位は[トップレベルの data](data.md)と共通ですが、check 内では data 固有のネスト内部を整列しません。

## 警告条件

外側の `data` → `assert` と assert 内部の順序を維持し、未知要素は一般属性・不透明ブロック群へ安定配置します。ラベル数や未知要素の存在だけでは拒否しません。一行本体、コメント所属、候補検証などの共通安全条件は[共通方針](README.md#共通方針)に従い、必要な場合は check 全体を変更せず警告します。

## 設定

`check_order` は check と内部の data / assert を制御します。
