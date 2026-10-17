# tfstylus

Terraform コードのスタイルを整える Go 製 CLI です。

```sh
# homebrew でインストール（現在はリリースから直接DLするかbrewしか用意できていません）
$ brew tap officel/tap
$ brew install tfstylus

# バージョン
$ tfstylus -v

# デフォルトでは何もしません
$ tfstylus
Version: dev
Usage: tfstylus <command> [flags]

Commands:
  init               Create a configuration with all rules disabled
  check              Check files without modifying them
  run                Apply style fixes

Flags:
  -c, --config PATH  Configuration file path (init: output path)
  --chdir DIR        Set the target directory (default: current directory)
  -r, --recursive    Search directories recursively (check/run)
  -d, --diff         Show a unified diff (check/run)
  -h, --help         Show this help
  -v, --version      Show version

# 設定ファイルを生成する
$ tfstylus init
Created configuration: .tfstylus.hcl

# 設定ファイルはデフォルトですべての機能をオフになることを確認します（差分はでません）
$ tfstylus check --diff --recursive

# 設定ファイルを削除するか必要な設定をオンにして調整してください（差分を見ながら調整できます）
$ tfstylus check --diff --recursive

# run で差分を適用します（戻せないので git 等でがんばって）
$ tfstylus run
```

- 今のところインストールは前述のとおり brew を使用するかリリースからダウンロードしてください
- [ドキュメント](./docs/README.md) は順次整理中です（機能ごとにAIにがんばってもらったのでなかなか）
- [開発者向け](./docs/dev.md) も少しずつ書き進めています
- 日本人なので日本語で書いています。海外で活用されるツールになるとも思えませんし、翻訳の予定はありません。
