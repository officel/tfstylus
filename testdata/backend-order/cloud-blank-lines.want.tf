terraform {
  required_version = ">= 1.5.0"

  cloud {
    organization = "example"
    workspaces {
      name = "example-users"
    }
  }
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

}

provider "azuread" {}
provider "random" {}
