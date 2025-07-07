package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/piske-alex/margin-offer-system/pkg/chain"
	"github.com/piske-alex/margin-offer-system/pkg/logger"
	"github.com/piske-alex/margin-offer-system/pkg/metrics"
	etl "github.com/piske-alex/margin-offer-system/realtime-etl"
	"github.com/piske-alex/margin-offer-system/store"
)

func main() {
	var (
		storeAddr     = flag.String("store-addr", "localhost:8080", "Store service gRPC address")
		chainEndpoint = flag.String("chain-endpoint", "wss://api.mainnet-beta.solana.com", "Blockchain endpoint")
		help          = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	log.Printf("Starting Real-time ETL Pipeline")
	log.Printf("Store address: %s", *storeAddr)
	log.Printf("Chain endpoint: %s", *chainEndpoint)

	// Create dependencies
	logger := logger.NewLogger("etl")
	metrics := metrics.NewMetricsCollector()
	chainClient := chain.NewSolanaClient(logger)

	// Create store client (in real implementation, this would be a gRPC client)
	memoryStore := store.NewMemoryStore(logger, metrics)

	// Create ETL processor
	processor := etl.NewProcessor(memoryStore, chainClient, logger, metrics)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start processor
	if err := processor.Start(ctx); err != nil {
		log.Fatalf("Failed to start ETL processor: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("ETL pipeline is running. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("Shutting down ETL pipeline...")
	if err := processor.Stop(); err != nil {
		log.Printf("Error stopping processor: %v", err)
	}

	log.Println("ETL pipeline stopped")
}
