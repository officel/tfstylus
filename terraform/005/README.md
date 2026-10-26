# tfstylus test

- このファイルからコードフェンスを抽出して terraform のファイルを生成し、繰り返しテスト実行を可能にする

## テストファイル

- module block の属性順をテストする
- ignore comment は `#` のみ有効
- 生成時は `//` にしておき、必要に応じて変更することでテスト可能にしている

### module

- `source`、`version`、メタ引数、`id` / `name`、その他の入力、`providers`、ネストブロック、`ignore_nested_deprecations`、`depends_on` の順に整列する
- 入力式や `providers` の内部は整列しない

```hcl:module.tf
// tfstylus-ignore: module_order
module "example" {
  depends_on = [terraform_data.example]
  name = var.name
  providers = {
    aws = aws.example
  }
  cidr = var.cidr
  count = 1
  version = "~> 1.0"
  id = var.id
  source = "example/network/aws"
  ignore_nested_deprecations = true
}
```
