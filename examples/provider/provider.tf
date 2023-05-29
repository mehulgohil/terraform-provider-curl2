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