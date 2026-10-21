# data ブロック

- [data block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/data)
- [共通方針](README.md)

## 整列順

```hcl
data "aws_instance" "example" {
	count = 1
	provider = aws.example
	instance_id = var.instance_id
	filter {
		name = "instance-state-name"
		values = ["running"]
	}
	tags = {
		Name = "example"
	}
	lifecycle {
		precondition {
			condition = var.instance_id != ""
			error_message = "An instance ID is required."
		}
		postcondition {
			condition = self.instance_state == "running"
			error_message = "The instance must be running."
		}
	}
	depends_on = [terraform_data.example]
}
```

共通順位の適用範囲と `tag` / `tags` の位置は[共通方針](README.md#共通方針)を参照してください。provider 固有のネストブロックと `dynamic` は同じグループです。
`lifecycle` 内の条件ブロックは precondition → postcondition の順に並べ替えますが、各条件内部は整列しません。check 内の data は [check ブロック](check.md)を参照してください。

## 警告条件

- 移動が必要な場合にコメント所属・要素境界を安全に確定できなければ、data ブロックを変更せず警告します。共通の安全条件は[共通方針](README.md#共通方針)を参照してください。
- `dynamic`、未知 block など順序依存の可能性がある要素は、不透明な block 群として入力相対順を保持します。

## 順位と保持

トップレベルの最終順位は[一覧](README.md#処理後の並び順)のとおりです。`lifecycle` 内は precondition → postcondition → 不透明な未知要素の順にし、条件内部は整列しません。check 内の data では lifecycle を含む固有ネスト内部を整列しません。

## 設定

`data_order` はトップレベルの data を制御します。check 内の data は `check_order` の対象です。
