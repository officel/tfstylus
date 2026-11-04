# 設定ファイル

- `tfstylus init` で生成できます
- デフォルトのファイル名は `.tfstylus.hcl` です
- `-c/--config` でファイル指定できます
- 各設定の内容は [整列順について](./blocks/README.md) を参照してください
- 個別のファイル単位での [コメントによる変更除外](./ignore.md) を参照してください

## BREAKING CHANGE: 設定名の prefix 統一

v0.1.17 リリースでは設定名を、処理の種類が先頭に来る `replace_*`（文字列置換）、`rename_*`（ファイル名変更）、`order_*`（並べ替え）に統一しました。
従来は `replace_*` と `*_file_rename` / `*_order` が混在し、設定名から処理の種類を見つけにくく、設定・ignore 指定・内部実装の命名も不統一でした。
この混在を解消し、設定の検索・分類と今後の命名を一貫させるため、旧名を廃止しました。互換エイリアスは残していません。

### 移行表

新名のアルファベット順に記載しています。

| 旧名（廃止） | 新名 |
| --- | --- |
| `action_order` | `order_action` |
| `backend_order` | `order_backend` |
| `check_order` | `order_check` |
| `cloud_order` | `order_cloud` |
| `cloud_workspaces_order` | `order_cloud_workspaces` |
| `data_order` | `order_data` |
| `ephemeral_order` | `order_ephemeral` |
| `import_order` | `order_import` |
| `list_order` | `order_list` |
| `module_order` | `order_module` |
| `moved_order` | `order_moved` |
| `output_order` | `order_output` |
| `provider_order` | `order_provider` |
| `removed_order` | `order_removed` |
| `required_providers_order` | `order_required_providers` |
| `required_version_order` | `order_required_version` |
| `resource_order` | `order_resource` |
| `variable_order` | `order_variable` |
| `locals_file_rename` | `rename_locals_file` |
| `outputs_file_rename` | `rename_outputs_file` |
| `terraform_file_rename` | `rename_terraform_file` |
| `variables_file_rename` | `rename_variables_file` |

`replace_required_version` はすでに prefix 形式のため変更しません。設定値・既定値・変換内容・適用順序は変更しません。
設定省略時は並べ替え・ファイル名変更が有効、置換は無効です。`init` は引き続き全設定を無効にしたファイルを生成します。

### 更新が必要な箇所

- 明示指定・自動探索される設定ファイルのキーを、値を変えずに新名へ置換してください。旧名は未知の属性として設定エラー（終了コード `2`）になり、`plan` / `apply` はファイルを変更しません。新旧両方を書いてもエラーです。
- HCL の `tfstylus-ignore` / `tfstylus-ignore-file` と、Terraform JSON の先頭 `"//"` プロパティ内のルール名も移行表にしたがって置換してください。未知のルール名を無視する既存仕様により、**旧名の ignore 指定は診断されず、変換を抑止しません**。設定ファイルだけでなくコメントも更新してから実行してください。
- `plan` / `apply --diff` の出力を参照するスクリプトでは、並べ替えのルール ID も新しい `order_*` に更新してください。`file-name/*` と `terraform/replace-required-version` の表示 ID は変更しません。
- `tfstylus init` は既存ファイルを上書き・自動移行しません。新しい雛形が必要なら、既存ファイルとは別のパスを `-c` で指定してください。
- 更新後は `tfstylus plan --recursive`（必要なら `--chdir` / `-c` を併用）で意図しない変換がないことを確認してから `apply` してください。

## 基本的には使用しなくて構いません

- `tfstylus` は既存コードをできるだけ破壊しないよう動作します
- 並び替えは一定の規則にしたがっているため、できるだけそのまま適用するのがいいでしょう
- どうしても適用したくないルールがある場合に、機能をオフにする目的で設定ファイルを用意しています
- 実験的機能で `replace_required_version` を指定したい場合にのみ使用するといいでしょう

## 設定項目の並び順

