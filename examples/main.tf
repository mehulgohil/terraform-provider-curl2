terraform {
  required_providers {
    curl2 = {
      source = "example.io/example/curl2"
      version = "0.3.1"
    }
  }
}

provider "curl2" {}

data "curl2" "get_request" {
  http_method = "GET"
  uri = "https://g3d99.mocklab.io/json"
  json = "{\"id\":12345,\"value\":\"abc-def-ghi\"}"
  auth_type = "Basic"
  basic_auth_username = "mag"
  basic_auth_password = "mag"
}

#locals {
#  json_data = jsondecode(data.curl2.getTodos.response)
#}

# Returns all Todos
output "all_todos_response" {
  value = jsondecode(data.curl2.getTodos.response.body)
}

output "all_todos_status" {
  value = data.curl2.getTodos.response.status_code
}