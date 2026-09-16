#region Auth Service
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
        automount_service_account_token = false

        container {
          name              = "auth"
          image             = "url-shortener-auth:latest"
          image_pull_policy = "IfNotPresent"

          security_context {
            allow_privilege_escalation = false
            read_only_root_filesystem  = true
            run_as_non_root            = true
            capabilities { drop = ["ALL"] }
          }

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
            name = "DATABASE_URL"
            value_from {
              secret_key_ref {
                name = kubernetes_secret.auth_database.metadata[0].name
                key  = "database_url"
              }
            }
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

          env {
            name  = "GOOGLE_CLIENT_ID"
            value = var.google_client_id
          }

          env {
            name  = "ALLOW_MOCK_OIDC"
            value = tostring(var.allow_mock_oidc)
          }

          env {
            name = "GOOGLE_CLIENT_SECRET"
            value_from {
              secret_key_ref {
                name = kubernetes_secret.google_oidc_secret.metadata[0].name
                key  = "GOOGLE_CLIENT_SECRET"
              }
            }
          }

          env {
            name  = "GOOGLE_OIDC_CALLBACK_TO_BACKEND_URL"
            value = var.google_oidc_callback_to_backend_url
          }

          env {
            name  = "COOKIE_SECURE"
            value = tostring(var.cookie_secure)
          }

          liveness_probe {
            http_get {
              path = "/health"
              port = 8002
            }
            initial_delay_seconds = 5
            period_seconds        = 10
          }

          readiness_probe {
            http_get {
              path = "/ready"
              port = 8002
            }
            initial_delay_seconds = 3
            period_seconds        = 5
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
#endregion
