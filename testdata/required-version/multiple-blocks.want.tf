terraform {
  required_version = ">= 1.0"
  unknown_settings = local.setting
  backend "local" {}
}

resource "example" "kept" {
  value = "unchanged"
}

resource "example" "second" {
  value = "also unchanged"
}

terraform {
  required_version = "~> 2.0"
  cloud {
    organization = "example"
  }
}
