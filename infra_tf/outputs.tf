output "namespace" {
  value       = kubernetes_namespace.url_shortener.metadata[0].name
  description = "Target Kubernetes namespace."
}

output "gateway_service_name" {
  value       = kubernetes_service.gateway.metadata[0].name
  description = "Kubernetes service name for API Gateway."
}

output "auth_service_name" {
  value       = kubernetes_service.auth.metadata[0].name
  description = "Kubernetes service name for Auth Service."
}

output "shortener_service_name" {
  value       = kubernetes_service.shortener.metadata[0].name
  description = "Kubernetes service name for Shortener Service."
}
