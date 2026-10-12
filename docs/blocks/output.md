# output ブロック

tfstylus は `output` ブロック内の属性と `precondition` ブロックを並べ替えます。
値や条件式は評価せず、元ソースを保持します。

- [Output block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/output)
- [tfstylus の共通方針](../../README.md#共通方針)

## 整列順

1. `description`
2. `deprecated`
3. `sensitive`
4. `ephemeral`
5. `type`
6. `value`
7. `precondition` ブロック
8. `depends_on`

この順序は tfstylus のスタイル規則であり、Terraform の必須順序ではありません。
存在する要素だけを整列し、複数の `precondition` は相対順を保持します。
Terraform では `value` が必須ですが、tfstylus は属性の欠落を検証しません。

## 保持する内容

- 要素と所属コメントを元ソースのまとまりとして移動し、式、文字列、heredoc、コメントを再生成しません。
- 値と `precondition` 内部の順序・内容を保持し、内部整列しません。
- root / child module の文脈は推測せず、その文脈に応じた属性の制約も検証しません。
- output ブロック同士やトップレベルブロックの順序は変更しません。

## 変更せず警告する条件

次の場合は、output ブロック全体を保持して警告します。

- `postcondition` を含む未知・非対応の属性・ブロックがある場合。
- output のラベル数や識別子が不正、または `precondition` にラベルがある場合。
- 本体が一行の場合。
- 並べ替えが必要で、コメントの所属や要素の境界を確定できない場合。

整列済みで移動が不要なら、曖昧な境界コメントだけでは警告しません。

## 設定

`output_order` は省略時に `true` です。
[設定ファイル](../../README.md#設定ファイル)で `false` にすると、output の整列による変更・警告を行いません。

```hcl
output_order = false
```
