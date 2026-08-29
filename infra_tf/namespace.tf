#region Namespace
resource "kubernetes_namespace" "url_shortener" {
  metadata {
    name = var.namespace
  }
}
#endregion
