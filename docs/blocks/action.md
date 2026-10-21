# action ブロック

- [action block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/action)
- [共通方針](README.md)

## 整列順

```hcl
action "aws_lambda_invoke" "example" {
  count = 3
  provider = aws.alias
  config {
    function_name = "example-function"
  }
}
```

action の最終順位は[一覧](README.md#処理後の並び順)のとおりです。`config` 内部は整列しません。

## 警告条件

本体が一行の場合は action ブロック全体を変更せず警告します。その他の共通安全条件は[共通方針](README.md#共通方針)を参照してください。

未知属性は一般属性、未知ブロックは不透明なブロックとして扱います。`config` と未知ブロックの内部は整列しません。ラベル数や未知要素の存在だけでは拒否しません。

## 設定

`action_order` で制御します。
