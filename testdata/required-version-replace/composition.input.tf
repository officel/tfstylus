terraform {
  backend "local" {}
  # minimum
  required_version = ">= 1.0" # keep
  required_providers {
    example = {
      version = "~> 1.0"
      source  = "example/example"
    }
  }
}

terraform {
  experiments      = [example]
  required_version = "~> 1.5"
}
