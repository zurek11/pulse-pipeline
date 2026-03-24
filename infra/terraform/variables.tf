variable "project_id" {
  description = "GCP project ID where all resources will be created."
  type        = string
}

variable "region" {
  description = "GCP region for all resources. europe-west1 (Belgium) is closest to Bratislava."
  type        = string
  default     = "europe-west1"
}

variable "environment" {
  description = "Deployment environment label (e.g. dev, staging, prod)."
  type        = string
  default     = "prod"
}

variable "gke_node_machine_type" {
  description = "GCE machine type for GKE nodes. e2-standard-2 = 2 vCPU, 8 GB RAM."
  type        = string
  default     = "e2-standard-2"
}

variable "gke_node_disk_size_gb" {
  description = "Boot disk size in GB per GKE node."
  type        = number
  default     = 50
}

variable "gke_min_nodes" {
  description = "Minimum number of nodes in the GKE node pool (autoscaling lower bound)."
  type        = number
  default     = 1
}

variable "gke_max_nodes" {
  description = "Maximum number of nodes in the GKE node pool (autoscaling upper bound)."
  type        = number
  default     = 3
}

variable "bigquery_dataset_id" {
  description = "BigQuery dataset ID for long-term event analytics."
  type        = string
  default     = "pulse_events"
}

variable "gcs_export_bucket_name" {
  description = "Cloud Storage bucket name for event exports and backups. Must be globally unique."
  type        = string
  default     = "pulse-pipeline-exports"
}

variable "gcs_export_retention_days" {
  description = "Number of days before exported objects are automatically deleted."
  type        = number
  default     = 90
}

variable "alert_notification_channel" {
  description = "Cloud Monitoring notification channel ID for alert policies (e.g. email or PagerDuty)."
  type        = string
  default     = ""
}

variable "api_error_rate_threshold" {
  description = "API error rate (fraction 0–1) above which the alert fires."
  type        = number
  default     = 0.05 # 5%
}

variable "consumer_lag_threshold" {
  description = "Kafka consumer lag (number of messages) above which the alert fires."
  type        = number
  default     = 10000
}
