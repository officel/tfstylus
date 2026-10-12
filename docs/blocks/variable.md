# variable ブロック

tfstylus は `variable` ブロック内の属性と `validation` ブロックを並べ替えます。
重要度と情報量を重視した順序で、型や値の内部には手を加えません。

- [Variable block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/variable)
- [tfstylus の共通方針](../../README.md#共通方針)

## 整列順

1. `description`
2. `deprecated`
3. `sensitive`
4. `nullable`
5. `ephemeral`
6. `const`
7. `type`
8. `default`
9. `validation` ブロック

この順序は tfstylus のスタイル規則であり、Terraform の必須順序ではありません。
存在する要素だけを整列し、複数の `validation` は相対順を保って最後に置きます。

## 保持する内容

- 要素と所属コメントを元ソースのまとまりとして移動し、式、文字列、heredoc、コメントを再生成しません。
- 型式、`default` のオブジェクト・リスト・タプル、`validation` 内部の順序と内容を保持し、評価・内部整列しません。
- 属性の欠落、型と default の整合性、validation の条件・必須属性、予約名、module をまたぐ重複や override の意味は検証しません。
- 変数宣言同士やトップレベルブロックの順序は変更しません。

## 変更せず警告する条件

次の場合は、variable ブロック全体を保持して警告します。

- 未知の属性・ブロックがある場合。
- ラベル数や識別子が不正、または `validation` にラベルがある場合。
- 本体が一行の場合。
- 並べ替えが必要で、コメントの所属や要素の境界を確定できない場合。

未知要素・不正構造・一行の本体がなく、整列済みで移動が不要なら、境界コメントがあっても変更・警告はありません。

## 設定

`variable_order` は省略時に `true` です。
[設定ファイル](../../README.md#設定ファイル)で `false` にすると、variable の整列による変更・警告を行いません。

```hcl
variable_order = false
```
