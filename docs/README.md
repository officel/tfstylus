# tfstylus -- Standardizing terraform code style

tfstylus は terraform のコードスタイルを統一するツールです。

terraform には公式の [スタイルガイド](https://developer.hashicorp.com/terraform/language/style) や
[モジュール向けプラクティス](https://developer.hashicorp.com/terraform/language/modules/develop/structure)
などがありますが、自由度が高く強い強制もしないため、ディレクトリ毎にスタイルが崩れてしまうことが頻繁におこります。

tfstylus は既存コードの意図を尊重しながら、スタイル崩れを修正して統一する試みです。

## 設定（`init`）

- [設定ファイル](./config.md) は基本的に不要です
- どうしても適用したくないスタイルがある場合に、機能をオフにできます
- `required_version` の置換など、一部の機能で値を設定することもできます

## 確認（`plan`）と実行（`apply`）

- `plan` はファイルを変更せず、変更箇所・理由と unified diff を表示します。正常・変更なしなら `No changes.` を1回表示します
- `plan` の終了コードは変更なし `0`、変更予定あり `1`、エラー・安全スキップ `2` です
- `apply` は現在の入力と設定から再計算・検証し、確認待ちなしで変更を適用します。保存済み plan を適用する機能ではありません
- `apply` の終了コードは成功 `0`、エラー `2` です。通常は無出力です
- `-c/--config PATH` は [設定ファイル](./config.md) を指定します
- `-r/--recursive` はディレクトリを再帰的に処理します
- `-d/--diff` は `apply` 専用で、適用成功後に差分を表示します。変更がなければ無出力です。`plan` では拒否します
- 旧 `check` / `run` は受け付けません。`plan` / `apply` に置き換えてください

```sh
# このような alias にしておくと楽です
alias tfsp='tfstylus plan --recursive'
alias tfsa='tfstylus apply --recursive --diff'
```

## スタイルの統一について

- 既存のコードの意図を壊さないように注意しています（処理後でも `terraform plan` の差分が発生しないように注意しています）
- 基本的には所定のルールで並んでいて欲しいものをそのとおりにするだけです。リソース自体の並べ替えや、リソース中の要素を意味なくアルファベット順に並べ替えたりはしません
- `terraform fmt` を自動で実行しないので、処理後には `fmt / validate` などを **ご自身で** 実行してください
- 各設定の内容は [整列順について](./blocks/README.md) から各ブロックのドキュメントを参照してください

## プロジェクトの状況

- [機能の実装状況](./feature-status.md) では、実装済み・意図的な対象外・保留・未決定を区別しています
- [リリース運用](./release.md) では、自動化の範囲とリリース後に確認する項目を説明しています

## use prek(pre-commit)

```yaml:.pre-commit-config.yaml
repos:
  - repo: https://github.com/officel/tfstylus
    rev: "v0.1.10"
    hooks:
      - id: tfstylus
```
