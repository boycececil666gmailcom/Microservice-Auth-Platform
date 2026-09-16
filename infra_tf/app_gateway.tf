#region Envoy ConfigMap
resource "kubernetes_config_map" "envoy_config" {
  metadata {
    name      = "envoy-config"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  data = {
    "envoy.yaml" = file("${path.module}/envoy.yaml")
  }
}
#endregion

#region Envoy Deployment
resource "kubernetes_deployment" "gateway" {
  metadata {
    name      = "gateway"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels    = { app = "gateway" }
  }

  spec {
    replicas = 1
    selector { match_labels = { app = "gateway" } }

    template {
      metadata { labels = { app = "gateway" } }

      spec {
        automount_service_account_token = false

        volume {
          name = "envoy-config"
          config_map {
            name = kubernetes_config_map.envoy_config.metadata[0].name
          }
        }

        volume {
          name = "tmp"
          empty_dir {}
        }

        container {
          name              = "gateway"
          image             = "envoyproxy/envoy:v1.39.1"
          image_pull_policy = "IfNotPresent"

          port { container_port = 8000 }

          volume_mount {
            name       = "envoy-config"
            mount_path = "/etc/envoy"
            read_only  = true
          }

          volume_mount {
            name       = "tmp"
            mount_path = "/tmp"
          }

          security_context {
            allow_privilege_escalation = false
            read_only_root_filesystem  = true
            run_as_non_root            = true
            run_as_user                = 101
            run_as_group               = 101

            capabilities {
              drop = ["ALL"]
            }
          }

          resources {
            requests = { cpu = "100m", memory = "128Mi" }
            limits   = { cpu = "500m", memory = "256Mi" }
          }

          liveness_probe {
            http_get {
              path = "/health"
              port = 8000
            }
            initial_delay_seconds = 5
            period_seconds        = 10
          }

          readiness_probe {
            http_get {
              path = "/health"
              port = 8000
            }
            initial_delay_seconds = 3
            period_seconds        = 5
          }
        }
      }
    }
  }
}
#endregion

#region Envoy Service
resource "kubernetes_service" "gateway" {
  metadata {
    name      = "gateway"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = { app = "gateway" }
    port {
      port        = 8000
      target_port = 8000
      protocol    = "TCP"
    }
  }
}
#endregion
