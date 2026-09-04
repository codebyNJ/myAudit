terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
  }
}

provider "docker" {}

resource "docker_network" "app" {
  name = "${var.app_name}-net"
}

resource "docker_image" "mongo" {
  name = "mongo:7"
}

resource "docker_image" "valkey" {
  name = "valkey/valkey:8"
}

resource "docker_image" "api" {
  name = var.api_image
}

resource "docker_container" "mongo" {
  name  = "${var.app_name}-mongo"
  image = docker_image.mongo.image_id
  networks_advanced {
    name = docker_network.app.name
  }
}

resource "docker_container" "valkey" {
  name  = "${var.app_name}-valkey"
  image = docker_image.valkey.image_id
  networks_advanced {
    name = docker_network.app.name
  }
}

resource "docker_container" "api" {
  name  = "${var.app_name}-api"
  image = docker_image.api.image_id

  networks_advanced {
    name = docker_network.app.name
  }

  ports {
    internal = 4400
    external = var.api_port
  }

  env = [
    "NODE_ENV=production",
    "PORT=4400",
    "HOST=0.0.0.0",
    "MONGODB_URI=mongodb://${var.app_name}-mongo:27017/${var.app_name}",
    "REDIS_URL=redis://${var.app_name}-valkey:6379",
    "CORS_ORIGIN=${var.cors_origin}",
    "JWT_ACCESS_SECRET=${var.jwt_secret}",
    "COOKIE_SECRET=${var.cookie_secret}",
    "MEMBERSHIP_CACHE=shadow",
  ]
}
