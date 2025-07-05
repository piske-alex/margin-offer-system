package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/piske-alex/margin-offer-system/backfillers"
	"github.com/piske-alex/margin-offer-system/store"
	"github.com/piske-alex/margin-offer-system/pkg/logger"
	"github.com/piske-alex/margin-offer-system/pkg/metrics"
	"github.com/piske-alex/margin-offer-system/pkg/chain"
	"github.com/piske-alex/margin-offer-system/types"
)

func main() {
	var (
		storeAddr = flag.String("store-addr", "localhost:8080", "Store service gRPC address")
		chainEndpoint = flag.String("chain-endpoint", "wss://api.mainnet-beta.solana.com", "Blockchain endpoint")
		runOnce = flag.Bool("run-once", false, "Run backfill once and exit")
		source = flag.String("source", "chain", "Backfill source (chain, glacier, off-chain)")
		hours = flag.Int("hours", 24, "Hours to backfill from current time")
		help = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	log.Printf("Starting Backfiller Service")
	log.Printf("Store address: %s", *storeAddr)
	log.Printf("Chain endpoint: %s", *chainEndpoint)
	log.Printf("Run once: %v", *runOnce)

	// Create dependencies
	logger := logger.NewLogger("backfiller")
	metrics := metrics.NewMetricsCollector()
	chainClient := chain.NewSolanaClient(logger)

	// Create store client (in real implementation, this would be a gRPC client)
	memoryStore := store.NewMemoryStore()

	// Create backfiller
	backfiller := backfillers.NewBackfiller(memoryStore, chainClient, logger, metrics)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *runOnce {
		// Run backfill once and exit
		log.Printf("Running one-time backfill for last %d hours", *hours)
		
		endTime := time.Now().UTC()
		startTime := endTime.Add(-time.Duration(*hours) * time.Hour)
		
		req := &types.BackfillRequest{
			Source:    *source,
			StartTime: startTime,
			EndTime:   endTime,
			BatchSize: 1000,
		}
		
		if *source == "chain" {
			req.ChainParams = &types.ChainBackfillParams{
				StartBlock: 0, // Would calculate from timestamp
				EndBlock:   1000000,
				Programs:   []string{"marginfi", "leverage"},
				Network:    "mainnet",
			}
		}
		
		if err := backfiller.BackfillRange(ctx, req); err != nil {
			log.Fatalf("Backfill failed: %v", err)
		}
		
		log.Println("Backfill completed successfully")
		return
	}

	// Start continuous backfiller service
	if err := backfiller.Start(ctx); err != nil {
		log.Fatalf("Failed to start backfiller: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Backfiller service is running. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("Shutting down backfiller service...")
	if err := backfiller.Stop(); err != nil {
		log.Printf("Error stopping backfiller: %v", err)
	}

	log.Println("Backfiller service stopped")
}