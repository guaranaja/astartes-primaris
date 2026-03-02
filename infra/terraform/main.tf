# Astartes Primaris — Infrastructure
# Terraform configuration for cloud deployment.
#
# This is the skeleton — fill in provider-specific details based on
# your cloud target (AWS ECS/EKS, GCP GKE, DigitalOcean, etc.)

terraform {
  required_version = ">= 1.6"

  required_providers {
    # Uncomment your cloud provider:
    # aws = { source = "hashicorp/aws", version = "~> 5.0" }
    # google = { source = "hashicorp/google", version = "~> 5.0" }
    # digitalocean = { source = "digitalocean/digitalocean", version = "~> 2.0" }
  }
}

# ─── Variables ───────────────────────────────────────────────

variable "environment" {
  description = "Fortress Monastery environment (prod, staging, dev)"
  type        = string
  default     = "dev"
}

variable "region" {
  description = "Cloud region for deployment"
  type        = string
  default     = "us-east-1"
}

# ─── Locals ──────────────────────────────────────────────────

locals {
  project_name = "astartes-primaris"
  common_tags = {
    Project     = "astartes-primaris"
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}

# ─── Outputs ─────────────────────────────────────────────────

output "environment" {
  value = var.environment
}
