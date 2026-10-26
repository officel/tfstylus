# tfstylus test

- このファイルからコードフェンスを抽出してテストファイルを生成し、繰り返しテスト実行を可能にする

## テストファイル

- 001〜006 にない ignore ルールと、指示の適用範囲・認識条件をテストする
- HCL の ignore comment は単独行の `#` のみ有効
- 生成時は `//` にしておき、必要に応じて指示行だけ `#` に変更する
- `task mktf NUM=7` で生成し、`task tfsp` で変更前後の差分を確認する
- `plan` は変更を適用しないため、指示を書き換えて繰り返し確認できる
- 整列ルールが設定で無効の場合は比較できないため、テストするルールを有効にしておく

### terraform

- `backend_order`: `backend` を先頭に移動する（この例には `required_version` がない）
- `cloud_order`: cloud 内を `organization`、`hostname`、`workspaces` の順に整列する
- `cloud_workspaces_order`: workspaces 内を `project`、`tags` の順に整列する
- `required_providers_order`: provider 要求内を `source`、`version` の順に整列する
- 指示はネストした backend / cloud / workspaces / required_providers ではなく、トップレベルの terraform block の直前に置く
- 各指示を `#` にすると、指定したルールだけ除外される。cloud の2行は片方ずつ変更し、親子の整列を独立して除外できることも確認する
- ファイル名は `terraform.tf` とし、ファイル名変更の差分を混在させない

```hcl:terraform.tf
// tfstylus-ignore: backend_order
terraform {
  required_providers {
    random = {
      source = "hashicorp/random"
      version = "~> 3.0"
    }
  }
  backend "local" {
    path = "terraform.tfstate"
  }
}

// tfstylus-ignore: cloud_order
// tfstylus-ignore: cloud_workspaces_order
terraform {
  cloud {
    workspaces {
      tags = ["example"]
      project = "example-project"
    }
    hostname = "app.terraform.io"
    organization = "example"
  }
}

// tfstylus-ignore: required_providers_order
terraform {
  required_providers {
    aws = {
      version = "~> 6.0"
      source = "hashicorp/aws"
    }
  }
}
```

### list

- `list_order`: `count`、`provider`、`include_resource`、`limit`、`config` の順に整列する
- `.tf` ではなく `.tfquery.hcl` が対象。config の内部は整列しない
- 指示を `#` にすると、この list block の整列が除外される
- `tfstylus-ignore-file: list_order` に変更した場合もファイル全体で除外できる

```hcl:query.tfquery.hcl
// tfstylus-ignore: list_order
list "aws_instance" "example" {
  config {
    region = "ap-northeast-1"
  }
  limit = 10
  include_resource = true
  provider = aws.example
  count = 1
}
```

### ファイル単位・複数ルール・重複・未知ルール

- 先頭コメント群では通常コメントと空行を挟んでもファイル指示が有効
- 2つの指示を `#` にすると、両方の resource と module が整列対象から除外される
- 複数の指示は結合され、重複した `resource_order` は1つとして扱われる
- 未知の `typo_order` は無視されるが、同じ指示内の既知ルールは有効

```hcl:file_scope.tf
// File header.

// tfstylus-ignore-file: resource_order, typo_order, resource_order
// tfstylus-ignore-file: module_order
// Keep the original order.

resource "terraform_data" "file_first" {
  input = "first"
  count = 1
}

resource "terraform_data" "file_second" {
  input = "second"
  count = 1
}

module "file_example" {
  version = "~> 1.0"
  source = "example/network/aws"
}
```

### block 単位・通常コメント・行頭の空白

- 指示から block まで空行がなければ、通常コメントを挟んでも有効
- 指示の行頭に空白があっても有効
- 指示を `#` にすると、直後の `block_ignored` だけ元の順序を保持する
- `block_sorted` は引き続き `count` が `input` より前に移動する

```hcl:block_scope.tf
  // tfstylus-ignore: resource_order
// Keep only the following block.
resource "terraform_data" "block_ignored" {
  input = "ignored"
  count = 1
}

resource "terraform_data" "block_sorted" {
  input = "sorted"
  count = 1
}
```

### 認識されない指示

- 以下の指示を `#` にしても、各 resource の `count` は `input` より前に移動する
- `blank_line`: block との間に空行がある
- `late_file`: 最初の HCL 要素より後にファイル指示がある
- `nested`: トップレベル block ではなく属性の直前に指示がある
- `extra_text`: 指示行に理由を追記している。理由は別行の通常コメントに記載する
- `unknown`: `all` による一括除外や未知のルール名だけでは除外できない
- `block_comment`: `/* ... */` は指示として認識されない
- `literal`: 文字列や heredoc 内の指示に見える文字列は指示として認識されない
- 有効な `#` 指示に変更しない限り、前節までの `//` 指示も無効
- 無効な指示自体の診断は出ない。差分で整列の有無を確認する

```hcl:invalid_directives.tf
// tfstylus-ignore: resource_order

resource "terraform_data" "blank_line" {
  input = "blank"
  count = 1
}

// tfstylus-ignore-file: resource_order
resource "terraform_data" "late_file" {
  input = "late"
  count = 1
}

resource "terraform_data" "nested" {
  input = "nested"
  // tfstylus-ignore: resource_order
  count = 1
}

// tfstylus-ignore: resource_order because manual
resource "terraform_data" "extra_text" {
  input = "extra"
  count = 1
}

// tfstylus-ignore: all, typo_order
resource "terraform_data" "unknown" {
  input = "unknown"
  count = 1
}

/* tfstylus-ignore: resource_order */
resource "terraform_data" "block_comment" {
  input = "comment"
  count = 1
}

resource "terraform_data" "literal" {
  input = {
    text = "# tfstylus-ignore-file: resource_order"
    script = <<-EOT
      # tfstylus-ignore: resource_order
    EOT
  }
  count = 1
}
```

## 後片付け

- `task rmtf` で `.tf` を削除する
- 現在の `task rmtf` は `.tfquery.hcl` を削除しないため、このディレクトリの query ファイルは別途削除する

```sh
rm -f 007/query.tfquery.hcl
```

## 対象外・追加確認時の注意

- ファイル名変更の ignore は 001 で確認する。block 指示では除外できず、`tfstylus-ignore-file` が必要
- 行末の指示は無効だが、コメントの所属が曖昧な場合は安全のため整列自体がスキップされることがある。「差分がない」だけでは ignore が有効とは判定しない
- `.tf.json` は先頭の `"//"` プロパティによるファイル指示のみ認識するが、現時点では変換自体が行われないため、ignore の有無による差分比較の対象外
