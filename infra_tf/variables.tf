variable "kubeconfig_path" {
  type        = string
  description = "Path to the local kubeconfig file."
  default     = "~/.kube/config"
}

variable "kubeconfig_context" {
  type        = string
  description = "Kubernetes context to use."
  default     = null
}

variable "kubeconfig_path" {
  type        = string
  description = "Path to the local kubeconfig file."
  default     = "~/.kube/config"
}

variable "kubeconfig_context" {
  type        = string
  description = "Kubernetes context to use."
  default     = null
}

variable "namespace" {
  type        = string
  description = "Target Kubernetes namespace for URL Shortener services."
  default     = "url-shortener"
}

variable "shortener_db_password" {
  type        = string
  description = "PostgreSQL password for the Shortener service database."
  sensitive   = true
}

variable "shortener_redis_url" {
  type        = string
  description = "Redis connection string for Shortener service"
  default     = "redis://shortener-redis:6379"
}

variable "kafka_broker_url" {
  type        = string
  description = "Kafka broker address used by the shortener and analytics services"
  default     = "kafka:9092"
}
