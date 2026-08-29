#region Shortener Service
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
#endregion
