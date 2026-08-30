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

variable "rsa_private_key_path" {
  type        = string
  description = "Path to the RSA private key file for token signing."
  default     = "../keys/private_key.pem"
}

variable "rsa_public_key_path" {
  type        = string
  description = "Path to the RSA public key file for token verification."
  default     = "../keys/public_key.pem"
}

variable "auth_db_url" {
  type        = string
  description = "PostgreSQL connection string for Auth service"
  default     = "postgresql://postgres:postgres@auth-db:5432/auth"
}

variable "auth_redis_url" {
  type        = string
  description = "Redis connection string for Auth service"
  default     = "redis://auth-redis:6379"
}

variable "shortener_db_url" {
  type        = string
  description = "PostgreSQL connection string for Shortener service"
  default     = "postgresql://postgres:postgres@shortener-db:5432/urlshortener"
}

variable "shortener_redis_url" {
  type        = string
  description = "Redis connection string for Shortener service"
  default     = "redis://shortener-redis:6379"
}

# ── Google OIDC Variables ──────────────────────────────────────────────────────
variable "google_client_id" {
  type        = string
  description = "Google OAuth 2.0 Client ID for OIDC authentication"
  default     = "mock-google-client-id.apps.googleusercontent.com"
}

variable "google_client_secret" {
  type        = string
  description = "Google OAuth 2.0 Client Secret for OIDC authentication"
  sensitive   = true
  default     = "mock-google-client-secret"
}

variable "google_oidc_callback_to_backend_url" {
  type        = string
  description = "Google OAuth 2.0 Authorized Redirect URI (Callback to Backend URL)"
  default     = "http://localhost/auth/google/callback"
}

# ── RSA Key Pem Variables ──────────────────────────────────────────────────────
variable "rsa_private_key_pem" {
  type        = string
  description = "RSA private key PEM string for token signing."
  sensitive   = true
  default     = null
}

variable "rsa_public_key_pem" {
  type        = string
  description = "RSA public key PEM string for token verification."
  default     = null
}


