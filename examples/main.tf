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
  json = "{\"title\":\"foo\",\"body\":\"bar\",\"userId\":\"1\"}"
}

output "post_posts_output" {
  value = data.curl2.postPosts.response
}