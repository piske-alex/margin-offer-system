package main

import (
	"context"
	"log"
	"time"

	"github.com/piske-alex/margin-offer-system/backfillers"
	"github.com/piske-alex/margin-offer-system/pkg/chain"
	"github.com/piske-alex/margin-offer-system/pkg/logger"
	"github.com/piske-alex/margin-offer-system/pkg/metrics"
	"github.com/piske-alex/margin-offer-system/store"
	"github.com/piske-alex/margin-offer-system/types"
)

func main() {
	// Create dependencies
	logger := logger.NewLogger("backfiller-example")
	metrics := metrics.NewMetricsCollector()
	chainClient := chain.NewSolanaClient(logger)

	// Create gRPC store client
	storeClient, err := store.NewGRPCClient("localhost:8080", logger)
	if err != nil {
		log.Fatalf("Failed to create store client: %v", err)
	}
	defer storeClient.Close()

	// Test connection to store service
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := storeClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to connect to store service: %v", err)
	}
	log.Printf("Successfully connected to store service")

	// Create backfiller with gRPC store client
	backfiller := backfillers.NewBackfiller(storeClient, chainClient, logger, metrics)

	// Example 1: Run a one-time backfill
	log.Println("Running one-time backfill...")

	endTime := time.Now().UTC()
	startTime := endTime.Add(-24 * time.Hour) // Last 24 hours

	req := &types.BackfillRequest{
		Source:    "chain",
		StartTime: startTime,
		EndTime:   endTime,
		BatchSize: 1000,
		ChainParams: &types.ChainBackfillParams{
			Programs: []string{"marginfi", "leverage"},
			Network:  "mainnet",
		},
	}

	if err := backfiller.BackfillRange(ctx, req); err != nil {
		log.Printf("Backfill failed: %v", err)
	} else {
		log.Println("Backfill completed successfully")
	}

	// Example 2: Start continuous backfiller service
	log.Println("Starting continuous backfiller service...")

	serviceCtx, serviceCancel := context.WithCancel(context.Background())
	defer serviceCancel()

	if err := backfiller.Start(serviceCtx); err != nil {
		log.Fatalf("Failed to start backfiller: %v", err)
	}

	// Let it run for a bit
	time.Sleep(30 * time.Second)

	// Check status
	status := backfiller.GetStatus()
	log.Printf("Backfiller status: %+v", status)

	// Stop the service
	log.Println("Stopping backfiller service...")
	if err := backfiller.Stop(); err != nil {
		log.Printf("Error stopping backfiller: %v", err)
	}

	log.Println("Example completed")
}
