# ── API Gateway Deployment & Service ──────────────────────────────────────────
resource "kubernetes_deployment" "gateway" {
  metadata {
    name      = "gateway"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels = {
      app = "gateway"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "gateway"
      }
    }

    template {
      metadata {
        labels = {
          app = "gateway"
        }
      }

      spec {
        container {
          name              = "gateway"
          image             = "url-shortener-gateway:v3"
          image_pull_policy = "IfNotPresent"

          port {
            container_port = 8000
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "128Mi"
            }
            limits = {
              cpu    = "500m"
              memory = "256Mi"
            }
          }

          env {
            name = "JWT_PUBLIC_KEY"
            value_from {
              secret_key_ref {
                name = kubernetes_secret.jwt_public_key.metadata[0].name
                key  = "public_key.pem"
              }
            }
          }

          env_from {
            config_map_ref {
              name = kubernetes_config_map.app_config.metadata[0].name
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "gateway" {
  metadata {
    name      = "gateway"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = {
      app = "gateway"
    }

    port {
      port        = 8000
      target_port = 8000
      protocol    = "TCP"
    }
  }
}

# ── Auth Service Deployment & Service ─────────────────────────────────────────
resource "kubernetes_deployment" "auth" {
  metadata {
    name      = "auth"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels = {
      app = "auth"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "auth"
      }
    }

    template {
      metadata {
        labels = {
          app = "auth"
        }
      }

      spec {
        container {
          name              = "auth"
          image             = "url-shortener-auth:v3"
          image_pull_policy = "IfNotPresent"

          port {
            container_port = 8002
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "128Mi"
            }
            limits = {
              cpu    = "500m"
              memory = "256Mi"
            }
          }

          env {
            name  = "DATABASE_URL"
            value = var.auth_db_url
          }

          env {
            name  = "REDIS_URL"
            value = var.auth_redis_url
          }

          env {
            name = "JWT_PRIVATE_KEY"
            value_from {
              secret_key_ref {
                name = kubernetes_secret.jwt_private_key.metadata[0].name
                key  = "private_key.pem"
              }
            }
          }

          env_from {
            config_map_ref {
              name = kubernetes_config_map.app_config.metadata[0].name
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "auth" {
  metadata {
    name      = "auth"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = {
      app = "auth"
    }

    port {
      port        = 8002
      target_port = 8002
      protocol    = "TCP"
    }
  }
}

# ── Shortener Service Deployment & Service ────────────────────────────────────
resource "kubernetes_deployment" "shortener" {
  metadata {
    name      = "shortener"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels = {
      app = "shortener"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "shortener"
      }
    }

    template {
      metadata {
        labels = {
          app = "shortener"
        }
      }

      spec {
        container {
          name              = "shortener"
          image             = "url-shortener-shortener:latest"
          image_pull_policy = "IfNotPresent"

          port {
            container_port = 8001
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "128Mi"
            }
            limits = {
              cpu    = "500m"
              memory = "256Mi"
            }
          }

          env {
            name  = "DATABASE_URL"
            value = var.shortener_db_url
          }

          env {
            name  = "REDIS_URL"
            value = var.shortener_redis_url
          }

          env_from {
            config_map_ref {
              name = kubernetes_config_map.app_config.metadata[0].name
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "shortener" {
  metadata {
    name      = "shortener"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = {
      app = "shortener"
    }

    port {
      port        = 8001
      target_port = 8001
      protocol    = "TCP"
    }
  }
}
