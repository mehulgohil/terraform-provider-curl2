terraform {
  required_providers {
    curl2 = {
      source = "mehulgohil/curl2"
      version = "1.6.1"
    }
  }
}

provider "curl2" {
    auth0 {
      client_id = "<AUTH0_CLIENT_ID>" //You can also set ENV AUTH0_CLIENT_ID
      client_secret = "<AUTH0_CLIENT_SECRET>" //You can also set ENV AUTH0_CLIENT_SECRET
      domain = "<AUTH0_DOMAIN>" //You can also set ENV AUTH0_DOMAIN
    }
}

data "curl2_auth0_token" auth0Token {
  audience = "https://xyx.fy"
}

output "auth_token" {
  value = data.curl2_auth0_token.auth0Token.response
}
