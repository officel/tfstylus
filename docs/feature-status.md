# 機能の実装状況

この一覧はソース、利用者向け文書、設計、およびローカル `issues/` の状態を照合したものです。2026-10-22 時点の実装と合意状態を示し、将来機能の要望や着手承認を意味しません。

## 実装済み

| 分野 | 現在の範囲 | 参照 |
| --- | --- | --- |
| CLI | `init`、非破壊の `plan`、変更を適用する `apply`。設定指定・自動探索、`--chdir`、再帰探索、diff、終了コードを含む | [README](../README.md)、[設定](config.md) |
| 変換の安全性 | HCL 解析、元ソース断片の保持、コメント所属を考慮した移動、候補の再解析・保持検証、安全に扱えない場合の警告、冪等性のテスト | [設計](../design.md#安全なソース変換)、[開発者向け](dev.md) |
| Terraform 順序規則 | `required_version` と backend/cloud の配置、cloud / workspaces 内の対象属性、`required_providers` 各 provider オブジェクト内のキー、および各ブロックの明示された属性順 | [terraform](blocks/terraform.md)、[ブロック共通方針](blocks/README.md) |
| ブロック内整列 | action、check、data、ephemeral、import、list、module、moved、output、provider、removed、resource、variable の文書化された順位・例外 | [ブロック一覧](blocks/README.md) |
| ファイル名変更 | `local.tf` → `locals.tf`、`output.tf` → `outputs.tf`、`variable.tf` → `variables.tf`、限定条件下で terraform block を含む候補 → `terraform.tf` | [設計](../design.md) |
| 明示的な版制約置換 | opt-in 設定 `replace_required_version` による既存の静的 `required_version` 値の置換 | [terraform 設定](blocks/terraform.md) |

ブロック固有の完全な順位、設定キー、警告条件は個別文書を正とします。ここで列挙したブロックでも、任意のネスト内部や provider 固有値まで再帰的に整列するわけではありません。
`resource`、`data`、`ephemeral`、`removed` 内の `lifecycle` 要素は共通順位で整列し、Terraform や provider ごとの対応可否は検証しません。

## 意図的に行わないこと

| 対象 | 方針 |
| --- | --- |
| トップレベル Terraform block / resource block 同士の並べ替え | 配置規則を適用せず、元の並びを保ちます。 |
| map / object のキー、list / tuple の要素 | 意味・評価順や利用者固有の意図を変え得るため、自動整列しません。 |
| 任意のネスト block、dynamic block、provider 固有 block の内部・相対順 | 文書で個別に対象とした範囲だけを整列し、それ以外は保持します。 |
| Terraform の構成妥当性検査・実行 | 必須性、型、値、排他等の検証や `terraform init` / `validate` / `plan` / `apply` は行いません。Terraform の実行、provider 取得、ネットワークアクセスを暗黙に起こしません。 |

これらは未実装機能として約束されたものではなく、現行の責務・安全境界です。方針を変える場合は個別に仕様を合意します。

## 現行スコープでは実施しないもの

- backend block 内部の引数整列（local、S3、AzureRM、GCS、HTTP、Consul、PostgreSQL、Kubernetes、OSS、COS、OCI、remote と各 nested object/block）は実装していません。ローカル `issues/048` で031〜045を変換合成の対象として採用せず、入力を保持する方針が記録されています。個別 issue の旧「保留」表示はこの判断と同期しました。
- 共通の backend 整列方針を決めるローカル `issues/030` は、個別機能を採用しない現行スコープでは前提となる作業がないため棚上げです。これらは実装待ちではなく、具体的な需要が出てユーザーが再検討を指示するまで着手しません。
- backend/cloud の Terraform block 内配置は上記とは別で、021〜023で実装・検証済みです。既存の `backend_order` は backend block 内部の引数を整列しません。
- 021〜029では terraform block 外側配置、cloud、required_providers の個別規則、安全性を実装・検証済みです。046〜048では provider_meta / experiments の不透明領域保持と変換合成を検証済みです。したがって、これらを「保留」に戻す必要はありません。
- Terraform block のより広い外側順序では `required_providers`、`provider_meta`、`experiments` すべての整列規則を設けたわけではありません。required_providers オブジェクト内部のキー順は025で実装済みです。provider_meta は046で不透明な byte と相対順の保持、experiments は047で外側移動時の式 byte 保持を検証済みで、いずれも独自の配置順は追加しない判断です。
- トップレベル block 同士の順序、map/object のキー、list/tuple の要素、任意のネストの整列、Terraform 自体の妥当性検査・実行は、現行の責務と安全境界として対象外です。

## 未決定・候補

- コメントによる `ignore` / `sort: off` 等の除外 directive は設計上の候補です。構文・対象範囲・診断は未決定で、現行の機能として提供していません。

## 状態の維持

新しい機能を追加した際は、該当する状態をこの一覧と関連設計・利用者向け文書でそろえて更新します。実装済み・現行スコープで不採用・未決定の提案を区別し、意図的に採用しなかったものを「保留中の実装」と誤表示しないようにします。
