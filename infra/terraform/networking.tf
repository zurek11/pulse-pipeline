# networking.tf — VPC network and firewall rules
#
# Why a dedicated VPC?
#   GKE clusters should not run in the default VPC. A dedicated network gives
#   full control over IP ranges, firewall rules, and peering — important for
#   production security and predictable CIDR allocation.
#
# Secondary ranges:
#   GKE requires two secondary IP ranges on the subnet:
#     - pods:     one IP per pod (each node gets a /24 block)
#     - services: one IP per ClusterIP service

resource "google_compute_network" "pulse_pipeline" {
  name                    = "pulse-pipeline-vpc"
  auto_create_subnetworks = false # We define subnets explicitly for full control.

  description = "Dedicated VPC for the Pulse Pipeline GKE cluster."
}

resource "google_compute_subnetwork" "gke" {
  name          = "pulse-pipeline-gke-subnet"
  network       = google_compute_network.pulse_pipeline.id
  region        = var.region
  ip_cidr_range = "10.0.0.0/20" # 4096 node IPs — more than enough for 1-3 nodes.

  # Secondary ranges required by GKE alias IP mode (VPC-native clusters).
  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = "10.1.0.0/16" # 65536 pod IPs (256 nodes × 256 pods each).
  }

  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = "10.2.0.0/20" # 4096 service IPs — sufficient for ClusterIP services.
  }

  description = "Subnet for GKE nodes, pods, and services."
}

# Allow nodes within the cluster to communicate with each other.
resource "google_compute_firewall" "allow_internal" {
  name    = "pulse-pipeline-allow-internal"
  network = google_compute_network.pulse_pipeline.name

  allow {
    protocol = "tcp"
  }

  allow {
    protocol = "udp"
  }

  allow {
    protocol = "icmp"
  }

  # Only traffic originating from within the VPC subnet is allowed.
  source_ranges = [google_compute_subnetwork.gke.ip_cidr_range]
  description   = "Allow all internal traffic between GKE nodes."
}

# Allow GKE master to reach nodes for health checks and webhook calls.
resource "google_compute_firewall" "allow_master_to_nodes" {
  name    = "pulse-pipeline-allow-master"
  network = google_compute_network.pulse_pipeline.name

  allow {
    protocol = "tcp"
    ports    = ["443", "8080", "8083", "10250"] # HTTPS, API, consumer metrics, kubelet.
  }

  # GKE master CIDR — Terraform resolves this from the cluster resource.
  source_ranges = ["172.16.0.0/28"]
  description   = "Allow GKE control plane to communicate with worker nodes."
}

# Cloud NAT — lets nodes pull images and reach external APIs without public IPs.
resource "google_compute_router" "pulse_pipeline" {
  name    = "pulse-pipeline-router"
  network = google_compute_network.pulse_pipeline.id
  region  = var.region
}

resource "google_compute_router_nat" "pulse_pipeline" {
  name                               = "pulse-pipeline-nat"
  router                             = google_compute_router.pulse_pipeline.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  # Log only translation errors to keep costs low.
  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}
