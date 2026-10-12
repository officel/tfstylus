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

## 現行の警告条件

次の場合は、action ブロック全体を変更せず警告します。

- ブロックのラベル構造が不正な場合。
- 外側に未知の属性・ブロックがある場合。
- ブロック本体が一行の場合。

## 共通化後の差分（未実装）

`count` / `for_each`、`provider`、一般属性、ブロックの共通順位を使い、`config` 内部は保持します。
ラベル構造や未知要素の存在だけを理由とする拒否は撤去します。
移動時のコメント所属・要素境界・非対象ソースの保持は[共通方針](README.md#共通化後の方針未実装)に従います。

## 設定

`action_order` で制御します。
