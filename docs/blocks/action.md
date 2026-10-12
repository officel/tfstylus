# action ブロック

tfstylus は `action "<TYPE>" "<LABEL>"` ブロック内の要素を並べ替えます。
provider 固有の引数やネストしたブロックは `config` 内に置かれ、外側の独立した整列対象にはしません。

- [action block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/action)
- [tfstylus の共通方針](../../README.md#共通方針)

## 整列順

1. `count` または `for_each`
2. `provider`
3. `config` ブロック

この順序は tfstylus のスタイル規則であり、Terraform の必須順序ではありません。
同じグループ内の要素は元の相対順を保持します。
Terraform では `count` と `for_each` は相互排他ですが、両方がある入力でも tfstylus は検証・修復せず、相対順を保持します。

整列後の配置例です。

```hcl
action "aws_lambda_invoke" "example" {
  count = 3
  provider = aws.alias
  config {
    function_name = "example-function"
  }
}
```

## 保持する内容

- 要素とそれに所属するコメントを、元ソースのまとまりとして移動します。式、文字列、heredoc、コメントは再生成しません。
- `config` 内部の属性・ブロックの順序と内容は変更しません。provider のスキーマを参照せず、元ソースをそのまま保持します。
- action ブロック同士や、トップレベルブロックの順序は変更しません。
- 必須属性、provider 固有のスキーマ、要素の重複、`count` / `for_each` の排他性、式や action の意味は検証・自動修復しません。

## 変更せず警告する条件

次の場合は、action ブロック全体を変更せず警告します。

- ブロックのラベル構造が不正な場合。
- 外側に未知の属性・ブロックがある場合。
- ブロック本体が一行の場合。
- 並べ替えが必要で、コメントの所属を確定できない場合。

すでに整列済みで移動が不要なら、曖昧な境界コメントだけでは警告しません。

## 設定

`action_order` は省略時に `true` です。
[設定ファイル](../../README.md#設定ファイル)で `false` にすると、action の整列による変更・警告を行いません。

```hcl
action_order = false
```
