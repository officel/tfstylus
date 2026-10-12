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

## 変更せず警告する条件

次の場合は、action ブロック全体を変更せず警告します。

- ブロックのラベル構造が不正な場合。
- 外側に未知の属性・ブロックがある場合。
- ブロック本体が一行の場合。

## 設定

`action_order` で制御します。
