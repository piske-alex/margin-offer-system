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
	runBackfillerJobsExample()
}

func runBackfillerJobsExample() {
	// Create dependencies
	logger := logger.NewLogger("backfiller-jobs-example")
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

	// Method 1: Load jobs from configuration file
	log.Println("Loading jobs from config file...")
	if err := backfiller.LoadJobsFromFile("config/backfiller-jobs.json"); err != nil {
		log.Printf("Warning: Failed to load jobs from config file: %v", err)
	}

	// Method 2: Load jobs from environment variables
	log.Println("Loading jobs from environment...")
	if err := backfiller.LoadJobsFromEnv(); err != nil {
		log.Printf("Warning: Failed to load jobs from environment: %v", err)
	}

	// Method 3: Register custom jobs programmatically
	log.Println("Registering custom jobs programmatically...")
	if err := registerCustomJobs(backfiller); err != nil {
		log.Printf("Warning: Failed to register custom jobs: %v", err)
	}

	// Method 4: Register scheduled jobs with cron expressions
	log.Println("Registering scheduled jobs...")
	if err := registerScheduledJobs(backfiller); err != nil {
		log.Printf("Warning: Failed to register scheduled jobs: %v", err)
	}

	// Start the backfiller service
	log.Println("Starting backfiller service...")
	serviceCtx, serviceCancel := context.WithCancel(context.Background())
	defer serviceCancel()

	if err := backfiller.Start(serviceCtx); err != nil {
		log.Fatalf("Failed to start backfiller: %v", err)
	}

	// Let it run for a bit to see the jobs in action
	log.Println("Backfiller is running with all jobs registered. Press Ctrl+C to stop.")
	time.Sleep(60 * time.Second)

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

// registerCustomJobs demonstrates registering custom jobs programmatically
func registerCustomJobs(backfiller *backfillers.Backfiller) error {
	// Example 1: Register a custom chain backfill job
	customChainJob := &types.BackfillRequest{
		Source:    "chain",
		BatchSize: 1500,
		ChainParams: &types.ChainBackfillParams{
			Programs: []string{"marginfi"},
			Network:  "mainnet",
		},
	}

	if err := backfiller.RegisterCustomJob(customChainJob); err != nil {
		return err
	}

	// Example 2: Register a custom off-chain job
	customOffChainJob := &types.BackfillRequest{
		Source:    "off-chain",
		BatchSize: 800,
		OffChainParams: &types.OffChainBackfillParams{
			APIEndpoint: "https://api.marginfi.com/v1/offers",
			Headers: map[string]string{
				"Authorization": "Bearer custom-token",
				"User-Agent":    "margin-offer-backfiller/1.0",
			},
		},
	}

	if err := backfiller.RegisterCustomJob(customOffChainJob); err != nil {
		return err
	}

	log.Println("Registered 2 custom jobs")
	return nil
}

// registerScheduledJobs demonstrates registering scheduled jobs with cron expressions
func registerScheduledJobs(backfiller *backfillers.Backfiller) error {
	// Example 1: Every 15 minutes quick sync
	quickSyncJob := &types.BackfillRequest{
		Source:    "chain",
		BatchSize: 300,
		ChainParams: &types.ChainBackfillParams{
			Programs: []string{"leverage"},
			Network:  "mainnet",
		},
	}

	if err := backfiller.RegisterScheduledJob("quick-sync", "*/15 * * * *", quickSyncJob); err != nil {
		return err
	}

	// Example 2: Every 4 hours medium sync
	mediumSyncJob := &types.BackfillRequest{
		Source:    "chain",
		BatchSize: 1000,
		ChainParams: &types.ChainBackfillParams{
			Programs: []string{"marginfi", "leverage"},
			Network:  "mainnet",
		},
	}

	if err := backfiller.RegisterScheduledJob("medium-sync", "0 */4 * * *", mediumSyncJob); err != nil {
		return err
	}

	// Example 3: Every Sunday at 1 AM full sync
	fullSyncJob := &types.BackfillRequest{
		Source:    "chain",
		BatchSize: 2500,
		ChainParams: &types.ChainBackfillParams{
			Programs: []string{"marginfi", "leverage", "CRSeeBqjDnm3UPefJ9gxrtngTsnQRhEJiTA345Q83X3v"},
			Network:  "mainnet",
		},
	}

	if err := backfiller.RegisterScheduledJob("full-sync", "0 1 * * 0", fullSyncJob); err != nil {
		return err
	}

	// Example 4: Glacier backup every day at 3 AM
	glacierJob := &types.BackfillRequest{
		Source:    "glacier",
		BatchSize: 1000,
		GlacierParams: &types.GlacierBackfillParams{
			VaultName: "margin-offers-backup",
			Retrieval: true,
		},
	}

	if err := backfiller.RegisterScheduledJob("glacier-backup", "0 3 * * *", glacierJob); err != nil {
		return err
	}

	log.Println("Registered 4 scheduled jobs")
	return nil
}
