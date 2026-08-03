# ── Auth PostgreSQL Database Deployment & Service ──────────────────────────────
resource "kubernetes_deployment" "auth_db" {
  metadata {
    name      = "auth-db"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels    = { app = "auth-db" }
  }

  spec {
    replicas = 1
    selector { match_labels = { app = "auth-db" } }

    template {
      metadata { labels = { app = "auth-db" } }
      spec {
        container {
          name  = "postgres"
          image = "postgres:16-alpine"
          port { container_port = 5432 }
          env {
            name  = "POSTGRES_USER"
            value = "postgres"
          }
          env {
            name  = "POSTGRES_PASSWORD"
            value = "postgres"
          }
          env {
            name  = "POSTGRES_DB"
            value = "auth"
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

# ── Shortener PostgreSQL Database Deployment & Service ─────────────────────────
resource "kubernetes_deployment" "shortener_db" {
  metadata {
    name      = "shortener-db"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels    = { app = "shortener-db" }
  }

  spec {
    replicas = 1
    selector { match_labels = { app = "shortener-db" } }

    template {
      metadata { labels = { app = "shortener-db" } }
      spec {
        container {
          name  = "postgres"
          image = "postgres:16-alpine"
          port { container_port = 5432 }
          env {
            name  = "POSTGRES_USER"
            value = "postgres"
          }
          env {
            name  = "POSTGRES_PASSWORD"
            value = "postgres"
          }
          env {
            name  = "POSTGRES_DB"
            value = "urlshortener"
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "shortener_db" {
  metadata {
    name      = "shortener-db"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = { app = "shortener-db" }
    port {
      port        = 5432
      target_port = 5432
    }
  }
}

# ── Auth Redis Deployment & Service ───────────────────────────────────────────
resource "kubernetes_deployment" "auth_redis" {
  metadata {
    name      = "auth-redis"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels    = { app = "auth-redis" }
  }

  spec {
    replicas = 1
    selector { match_labels = { app = "auth-redis" } }

    template {
      metadata { labels = { app = "auth-redis" } }
      spec {
        container {
          name  = "redis"
          image = "redis:7-alpine"
          port { container_port = 6379 }
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

# ── Shortener Redis Deployment & Service ──────────────────────────────────────
resource "kubernetes_deployment" "shortener_redis" {
  metadata {
    name      = "shortener-redis"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels    = { app = "shortener-redis" }
  }

  spec {
    replicas = 1
    selector { match_labels = { app = "shortener-redis" } }

    template {
      metadata { labels = { app = "shortener-redis" } }
      spec {
        container {
          name  = "redis"
          image = "redis:7-alpine"
          port { container_port = 6379 }
        }
      }
    }
  }
}

resource "kubernetes_service" "shortener_redis" {
  metadata {
    name      = "shortener-redis"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = { app = "shortener-redis" }
    port {
      port        = 6379
      target_port = 6379
    }
  }
}
