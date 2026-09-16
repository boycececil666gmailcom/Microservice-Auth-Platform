#region Auth Databases
resource "kubernetes_persistent_volume_claim" "auth_db" {
  metadata {
    name      = "auth-db-data"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = { storage = "2Gi" }
    }
  }
}

resource "kubernetes_deployment" "auth_db" {
  metadata {
    name      = "auth-db"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels    = { app = "auth-db" }
  }

  spec {
    replicas = 1
    strategy { type = "Recreate" }
    selector { match_labels = { app = "auth-db" } }

    template {
      metadata { labels = { app = "auth-db" } }
      spec {
        volume {
          name = "data"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.auth_db.metadata[0].name
          }
        }

        container {
          name  = "postgres"
          image = "postgres:16-alpine"
          port { container_port = 5432 }
          env {
            name  = "POSTGRES_USER"
            value = "postgres"
          }
          env {
            name = "POSTGRES_PASSWORD"
            value_from {
              secret_key_ref {
                name = kubernetes_secret.auth_database.metadata[0].name
                key  = "password"
              }
            }
          }
          env {
            name  = "POSTGRES_DB"
            value = "auth"
          }

          volume_mount {
            name       = "data"
            mount_path = "/var/lib/postgresql/data"
          }

          resources {
            requests = { cpu = "100m", memory = "256Mi" }
            limits   = { cpu = "500m", memory = "512Mi" }
          }

          liveness_probe {
            exec { command = ["pg_isready", "-U", "postgres", "-d", "auth"] }
            initial_delay_seconds = 10
            period_seconds        = 10
          }

          readiness_probe {
            exec { command = ["pg_isready", "-U", "postgres", "-d", "auth"] }
            initial_delay_seconds = 5
            period_seconds        = 5
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "auth_db" {
  metadata {
    name      = "auth-db"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = { app = "auth-db" }
    port {
      port        = 5432
      target_port = 5432
    }
  }
}

resource "kubernetes_persistent_volume_claim" "auth_redis" {
  metadata {
    name      = "auth-redis-data"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = { storage = "1Gi" }
    }
  }
}

resource "kubernetes_deployment" "auth_redis" {
  metadata {
    name      = "auth-redis"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels    = { app = "auth-redis" }
  }

  spec {
    replicas = 1
    strategy { type = "Recreate" }
    selector { match_labels = { app = "auth-redis" } }

    template {
      metadata { labels = { app = "auth-redis" } }
      spec {
        volume {
          name = "data"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.auth_redis.metadata[0].name
          }
        }

        container {
          name  = "redis"
          image = "redis:7-alpine"
          port { container_port = 6379 }

          args = ["redis-server", "--appendonly", "yes"]

          volume_mount {
            name       = "data"
            mount_path = "/data"
          }

          resources {
            requests = { cpu = "50m", memory = "64Mi" }
            limits   = { cpu = "250m", memory = "128Mi" }
          }

          liveness_probe {
            exec { command = ["redis-cli", "ping"] }
            initial_delay_seconds = 5
            period_seconds        = 10
          }

          readiness_probe {
            exec { command = ["redis-cli", "ping"] }
            initial_delay_seconds = 3
            period_seconds        = 5
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "auth_redis" {
  metadata {
    name      = "auth-redis"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = { app = "auth-redis" }
    port {
      port        = 6379
      target_port = 6379
    }
  }
}
#endregion
