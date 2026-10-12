# resource ブロック

tfstylus は `resource "<TYPE>" "<LABEL>"` ブロック内の要素を並べ替えます。
provider 固有の属性・ブロックは、名前やスキーマを推測せず、グループ内の相対順を保ちます。

- [resource block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/resource)
- [tfstylus の共通方針](../../README.md#共通方針)

## 整列順

1. `count` または `for_each`
2. `provider`
3. provider 固有属性
4. provider 固有のネストブロック、`dynamic`、`connection`、`provisioner`
5. `lifecycle` ブロック
6. `depends_on`

この順序は tfstylus のスタイル規則であり、Terraform の必須順序ではありません。
存在する要素だけを整列し、同じグループ内の相対順は保持します。
Terraform では `count` と `for_each` は相互排他ですが、tfstylus は同時指定を検証・修復せず、相対順を保持します。

provider 固有属性は1つのグループとして扱います。
`id`、`name`、prefix、必須属性、tag などの名前や意味で優先順位を付けず、アルファベット順にも変更しません。

## 保持する内容

- 要素と所属コメントを元ソースのまとまりとして移動し、式、文字列、heredoc、コメントを再生成しません。
- ネストブロック同士と反復する同型ブロックの相対順、各ブロック内部を保持します。`lifecycle` 内部も整列しません。
- `depends_on` の式と参照リスト内の順序を保持します。
- provider のスキーマ取得、必須属性・重複・排他制約の検証、provider の取得や Terraform の実行は行いません。
- resource ブロック同士やトップレベルブロックの順序は変更しません。

## 変更せず警告する条件

- ラベル数が2でない場合は、resource ブロック全体を保持して警告します。
- 並べ替えが必要で、コメントの所属や要素の境界を確定できず、安全性を確認できない場合は、resource ブロック全体を保持して警告します。

provider 固有の未知の属性・ブロックは、内容を解釈せずグループ内に保持します。
整列済みで移動が不要なら、曖昧な境界コメントだけでは警告しません。

## 設定

`resource_order` は省略時に `true` です。
[設定ファイル](../../README.md#設定ファイル)で `false` にすると、resource の整列による変更・警告を行いません。

```hcl
resource_order = false
```
