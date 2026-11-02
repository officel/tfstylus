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
