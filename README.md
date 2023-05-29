# terraform-provider-curl2

## Overview
terraform-provider-curl2 is designed to help make HTTP(s) requests,
with additional support for providing JSON bodies and authentication headers.

* [Curl2 Provider Documentation](https://registry.terraform.io/providers/mehulgohil/curl2/latest/docs)

## Key Features
HTTP Method Support:
1. The custom provider allows you to perform various HTTP methods like GET, POST, PUT, DELETE, etc.
2. JSON Body Support: You can provide JSON payloads as the request body for methods like POST or PUT.
This enables you to send structured data to the API endpoints.
3. Authentication Headers: The custom provider supports the inclusion of authentication headers in the HTTP requests.
You can provide headers like API keys, tokens, or other authentication mechanisms required by the API.
4. Custom Headers: The custom provider supports the inclusion of custom additional headers in the HTTP requests.

## Example

```hcl
terraform {
  required_providers {
    curl2 = {
      source = "mehulgohil/curl2"
      version = "1.2.0"
    }
  }
}

provider "curl2" {
  #  disable_tls = true
}

data "curl2" "getPosts" {
  http_method = "GET"
  uri = "https://jsonplaceholder.typicode.com/posts"
  #  auth_type = "Basic"
  #  basic_auth_username = "<UserName>"
  #  basic_auth_password = "<Password>"
  #  headers = {
  #    Accept = "*/*"
  #  }
}

output "all_posts_response" {
  value = jsondecode(data.curl2.getPosts.response.body)
}

output "all_posts_status" {
  value = data.curl2.getPosts.response.status_code
}
```



