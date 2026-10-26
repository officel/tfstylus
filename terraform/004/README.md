# tfstylus test

- このファイルからコードフェンスを抽出して terraform のファイルを生成し、繰り返しテスト実行を可能にする

## テストファイル

- data、provider block の属性順をテストする
- ignore comment は `#` のみ有効
- 生成時は `//` にしておき、必要に応じて変更することでテスト可能にしている

### data

- `count`、`provider`、データ属性、ネストブロック、`tags`、`lifecycle`、`depends_on` の順に整列する
- lifecycle の precondition / postcondition の内部は整列しない

```hcl:data.tf
// tfstylus-ignore: data_order
data "aws_instance" "example" {
  depends_on = [terraform_data.example]
  tags = {
    Name = "example"
  }
  instance_id = var.instance_id
  provider = aws.example
  count = 1
  filter {
    name = "instance-state-name"
    values = ["running"]
  }
}
```

### provider

- `alias`、`region` 以外の属性を先に配置する
- `default_tags` などのネストブロックは属性の後に配置する

```hcl:provider.tf
// tfstylus-ignore: provider_order
provider "aws" {
  default_tags {
    tags = { Environment = "example" }
  }
  region = "ap-northeast-1"
  alias = "example"
  profile = "example"
}
```
