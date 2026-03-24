package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	consumerkafka "github.com/zurek11/pulse-pipeline/services/consumer/kafka"
	"github.com/zurek11/pulse-pipeline/services/consumer/metrics"
	"github.com/zurek11/pulse-pipeline/services/consumer/mongodb"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("consumer starting")

	// --- Config from environment ---
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}
	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		topic = "pulse.events.v1"
	}
	dlqTopic := os.Getenv("KAFKA_DLQ_TOPIC")
	if dlqTopic == "" {
		dlqTopic = "pulse.events.dlq.v1"
	}
	groupID := os.Getenv("KAFKA_GROUP_ID")
	if groupID == "" {
		groupID = "pulse-consumer-group"
	}
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	metricsAddr := os.Getenv("METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = ":8083"
	}
	batchSize := 100
	if v := os.Getenv("CONSUMER_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			batchSize = n
		}
	}
	flushInterval := time.Second
	if v := os.Getenv("CONSUMER_FLUSH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			flushInterval = d
		}
	}

	// --- Metrics ---
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	m := metrics.NewConsumer(reg)

	// --- MongoDB ---
	ctx := context.Background()
	mongoClient, collection, err := mongodb.Connect(ctx, mongoURI, "pulse", "events")
	if err != nil {
		logger.Error("failed to connect to mongodb", "error", err)
		os.Exit(1)
	}
	logger.Info("mongodb connected", "uri", mongoURI)

	writer := mongodb.NewBulkWriter(collection, logger, m)

	// --- Kafka consumer ---
	consumer := consumerkafka.NewConsumer(
		strings.Split(brokers, ","),
		topic,
		groupID,
		dlqTopic,
		writer,
		logger,
		m,
		batchSize,
		flushInterval,
	)
	logger.Info("kafka consumer created",
		"topic", topic,
		"group", groupID,
		"batch_size", batchSize,
		"flush_interval", flushInterval,
	)

	// --- Health + metrics server ---
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:         metricsAddr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("health server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("health server error", "error", err)
		}
	}()

	// --- Consumer loop ---
	consumerCtx, consumerCancel := context.WithCancel(context.Background())

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		if err := consumer.Start(consumerCtx); err != nil {
			logger.Error("consumer error", "error", err)
		}
	}()

	// --- Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("shutdown signal received", "signal", sig)

	// 1. Stop consumer loop (flushes remaining buffer internally).
	consumerCancel()
	<-consumerDone
	logger.Info("consumer stopped")

	// 2. Close Kafka connections.
	if err := consumer.Close(); err != nil {
		logger.Error("consumer close error", "error", err)
	}

	// 3. Stop health/metrics server.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("health server shutdown error", "error", err)
	}

	// 4. Disconnect MongoDB.
	if err := mongoClient.Disconnect(shutdownCtx); err != nil {
		logger.Error("mongodb disconnect error", "error", err)
	}

	logger.Info("consumer stopped gracefully")
}
