#
# Copyright (c) 2021-present Sonatype, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# --------------------------------------------------------------------------
# Create k8s Namespace
# --------------------------------------------------------------------------
resource "kubernetes_namespace" "the_cla" {
  metadata {
    name        = "the-cla"
  }
}

# --------------------------------------------------------------------------
# Create k8s Secrets
# --------------------------------------------------------------------------
resource "kubernetes_secret" "the_cla" {
  metadata {
    name        = "the-cla"
    namespace   = kubernetes_namespace.the_cla.metadata[0].name
  }

  binary_data = {
    "the-cla.pem"   = "${var.the_cla_pem}"
  }

  data = {
    "env_gh_app_id" = var.env_gh_app_id
    "env_github_client_secret" = var.env_github_client_secret
    "env_github_webhook_secret" = var.env_github_webhook_secret
    "psql_password" = module.database.user_password
  }

  type = "Opaque"
}

# --------------------------------------------------------------------------
# Create k8s Deployment
# --------------------------------------------------------------------------
resource "kubernetes_deployment" "the_cla" {
  metadata {
    name            = "the-cla"
    namespace       = kubernetes_namespace.the_cla.metadata[0].name
    labels = {
      app = "the-cla"
    }
  }
  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "the-cla"
      }
    }

    template {
      metadata {
        labels = {
          app = "the-cla"
        }
      }

      spec {
        container {
          image             = "sonatypecommunity/the-cla:latest"
          name              = "the-cla"
          image_pull_policy = "IfNotPresent"

          env {
            name = "GITHUB_CLIENT_SECRET"
            value_from {
              secret_key_ref {
                name = "the-cla"
                key  = "env_github_client_secret"
              }
            }
          }

          env {
            name = "GH_APP_ID"
            value_from {
              secret_key_ref {
                name = "the-cla"
                key  = "env_gh_app_id"
              }
            }
          }

          env {
            name = "GH_WEBHOOK_SECRET"
            value_from {
              secret_key_ref {
                name = "the-cla"
                key  = "env_github_webhook_secret"
              }
            }
          }

          env {
            name = "PG_HOST"
            value = module.shared.pgsql_cluster_endpoint_write
          }

          env {
            name = "PG_PORT"
            value = module.shared.pgsql_cluster_port
          }

          env {
            name = "PG_USERNAME"
            value = local.cla_db_username
          }

          env {
            name = "PG_DB_NAME"
            value = local.cla_db_name
          }

          env {
            name = "PG_PASSWORD"
            value_from {
              secret_key_ref {
                name = "the-cla"
                key  = "psql_password"
              }
            }
          }

          env {
            name  = "SSL_MODE"
            value = "require"
          }

          port {
            name           = "app"
            container_port = 4200
          }

          # security_context {
          #   run_as_user = 1000
          # }

          volume_mount {
            mount_path = "/the-cla-secrets"
            name       = "the-cla-secrets"
          }
        }

        volume {
          name = "the-cla-secrets"
          secret {
            secret_name = "the-cla"
            items {
              key = "the-cla.pem"
              path = "the-cla.pem"
            }
          }
        }

        # volume {
        #   name = "nxiq-data"
        #   persistent_volume_claim {
        #     claim_name = kubernetes_persistent_volume_claim.nxiq.metadata[0].name
        #   }
        # }

        # volume {
        #   name = "nxiq-config"
        #   config_map {
        #     name = kubernetes_config_map.nxiq.metadata[0].name
        #     items {
        #       key  = "config.yml"
        #       path = "config.yml"
        #     }
        #   }
        # }
      }
    }
  }
}

# --------------------------------------------------------------------------
# Create k8s Services
# --------------------------------------------------------------------------
resource "kubernetes_service" "the_cla" {
  metadata {
    name            = "the-cla-svc"
    namespace       = kubernetes_namespace.the_cla.metadata[0].name
    labels = {
      app = "the-cla"
    }
  }
  spec {
    selector = {
      app = kubernetes_deployment.the_cla.metadata.0.labels.app
    }

    port {
      name        = "http"
      port        = 4200
      target_port = 4200
      protocol    = "TCP"
    }

    type = "NodePort"
  }
}

##############################################################################
# Create Ingress for NXIQ
##############################################################################
resource "kubernetes_ingress_v1" "the_cla" {
  metadata {
    name      = "the-cla-ingress"
    namespace = kubernetes_namespace.the_cla.metadata[0].name
    labels = {
      app = "the-cla"
    }
    annotations = {
      "kubernetes.io/ingress.class"               = "alb"
      "alb.ingress.kubernetes.io/group.name"      = "the-cla-${terraform.workspace}"
      # "alb.ingress.kubernetes.io/healthcheck-path"= "/assets/index.html"
      # "alb.ingress.kubernetes.io/inbound-cidrs"   = join(", ", var.ip_cidr_whitelist)
      "alb.ingress.kubernetes.io/scheme"          = "internet-facing"
      "alb.ingress.kubernetes.io/certificate-arn" = module.shared_private.bma_cert_arn
      "external-dns.alpha.kubernetes.io/hostname" = "the-cla.${module.shared_private.dns_zone_bma_name}"
    }
  }

  spec {
    rule {
      host = "the-cla.${module.shared_private.dns_zone_bma_name}"
      http {
        path {
          path = "/*"
          backend {
            service {
              name = kubernetes_service.the_cla.metadata[0].name
              port {
                number = 4200
              }
            }
          }
        }
      }
    }
  }

  wait_for_load_balancer = true
}