設定の一覧と `init` の出力は、設定名のアルファベット順（`order_*` → `rename_*` → `replace_*`）に揃えています。
設定ファイルには任意の順序で記述できます。読み込み後の値の検証は設定名順に行い、複数の設定値が不正な場合は最初の項目を報告します。
この並び順は変換の実行順序を指定するものではありません。設定の探索優先順位、依存関係のある並べ替え、並べ替え後の置換、内容変更後のファイル名変更は従来の順序を保ちます。

## 設定ファイルの自動検索

`-c/--config` で設定ファイルを指定しない場合は、次の順序で検索します

1. カレントディレクトリ（`./`）で、次のファイルを順に検索します

   - `.tfstylus.hcl`
   - `.config/.tfstylus.hcl`

2. 親ディレクトリへ順にさかのぼり、各ディレクトリで同じ2つのパスを検索します。ルートディレクトリ `/` まで検索対象になります
3. ルートディレクトリまで見つからなかった場合は、ユーザーホームディレクトリで次のファイルを順に検索します

   - `~/.tfstylus.hcl`
   - `~/.config/.tfstylus.hcl`

> [!CAUTION]
> **最初に見つかった設定ファイルだけを使用します。** 複数の設定ファイルがあってもマージしません。

たとえば、カレントディレクトリが `/home/user/repos/repo/terraform/dir1` の場合、検索するディレクトリの順序は次のとおりです。各ディレクトリでは、`.tfstylus.hcl` → `.config/.tfstylus.hcl` の順に検索します。

1. `/home/user/repos/repo/terraform/dir1`
2. `/home/user/repos/repo/terraform`
3. `/home/user/repos/repo`
4. `/home/user/repos`
5. `/home/user`
6. `/home`
7. `/`
8. ホームディレクトリ（`/home/user` の場合は再検索）

> [!CAUTION]
> 検索中にアクセス権限などのエラーが発生した場合や、候補のパスが通常のファイルではない場合は、検索を中止してエラーを報告します。

## 参考： init で生成される `.tfstylus.hcl`

```hcl:.tfstylus.hcl
# 全設定を無効にした初期設定です。必要なルールだけ有効にしてください。

# --- 並べ替え ---
# action ブロック内の要素を所定の順序に整列します。
order_action = false

# backend / cloud ブロックを required_version の直後に配置します。
order_backend = false

# check ブロック内の要素を所定の順序に整列します。
order_check = false

# cloud ブロック内の要素を所定の順序に整列します。
order_cloud = false

# cloud.workspaces ブロック内の要素を所定の順序に整列します。
order_cloud_workspaces = false

# data ブロック内の要素を所定の順序に整列します。
order_data = false

# ephemeral ブロック内の要素を所定の順序に整列します。
order_ephemeral = false

# import ブロック内の要素を所定の順序に整列します。
order_import = false

# .tfquery.hcl の list ブロック内の要素を所定の順序に整列します。
order_list = false

# module ブロック内の要素を所定の順序に整列します。
order_module = false

# moved ブロック内の要素を所定の順序に整列します。
order_moved = false

# output ブロック内の要素を所定の順序に整列します。
order_output = false

# provider ブロック内の要素を所定の順序に整列します。
order_provider = false

# removed ブロック内の要素を所定の順序に整列します。
order_removed = false

# required_providers の各 provider オブジェクト内の項目を整列します。
order_required_providers = false

# required_version を terraform ブロックの先頭に配置します。
order_required_version = false

# resource ブロック内の要素を所定の順序に整列します。
order_resource = false

# variable ブロック内の要素を所定の順序に整列します。
order_variable = false

# --- ファイル名の変更 ---
# locals ブロックを含む local.tf を、宛先がない場合に locals.tf へ変更します。
rename_locals_file = false

# output ブロックを含む output.tf を、宛先がない場合に outputs.tf へ変更します。
rename_outputs_file = false

# terraform.tf がなければ、terraform ブロックを含む backend.tf / providers.tf / versions.tf の最初の候補を変更します。
rename_terraform_file = false

# variable ブロックを含む variable.tf を、宛先がない場合に variables.tf へ変更します。
rename_variables_file = false

# --- 文字列の置換 ---
# 既存の required_version を置換します。有効化する場合は false を制約文字列に変更してください。
# replace_required_version = ">= 1.7.0"
replace_required_version = false
```
