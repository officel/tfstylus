# tfstylus test

- このファイルからコードフェンスを抽出して terraform のファイルを生成し、繰り返しテスト実行を可能にする

## テストファイル

- import、moved、removed block の属性順をテストする
- ignore comment は `#` のみ有効
- 生成時は `//` にしておき、必要に応じて変更することでテスト可能にしている

### import

- `for_each`、`provider`、`id`、`to` の順に整列する

```hcl:import.tf
// tfstylus-ignore: import_order
import {
  to        = aws_s3_bucket.this
  id        = "bucket-123"
  provider  = aws.west
  for_each  = local.buckets
}
```

### moved

- `from`、`to` の順に整列する

```hcl:moved.tf
// tfstylus-ignore: moved_order
moved {
  to   = module.new
  from = module.old
}
```

### removed

- `from`、`lifecycle` の順に整列する

```hcl:removed.tf
// tfstylus-ignore: removed_order
removed {
  lifecycle {
    destroy = false
  }
  from = aws_instance.old
}
```
