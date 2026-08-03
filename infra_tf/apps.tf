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
          image             = "url-shortener-gateway:latest"
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

          env_from {
            config_map_ref {
              name = kubernetes_config_map.app_config.metadata[0].name
            }
          }

          volume_mount {
            name       = "jwt-public-key"
            mount_path = "/etc/jwt-keys"
            read_only  = true
          }
        }

        volume {
          name = "jwt-public-key"
          secret {
            secret_name = kubernetes_secret.jwt_public_key.metadata[0].name
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
          image             = "url-shortener-auth:latest"
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
            name = "DB_PASSWORD"
            value_from {
              secret_key_ref {
                name = "postgres.auth-db.credentials.postgresql.acid.zalan.do"
                key  = "password"
              }
            }
          }

          env {
            name  = "DATABASE_URL"
            value = "postgresql://postgres:$(DB_PASSWORD)@auth-db:5432/auth"
          }

          env {
            name = "REDIS_URL"
            value_from {
              config_map_key_ref {
                name = kubernetes_config_map.app_config.metadata[0].name
                key  = "AUTH_REDIS_URL"
              }
            }
          }

          env_from {
            config_map_ref {
              name = kubernetes_config_map.app_config.metadata[0].name
            }
          }

          volume_mount {
            name       = "jwt-private-key"
            mount_path = "/etc/jwt-keys"
            read_only  = true
          }
        }

        volume {
          name = "jwt-private-key"
          secret {
            secret_name = kubernetes_secret.jwt_private_key.metadata[0].name
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
            name = "DB_PASSWORD"
            value_from {
              secret_key_ref {
                name = "postgres.shortener-db.credentials.postgresql.acid.zalan.do"
                key  = "password"
              }
            }
          }

          env {
            name  = "DATABASE_URL"
            value = "postgresql://postgres:$(DB_PASSWORD)@shortener-db:5432/urlshortener"
          }

          env {
            name = "REDIS_URL"
            value_from {
              config_map_key_ref {
                name = kubernetes_config_map.app_config.metadata[0].name
                key  = "SHORTENER_REDIS_URL"
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
