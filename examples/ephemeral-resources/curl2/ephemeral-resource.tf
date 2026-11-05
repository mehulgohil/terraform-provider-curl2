# Requires Terraform 1.10 or later. Neither the request nor the response is
# written to the plan or state, so it can carry an ephemeral token.
terraform {
  required_providers {
    curl2 = {
      source  = "mehulgohil/curl2"
      version = "1.8.0"
    }
  }
}

provider "curl2" {}

variable "api_token" {
  type      = string
  sensitive = true
  ephemeral = true
}

ephemeral "curl2" "secret_config" {
  http_method  = "GET"
  uri          = "https://api.example.com/config"
  auth_type    = "Bearer"
  bearer_token = var.api_token
}

# Ephemeral values can be used in locals, provider blocks and other
# ephemeral resources.
locals {
  config = jsondecode(ephemeral.curl2.secret_config.response.body)
}
