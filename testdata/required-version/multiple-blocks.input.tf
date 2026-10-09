terraform {
  unknown_settings = local.setting
  backend "local" {}
  required_version = ">= 1.0"
}

resource "example" "kept" {
  value = "unchanged"
}

resource "example" "second" {
  value = "also unchanged"
}

terraform {
  cloud {
    organization = "example"
  }
  required_version = "~> 2.0"
}
