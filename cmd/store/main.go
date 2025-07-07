package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/piske-alex/margin-offer-system/store"
	"github.com/piske-alex/margin-offer-system/types"
)

// SimpleLogger implements the types.Logger interface
// (prints to standard log)
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, fields ...interface{})               { log.Println("DEBUG:", msg, fields) }
func (l *SimpleLogger) Info(msg string, fields ...interface{})                { log.Println("INFO:", msg, fields) }
func (l *SimpleLogger) Warn(msg string, fields ...interface{})                { log.Println("WARN:", msg, fields) }
func (l *SimpleLogger) Error(msg string, fields ...interface{})               { log.Println("ERROR:", msg, fields) }
func (l *SimpleLogger) Fatal(msg string, fields ...interface{})               { log.Println("FATAL:", msg, fields) }
func (l *SimpleLogger) WithField(key string, value interface{}) types.Logger  { return l }
func (l *SimpleLogger) WithFields(fields map[string]interface{}) types.Logger { return l }

// SimpleMetrics implements the types.MetricsCollector interface (no-op)
type SimpleMetrics struct{}

func (m *SimpleMetrics) IncCounter(name string, tags map[string]string)                             {}
func (m *SimpleMetrics) AddCounter(name string, value float64, tags map[string]string)              {}
func (m *SimpleMetrics) SetGauge(name string, value float64, tags map[string]string)                {}
func (m *SimpleMetrics) RecordDuration(name string, duration time.Duration, tags map[string]string) {}
func (m *SimpleMetrics) RecordValue(name string, value float64, tags map[string]string)             {}
func (m *SimpleMetrics) SetHealthy(service string)                                                  {}
func (m *SimpleMetrics) SetUnhealthy(service string, reason string)                                 {}

func main() {
	var (
		addr = flag.String("addr", ":8080", "gRPC server address")
		help = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	log.Printf("Starting Margin Offer Store Service on %s", *addr)

	// Create in-memory store with logger and metrics
	logger := &SimpleLogger{}
	metrics := &SimpleMetrics{}
	memoryStore := store.NewMemoryStore(logger, metrics)

	// Create gRPC server
	grpcServer := store.NewGRPCServer(memoryStore)

	// Start server in goroutine
	go func() {
		if err := grpcServer.Start(*addr); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Store service is running. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("Shutting down store service...")
	grpcServer.Stop()

	if err := memoryStore.Close(); err != nil {
		log.Printf("Error closing store: %v", err)
	}

	log.Println("Store service stopped")
}
