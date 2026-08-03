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
