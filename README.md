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

`.tfstylus.hcl` または `.config/.tfstylus.hcl` にルール設定を記述できます。`required_version_order` は `terraform` ブロック内の `required_version` を先頭へ移すルールです。省略時は有効です。

```hcl
required_version_order = false
```

`check` は `required_version_order` ルールを検査し、変更予定のファイルと元入力上の行・列を表示します。`--diff` を指定すると unified diff も表示します。いずれも入力ファイルは変更しません。整列済みなら出力なしで終了します。

`check` の終了コードは、変更なしが `0`、変更予定ありが `1`、解析・設定エラーまたは安全上の理由でルールを適用できない場合が `2` です。曖昧なコメントなどで変換をスキップした場合は警告を表示します。

`run` は `check` と同じ変換候補をファイルへ適用します。変更のあるファイルは同じディレクトリの一時ファイル経由で置換し、通常ファイルのpermission modeを維持します。変更対象がシンボリックリンクの場合は書き込みを拒否します。読み込み後の外部変更も置換直前に検出した場合は中止します。

複数ファイルを処理する場合、すべての入力と変換候補を先に検証してから、変更対象を順次置換します。置換はファイル単位でatomicですが、複数ファイル全体のtransactionではありません。置換中に失敗すると、先行ファイルだけ更新済みになることがあります。成功時は出力なしで終了し、実行エラーは終了コード `2` です。

現在の変換ルールは `.tf` ファイルが対象です。`.tf.json` ファイルは構文検証のみ行います。

## テスト

```sh
go test ./...
```
