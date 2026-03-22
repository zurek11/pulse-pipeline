---
name: terraform
description: 'REQUIRED when editing any .tf file or Kubernetes manifest. Invoke this skill before infrastructure changes.'
allowed-tools: Read, Write, Edit, Glob, Grep
---

# Terraform Patterns (GCP)

## When to Use

- Creating or editing `.tf` files in `infra/terraform/`
- Creating Kubernetes manifests in `infra/k8s/`
- Defining GCP resources

## File Structure

```
infra/terraform/
├── main.tf          # Provider config, backend
├── variables.tf     # All input variables with descriptions
├── outputs.tf       # Exported values
├── gke.tf           # GKE cluster + node pool
├── bigquery.tf      # Dataset + events table
├── gcs.tf           # Storage bucket
├── monitoring.tf    # Alert policies
└── networking.tf    # VPC + firewall
```

## Provider Template

```hcl
terraform {
  required_version = ">= 1.7"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
```

## GKE Template

```hcl
resource "google_container_cluster" "pulse_pipeline" {
  name     = "pulse-pipeline-cluster"
  location = var.region

  initial_node_count       = 1
  remove_default_node_pool = true

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }
}

resource "google_container_node_pool" "primary" {
  name       = "pulse-pipeline-nodes"
  location   = var.region
  cluster    = google_container_cluster.pulse_pipeline.name

  autoscaling {
    min_node_count = 1
    max_node_count = 3
  }

  node_config {
    machine_type = "e2-standard-2"
    disk_size_gb = 50

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
}
```

## Rules

1. NOT deployed — config only. `terraform validate` must pass, `terraform plan` is optional.
2. Region: `europe-west1` (closest to Bratislava)
3. All values parameterized via `variables.tf` — no hardcoded strings
4. Resource names prefixed with `pulse-pipeline-`
5. One resource type per file
6. Comments explaining WHY each resource exists (learning project)
7. BigQuery: partition by DATE(timestamp), cluster by customer_id + event_type
8. GKE: Workload Identity enabled, autoscaling 1-3 nodes
9. GCS: versioning enabled, lifecycle rule to delete after 90 days
10. K8s manifests: reference Docker images as `pulse-pipeline-api:latest` and `pulse-pipeline-consumer:latest`
