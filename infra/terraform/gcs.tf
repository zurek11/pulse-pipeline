# gcs.tf — Cloud Storage bucket for event exports and backups
#
# Why Cloud Storage?
#   Acts as the staging layer between MongoDB and BigQuery. A scheduled export
#   job (future work) dumps MongoDB collections as newline-delimited JSON to GCS,
#   then a BigQuery load job ingests them. This decouples export cadence from
#   query availability.
#
#   Also used for general backups: Terraform state, Grafana snapshots, etc.
#
# Versioning + lifecycle:
#   Versioning keeps a history of overwritten objects — useful for debugging
#   corrupt exports. The lifecycle rule deletes objects older than 90 days to
#   prevent unbounded storage costs.

resource "google_storage_bucket" "exports" {
  name          = "${var.gcs_export_bucket_name}-${var.project_id}"
  location      = var.region
  force_destroy = false # Prevent accidental data loss via `terraform destroy`.

  # Versioning retains previous versions of objects on overwrite or delete.
  versioning {
    enabled = true
  }

  # Automatically delete objects (and non-current versions) after the retention window.
  lifecycle_rule {
    condition {
      age = var.gcs_export_retention_days
    }
    action {
      type = "Delete"
    }
  }

  # Also delete old non-current versions after 30 days to reclaim storage.
  lifecycle_rule {
    condition {
      age                = 30
      with_state         = "ARCHIVED"
    }
    action {
      type = "Delete"
    }
  }

  # Uniform bucket-level access: disables per-object ACLs, enforces IAM-only access.
  uniform_bucket_level_access = true

  labels = {
    environment = var.environment
    app         = "pulse-pipeline"
  }
}

# Organise exports by date prefix. Objects will land at:
#   gs://pulse-pipeline-exports-<project>/events/YYYY-MM-DD/events.ndjson
# The export job (future) writes here; BigQuery load jobs read from here.
# This resource documents the intended structure as a commented convention —
# GCS doesn't have real "folders", but the prefix convention is important.
