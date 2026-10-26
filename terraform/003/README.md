# tfstylus test

- このファイルからコードフェンスを抽出して terraform のファイルを生成し、繰り返しテスト実行を可能にする

## テストファイル

- action、check、ephemeral block の属性順をテストする
- ignore comment は `#` のみ有効
- 生成時は `//` にしておき、必要に応じて変更することでテスト可能にしている

### action

- `count`、`provider`、`config` の順に整列する
- `config` の内部は整列しない

```hcl:action.tf
// tfstylus-ignore: action_order
action "aws_lambda_invoke" "example" {
  config {
    function_name = "example-function"
  }
  provider = aws.alias
  count = 3
}
```

### check

- check 内では `data` を `assert` より前に整列する
- assert 内は `condition`、`error_message` の順に整列する

```hcl:check.tf
// tfstylus-ignore: check_order
check "example" {
  assert {
    error_message = "Unexpected status code."
    condition = data.http.example.status_code == 200
  }
  data "http" "example" {
    depends_on = [terraform_data.example]
    url = "https://example.com"
    provider = http.example
  }
}
```

### ephemeral

- `count` / `for_each`、`id`、`name`、`display_name`、`provider`、provider 固有属性、ネストブロック、`tags`、`lifecycle`、`depends_on` の順に整列する
- `lifecycle` 内の precondition / postcondition も整列する

```hcl:ephemeral.tf
// tfstylus-ignore: ephemeral_order
ephemeral "example_session" "example" {
  depends_on = [terraform_data.example]
  lifecycle {
    postcondition {
      error_message = "A token is required."
      condition = self.token != ""
    }
    precondition {
      error_message = "A name is required."
      condition = var.name != ""
    }
  }
  tags = {
    Name = "example"
  }
  options {
    timeout = 30
  }
  name = var.name
  provider = example.alias
  count = 1
}
```
