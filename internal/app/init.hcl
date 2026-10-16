# 全設定を無効にした初期設定です。必要なルールだけ有効にしてください。

# --- terraform ブロック ---
# backend / cloud ブロックを required_version の直後に配置します。
backend_order = false

# cloud ブロック内の要素を所定の順序に整列します。
cloud_order = false

# cloud.workspaces ブロック内の要素を所定の順序に整列します。
cloud_workspaces_order = false

# 既存の required_version を置換します。有効化する場合は false を制約文字列に変更してください。
# replace_required_version = ">= 1.7.0"
replace_required_version = false

# required_providers の各 provider オブジェクト内の項目を整列します。
required_providers_order = false

# required_version を terraform ブロックの先頭に配置します。
required_version_order = false

# --- 各ブロック内の整列 ---
# action ブロック内の要素を所定の順序に整列します。
action_order = false

# check ブロック内の要素を所定の順序に整列します。
check_order = false

# data ブロック内の要素を所定の順序に整列します。
data_order = false

# ephemeral ブロック内の要素を所定の順序に整列します。
ephemeral_order = false

# import ブロック内の要素を所定の順序に整列します。
import_order = false

# .tfquery.hcl の list ブロック内の要素を所定の順序に整列します。
list_order = false

# module ブロック内の要素を所定の順序に整列します。
module_order = false

# moved ブロック内の要素を所定の順序に整列します。
moved_order = false

# output ブロック内の要素を所定の順序に整列します。
output_order = false

# provider ブロック内の要素を所定の順序に整列します。
provider_order = false

# removed ブロック内の要素を所定の順序に整列します。
removed_order = false

# resource ブロック内の要素を所定の順序に整列します。
resource_order = false

# variable ブロック内の要素を所定の順序に整列します。
variable_order = false

# --- ファイル名の変更 ---
# locals ブロックを含む local.tf を、宛先がない場合に locals.tf へ変更します。
locals_file_rename = false

# output ブロックを含む output.tf を、宛先がない場合に outputs.tf へ変更します。
outputs_file_rename = false

# terraform.tf がなければ、terraform ブロックを含む backend.tf / providers.tf / versions.tf の最初の候補を変更します。
terraform_file_rename = false

# variable ブロックを含む variable.tf を、宛先がない場合に variables.tf へ変更します。
variables_file_rename = false
