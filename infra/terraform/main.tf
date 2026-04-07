# Astartes Primaris — Google Cloud Run Infrastructure
# "The Imperium deploys to the cloud."
#
# Usage:
#   cd infra/terraform
#   terraform init
#   terraform plan -var="project_id=YOUR_GCP_PROJECT"
#   terraform apply -var="project_id=YOUR_GCP_PROJECT"

terraform {
  required_version = ">= 1.6"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

# ─── Variables ───────────────────────────────────────────────

variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "environment" {
  description = "Fortress Monastery environment (prod, staging, dev)"
  type        = string
  default     = "dev"
}

variable "region" {
  description = "GCP region for Cloud Run services"
  type        = string
  default     = "us-central1"
}

# ─── Provider ──────────────────────────────────────────────

provider "google" {
  project = var.project_id
  region  = var.region
}

# ─── Locals ──────────────────────────────────────────────────

locals {
  project_name = "astartes-primaris"
  ar_repo      = "${var.region}-docker.pkg.dev/${var.project_id}/${local.project_name}"

  labels = {
    project     = "astartes-primaris"
    environment = var.environment
    managed-by  = "terraform"
  }
}

# ─── Enable APIs ──────────────────────────────────────────────

resource "google_project_service" "apis" {
  for_each = toset([
    "run.googleapis.com",
    "artifactregistry.googleapis.com",
    "cloudbuild.googleapis.com",
    "secretmanager.googleapis.com",
    "sqladmin.googleapis.com",
    "vpcaccess.googleapis.com",
  ])

  service            = each.value
  disable_on_destroy = false
}

# ─── Artifact Registry ──────────────────────────────────────

resource "google_artifact_registry_repository" "images" {
  location      = var.region
  repository_id = local.project_name
  format        = "DOCKER"
  labels        = local.labels

  depends_on = [google_project_service.apis]
}

# ─── Cloud SQL (PostgreSQL + TimescaleDB) ─────────────────────

resource "google_sql_database_instance" "librarium" {
  name             = "librarium-${var.environment}"
  database_version = "POSTGRES_16"
  region           = var.region

  settings {
    tier              = var.environment == "prod" ? "db-custom-2-4096" : "db-f1-micro"
    availability_type = var.environment == "prod" ? "REGIONAL" : "ZONAL"

    database_flags {
      name  = "shared_preload_libraries"
      value = "timescaledb"
    }

    backup_configuration {
      enabled    = var.environment == "prod"
      start_time = "03:00"
    }

    ip_configuration {
      ipv4_enabled = true
    }
  }

  deletion_protection = var.environment == "prod"

  depends_on = [google_project_service.apis]
}

resource "google_sql_database" "librarium_db" {
  name     = "librarium"
  instance = google_sql_database_instance.librarium.name
}

resource "google_sql_user" "librarium_user" {
  name     = "librarium"
  instance = google_sql_database_instance.librarium.name
  password = random_password.db_password.result
}

resource "random_password" "db_password" {
  length  = 32
  special = false
}

# ─── Secrets ──────────────────────────────────────────────────

resource "google_secret_manager_secret" "db_password" {
  secret_id = "librarium-db-password-${var.environment}"

  replication {
    auto {}
  }

  depends_on = [google_project_service.apis]
}

resource "google_secret_manager_secret_version" "db_password" {
  secret      = google_secret_manager_secret.db_password.id
  secret_data = random_password.db_password.result
}

# ─── VPC Connector (for Cloud Run → Cloud SQL) ───────────────

resource "google_vpc_access_connector" "connector" {
  name          = "astartes-vpc-${var.environment}"
  region        = var.region
  ip_cidr_range = "10.8.0.0/28"
  network       = "default"

  depends_on = [google_project_service.apis]
}

# ─── Primarch (Cloud Run Service) ─────────────────────────────

resource "google_cloud_run_v2_service" "primarch" {
  name     = "primarch-${var.environment}"
  location = var.region
  labels   = local.labels

  template {
    labels = local.labels

    containers {
      image = "${local.ar_repo}/primarch:latest"

      ports {
        container_port = 8401
      }

      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }

      env {
        name  = "DATABASE_URL"
        value = "postgres://librarium:${random_password.db_password.result}@/${google_sql_database.librarium_db.name}?host=/cloudsql/${google_sql_database_instance.librarium.connection_name}&sslmode=disable"
      }

      resources {
        limits = {
          cpu    = var.environment == "prod" ? "2" : "1"
          memory = var.environment == "prod" ? "1Gi" : "512Mi"
        }
      }

      startup_probe {
        http_get {
          path = "/health"
        }
        initial_delay_seconds = 5
        period_seconds        = 10
        failure_threshold     = 3
      }

      liveness_probe {
        http_get {
          path = "/health"
        }
        period_seconds = 30
      }
    }

    scaling {
      min_instance_count = var.environment == "prod" ? 1 : 0
      max_instance_count = var.environment == "prod" ? 5 : 2
    }

    vpc_access {
      connector = google_vpc_access_connector.connector.id
      egress    = "PRIVATE_RANGES_ONLY"
    }

    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = [google_sql_database_instance.librarium.connection_name]
      }
    }
  }

  depends_on = [google_project_service.apis]
}

