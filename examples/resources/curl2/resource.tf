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
}

# A webhook managed through a REST API.
resource "curl2" "webhook" {
  auth_type    = "Bearer"
  bearer_token = var.api_token

  # Where the new object's ID is in the create response, e.g. {"data": {"id": 42}}.
  id_path = "data.id"

  create = {
    uri  = "https://api.example.com/hooks" # POST by default
    json = jsonencode({ url = "https://example.com/hook", events = ["push"] })
  }

  # Optional: refresh. A 404 or 410 means it was deleted outside Terraform, so it is created again.
  read = {
    uri = "https://api.example.com/hooks/{id}" # GET by default
  }

  # Optional: in-place update. Without it, a change to create replaces the object.
  update = {
    uri  = "https://api.example.com/hooks/{id}" # PUT by default
    json = jsonencode({ url = "https://example.com/hook", events = ["push"] })
  }

  # Optional: without it, destroy only removes the object from state.
  destroy = {
    uri = "https://api.example.com/hooks/{id}" # DELETE by default
  }
}

output "webhook_id" {
  value = curl2.webhook.id
}
