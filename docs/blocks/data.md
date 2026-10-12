# data ブロック

tfstylus はトップレベルの `data "<TYPE>" "<LABEL>"` ブロック内の要素を並べ替えます。
provider 固有の属性・ブロックは内容を解釈せず、グループ内の相対順を保持します。

- [data block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/data)
- [tfstylus の共通方針](../../README.md#共通方針)

## 整列順

1. `count` または `for_each`
2. provider 固有属性
3. `provider`
4. provider 固有のネストブロックと `dynamic`
5. `lifecycle` ブロック
6. `depends_on`

この順序は tfstylus のスタイル規則であり、Terraform の必須順序ではありません。
存在する要素だけを整列し、各グループ内の相対順を保持します。
Terraform では `count` と `for_each` は相互排他ですが、同時指定でも tfstylus は検証・修復せず、相対順を保持します。

provider 固有属性を名前・必須性・tag などで分類したり、アルファベット順に変更したりしません。
check にネストした data には別の順序を使います。詳細は [check ブロック](check.md)を参照してください。

## lifecycle 内部

1. `precondition` ブロック
2. `postcondition` ブロック

同じ種類の条件ブロックの相対順を保持します。
条件式や各条件ブロック内部の元ソースは変更しません。

## 保持する内容

- 要素と所属コメントを元ソースのまとまりとして移動し、式、文字列、heredoc、コメントを再生成しません。
- provider 固有ブロックと `dynamic` の型・ラベル・生成内容を解釈せず、内部の順序と内容を保持します。
- `depends_on` の参照リスト内の順序を保持します。
- provider のスキーマ取得、必須属性の検証、provider の取得や Terraform の実行は行いません。
- data ブロック同士やトップレベルブロックの順序は変更しません。

## 変更せず警告する条件

- 並べ替えが必要な本体が一行の場合は、data ブロック全体を保持して警告します。
- 並べ替えが必要で、コメントの所属や要素の境界を確定できない場合は、data ブロック全体を保持して警告します。
- 並べ替えが必要で、未知要素、`dynamic`、順序依存の可能性がある要素を安全に保てない場合も、data ブロック全体を保持して警告します。

provider 固有要素は、境界とコメントの所属を確定できれば、グループ内の相対順を保って移動できます。
移動が不要なら変更・警告はありません。

## 設定

`data_order` は省略時に `true` です。
[設定ファイル](../../README.md#設定ファイル)で `false` にすると、トップレベル data の整列による変更・警告を行いません。
check 内の data は `check_order` の対象です。

```hcl
data_order = false
```
