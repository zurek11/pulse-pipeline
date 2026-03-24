# bigquery.tf — BigQuery dataset and events table
#
# Why BigQuery?
#   MongoDB is the operational store (fast writes, real-time reads). BigQuery is
#   the analytical store — it handles full table scans over billions of rows in
#   seconds using columnar storage and massive parallel query execution.
#
# Data flow: MongoDB → scheduled export job → Cloud Storage → BigQuery load job
#   (The export job itself is a future TODO — Terraform just provisions the table.)
#
# Partitioning + clustering:
#   Partitioning by DATE(timestamp) means queries filtered to a date range only
#   scan the relevant partitions — massive cost and performance win.
#   Clustering by customer_id + event_type means rows are physically sorted on
#   disk, so queries like "all page_views for customer X this week" skip most data.

resource "google_bigquery_dataset" "pulse_events" {
  dataset_id    = var.bigquery_dataset_id
  friendly_name = "Pulse Events"
  description   = "Long-term analytics storage for Pulse Pipeline tracking events, exported from MongoDB."
  location      = var.region

  # Keep data for 365 days; individual table TTLs can override this.
  default_table_expiration_ms = null # No expiration — analytics data is kept indefinitely.

  labels = {
    environment = var.environment
    app         = "pulse-pipeline"
  }
}

resource "google_bigquery_table" "events" {
  dataset_id = google_bigquery_dataset.pulse_events.dataset_id
  table_id   = "events"
  description = "Tracking events exported from MongoDB. Mirrors the pulse.events collection schema."

  # Partition by event date to minimize bytes scanned on time-range queries.
  time_partitioning {
    type  = "DAY"
    field = "timestamp"
  }

  # Cluster by the most common query dimensions — reduces bytes scanned further.
  clustering = ["customer_id", "event_type"]

  # Schema mirrors the MongoDB events collection (see docs/PROJECT_SPEC.md).
  schema = jsonencode([
    {
      name        = "event_id"
      type        = "STRING"
      mode        = "REQUIRED"
      description = "Unique event identifier. Deduplication key."
    },
    {
      name        = "customer_id"
      type        = "STRING"
      mode        = "REQUIRED"
      description = "Customer who generated the event. Partition key in Kafka."
    },
    {
      name        = "event_type"
      type        = "STRING"
      mode        = "REQUIRED"
      description = "Event category: page_view, click, purchase, add_to_cart, search, custom."
    },
    {
      name        = "timestamp"
      type        = "TIMESTAMP"
      mode        = "REQUIRED"
      description = "When the event occurred (client-provided or server-assigned)."
    },
    {
      name        = "properties"
      type        = "JSON"
      mode        = "NULLABLE"
      description = "Arbitrary event-specific key-value pairs (e.g. page URL, price)."
    },
    {
      name        = "context"
      type        = "JSON"
      mode        = "NULLABLE"
      description = "Device and session metadata (device type, session_id, IP, user agent)."
    },
    {
      name        = "received_at"
      type        = "TIMESTAMP"
      mode        = "NULLABLE"
      description = "When the API server received the event."
    },
    {
      name        = "processed_at"
      type        = "TIMESTAMP"
      mode        = "NULLABLE"
      description = "When the consumer wrote the event to MongoDB."
    },
  ])

  labels = {
    environment = var.environment
    app         = "pulse-pipeline"
  }
}
