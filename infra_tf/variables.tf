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

variable "auth_db_password" {
  type        = string
  description = "PostgreSQL password for the Auth service database."
  sensitive   = true
}

variable "auth_redis_url" {
  type        = string
  description = "Redis connection string for Auth service"
  default     = "redis://auth-redis:6379"
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

# ── Google OIDC Variables ──────────────────────────────────────────────────────
variable "google_client_id" {
  type        = string
  description = "Google OAuth 2.0 Client ID for OIDC authentication"
}

variable "google_client_secret" {
  type        = string
  description = "Google OAuth 2.0 Client Secret for OIDC authentication"
  sensitive   = true
}

variable "allow_mock_oidc" {
  type        = bool
  description = "Enable mock Google codes for isolated E2E testing only. Never enable in production."
  default     = false
}

variable "google_oidc_callback_to_backend_url" {
  type        = string
  description = "Google OAuth 2.0 Authorized Redirect URI (Callback to Backend URL)"
  default     = "http://localhost/auth/google/callback"
}

variable "cookie_secure" {
  type        = bool
  description = "Mark refresh cookies Secure; enable for HTTPS deployments."
  default     = false
}

# ── RSA Key Pem Variables ──────────────────────────────────────────────────────
variable "rsa_private_key_pem" {
  type        = string
  description = "RSA private key PEM string for token signing."
  sensitive   = true
}


