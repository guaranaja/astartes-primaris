# Astartes Primaris — Cloud Workstations
# Managed dev environment that runs inside the project VPC, with direct
# access to Cloud SQL, Memorystore, and other GCP-managed services.
#
# Resources:
#   1. Workstation cluster  — defines region + VPC attachment
#   2. Workstation config   — container image, machine type, disk, env vars
#   3. IAM binding          — who can use workstations
#
# The custom container image is built from infra/docker/workstation/
# and pushed to the same Artifact Registry repo as other service images.

# ─── Enable Workstations API ─────────────────────────────────

resource "google_project_service" "workstations_api" {
  service            = "workstations.googleapis.com"
  disable_on_destroy = false
}

# ─── Workstation Cluster ─────────────────────────────────────
# A cluster defines where workstations run and how they connect to the VPC.

resource "google_workstations_workstation_cluster" "dev" {
  provider               = google
  workstation_cluster_id = "astartes-dev-${var.environment}"
  network                = "projects/${var.project_id}/global/networks/default"
  subnetwork             = "projects/${var.project_id}/regions/${var.region}/subnetworks/default"
  location               = var.region

  labels = local.labels

  depends_on = [google_project_service.workstations_api]
}

# ─── Workstation Configuration ───────────────────────────────
# Template that defines machine shape, disk, container image, and env vars
# for every workstation created from it.

resource "google_workstations_workstation_config" "dev" {
  provider               = google
  workstation_config_id  = "astartes-config-${var.environment}"
  workstation_cluster_id = google_workstations_workstation_cluster.dev.workstation_cluster_id
  location               = var.region

  labels = local.labels

  # ── Machine shape ──────────────────────────────────────────
  host {
    gce_instance {
      machine_type = var.environment == "prod" ? "e2-standard-8" : "e2-standard-4"
      boot_disk_size_gb = 50

      # Use the same VPC connector pool so workstations can reach
      # Cloud SQL over private IP.
      service_account = google_service_account.workstation.email
    }
  }

  # ── Persistent home directory ──────────────────────────────
  persistent_directories {
    mount_path = "/home/user"
    gce_pd {
      size_gb         = 50
      fs_type         = "ext4"
      reclaim_policy  = "RETAIN"
    }
  }

  # ── Container image ────────────────────────────────────────
  container {
    image = "${local.ar_repo}/workstation:latest"

    env = {
      ENVIRONMENT           = var.environment
      GCP_PROJECT           = var.project_id
      GCP_REGION            = var.region
      CLOUD_SQL_CONNECTION  = google_sql_database_instance.librarium.connection_name
      DATABASE_URL          = "postgres://librarium@localhost:5432/librarium?sslmode=disable"
      VAULT_ADDR            = "http://localhost:8200"
      VAULT_TOKEN           = "dev-root-token"
    }
  }

  # ── Idle timeout ───────────────────────────────────────────
  # Workstations stop after 2h of inactivity to save cost.
  idle_timeout = "7200s"

  depends_on = [google_workstations_workstation_cluster.dev]
}

# ─── Service Account for Workstations ────────────────────────
# Grants workstation VMs access to Cloud SQL, Artifact Registry,
# Secret Manager, and other project resources.

resource "google_service_account" "workstation" {
  account_id   = "workstation-${var.environment}"
  display_name = "Astartes Workstation (${var.environment})"
}

# Cloud SQL client — lets the Cloud SQL Proxy connect
resource "google_project_iam_member" "workstation_sql" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.workstation.email}"
}

# Artifact Registry reader — lets the workstation pull images
resource "google_project_iam_member" "workstation_ar" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${google_service_account.workstation.email}"
}

# Secret Manager accessor — lets the workstation read secrets
resource "google_project_iam_member" "workstation_secrets" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.workstation.email}"
}

# Cloud Run developer — lets the workstation deploy services
resource "google_project_iam_member" "workstation_run" {
  project = var.project_id
  role    = "roles/run.developer"
  member  = "serviceAccount:${google_service_account.workstation.email}"
}

# ─── Outputs ─────────────────────────────────────────────────

output "workstation_cluster" {
  description = "Cloud Workstation cluster name"
  value       = google_workstations_workstation_cluster.dev.workstation_cluster_id
}

output "workstation_config" {
  description = "Cloud Workstation configuration name"
  value       = google_workstations_workstation_config.dev.workstation_config_id
}
