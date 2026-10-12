# terraform ブロック

tfstylus は `terraform` ブロック内の要素の配置と、`required_providers`、`cloud`、`workspaces` の内部を整えます。
外側の配置と内部の整列は、それぞれ独立した設定で制御します。

- [Terraform block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/terraform)
- [tfstylus の共通方針](../../README.md#共通方針)

## 外側の配置

1. `required_version` を先頭に置きます。
2. `backend` または `cloud` ブロックを `required_version` の直後に置きます。`required_version` がなければ先頭に置きます。

この順序は tfstylus のスタイル規則であり、Terraform の必須順序ではありません。
その他の要素を含む全体の順位は定めず、移動対象以外の要素の相対順は保持します。
`backend` 内部の引数は整列しません。

```hcl
terraform {
  required_version = ">= 1.0"
  backend "local" {
    path = "terraform.tfstate"
  }
}
```

## required_providers

各 provider 要求のオブジェクト内を次の順序に整列します。
存在しないキーは飛ばし、provider 要求の宣言順は変更しません。

1. `source`
2. `version`
3. `configuration_aliases`

```hcl
terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
      version = "~> 6.0"
      configuration_aliases = [aws.west]
    }
  }
}
```

対象は、複数行でキーが静的かつ一意であり、項目の境界とコメントの所属を確定できるオブジェクトです。
引用符の有無を問わず静的キーを扱い、HCL で有効な `=` / `:` 区切りを受け入れます。
キー表記、区切り、カンマ、値の元ソースは保持します。

`configuration_aliases` は provider 設定参照のリストとして扱い、定数として評価せず、リスト内の順序を保持します。
従来の文字列形式（例: `aws = "~> 5.0"`）は保持し、オブジェクト形式へ移行しません。
このルールは provider 要求だけが対象であり、任意の map やオブジェクトは整列しません。

## cloud

`cloud` 内は次の順序に整列します。

1. `organization`
2. `hostname`
3. `token`
4. `workspaces` ブロック

`workspaces` 内は次の順序に整列します。

1. `name`
2. `project`
3. `tags`

いずれも存在する要素だけを整列します。
`cloud_order` は `cloud` 内の配置だけを扱い、`cloud_workspaces_order` は `workspaces` 内部だけを扱います。
片方を無効にしても、もう片方は独立して適用されます。

空の `cloud`、属性の省略、`null` や空値は、環境から補完される可能性があるため、それだけではエラーにしません。
tfstylus はプロセス環境を参照せず、`null` や未評価の値から `name` / `tags` の競合を推測しません。

## 保持する内容

- 要素と所属コメントを元ソースのまとまりとして移動し、式、文字列、heredoc、コメントを再生成しません。
- `tags` の値や `configuration_aliases` のリスト内部の順序は変更しません。
- `provider_meta` の内容は解釈・整列しません。個数、ラベル、同型ブロック間の相対順と元ソースを保持し、専用の配置順位も定めません。保持条件を満たさない候補は適用しません。スキーマの検証や provider の取得も行いません。
- `experiments` の値は評価・整列せず、元ソースを保持します。
- terraform ブロック同士やトップレベルブロックの順序、ファイル間の配置は変更しません。

## 変更せず警告する条件

- 外側に複数の `backend` / `cloud` がある場合、`backend_order` は配置を変更せず警告します。両方を併用する場合も含みます。
- 外側の移動でコメントの所属を確定できない場合、その配置変更を行わず警告します。
- provider 要求のオブジェクトが一行、未知・重複・動的キーを含む、または項目境界やコメントの所属が曖昧な場合、そのオブジェクトを保持して警告します。
- `cloud` / `workspaces` の整列が必要で、未知要素、独立コメント、曖昧な境界により安全に移動できない場合、該当する本体を保持して警告します。

`cloud` に複数の `workspaces` がある場合、または `name` と `tags` の両方が静的に `null` でないと確認できる場合は、対象ルールが有効なら整列済みでも警告します。
それ以外の通常の整列済みの `cloud` / `workspaces` では、移動不要なら警告しません。

## 設定

[設定ファイル](../../README.md#設定ファイル)で、各ルールを独立して無効にできます。
いずれも省略時は `true` で、`false` にしたルールによる変更・警告は行いません。

| 設定 | 対象 |
| --- | --- |
| `required_version_order` | 外側の `required_version` の配置 |
| `backend_order` | 外側の `backend` / `cloud` の配置 |
| `required_providers_order` | provider 要求のオブジェクト内部 |
| `cloud_order` | `cloud` 内部 |
| `cloud_workspaces_order` | `cloud.workspaces` 内部 |

内部整列の設定は、外側の配置ルールを暗黙に有効にしません。

```hcl
required_version_order = false
backend_order = false
required_providers_order = false
cloud_order = false
cloud_workspaces_order = false
```
