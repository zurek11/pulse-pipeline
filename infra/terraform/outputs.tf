# Outputs expose key resource attributes after `terraform apply`.
# Useful for wiring CI/CD pipelines and Kubernetes configuration.

output "gke_cluster_name" {
  description = "Name of the GKE cluster."
  value       = google_container_cluster.pulse_pipeline.name
}

output "gke_cluster_endpoint" {
  description = "HTTPS endpoint for the GKE master. Used to configure kubectl."
  value       = google_container_cluster.pulse_pipeline.endpoint
  sensitive   = true
}

output "gke_cluster_ca_certificate" {
  description = "Base64-encoded public certificate authority for the GKE cluster."
  value       = google_container_cluster.pulse_pipeline.master_auth[0].cluster_ca_certificate
  sensitive   = true
}

output "bigquery_dataset_id" {
  description = "Fully-qualified BigQuery dataset ID."
  value       = "${var.project_id}:${google_bigquery_dataset.pulse_events.dataset_id}"
}

output "bigquery_table_id" {
  description = "Fully-qualified BigQuery table ID for the events table."
  value       = "${var.project_id}:${google_bigquery_dataset.pulse_events.dataset_id}.${google_bigquery_table.events.table_id}"
}

output "gcs_export_bucket_url" {
  description = "gs:// URL of the Cloud Storage export bucket."
  value       = "gs://${google_storage_bucket.exports.name}"
}

output "vpc_network_name" {
  description = "Name of the VPC network used by the GKE cluster."
  value       = google_compute_network.pulse_pipeline.name
}

output "vpc_subnet_name" {
  description = "Name of the subnet used by the GKE cluster."
  value       = google_compute_subnetwork.gke.name
}
