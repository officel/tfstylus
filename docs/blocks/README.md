# トップレベルブロックの整列

このページでは共通ルールと、各整列ルールを有効にした後の並び順を一覧します。各ブロックの固有条件・内部整列・警告はリンク先を参照してください。

入力は `terraform fmt` と `terraform validate` 済みを期待します。tfstylus は Terraform の実行や provider の取得を行いません。トップレベルブロック同士の順序も変更しません。

## 処理後の並び順

以下は順位グループを左から並べたものです。存在する要素だけを対象にし、同じグループ内は入力時の相対順を保ちます。詳細な例外・内部整列・警告条件は各ブロックの文書に記載しています。

| ブロック | 処理後の順序 |
| --- | --- |
| [resource](resource.md) | `count` / `for_each` → `id` → `name` → `display_name` → `provider` → 一般属性 → 一般ブロック → `tag` / `tags` → `lifecycle` → `depends_on` |
| [data](data.md) | `count` / `for_each` → `id` → `name` → `display_name` → `provider` → 一般属性 → 一般ブロック → `tag` / `tags` → `lifecycle` → `depends_on` |
| [ephemeral](ephemeral.md) | `count` / `for_each` → `id` → `name` → `display_name` → `provider` → 一般属性 → 一般ブロック → `tag` / `tags` → `lifecycle` → `depends_on` |
| [check](check.md) | `data` → `assert` → その他の属性 → その他のブロック。内側の `data` は上記共通順、`assert` は `condition` → `error_message` → その他の属性 |
| [list](list.md) (`.tfquery.hcl` のみ) | `count` / `for_each` → `id` → `name` → `display_name` → `provider` → `include_resource` → `limit` → 一般属性 → ブロック |
| [module](module.md) | `source` → `version` → `count` / `for_each` → `id` → `name` → その他の入力 → `providers` → ブロック → `ignore_nested_deprecations` → `depends_on` |
| [action](action.md) | `count` / `for_each` → `provider` → 一般属性 → ブロック |
| [provider](provider.md) | 一般属性 → `alias` → `region` → ブロック |
| [variable](variable.md) | `description` → `deprecated` → `sensitive` → `nullable` → `ephemeral` → `const` → `type` → `default` → 一般属性 → `validation` → その他のブロック |
| [output](output.md) | `description` → `deprecated` → `sensitive` → `ephemeral` → `type` → `value` → 一般属性 → ブロック（`precondition` を含む）→ `depends_on` |
| [import](import.md) | `for_each` → `provider` → `id` / `identity` → `to` → 一般属性 → ブロック |
| [moved](moved.md) | `from` → `to` → 一般属性 → ブロック |
| [removed](removed.md) | `from` → 一般属性 → `lifecycle` → その他のブロック |
| [terraform](terraform.md) | 下記「terraform ブロック」の限定ルール。その他の要素は入力時の相対順を保つ |

一般属性・一般ブロックは、そのブロック文書に記載するグループを指します。`resource` / `data` / `ephemeral` と `check` 内の `data` では、`tag` / `tags` 属性・ブロックを一般ブロックの後にまとめ、相互の入力順を保ちます。`list` の `tag` / `tags` は一般属性・ブロックとして扱います。

## 共通方針

- 共通順位は `count` / `for_each`、アイデンティティ属性、`provider`、一般属性、一般ブロック、`tag` / `tags`、`lifecycle`、`depends_on` です。適用しない属性やブロックは各文書に示します。
- アイデンティティ順位は対象文脈にある完全一致の属性 `id` → `name` → `display_name` だけに適用します。`*_id`、その他の `*_name`、同名ブロックやネスト内のキーは一般要素です。`module` の `id` / `name` は module 固有順序に従います。
- `count` と `for_each` は同順位です。相互排他の検証・修復や、存在しない要素の追加はしません。
- 順位の計算には安定整列を使います。式、文字列、heredoc、コメントは再生成せず、要素と所属コメントを元ソースのまとまりとして移動します。
- provider 固有の要素をスキーマで分類したり、一般属性を名前順に並べたりしません。未知属性・ブロックは文脈ごとの一般グループで保持し、同順位の要素の相対順を変えません。
- 明示された範囲を除き、ネストブロック、式、リスト、map の内部は整列しません。意味、必須性、排他、型や値の検査は Terraform に任せます。
- コメント所属や要素境界を安全に特定できない移動は行わず警告します。移動が不要な場合、曖昧なコメントだけを理由に警告しません。各ブロック固有のスキップ条件は個別文書を参照してください。
- [設定ファイル](../config.md)の各整列設定は省略時 `true` です。`false` のルールは変更も警告も行いません。

### terraform ブロック

`terraform` ブロックは共通順位の対象外です。`required_version`、`backend` / `cloud`、`required_providers`、`cloud` / `workspaces` の限定ルールと各設定の独立性は[terraform ブロックの文書](terraform.md)を参照してください。`replace_required_version` は例外的に既定 `false` で、明示設定した場合に限り既存の静的引用文字列を置換します。

## ブロック別の詳細

### Terraform 構成ブロック

- [terraform](terraform.md)

### リソース・データ・モジュール

- [resource](resource.md)
- [data](data.md)
- [ephemeral](ephemeral.md)
- [module](module.md)
- [provider](provider.md)
- [check](check.md)
- [action](action.md)
- [list](list.md)

### 状態移行・公開値

- [variable](variable.md)
- [output](output.md)
- [import](import.md)
- [moved](moved.md)
- [removed](removed.md)
