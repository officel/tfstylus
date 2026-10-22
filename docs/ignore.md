# コメントによる変換除外

設定で有効にした変換を、HCL のコメントからファイル単位またはトップレベル block 単位で除外できます。Terraform JSON ではルート object の先頭 `"//"` プロパティにファイル単位の指示を記述できます。コメント指示は設定で無効な変換を有効にしません。

## HCL の指示

指示には、行頭（空白の後でも可）から始まる単独行の `#` コメントを使います。

```hcl
# tfstylus-ignore-file: <rule>, <rule>
# tfstylus-ignore: <rule>, <rule>
```

- `# tfstylus-ignore-file:` は、最初の HCL 要素より前のコメント群に置きます。空行や通常コメントを含められます。指定した変換をファイル全体で除外します。
- `# tfstylus-ignore:` は、次のトップレベル block の直前に置きます。指示から block まで空行を入れなければ、間に通常コメントを置けます。対象はその block 1つです。
- block 指示は block 内部に行う該当変換も除外します。兄弟 block の並べ替えでは対象 block を元の位置に固定し、前後を別々に整列します。
- 指示名・ルール名は小文字の設定キーを完全一致で記述します。ルールを複数指定するときはカンマで区切り、カンマの前後に空白を置けます。

例: ファイル全体で複数の変換を除外する場合

```hcl
# このファイルは既存の配置を保ちます。
# tfstylus-ignore-file: required_version_order, resource_order

terraform {
  required_version = ">= 1.7.0"
}
```

例: 通常コメントを理由として添え、1つの block だけ除外する場合

```hcl
# tfstylus-ignore: resource_order
# 手動で管理しているため、この block の順序を保ちます。
resource "example" "manual" {
  depends_on = [local.first]
  count      = 1
}
```

例: block 内の変換を複数除外する場合

```hcl
# tfstylus-ignore: required_version_order, backend_order
terraform {
  backend "local" {}
  required_version = ">= 1.0"
}
```

## ルール名と適用範囲

ルール名は設定キーと同じです。設定で該当ルールを有効にした場合に限り、既存の変換対象に作用します。

- **並び替え**（ファイル指示・block 指示）: `required_version_order`、`backend_order`、`cloud_order`、`cloud_workspaces_order`、`required_providers_order`、`provider_order`、`variable_order`、`output_order`、`data_order`、`action_order`、`check_order`、`ephemeral_order`、`resource_order`、`module_order`、`list_order`、`import_order`、`moved_order`、`removed_order`
- **置換**（ファイル指示・block 指示）: `replace_required_version`
- **ファイル名変更**（ファイル指示のみ）: `locals_file_rename`、`outputs_file_rename`、`variables_file_rename`、`terraform_file_rename`

ファイル名変更ルールは block 指示に書いても適用されません。ファイル指示と block 指示など、同じ対象に対する複数の指示はルールの集合として結合され、重複は1つとして扱われます。

HCL 指示は `.tf`（override 版を含む）と `.tfquery.hcl` で認識されます。ファイル形式やルールの対象に該当しない変換には効果がありません。

## Terraform JSON

`.tf.json` では、ルート object の最初の `"//"` プロパティにファイル単位指示を記述できます。値は `tfstylus-ignore-file: <rule>, <rule>` の文字列です。HCL と同じルール名を使います。

```json
{
  "//": "tfstylus-ignore-file: resource_order",
  "resource": {
    "example": {
      "manual": {
        "count": 1
      }
    }
  }
}
```

JSON の block 単位指示はありません。現在 `.tf.json` に対する変換は実行されないため、この指示によって抑止される変換はありません。

## 認識されない指示

`//` や `/* ... */` の HCL コメント、行末コメント、属性やネストした block の直前に置いた指示は認識されません。指示行に理由などの追加文を付けず、理由は別の通常コメントに記載してください。文字列や heredoc の中にある指示に見える文字列も対象外です。

`all` の省略形はありません。未知のルール名は無視されますが、同じ指示に有効な既知のルール名があれば、そのルールは適用されます。文法や配置が不正な指示、対象に適用できないルールは診断されず、除外もされません。誤記により変換が実行されることがあるため、ルール名と配置を確認してください。
