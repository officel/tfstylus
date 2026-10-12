# ephemeral ブロック

tfstylus は `ephemeral "<TYPE>" "<LABEL>"` ブロック内の要素と、`lifecycle` 内の条件ブロックを並べ替えます。
provider 固有の属性・ブロックは内容を解釈せず、グループ内の相対順を保持します。

- [Ephemeral block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/ephemeral)
- [tfstylus の共通方針](../../README.md#共通方針)

## 整列順

1. `count` または `for_each`
2. provider 固有属性
3. `provider`
4. provider 固有のネストブロックと `dynamic`
5. `lifecycle` ブロック
6. `depends_on`

この順序は tfstylus のスタイル規則であり、Terraform の必須順序ではありません。
存在する要素だけを整列し、同じグループ内の相対順を保持します。
Terraform では `count` と `for_each` は相互排他ですが、tfstylus は同時指定や重複を検証・修復しません。

## lifecycle 内部

1. `precondition` ブロック
2. `postcondition` ブロック

同じ種類の条件ブロックの相対順を保持します。
各条件ブロック内部は `condition`、`error_message` の順に整列し、条件式は評価・書き換えません。

## 保持する内容

- 要素と所属コメントを元ソースのまとまりとして移動し、式、文字列、heredoc、コメントを再生成しません。
- provider 固有ブロックと `dynamic` の内部は解釈・整列せず、元ソースを保持します。
- provider のスキーマ取得、必須要素の検証、provider の取得や Terraform の実行は行いません。
- ephemeral ブロック同士やトップレベルブロックの順序は変更しません。

## 変更せず警告する条件

- ラベル数が2でない場合は、ephemeral ブロック全体を保持して警告します。ラベルの命名制約などの意味検証は行いません。
- `lifecycle`、`precondition`、`postcondition` にラベルがある場合も、ephemeral ブロック全体を保持して警告します。
- 並べ替えが必要な本体が一行、または条件の内部整列が必要なのに複数の lifecycle がある場合は、ephemeral ブロック全体を保持して警告します。
- lifecycle 内の未知要素・不正構造や、安全に分類できない要素があり、移動が必要な場合は、ephemeral ブロック全体を保持して警告します。
- 並べ替えが必要で、コメントの所属や要素の境界を確定できない場合も、ephemeral ブロック全体を保持して警告します。

整列済みで移動が不要なら、曖昧な境界コメントだけでは警告しません。

## 設定

`ephemeral_order` は省略時に `true` です。
[設定ファイル](../../README.md#設定ファイル)で `false` にすると、ephemeral の整列による変更・警告を行いません。

```hcl
ephemeral_order = false
```
