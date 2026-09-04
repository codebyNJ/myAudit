variable "app_name" {
  type    = string
  default = "acme-saas"
}

variable "api_image" {
  type        = string
  description = "Built backend image (from templates/deploy/Dockerfile), e.g. acme-saas-api:latest"
}

variable "api_port" {
  type    = number
  default = 4400
}

variable "cors_origin" {
  type    = string
  default = "http://localhost:5173"
}

variable "jwt_secret" {
  type      = string
  sensitive = true
}

variable "cookie_secret" {
  type      = string
  sensitive = true
}
