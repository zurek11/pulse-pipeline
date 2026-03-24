# monitoring.tf — Cloud Monitoring alert policies
#
# Why Cloud Monitoring alerts?
#   Prometheus + Grafana handle real-time dashboards, but Cloud Monitoring
#   provides managed alerting with notification channels (email, PagerDuty, Slack)
#   and SLO burn-rate alerts. For production, you'd want both.
#
# Alert design:
#   Three alerts cover the most critical failure modes for a pipeline:
#   1. High API error rate     → users are seeing 5xx errors
#   2. High consumer lag       → events are queuing up, pipeline is slow
#   3. API service uptime      → API is completely down

locals {
  # notification_channels is a list attribute — use an empty list when no
  # channel is configured so the alert still creates without sending notifications.
  notification_channels = var.alert_notification_channel != "" ? [var.alert_notification_channel] : []
}

# Alert 1: API error rate too high
# Fires when more than 5% of API requests return 5xx over a 5-minute window.
resource "google_monitoring_alert_policy" "api_error_rate" {
  display_name          = "pulse-pipeline — API Error Rate High"
  combiner              = "OR"
  notification_channels = local.notification_channels

  conditions {
    display_name = "API 5xx error rate > ${var.api_error_rate_threshold * 100}%"

    condition_threshold {
      # pulse_api_requests_total{status=~"5.."} / pulse_api_requests_total
      # Approximated via Cloud Monitoring's Prometheus metric ingestion.
      filter          = "metric.type=\"prometheus.googleapis.com/pulse_api_requests_total/counter\" AND metric.labels.status=monitoring.regex.full_match(\"5..\")"
      duration        = "300s" # Alert fires if condition holds for 5 minutes.
      comparison      = "COMPARISON_GT"
      threshold_value = var.api_error_rate_threshold

      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_RATE"
      }
    }
  }

  documentation {
    content   = "API error rate exceeded ${var.api_error_rate_threshold * 100}%. Check API service logs and Kafka connectivity. Dashboard: http://localhost:3000"
    mime_type = "text/markdown"
  }

  alert_strategy {
    auto_close = "1800s" # Auto-close the incident after 30 minutes of recovery.
  }
}

# Alert 2: Consumer lag too high
# Fires when Kafka consumer lag exceeds the threshold — events are piling up.
resource "google_monitoring_alert_policy" "consumer_lag" {
  display_name          = "pulse-pipeline — Consumer Lag High"
  combiner              = "OR"
  notification_channels = local.notification_channels

  conditions {
    display_name = "Consumer lag > ${var.consumer_lag_threshold} messages"

    condition_threshold {
      filter          = "metric.type=\"prometheus.googleapis.com/pulse_consumer_kafka_lag/gauge\""
      duration        = "300s"
      comparison      = "COMPARISON_GT"
      threshold_value = var.consumer_lag_threshold

      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_MEAN"
      }
    }
  }

  documentation {
    content   = "Kafka consumer lag exceeded ${var.consumer_lag_threshold} messages. Consumer may be down or MongoDB writes are too slow. Check consumer service logs."
    mime_type = "text/markdown"
  }

  alert_strategy {
    auto_close = "1800s"
  }
}

# Alert 3: API uptime check
# GCP sends synthetic requests to the API's /health endpoint every minute.
# Fires if health checks fail from multiple regions.
resource "google_monitoring_uptime_check_config" "api_health" {
  display_name = "pulse-pipeline — API Health Check"
  timeout      = "10s"
  period       = "60s"

  http_check {
    path         = "/health"
    port         = 8080
    use_ssl      = false
    validate_ssl = false
  }

  # In production the host would be the GKE LoadBalancer external IP or domain.
  # Placeholder used here since infrastructure is not deployed.
  monitored_resource {
    type = "uptime_url"
    labels = {
      project_id = var.project_id
      host       = "api.pulse-pipeline.example.com"
    }
  }
}

resource "google_monitoring_alert_policy" "api_uptime" {
  display_name          = "pulse-pipeline — API Down"
  combiner              = "OR"
  notification_channels = local.notification_channels

  conditions {
    display_name = "API /health check failing"

    condition_threshold {
      filter          = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND metric.labels.check_id=\"${google_monitoring_uptime_check_config.api_health.uptime_check_id}\""
      duration        = "120s" # Fire after 2 consecutive failures.
      comparison      = "COMPARISON_LT"
      threshold_value = 1

      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_NEXT_OLDER"
        cross_series_reducer = "REDUCE_COUNT_TRUE"
      }
    }
  }

  documentation {
    content   = "API /health endpoint is failing. Service may be down or unreachable. Check GKE pod status: `kubectl get pods -n pulse-pipeline`"
    mime_type = "text/markdown"
  }

  alert_strategy {
    auto_close = "1800s"
  }
}
