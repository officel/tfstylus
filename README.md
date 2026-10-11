# tfstylus

Terraform コードのスタイルを整える Go 製 CLI です。

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
go run ./cmd/tfstylus run --chdir environments --recursive
```

指定なしではカレントディレクトリ直下を対象にします。`--chdir` で起点を指定し、`--recursive` で配下も探索できます。再帰では隠しディレクトリとディレクトリリンクを辿りません。

対象は `.tf` と `.tf.json` です。隠しファイル・エディター一時ファイルは除外し、ディレクトリごとに安定した順序で列挙します。

設定の自動探索と相対 `--config` は各対象ディレクトリを基準にします。共通設定を指定する場合は絶対パスを使用してください。

## 設定ファイル

`.tfstylus.hcl` または `.config/.tfstylus.hcl` にルール設定を記述できます。
`required_version_order` は `terraform` ブロック内の `required_version` を先頭へ、
`backend_order` は `backend` ブロックを `required_version` の次へ移動します。
`cloud_order` は `cloud` body を `organization`、`hostname`、`token`、`workspaces` の順に整列します。
`cloud_workspaces_order` は cloud の `workspaces` body を `name`、`project`、`tags` の順に整列します。
`required_providers_order` は provider 要求 object 内を `source`、`version`、`configuration_aliases` の順に整列します。
provider block は provider 固有属性の相対順を保ち、`alias`、`region`、nested block の順に整列します。provider 固有 schema は解釈せず、式・コメントを含む元ソースを保持します。
`variable_order` は variable body を `description`、`deprecated`、`sensitive`、`nullable`、`ephemeral`、`const`、`type`、`default`、`validation` の順に整列します。この順序はプラクティスに基づく重要度および情報量の順です。
`output_order` は output body を `description`、`deprecated`、`sensitive`、`ephemeral`、`type`、`value`、`precondition`、`depends_on` の順に整列します。複数 precondition の相対順と式・値の内容は保持します。
`data_order` は data body を `count` / `for_each`、provider 固有属性、`provider`、provider 固有・`dynamic` block、`lifecycle`、`depends_on` の順に整列します。lifecycle 内は `precondition` を `postcondition` より前に置き、provider 固有要素の相対順と式・コメントを含む元ソースを保持します。
`action_order` は action body を `count` / `for_each`、`provider`、`config` の順に整列します。provider 固有要素を含む `config` の内部と各 group 内の相対順は変更しません。
複数 validation の相対順、型式・default・validation 内部と所属コメントは保持します。未知要素・不正なラベル構造・一行 body は変更せず警告します。未整列でコメント所属が曖昧な body も保持して警告します。
Terraform の型・値の意味検証や対応版の判定は行いません。`const` / `deprecated` は現行公式資料に基づく認識であり、比較タグ v1.14.0 には含まれません。
各ルールは省略時に有効です。

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
```

`check` は有効なルールを検査し、変更予定のファイルと元入力上の行・列を表示します。`--diff` を指定すると unified diff も表示します。いずれも入力ファイルは変更しません。整列済みでスキップ条件がなければ出力なしで終了します。

`check` の終了コードは、変更なしが `0`、変更予定ありが `1`、解析・設定エラーまたは安全上の理由でルールを適用できない場合が `2` です。曖昧なコメントなどで変換をスキップした場合は警告を表示します。

`run` は `check` と同じ変換候補をファイルへ適用します。変更のあるファイルは同じディレクトリの一時ファイル経由で置換し、通常ファイルのpermission modeを維持します。変更対象がシンボリックリンクの場合は書き込みを拒否します。読み込み後の外部変更も置換直前に検出した場合は中止します。

複数ファイルを処理する場合、すべての入力と変換候補を先に検証してから、変更対象を順次置換します。置換はファイル単位でatomicですが、複数ファイル全体のtransactionではありません。置換中に失敗すると、先行ファイルだけ更新済みになることがあります。成功時は出力なしで終了し、実行エラーは終了コード `2` です。

現在の変換ルールは `.tf` ファイルが対象です。`.tf.json` ファイルは構文検証のみ行います。

## テスト

```sh
go test ./...
```
