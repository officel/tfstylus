# tfstylus 概要

- tfstylus は terraform のスタイルを統一することを目的としたツールです
- 鉄筆やけがき針のような硬質な素材で線を引くように、スタイルを揃えたいという願いが込められています
- 主に terraform のスタイルガイドを参考にスタイルをチェックして整えます
- [Style Guide - Configuration Language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/style)
- [Standard Module Structure | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/modules/develop/structure)
- ユーザープラクティスに基づいた恣意的なスタイル設定を取り込むことで、prettier 同様にスタイルに対する論争を終わらせます

# 動作に関する期待

## デフォルトの挙動

- 引数なしで実行された場合、ヘルプおよびバージョン情報を出力して終了します

## -c/--config 引数（全サブコマンドで有効）

- `-c/--config string` 引数を渡した場合、`string` を設定ファイルとして読み込みます
- デフォルトの設定ファイル名は `.tfstylus` とし、以下の順序で検索して最初に見つかったものを使用します

  - `./.tfstylus` カレントディレクトリ
  - `./.config/.tfstylus` .config ディレクトリ
  - 見つかるまでこの順序で上位のディレクトリを検索します

## check サブコマンド

- ファイルは変更しない
- 変更されるファイルと変更箇所をリストします

## run サブコマンド

- ファイルをチェックして変更します

# 処理に関する期待

## 設定ファイル

- すべての変換処理は設定によりオンオフを可能にします

## ファイルの命名規則

- 公式のスタイルガイドは、通常用とモジュール用の2つのドキュメントがあります
- 通常用は若干初心者向け、モジュール用は高度な使い方として terraform module のリポジトリ用になっています
- tfstylus ではこれらを組み合わせた命名規則およびファイル名を採用します

  - `locals.tf`
  - `main.tf`
  - `outputs.tf`
  - `terraform.tf`
  - `variables.tf`
  - また `backend.tf` および `providers.tf` を使用しないように求めます（`terraform.tf` に統一する）
  - `override.tf` は設計思想によるものなので命名規則としては無視します（あってもなくてもよい）

## リソースの並べ替え

- リソースの並べ替えは実施しません
- 特定の順番に並んでいるべきかもしれませんが、すべてのケースを洗うことはできないからです
- リソースの並び順による可読性はコードを書く人の領分だということです

## プロパティの並べ替え

- `terraform fmt` よりも少しだけ強い強制力で書式を統一します
- ブロック毎に期待値が異なります

### terraform ブロック

- [Terraform block reference for the Terraform configuration language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/terraform)
- `terraform` ブロックは `terraform.tf` ファイルに統一して配置します
- [Terraform block reference for the Terraform configuration language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/terraform#complete-configuration)
- 基本的にはこの方針に従いますがいくつか異なる点があります

  - 先頭は `required_version` 次は `backend` block です
  - `required_providers.<provider>` は source, version の順です
  - `cloud` は organization, hostname, token, workspace の順です
  - `cloud.workspace` は name, project, tags の順です

### provider ブロック

- [Provider block reference for the Terraform configuration language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/provider)

### リソースブロック

- [resource block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/resource)
- リソースブロック内のプロパティをプラクティスに従いつつ次のように並び替えます

  - `count` および `for_each` のメタ引数
  - `id`、`name`、およびこれらの prefix
  - リソース毎の必須パラメーター
  - 非ブロックパラメーター
  - ブロックパラメーター
  - タグ引数（`tag`,`tags`,`tag_name`等）
  - `lifecycle` ブロック
  - `depends_on` メタ引数

### variable ブロック

- [Output block reference for the Terraform configuration language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/output)

### output ブロック

- [Output block reference for the Terraform configuration language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/output)

### data ブロック

- [data block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/data)
