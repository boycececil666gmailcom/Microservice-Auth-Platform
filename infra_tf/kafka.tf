# Single-node KRaft broker for local and test clusters. Production deployments
# should use a managed or multi-node Kafka installation and override
# kafka_broker_url.
resource "kubernetes_deployment" "kafka" {
  metadata {
    name      = "kafka"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels    = { app = "kafka" }
  }

  spec {
    replicas = 1
    selector { match_labels = { app = "kafka" } }

    template {
      metadata { labels = { app = "kafka" } }
      spec {
        container {
          name              = "kafka"
          image             = "apache/kafka:3.9.2"
          image_pull_policy = "IfNotPresent"

          port { container_port = 9092 }
          port { container_port = 9093 }

          env {
            name  = "KAFKA_NODE_ID"
            value = "1"
          }
          env {
            name  = "KAFKA_PROCESS_ROLES"
            value = "broker,controller"
          }
          env {
            name  = "KAFKA_LISTENERS"
            value = "PLAINTEXT://:9092,CONTROLLER://:9093"
          }
          env {
            name  = "KAFKA_ADVERTISED_LISTENERS"
            value = "PLAINTEXT://kafka:9092"
          }
          env {
            name  = "KAFKA_CONTROLLER_LISTENER_NAMES"
            value = "CONTROLLER"
          }
          env {
            name  = "KAFKA_LISTENER_SECURITY_PROTOCOL_MAP"
            value = "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT"
          }
          env {
            name  = "KAFKA_CONTROLLER_QUORUM_VOTERS"
            value = "1@localhost:9093"
          }
          env {
            name  = "KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR"
            value = "1"
          }
          env {
            name  = "KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR"
            value = "1"
          }
          env {
            name  = "KAFKA_TRANSACTION_STATE_LOG_MIN_ISR"
            value = "1"
          }
          env {
            name  = "KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS"
            value = "0"
          }
          env {
            name  = "KAFKA_NUM_PARTITIONS"
            value = "1"
          }

          resources {
            requests = {
              cpu    = "250m"
              memory = "512Mi"
            }
            limits = {
              cpu    = "1"
              memory = "1Gi"
            }
          }

          liveness_probe {
            tcp_socket { port = 9092 }
            initial_delay_seconds = 20
            period_seconds        = 10
          }

          readiness_probe {
            tcp_socket { port = 9092 }
            initial_delay_seconds = 10
            period_seconds        = 5
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "kafka" {
  metadata {
    name      = "kafka"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = { app = "kafka" }
    port {
      name        = "broker"
      port        = 9092
      target_port = 9092
    }
  }
}

resource "kubernetes_job_v1" "kafka_topics" {
  metadata {
    name      = "kafka-topics"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  wait_for_completion = true

  spec {
    backoff_limit = 6
    template {
      metadata { labels = { app = "kafka-topics" } }
      spec {
        automount_service_account_token = false
        restart_policy                  = "OnFailure"

        container {
          name  = "create-topics"
          image = "apache/kafka:3.9.2"
          command = [
            "/bin/bash",
            "-ec",
            <<-EOT
              until /opt/kafka/bin/kafka-broker-api-versions.sh --bootstrap-server kafka:9092 >/dev/null 2>&1; do sleep 2; done
              /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 --create --if-not-exists --topic url-redirects --partitions 1 --replication-factor 1
            EOT
          ]

          resources {
            requests = { cpu = "50m", memory = "128Mi" }
            limits   = { cpu = "250m", memory = "256Mi" }
          }
        }
      }
    }
  }

  timeouts {
    create = "5m"
  }

  depends_on = [kubernetes_deployment.kafka, kubernetes_service.kafka]
}
