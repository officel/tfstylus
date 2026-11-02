# tfstylus test

## テストファイル

- 簡単な初期テスト
- ignore comment は `#` のみ有効
- 生成時は `//` にしておき、必要に応じて変更することでテスト可能にしている

### variables

- 複数の `variable` がある前提で複数形のファイル名が推奨されている
- ❌ `variable.tf`
- ✅ `variables.tf`

```hcl:variable.tf
// tfstylus-ignore-file: rename_variables_file
// tfstylus-ignore: order_variable
variable "test" {
  default     = "test001"
  type        = "string"
  description = "テスト"
}
```

### outputs

- 複数の `output` がある前提で複数形のファイル名が推奨されている
- ❌️ `output.tf`
- ✅ `outputs.tf`

```hcl:output.tf
// tfstylus-ignore-file: rename_outputs_file
// tfstylus-ignore: order_output
output "test" {
  value = var.test
  description = "test"
}
```

### locals

- 推奨事項ではないがブロック名が `locals` であることからも複数形のファイル名に統一する
- ❌ `local.tf`
- ✅ `locals.tf`

```hcl:local.tf
// tfstylus-ignore-file: rename_locals_file
locals {
  value = var.test
  description = "test"
}
```

### terraform

- `terraform` block は複数に分けて記述できるがひとつに統一する（のが望ましい）
- ❌ `backend.tf`
- ❌ `providers.tf`
- ❌ `versions.tf`
- ✅ `terraform.tf`

```hcl:backend.tf
// tfstylus-ignore-file: rename_terraform_file
// tfstylus-ignore: order_required_version, replace_required_version
terraform {
  required_providers {
    random = {
      source  = "hashicorp/random"
      version = ">= 3.5.1"
    }
  }
  required_version = ">= 1.1.0"
}
```

### .tfstylus.hcl

- `tfstylus init` で設定ファイルを生成すると初期ですべての処理がオフになる
- このディレクトリでは `replace_required_version` の置換をテストできるようにしている
- 置換処理をオフにするには、次の三種類の方法がある

  - 設定ファイル自体を削除（`.tfstylus.hcl` を使用しない）
  - 設定ファイル中でオフ（`replace_required_version` に文字列を渡さないか `false` などを設定する）
  - terraform block の直上で ignore

```hcl:.tfstylus.hcl
replace_required_version = ">= 1.17.0"
```
