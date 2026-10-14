terraform {
  # minimum
  required_version = ">= 1.7.0" # keep
  backend "local" {}
  required_providers {
    example = {
      source  = "example/example"
      version = "~> 1.0"
    }
  }
}

terraform {
  required_version = ">= 1.7.0"
  experiments      = [example]
}
