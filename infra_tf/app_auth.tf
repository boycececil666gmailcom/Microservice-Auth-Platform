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

          env {
            name  = "GOOGLE_CLIENT_ID"
            value = var.google_client_id
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
            name  = "GOOGLE_REDIRECT_URI"
            value = var.google_redirect_uri
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
