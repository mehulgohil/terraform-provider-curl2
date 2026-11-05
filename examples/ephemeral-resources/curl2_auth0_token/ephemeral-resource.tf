# Requires Terraform 1.10 or later. The token is never written to the plan or state.
terraform {
  required_providers {
    curl2 = {
      source  = "mehulgohil/curl2"
      version = "1.7.0"
    }
  }
}

provider "curl2" {
  auth0 {
    client_id     = "<AUTH0_CLIENT_ID>"     //You can also set ENV AUTH0_CLIENT_ID
    client_secret = "<AUTH0_CLIENT_SECRET>" //You can also set ENV AUTH0_CLIENT_SECRET
    domain        = "<AUTH0_DOMAIN>"        //You can also set ENV AUTH0_DOMAIN
  }
}

ephemeral "curl2_auth0_token" "api" {
  audience = "https://api.example.com"
}

# Use the token in an ephemeral request...
ephemeral "curl2" "me" {
  http_method  = "GET"
  uri          = "https://api.example.com/me"
  auth_type    = "Bearer"
  bearer_token = ephemeral.curl2_auth0_token.api.token
}

# ...or in another provider's configuration, e.g.
# provider "restapi" {
#   headers = { Authorization = "Bearer ${ephemeral.curl2_auth0_token.api.token}" }
# }
