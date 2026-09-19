output "namespace" {
  value       = kubernetes_namespace.url_shortener.metadata[0].name
  description = "Target Kubernetes namespace."
}
output "namespace" {
  value       = kubernetes_namespace.url_shortener.metadata[0].name
  description = "Target Kubernetes namespace."
}

output "gateway_service_name" {
  value       = kubernetes_service.gateway.metadata[0].name
  description = "Kubernetes service name for API Gateway."
}

output "shortener_service_name" {
  value       = kubernetes_service.shortener.metadata[0].name
  description = "Kubernetes service name for Shortener Service."
}

output "analytics_service_name" {
  value       = kubernetes_service.analytics.metadata[0].name
  description = "Kubernetes service name for Analytics Service."
}

output "kafka_service_name" {
  value       = kubernetes_service.kafka.metadata[0].name
  description = "Kubernetes service name for the local/test Kafka broker."
}
