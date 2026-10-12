# terraform ブロック

- [Terraform block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/terraform)
- [共通方針](README.md)

## 外側の配置

```hcl
terraform {
  required_version = ">= 1.0"
  backend "local" {
    path = "terraform.tfstate"
  }
}
```

`backend` の代わりに `cloud` も同じ位置に置きます。`required_version` がなければ先頭です。
その他の要素を含む全体の順位は定めず、移動対象以外の相対順を保持します。
`backend` 内部は整列しません。

## required_providers

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

provider 要求の宣言順は変更しません。
対象は、複数行でキーが静的かつ一意であり、項目の境界とコメントの所属を確定できるオブジェクトです。
引用符の有無を問わず静的キーを扱い、HCL で有効な `=` / `:` 区切りを受け入れます。
キー表記、区切り、カンマ、値の元ソースは保持します。

`configuration_aliases` は provider 設定参照のリストとして扱い、定数として評価しません。
従来の文字列形式（例: `aws = "~> 5.0"`）は保持し、オブジェクト形式へ移行しません。
このルールは provider 要求だけが対象であり、任意の map やオブジェクトは整列しません。

## cloud

```hcl
terraform {
  required_version = ">= 1.0"
  cloud {
    organization = "example"
    hostname = "app.terraform.io"
    token = var.token
    workspaces {
      name = "example"
      project = "example-project"
    }
  }
}
```

`name` の代わりに `tags` を使う場合は、`project` の後に置きます。

```hcl
workspaces {
  project = "example-project"
  tags = ["example"]
}
```

空の `cloud`、属性の省略、`null` や空値は、環境から補完される可能性があるため、それだけではエラーにしません。
tfstylus はプロセス環境を参照せず、`null` や未評価の値から `name` / `tags` の競合を推測しません。

## 保持する内容

- `provider_meta` は解釈・整列せず、配置順位も定めません。個数・ラベルを含めて保持できない候補は適用しません。
- `experiments` の値は評価・整列しません。

## 変更せず警告する条件

- 外側に複数の `backend` / `cloud` がある場合、`backend_order` は配置を変更せず警告します。両方を併用する場合も含みます。
- provider 要求のオブジェクトが一行、未知・重複・動的キーを含む、または項目境界やコメントの所属が曖昧な場合、そのオブジェクトを保持して警告します。
- `cloud` / `workspaces` の整列が必要で、未知要素や独立コメントを安全に保てない場合、該当する本体を保持して警告します。

共通方針の安全に移動できない場合は、外側ではその配置変更、provider 要求では該当オブジェクト、`cloud` / `workspaces` では該当本体を保持します。

`cloud` に複数の `workspaces` がある場合、または `name` と `tags` の両方が静的に `null` でないと確認できる場合は、対象ルールが有効なら整列済みでも警告します。

## 設定

各ルールを独立して制御します。

| 設定 | 対象 |
| --- | --- |
| `required_version_order` | 外側の `required_version` の配置 |
| `backend_order` | 外側の `backend` / `cloud` の配置 |
| `required_providers_order` | provider 要求のオブジェクト内部 |
| `cloud_order` | `cloud` 内部 |
| `cloud_workspaces_order` | `cloud.workspaces` 内部 |

内部整列の設定は、外側の配置ルールを暗黙に有効にしません。
