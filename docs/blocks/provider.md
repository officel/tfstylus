# provider ブロック

tfstylus は `provider` ブロック内の属性とネストブロックを並べ替えます。
provider ごとのスキーマを解釈せず、属性名が `alias`、`region` であるか、ネストブロックであるかで配置を決めます。

- [Provider block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/provider)
- [tfstylus の共通方針](../../README.md#共通方針)

## 整列順

1. `alias` / `region` 以外の provider 固有属性
2. `alias`
3. `region`
4. ネストブロック

この順序は tfstylus のスタイル規則であり、Terraform の必須順序ではありません。
存在する要素だけを整列し、provider 固有属性同士とネストブロック同士の相対順を保持します。
`region` は名前に基づくスタイル上の扱いであり、すべての provider に存在するとは限りません。
`default_tags` がブロックとして存在する場合も、他のネストブロックと同じグループに置きます。

## 保持する内容

- 要素と所属コメントを元ソースのまとまりとして移動し、式、文字列、heredoc、コメントを再生成しません。
- ネストブロックの型・ラベルを解釈せず、内部の属性・ブロックの順序と内容を保持します。
- provider 固有属性をアルファベット順に変更しません。属性やブロックのスキーマ検証、provider の取得は行いません。
- provider ブロック同士やトップレベルブロックの順序は変更しません。

## 変更せず警告する条件

並べ替えが必要で、次のいずれかに該当する場合は provider ブロックを変更せず警告します。

- 本体が一行の場合。
- コメントの所属を確定できない場合。
- 要素が独立した行にないなど、移動する元ソースの境界を確定できない場合。

整列済みで移動が不要なら、曖昧なコメントだけでは警告しません。

## 設定

provider の整列は常に有効です。現時点では、個別に無効にする設定はありません。
