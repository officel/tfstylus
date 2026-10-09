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
go run ./cmd/tfstylus run --chdir environments --recursive
```

指定なしではカレントディレクトリ直下を対象にします。`--chdir` で起点を指定し、`--recursive` で配下も探索できます。再帰では隠しディレクトリとディレクトリリンクを辿りません。

対象は `.tf` と `.tf.json` です。隠しファイル・エディター一時ファイルは除外し、ディレクトリごとに安定した順序で列挙します。

設定の自動探索と相対 `--config` は各対象ディレクトリを基準にします。共通設定を指定する場合は絶対パスを使用してください。

現在は対象探索と設定検査まで実装済みです。`check` / `run` の整形・差分表示・書き込みは未実装であり、検査後も未実装エラーで終了します。

## テスト

```sh
go test ./...
```
