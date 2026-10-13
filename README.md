# tfstylus

Terraform コードのスタイルを整える Go 製 CLI です。

## 共通方針

- `terraform fmt` と `terraform validate` が済んだユーザーコードを入力として期待します。tfstylus は実行済みかを確認せず、Terraform コマンドも実行しません。
- 属性やブロックなどの要素は並べ替えるだけで、追加・削除しません。
- Terraform の実行版や Terraform Enterprise の版、タグ形式の互換性は判定しません。版を推測した補完・削除も行いません。
- 公式ドキュメントの表示バージョンや照合に使ったタグは仕様確認用であり、tfstylus の最低対応版や他バージョンとの互換性を保証するものではありません。
- Terraform の型・値の意味検証や自動修復は行いません。

入力の妥当性は Terraform、配置規則と変換時の元ソース・コメントの保持は tfstylus の責務です。
共通順位とブロック固有の例外、未知要素・安全なソース移動の扱いは、[ブロック共通方針](docs/blocks/README.md#共通方針)を参照してください。

## 実行

```sh
go run ./cmd/tfstylus
```

引数なしで実行すると、バージョンと使い方を表示します。

## 対象ディレクトリ

```sh
go run ./cmd/tfstylus check
go run ./cmd/tfstylus check --chdir=environments/production
go run ./cmd/tfstylus check --recursive
go run ./cmd/tfstylus check --diff
go run ./cmd/tfstylus check -r -d
go run ./cmd/tfstylus run --chdir environments --recursive
```

指定なしではカレントディレクトリ直下を対象にします。`--chdir` で起点を指定し、`--recursive`（短縮形 `-r`）で配下も探索できます。`--diff` の短縮形は `-d` です。再帰では隠しディレクトリとディレクトリリンクを辿りません。

対象は `.tf`、`.tf.json`、`.tfquery.hcl` です。隠しファイル・エディター一時ファイルは除外し、ディレクトリごとに安定した順序で列挙します。`.tfquery.hcl` には [list block](docs/blocks/list.md) の整列だけを適用します。

設定の自動探索と相対 `--config` は各対象ディレクトリを基準にします。共通設定を指定する場合は絶対パスを使用してください。

## 設定ファイル

`.tfstylus.hcl` または `.config/.tfstylus.hcl` にルール設定を記述できます。
整列順、保持条件、警告条件は各ブロックの文書を参照してください。

| ブロック | 設定 |
| --- | --- |
| [terraform](docs/blocks/terraform.md) | `required_version_order`、`backend_order`、`required_providers_order`、`cloud_order`、`cloud_workspaces_order` |
| [provider](docs/blocks/provider.md) | 個別設定なし（常に有効） |
| [resource](docs/blocks/resource.md) | `resource_order` |
| [module](docs/blocks/module.md) | `module_order` |
| [variable](docs/blocks/variable.md) | `variable_order` |
| [output](docs/blocks/output.md) | `output_order` |
| [data](docs/blocks/data.md) | `data_order` |
| [action](docs/blocks/action.md) | `action_order` |
| [check](docs/blocks/check.md) | `check_order` |
| [ephemeral](docs/blocks/ephemeral.md) | `ephemeral_order` |
| [import](docs/blocks/import.md) | `import_order` |
| [list](docs/blocks/list.md) | `list_order` |
| [moved](docs/blocks/moved.md) | `moved_order` |
| [removed](docs/blocks/removed.md) | `removed_order` |
| ファイル名 | `locals_file_rename`、`outputs_file_rename`、`variables_file_rename` |

各個別設定は省略時に有効です。`false` にすると、そのルールによる変更・警告を行いません。

```hcl
required_version_order = false
backend_order = false
cloud_order = false
cloud_workspaces_order = false
required_providers_order = false
variable_order = false
output_order = false
data_order = false
action_order = false
check_order = false
ephemeral_order = false
resource_order = false
module_order = false
import_order = false
list_order = false
moved_order = false
removed_order = false
```

`locals_file_rename = false` で、次の `local.tf` の rename を無効にできます。

### locals.tf への rename

同じディレクトリで、トップレベルの `locals` ブロックを含む `local.tf` があり、`locals.tf` がない場合だけ、`local.tf` を `locals.tf` へ rename します。`locals.tf` のみ、`locals` ブロックなし、両方存在の場合は何もせず、警告もしません。`.tf` のみが対象で、`local.tf` がシンボリックリンクの場合はエラーです。

`check` は `path: rename to path [file-name/locals] reason` を表示し、`--diff` では git 形式の `rename from` / `rename to` と `similarity index` を表示します。`run` は内容の置換をすべて終えてから、既存の宛先を上書きしない `os.Link` と `os.Remove` で rename します。ハードリンクを使えない環境は対象外で、エラーとなり `local.tf` が残ります。

### outputs.tf への rename

同じディレクトリで、トップレベルの `output` ブロックを含む `output.tf` があり、`outputs.tf` がない場合だけ rename します。混在ファイルも対象ですが、ブロックの移動・統合は行いません。`outputs_file_rename = false` で無効にでき、`output_order` とは独立です。

ルール名は `file-name/outputs` です。非破壊の `check`、git 形式の diff、`run`、衝突・リンク・失敗時の内容保持は locals と同じです。壊れた宛先リンクも既存とみなしますが、CLI のファイル探索で先にエラーになる場合があります。

### variables.tf への rename

同じディレクトリで、トップレベルの `variable` ブロックを含む `variable.tf` があり、`variables.tf` がない場合だけ rename します。混在ファイルも対象ですが、ブロックの移動・統合は行いません。`variables_file_rename = false` で無効にでき、`variable_order` や他の rename 設定とは独立です。

ルール名は `file-name/variables` です。非対象、非破壊の `check`、diff、`run`、衝突・リンク・失敗時の内容保持は locals・outputs と同じです。

`check` は有効なルールを検査し、変更予定のファイルと元入力上の行・列を表示します。`--diff` を指定すると unified diff も表示します。いずれも入力ファイルは変更しません。整列済みでスキップ条件がなければ出力なしで終了します。

`check` の終了コードは、変更なしが `0`、変更予定ありが `1`、解析・設定エラーまたは安全上の理由でルールを適用できない場合が `2` です。曖昧なコメントなどで変換をスキップした場合は警告を表示します。

`run` は `check` と同じ変換候補をファイルへ適用します。変更のあるファイルは同じディレクトリの一時ファイル経由で置換し、通常ファイルのpermission modeを維持します。変更対象がシンボリックリンクの場合は書き込みを拒否します。読み込み後の外部変更も置換直前に検出した場合は中止します。

複数ファイルを処理する場合、すべての入力と変換候補を先に検証してから、変更対象を順次置換します。置換はファイル単位でatomicですが、複数ファイル全体のtransactionではありません。置換中に失敗すると、先行ファイルだけ更新済みになることがあります。成功時は出力なしで終了し、実行エラーは終了コード `2` です。

`.tf` ファイルには Terraform の整列ルールを適用します。`.tf.json` ファイルは構文検証のみ行い、`.tfquery.hcl` ファイルには `list_order` のみを適用します。

## テスト

```sh
go test ./...
```
