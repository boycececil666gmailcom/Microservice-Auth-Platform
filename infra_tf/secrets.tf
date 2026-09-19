#region Secrets
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
