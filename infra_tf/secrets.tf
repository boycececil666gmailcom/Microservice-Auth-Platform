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
    "private_key.pem" = var.rsa_private_key_pem
  }
}

resource "kubernetes_secret" "auth_database" {
  metadata {
    name      = "auth-database"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  type = "Opaque"

  data = {
    password     = var.auth_db_password
    database_url = "postgresql://postgres:${urlencode(var.auth_db_password)}@auth-db:5432/auth"
  }
}

resource "kubernetes_secret" "shortener_database" {
  metadata {
    name      = "shortener-database"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  type = "Opaque"

  data = {
    password     = var.shortener_db_password
    database_url = "postgresql://postgres:${urlencode(var.shortener_db_password)}@shortener-db:5432/urlshortener"
  }
}
#endregion
