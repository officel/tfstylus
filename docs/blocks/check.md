# check ブロック

tfstylus は `check` ブロック内の配置と、ネストした `data` / `assert` の内部を整えます。
Terraform の必須ブロック数や条件の意味は検証せず、ラベル構造と安全に移動できる要素を確認します。

- [check block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/check)
- [tfstylus の共通方針](../../README.md#共通方針)

## 整列順

check 本体は次の順序に整列します。

1. `data` ブロック
2. `assert` ブロック

この順序は tfstylus のスタイル規則であり、Terraform の必須順序ではありません。
複数の `data`、複数の `assert` は、それぞれ元の相対順を保持します。

### data 内部

1. provider 固有の属性とネストブロック
2. `depends_on`
3. `provider`

provider 固有の属性・ブロックは同じグループとして扱い、相互の相対順を保持します。
[トップレベルの data](data.md)の整列順は適用しません。

### assert 内部

1. `condition`
2. `error_message`

Terraform は check に1つ以上の assert を要求し、各 assert に `condition` と `error_message` を要求しますが、tfstylus はその欠落や条件式の意味を検証しません。

## 保持する内容

- 要素と所属コメントを元ソースのまとまりとして移動し、式、文字列、heredoc、コメントを再生成しません。
- provider 固有要素の名前・意味・スキーマを推測せず、ブロック内部の元ソースを保持します。
- `depends_on` の参照リスト内の順序を保持します。
- provider のスキーマ取得、provider の取得や Terraform の実行は行いません。
- check ブロック同士やトップレベルブロックの順序は変更しません。

## 変更せず警告する条件

次の場合は、check ブロック全体を保持して警告します。

- check のラベル数が1でない、data のラベル数が2でない、または assert にラベルがある場合。
- check / assert 本体に未知の属性・ブロックがある場合。
- 並べ替えが必要な本体が一行の場合。
- 並べ替えが必要で、コメントの所属や要素の境界を確定できない場合。

整列済みで移動が不要なら、曖昧な境界コメントだけでは警告しません。

## 設定

`check_order` は省略時に `true` です。
[設定ファイル](../../README.md#設定ファイル)で `false` にすると、check とその内部の data / assert の整列による変更・警告を行いません。

```hcl
check_order = false
```
