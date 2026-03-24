# gke.tf — GKE cluster and node pool
#
# Why GKE?
#   GKE (Google Kubernetes Engine) manages the Kubernetes control plane so we
#   only need to think about node pools and workloads. It integrates natively
#   with Workload Identity, Cloud Monitoring, and Artifact Registry.
#
# Key decisions:
#   - remove_default_node_pool = true: the default pool is created with 1 node
#     just to satisfy the API, then immediately deleted. Our separately defined
#     node pool gives full control over autoscaling and machine type.
#   - Workload Identity: pods authenticate to GCP APIs using a Kubernetes
#     service account bound to a GCP service account — no JSON key files.
#   - VPC-native (alias IP): required for better network performance and
#     compatibility with Network Policies and Cloud NAT.

resource "google_container_cluster" "pulse_pipeline" {
  name     = "pulse-pipeline-cluster"
  location = var.region # Regional cluster — control plane spans 3 zones for HA.

  # Delete the default node pool immediately; we manage nodes separately below.
  remove_default_node_pool = true
  initial_node_count       = 1

  network    = google_compute_network.pulse_pipeline.name
  subnetwork = google_compute_subnetwork.gke.name

  # VPC-native mode: pods get IPs from the secondary range, enabling direct
  # pod-to-pod routing without NAT masquerading.
  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  # Workload Identity allows pods to impersonate GCP service accounts without
  # mounting key files — the recommended approach for GKE workloads.
  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  # Enable Cloud Logging and Cloud Monitoring integrations.
  logging_service    = "logging.googleapis.com/kubernetes"
  monitoring_service = "monitoring.googleapis.com/kubernetes"

  # Automatically upgrade nodes to stay on a supported GKE minor version.
  release_channel {
    channel = "REGULAR"
  }

  description = "GKE cluster running the Pulse Pipeline API and Consumer services."
}

resource "google_container_node_pool" "primary" {
  name     = "pulse-pipeline-nodes"
  location = var.region
  cluster  = google_container_cluster.pulse_pipeline.name

  # Autoscaling: scale from 1 node (idle) up to 3 nodes (under load).
  autoscaling {
    min_node_count = var.gke_min_nodes
    max_node_count = var.gke_max_nodes
  }

  # Automatically repair unhealthy nodes and keep them on the latest patch.
  management {
    auto_repair  = true
    auto_upgrade = true
  }

  node_config {
    machine_type = var.gke_node_machine_type # e2-standard-2: 2 vCPU, 8 GB RAM
    disk_size_gb = var.gke_node_disk_size_gb

    # cloud-platform scope grants access to all GCP APIs; IAM controls what
    # the node's service account (and pods via Workload Identity) can actually do.
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]

    # Apply environment label to nodes for cost attribution in billing reports.
    labels = {
      environment = var.environment
      app         = "pulse-pipeline"
    }

    # Workload Identity must also be enabled at the node pool level.
    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }
}
