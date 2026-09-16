#region Analytics Service
resource "kubernetes_deployment" "analytics" {
  depends_on = [kubernetes_job_v1.kafka_topics]

  metadata {
    name      = "analytics"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels = {
      app = "analytics"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "analytics"
      }
    }

    template {
      metadata {
        labels = {
          app = "analytics"
        }
      }

      spec {
        automount_service_account_token = false

        container {
          name              = "analytics"
          image             = "url-shortener-analytics:latest"
          image_pull_policy = "IfNotPresent"

          security_context {
            allow_privilege_escalation = false
            read_only_root_filesystem  = true
            run_as_non_root            = true
            capabilities { drop = ["ALL"] }
          }

          port {
            container_port = 8003
          }

          resources {
            requests = {
              cpu    = "50m"
              memory = "64Mi"
            }
            limits = {
              cpu    = "250m"
              memory = "128Mi"
            }
          }

          env {
            name  = "KAFKA_BROKER_URL"
            value = var.kafka_broker_url
          }

          liveness_probe {
            http_get {
              path = "/health"
              port = 8003
            }
            initial_delay_seconds = 5
            period_seconds        = 10
          }

          readiness_probe {
            http_get {
              path = "/ready"
              port = 8003
            }
            initial_delay_seconds = 3
            period_seconds        = 5
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "analytics" {
  metadata {
    name      = "analytics"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = {
      app = "analytics"
    }

    port {
      port        = 8003
      target_port = 8003
      protocol    = "TCP"
    }
  }
}
#endregion
