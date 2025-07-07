package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/piske-alex/margin-offer-system/backfillers"
	"github.com/piske-alex/margin-offer-system/pkg/chain"
	"github.com/piske-alex/margin-offer-system/pkg/logger"
	"github.com/piske-alex/margin-offer-system/pkg/metrics"
	"github.com/piske-alex/margin-offer-system/store"
	"github.com/piske-alex/margin-offer-system/types"
)

func main() {
	var (
		storeAddr     = flag.String("store-addr", "localhost:8080", "Store service gRPC address")
		chainEndpoint = flag.String("chain-endpoint", "wss://api.mainnet-beta.solana.com", "Blockchain endpoint")
		runOnce       = flag.Bool("run-once", false, "Run backfill once and exit")
		source        = flag.String("source", "chain", "Backfill source (chain, glacier, off-chain)")
		hours         = flag.Int("hours", 24, "Hours to backfill from current time")
		configFile    = flag.String("config", "", "Path to job configuration file")
		help          = flag.Bool("help", false, "Show help message")
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
	log.Printf("Config file: %s", *configFile)

	// Create dependencies
	logger := logger.NewLogger("backfiller")
	metrics := metrics.NewMetricsCollector()
	chainClient := chain.NewSolanaClient(logger)

	// Create gRPC store client
	storeClient, err := store.NewGRPCClient(*storeAddr, logger)
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
	log.Printf("Successfully connected to store service at %s", *storeAddr)

	// Create backfiller with gRPC store client
	backfiller := backfillers.NewBackfiller(storeClient, chainClient, logger, metrics)

	// Load job configurations
	if *configFile != "" {
		log.Printf("Loading jobs from config file: %s", *configFile)
		if err := backfiller.LoadJobsFromFile(*configFile); err != nil {
			log.Printf("Warning: Failed to load jobs from config file: %v", err)
		}
	}

	// Load jobs from environment variables
	if err := backfiller.LoadJobsFromEnv(); err != nil {
		log.Printf("Warning: Failed to load jobs from environment: %v", err)
	}

	// Register additional custom jobs programmatically
	if err := registerCustomJobs(backfiller); err != nil {
		log.Printf("Warning: Failed to register custom jobs: %v", err)
	}

	// Handle graceful shutdown
	ctx, cancel = context.WithCancel(context.Background())
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

// registerCustomJobs registers additional jobs programmatically
func registerCustomJobs(backfiller *backfillers.Backfiller) error {
	// Example: Register a weekly full sync job
	weeklySyncReq := &types.BackfillRequest{
		Source:    "chain",
		BatchSize: 2000,
		ChainParams: &types.ChainBackfillParams{
			Programs: []string{"marginfi", "leverage", "CRSeeBqjDnm3UPefJ9gxrtngTsnQRhEJiTA345Q83X3v"},
			Network:  "mainnet",
		},
	}

	if err := backfiller.RegisterScheduledJob("weekly-full-sync", "0 3 * * 0", weeklySyncReq); err != nil {
		return fmt.Errorf("failed to register weekly sync job: %w", err)
	}

	// Example: Register an hourly quick sync job
	hourlySyncReq := &types.BackfillRequest{
		Source:    "chain",
		BatchSize: 500,
		ChainParams: &types.ChainBackfillParams{
			Programs: []string{"leverage"},
			Network:  "mainnet",
		},
	}

	if err := backfiller.RegisterScheduledJob("hourly-quick-sync", "0 * * * *", hourlySyncReq); err != nil {
		return fmt.Errorf("failed to register hourly sync job: %w", err)
	}

	return nil
}
