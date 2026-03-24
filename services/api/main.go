package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zurek11/pulse-pipeline/services/api/handlers"
	"github.com/zurek11/pulse-pipeline/services/api/kafka"
	"github.com/zurek11/pulse-pipeline/services/api/middleware"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}
	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		topic = "pulse.events.v1"
	}

	producer := kafka.NewProducer(strings.Split(brokers, ","), topic, logger)

	trackHandler := handlers.NewTrackHandler(producer, logger)
	batchHandler := handlers.NewBatchHandler(producer, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/api/v1/track", trackHandler)
	mux.Handle("/api/v1/track/batch", batchHandler)

	// Chain middleware: Recovery (outermost) → RequestID → handler
	var handler http.Handler = mux
	handler = middleware.RequestID(handler)
	handler = middleware.Recovery(logger)(handler)

	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("shutdown signal received", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	if err := producer.Close(); err != nil {
		logger.Error("kafka producer close error", "error", err)
	}

	logger.Info("server stopped gracefully")
}
