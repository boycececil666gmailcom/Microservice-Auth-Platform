#region Secrets
resource "kubernetes_secret" "google_oidc_secret" {
  metadata {
    name      = "google-oidc-secret"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  type = "Opaque"

  data = {
    GOOGLE_CLIENT_SECRET = var.google_client_secret
  }
}

resource "kubernetes_secret" "jwt_private_key" {
  metadata {
    name      = "jwt-private-key"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  type = "Opaque"

  data = {
    "private_key.pem" = var.rsa_private_key_pem != null ? var.rsa_private_key_pem : file("${path.module}/${var.rsa_private_key_path}")
  }
}

resource "kubernetes_secret" "jwt_public_key" {
  metadata {
    name      = "jwt-public-key"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  type = "Opaque"

  data = {
    "public_key.pem" = var.rsa_public_key_pem != null ? var.rsa_public_key_pem : file("${path.module}/${var.rsa_public_key_path}")
  }
}

resource "kubernetes_secret" "auth_db_credentials" {
  metadata {
    name      = "postgres.auth-db.credentials.postgresql.acid.zalan.do"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  type = "Opaque"

  data = {
    username = "postgres"
    password = "postgres"
  }
}

resource "kubernetes_secret" "shortener_db_credentials" {
  metadata {
    name      = "postgres.shortener-db.credentials.postgresql.acid.zalan.do"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  type = "Opaque"

  data = {
    username = "postgres"
    password = "postgres"
  }
}
#endregion
