terraform {
  required_providers {
    curl2 = {
      source = "mehulgohil/curl2"
      version = "1.5.0"
    }
  }
}

provider "curl2" {
  azure_ad {
    client_id = "<AZURE_CLIENT_ID>" //You can also set ENV AZURE_CLIENT_ID
    client_secret = "<AZURE_CLIENT_SECRET>" //You can also set ENV AZURE_CLIENT_SECRET
    tenant_id = "<AZURE_TENANT_ID>" //You can also set ENV AZURE_TENANT_ID
  }
}

data "curl2_azuread_token" azureADToken {
  scopes = ["https://graph.microsoft.com/.default"]
}

output "azure_ad_token" {
  value = data.curl2_azuread_token.azureADToken.response
}
