#region Envoy ConfigMap
resource "kubernetes_config_map" "envoy_config" {
  metadata {
    name      = "envoy-config"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  data = {
    "envoy.yaml" = <<-EOF
      static_resources:
        listeners:
        - name: ingress_listener
          address:
            socket_address:
              address: 0.0.0.0
              port_value: 8000
          filter_chains:
          - filters:
            - name: envoy.filters.network.http_connection_manager
              typed_config:
                "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
                stat_prefix: ingress_http
                route_config:
                  name: local_route
                  virtual_hosts:
                  - name: platform_backend
                    domains: ["*"]
                    routes:
                    - match:
                        prefix: "/health"
                      direct_response:
                        status: 200
                        body:
                          inline_string: '{"status":"ok"}'
                    - match:
                        prefix: "/auth"
                      route:
                        cluster: auth_service
                    - match:
                        prefix: "/api/v1/analytics/stats"
                      route:
                        cluster: analytics_service
                        prefix_rewrite: "/stats"
                    - match:
                        prefix: "/api/v1/shorten"
                      route:
                        cluster: shortener_service
                        prefix_rewrite: "/shorten"
                    - match:
                        prefix: "/api/v1/urls/"
                      route:
                        cluster: shortener_service
                        prefix_rewrite: "/urls/"
                    - match:
                        prefix: "/r/"
                      route:
                        cluster: shortener_service
                http_filters:
                - name: envoy.filters.http.jwt_authn
                  typed_config:
                    "@type": type.googleapis.com/envoy.extensions.filters.http.jwt_authn.v3.JwtAuthentication
                    providers:
                      auth_provider:
                        issuer: "auth_service"
                        local_jwks:
                          inline_string: '{"keys":[{"kty":"RSA","alg":"RS256","use":"sig","n":"wQn4lPAu8uCagIaIpxB83qTZcRGv2wOphDwmDNwJtrk3957znZl0QIvASbXBO_JJ1FcWYJ3E0-Sw6uEXMnyDoBhGJK2HNtColzRnTPYJccNyQrJ3bBoLGSYnNvdBBBZQIRknRf_ZOznfO-MmgguEV4CPchAyklR34tVLPgHByTpKN_ahSJ0eHJ3wwRZEpKojeWFNaGcJ_4NoCKH8YsaY6yiy1k26J-h15OoKdDN6j7Sin1l8KKT1begEYn85631gqY3OktoAjpaaSBJW_fEYl5WDo-KAm79Ru-Bf0ecKk1tdm3oLMgNbpj-4SblzCPV-ItvVew-Gsduc5Y9QZklskw","e":"AQAB"}]}'
                        from_headers:
                        - name: Authorization
                          value_prefix: "Bearer "
                    rules:
                    - match:
                        prefix: "/api/v1/shorten"
                      requires:
                        provider_name: auth_provider
                    - match:
                        prefix: "/api/v1/urls/"
                      requires:
                        provider_name: auth_provider
                    - match:
                        prefix: "/api/v1/analytics/stats"
                      requires:
                        provider_name: auth_provider
                - name: envoy.filters.http.router
                  typed_config:
                    "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
        clusters:
        - name: auth_service
          type: STRICT_DNS
          dns_lookup_family: V4_ONLY
          connect_timeout: 0.25s
          lb_policy: ROUND_ROBIN
          load_assignment:
            cluster_name: auth_service
            endpoints:
            - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: auth
                      port_value: 8002
        - name: shortener_service
          type: STRICT_DNS
          dns_lookup_family: V4_ONLY
          connect_timeout: 0.25s
          lb_policy: ROUND_ROBIN
          load_assignment:
            cluster_name: shortener_service
            endpoints:
            - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: shortener
                      port_value: 8001
        - name: analytics_service
          type: STRICT_DNS
          dns_lookup_family: V4_ONLY
          connect_timeout: 0.25s
          lb_policy: ROUND_ROBIN
          load_assignment:
            cluster_name: analytics_service
            endpoints:
            - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: analytics
                      port_value: 8003
      admin:
        address:
          socket_address:
            address: 127.0.0.1
            port_value: 9901
    EOF
  }
}
#endregion

#region Envoy Deployment
resource "kubernetes_deployment" "gateway" {
  metadata {
    name      = "gateway"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
    labels = {
      app = "gateway"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "gateway"
      }
    }

    template {
      metadata {
        labels = {
          app = "gateway"
        }
      }

      spec {
        volume {
          name = "envoy-config"
          config_map {
            name = kubernetes_config_map.envoy_config.metadata[0].name
          }
        }

        container {
          name              = "gateway"
          image             = "envoyproxy/envoy:v1.31-latest"
          image_pull_policy = "IfNotPresent"

          port {
            container_port = 8000
          }

          volume_mount {
            name       = "envoy-config"
            mount_path = "/etc/envoy"
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "128Mi"
            }
            limits = {
              cpu    = "500m"
              memory = "256Mi"
            }
          }

          liveness_probe {
            http_get {
              path = "/health"
              port = 8000
            }
            initial_delay_seconds = 5
            period_seconds        = 10
          }

          readiness_probe {
            http_get {
              path = "/health"
              port = 8000
            }
            initial_delay_seconds = 3
            period_seconds        = 5
          }
        }
      }
    }
  }
}
#endregion

#region Envoy Service
resource "kubernetes_service" "gateway" {
  metadata {
    name      = "gateway"
    namespace = kubernetes_namespace.url_shortener.metadata[0].name
  }

  spec {
    selector = {
      app = "gateway"
    }

    port {
      port        = 8000
      target_port = 8000
      protocol    = "TCP"
    }
  }
}
#endregion
