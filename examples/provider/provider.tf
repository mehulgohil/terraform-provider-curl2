terraform {
  required_providers {
    curl2 = {
      source = "mehulgohil/curl2"
      version = "1.5.0"
    }
  }
}

provider "curl2" {
  #  disable_tls = true
  #  timeout_ms = 500
  #  retry {
  #    retry_attempts = 5
  #    min_delay_ms = 5
  #    max_delay_ms = 10
  #  }
  #  azure_ad {
  #    client_id = "<AZURE_CLIENT_ID>"
  #    client_secret = "<AZURE_CLIENT_SECRET>"
  #    tenant_id = "<AZURE_TENANT_ID>"
  #  }
}