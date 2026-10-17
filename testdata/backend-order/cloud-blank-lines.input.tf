terraform {
  required_version = ">= 1.5.0"

  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = ">= 3.9.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.6.0"
    }
  }

  cloud {
    organization = "example"
    workspaces {
      name = "example-users"
    }
  }
}

provider "azuread" {}
provider "random" {}