# ─── Aurum (Cloud Run Service) ────────────────────────────────

resource "google_cloud_run_v2_service" "aurum" {
  name     = "aurum-${var.environment}"
  location = var.region
  labels   = local.labels

  template {
    labels = local.labels

    containers {
      image = "${local.ar_repo}/aurum:latest"

      ports {
        container_port = 8080
      }

      env {
        name  = "AURUM_PORT"
        value = "8080"
      }

      env {
        name  = "PRIMARCH_URL"
        value = google_cloud_run_v2_service.primarch.uri
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "256Mi"
        }
      }
    }

    scaling {
      min_instance_count = var.environment == "prod" ? 1 : 0
      max_instance_count = var.environment == "prod" ? 3 : 1
    }
  }

  depends_on = [google_project_service.apis]
}

# ─── Make Aurum public (unauthenticated) ─────────────────────

resource "google_cloud_run_v2_service_iam_member" "aurum_public" {
  name     = google_cloud_run_v2_service.aurum.name
  location = var.region
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ─── Forge (Cloud Run Job — batch backtests) ──────────────────

resource "google_cloud_run_v2_job" "forge" {
  name     = "forge-${var.environment}"
  location = var.region
  labels   = local.labels

  template {
    template {
      containers {
        image = "${local.ar_repo}/forge:latest"

        env {
          name  = "ENVIRONMENT"
          value = var.environment
        }

        env {
          name  = "PRIMARCH_URL"
          value = google_cloud_run_v2_service.primarch.uri
        }

        resources {
          limits = {
            cpu    = "4"
            memory = "8Gi"
          }
        }
      }

      timeout     = "3600s"
      max_retries = 1

      vpc_access {
        connector = google_vpc_access_connector.connector.id
        egress    = "PRIVATE_RANGES_ONLY"
      }
    }
  }

  depends_on = [google_project_service.apis]
}

# ─── Registry (Cloud Run Service — Strategy Marketplace) ─────

resource "google_storage_bucket" "registry_bundles" {
  name          = "${var.project_id}-registry-bundles"
  location      = var.region
  force_destroy = var.environment != "prod"

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      num_newer_versions = 30
    }
    action {
      type = "Delete"
    }
  }

  labels = local.labels
}

resource "google_secret_manager_secret" "registry_master_token" {
  secret_id = "registry-master-token-${var.environment}"
  replication { auto {} }
  depends_on = [google_project_service.apis]
}

resource "google_secret_manager_secret" "registry_signing_key" {
  secret_id = "registry-signing-key-${var.environment}"
  replication { auto {} }
  depends_on = [google_project_service.apis]
}

resource "google_cloud_run_v2_service" "registry" {
  name     = "registry-${var.environment}"
  location = var.region
  labels   = local.labels

  template {
    labels = local.labels

    containers {
      image = "${local.ar_repo}/registry:latest"

      ports {
        container_port = 8701
      }

      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }

      env {
        name  = "REGISTRY_PORT"
        value = "8701"
      }

      env {
        name  = "DATABASE_URL"
        value = "postgres://librarium:${random_password.db_password.result}@/${google_sql_database.librarium_db.name}?host=/cloudsql/${google_sql_database_instance.librarium.connection_name}&sslmode=disable"
      }

      env {
        name  = "REGISTRY_GCS_BUCKET"
        value = google_storage_bucket.registry_bundles.name
      }

      resources {
        limits = {
          cpu    = var.environment == "prod" ? "2" : "1"
          memory = var.environment == "prod" ? "1Gi" : "512Mi"
        }
      }

      startup_probe {
        http_get {
          path = "/healthz"
        }
        initial_delay_seconds = 5
        period_seconds        = 10
        failure_threshold     = 3
      }

      liveness_probe {
        http_get {
          path = "/healthz"
        }
        period_seconds = 30
      }
    }

    scaling {
      min_instance_count = var.environment == "prod" ? 1 : 0
      max_instance_count = var.environment == "prod" ? 5 : 2
    }

    vpc_access {
      connector = google_vpc_access_connector.connector.id
      egress    = "PRIVATE_RANGES_ONLY"
    }

    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = [google_sql_database_instance.librarium.connection_name]
      }
    }
  }

  depends_on = [google_project_service.apis]
}

# ─── Outputs ─────────────────────────────────────────────────

output "aurum_url" {
  description = "Aurum dashboard URL"
  value       = google_cloud_run_v2_service.aurum.uri
}

output "primarch_url" {
  description = "Primarch API URL"
  value       = google_cloud_run_v2_service.primarch.uri
}

output "db_connection_name" {
  description = "Cloud SQL connection name"
  value       = google_sql_database_instance.librarium.connection_name
}

output "registry_url" {
  description = "Registry API URL"
  value       = google_cloud_run_v2_service.registry.uri
}

output "registry_bundles_bucket" {
  description = "GCS bucket for strategy bundles"
  value       = google_storage_bucket.registry_bundles.name
}

output "environment" {
  value = var.environment
}
