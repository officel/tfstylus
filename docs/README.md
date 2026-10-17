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

## 確認（`check`）と実行（`run`）

- `check` サブコマンドで、処理後の変更を確認します
- `run` サブコマンドで、変更を適用します
- 共通のオプションで表示を変更できます
- `-c/--config PATH` は [設定ファイル](./config.md) を指定します
- `-r/--recursive` はディレクトリを再帰的に処理します
- `-d/--diff` は差分の内容を git に似た形式で表示します

```sh
# このような alias にしておくと楽です
alias tfsc='tfstylus check --recursive --diff'
alias tfsr='tfstylus run --recursive --diff'
```

## スタイルの統一について

- 既存のコードの意図を壊さないように注意しています（処理後でも plan 差分が発生しないように注意しています）
- 基本的には所定のルールで並んでいて欲しいものをそのとおりにするだけです。リソース自体の並べ替えや、リソース中の要素を意味なくアルファベット順に並べ替えたりはしません
- `terraform fmt` を自動で実行しないので、処理後には `fmt / validate` などを **ご自身で** 実行してください
- 各設定の内容は [整列順について](./blocks/README.md) から各ブロックのドキュメントを参照してください
