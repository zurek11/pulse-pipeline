# Pulse Pipeline — GCP Infrastructure
#
# This file is CONFIG ONLY — it is not deployed.
# Run `terraform validate` to verify syntax.
# Run `terraform plan` to preview resources (requires GCP credentials).
#
# Purpose: demonstrate how this local pipeline would look on GCP:
#   - GKE runs the API + Consumer services as pods
#   - BigQuery stores events for long-term analytics
#   - Cloud Storage holds event exports and backups
#   - Cloud Monitoring fires alerts on SLO breaches
#   - VPC isolates the cluster from the public internet

terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }

  # Backend configuration would be added here for real deployments.
  # Keeping it local for this learning project.
  # backend "gcs" {
  #   bucket = "pulse-pipeline-tfstate"
  #   prefix = "terraform/state"
  # }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
