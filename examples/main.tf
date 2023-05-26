terraform {
  required_providers {
    curl2 = {
      source = "mehulgohil/curl2"
      version = "1.0.0"
    }
  }
}

provider "curl2" {}

data "curl2" "getPosts" {
  http_method = "GET"
  uri = "https://jsonplaceholder.typicode.com/posts"
#  auth_type = "Basic"
#  basic_auth_username = "<UserName>"
#  basic_auth_password = "<Password>"
}

output "all_posts_response" {
  value = jsondecode(data.curl2.getPosts.response.body)
}

output "all_posts_status" {
  value = data.curl2.getPosts.response.status_code
}

data "curl2" "postPosts" {
  http_method = "POST"
  uri = "https://jsonplaceholder.typicode.com/posts"
  json = "{\"title\":\"foo\",\"body\":\"bar\",\"userId\":\"1\"}" //need the json in string format
#  auth_type = "Bearer"
#  bearer_token = "<Any Bearer Token>"
}

output "post_posts_output" {
  value = data.curl2.postPosts.response
}