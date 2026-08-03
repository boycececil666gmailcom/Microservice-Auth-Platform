terraform {
  required_version = ">= 1.5.0"

  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.30"
    }
  }
}

provider "kubernetes" {
  config_path    = var.kubeconfig_path
  config_context = var.kubeconfig_context
}

# ── Namespace ─────────────────────────────────────────────────────────────────
resource "kubernetes_namespace" "url_shortener" {
  metadata {
    name = var.namespace
  }
}

# ── Configuration Map ─────────────────────────────────────────────────────────
resource "kubernetes_config_map" "app_config" {
  metadata {
    name      = "app-config"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  data = {
    SHORTENER_DB_URL          = "postgresql://postgres:postgres@shortener-db:5432/urlshortener"
    SHORTENER_REDIS_URL       = "redis://rfrm-shortener-redis:6379"
    AUTH_DB_URL               = "postgresql://postgres:postgres@auth-db:5432/auth"
    AUTH_REDIS_URL            = "redis://rfrm-auth-redis:6379"
    SHORTENER_URL             = "http://shortener:8001"
    AUTH_URL                  = "http://auth:8002"
    CACHE_TTL_SECONDS         = "86400"
    JWT_EXPIRATION_MINUTES    = "15"
    REFRESH_TOKEN_TTL_SECONDS = "2592000"
    JWT_PRIVATE_KEY_PATH      = "/etc/jwt-keys/private_key.pem"
    JWT_PUBLIC_KEY_PATH       = "/etc/jwt-keys/public_key.pem"
  }
}

# ── Secrets ───────────────────────────────────────────────────────────────────
resource "kubernetes_secret" "jwt_private_key" {
  metadata {
    name      = "jwt-private-key"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  type = "Opaque"

  data = {
    "private_key.pem" = file("${path.module}/${var.rsa_private_key_path}")
  }
}

resource "kubernetes_secret" "jwt_public_key" {
  metadata {
    name      = "jwt-public-key"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  type = "Opaque"

  data = {
    "public_key.pem" = file("${path.module}/${var.rsa_public_key_path}")
  }
}
