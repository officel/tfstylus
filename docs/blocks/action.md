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

`count` の代わりに `for_each` も同じ位置に置きます。`config` 内部は整列しません。

## 警告条件

次の場合は、action ブロック全体を変更せず警告します。

- ブロック本体が一行の場合。
- 移動時にコメントの所属や要素境界を安全に確定できない場合。
- 変換候補の検証で、式・コメント所属・非対象ソースの保持を確認できない場合。

## 順位と保持

`count` / `for_each`、`provider`、一般属性、ブロックの順に整列します。同順位の要素は元の相対順を維持します。
未知属性は一般属性、未知ブロックは不透明なブロックとして扱います。`config` と未知ブロックの内部は整列しません。
ラベル数や未知要素の存在だけでは拒否しません。移動時のコメント所属・要素境界・非対象ソースの保持は[共通方針](README.md#共通方針)に従います。

## 設定

`action_order` で制御します。
