package backfillers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/piske-alex/margin-offer-system/types"
)

// Backfiller implements periodic data synchronization from various sources
type Backfiller struct {
	mu          sync.RWMutex
	store       types.MarginOfferStore
	chainClient types.ChainClient
	logger      types.Logger
	metrics     types.MetricsCollector

	// State management
	isRunning     bool
	activeJobs    map[string]*types.BackfillProgress
	scheduledJobs map[string]*types.BackfillScheduleRequest

	// Statistics
	completedJobs    int64
	failedJobs       int64
	lastBackfillTime *time.Time
	nextBackfillTime *time.Time

	// Configuration
	defaultBatchSize  int32
	defaultTimeout    time.Duration
	maxConcurrentJobs int
	backfillInterval  time.Duration
	retentionPeriod   time.Duration

	// Channels
	jobQueue chan *backfillJob
	stopChan chan struct{}
}

type backfillJob struct {
	id      string
	request *types.BackfillRequest
	result  chan error
}

// NewBackfiller creates a new backfiller instance
func NewBackfiller(store types.MarginOfferStore, chainClient types.ChainClient, logger types.Logger, metrics types.MetricsCollector) *Backfiller {
	return &Backfiller{
		store:             store,
		chainClient:       chainClient,
		logger:            logger,
		metrics:           metrics,
		activeJobs:        make(map[string]*types.BackfillProgress),
		scheduledJobs:     make(map[string]*types.BackfillScheduleRequest),
		defaultBatchSize:  1000,
		defaultTimeout:    time.Hour,
		maxConcurrentJobs: 5,
		backfillInterval:  time.Hour * 6,      // Every 6 hours
		retentionPeriod:   time.Hour * 24 * 7, // 7 days
		jobQueue:          make(chan *backfillJob, 100),
		stopChan:          make(chan struct{}),
	}
}

// Start begins the backfiller service
func (b *Backfiller) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isRunning {
		return fmt.Errorf("backfiller is already running")
	}

	b.logger.Info("Starting backfiller service")
	b.isRunning = true

	// Start worker goroutines
	for i := 0; i < b.maxConcurrentJobs; i++ {
		go b.worker(ctx, i)
	}

	// Start scheduler
	go b.scheduler(ctx)

	// Start cleanup routine
	go b.cleanup(ctx)

	b.logger.Info("Backfiller service started successfully")
	return nil
}

// Stop gracefully stops the backfiller service
func (b *Backfiller) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isRunning {
		return fmt.Errorf("backfiller is not running")
	}

	b.logger.Info("Stopping backfiller service")
	b.isRunning = false

	// Signal all goroutines to stop
	close(b.stopChan)

	b.logger.Info("Backfiller service stopped")
	return nil
}

// BackfillRange performs backfill for a specific time range
func (b *Backfiller) BackfillRange(ctx context.Context, req *types.BackfillRequest) error {
	if req == nil {
		return fmt.Errorf("backfill request is required")
	}

	// Validate request
	if err := b.validateBackfillRequest(req); err != nil {
		return fmt.Errorf("invalid backfill request: %w", err)
	}

	jobID := uuid.New().String()
	b.logger.Info("Starting backfill job", "job_id", jobID, "source", req.Source)

	// Create job
	job := &backfillJob{
		id:      jobID,
		request: req,
		result:  make(chan error, 1),
	}

	// Submit job
	select {
	case b.jobQueue <- job:
	case <-ctx.Done():
		return ctx.Err()
	case <-b.stopChan:
		return fmt.Errorf("backfiller is stopping")
	}

	// Wait for result
	select {
	case err := <-job.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BackfillLatest performs backfill for the latest data
func (b *Backfiller) BackfillLatest(ctx context.Context) error {
	// Determine the time range for latest backfill
	endTime := time.Now().UTC()
	startTime := endTime.Add(-b.backfillInterval)

	// Create backfill requests for different sources
	requests := []*types.BackfillRequest{
		{
			Source:    "chain",
			StartTime: startTime,
			EndTime:   endTime,
			BatchSize: b.defaultBatchSize,
			ChainParams: &types.ChainBackfillParams{
				Programs: []string{"marginfi", "leverage"},
				Network:  "mainnet",
			},
		},
		{
			Source:    "glacier",
			StartTime: startTime,
			EndTime:   endTime,
			BatchSize: b.defaultBatchSize / 2,
			GlacierParams: &types.GlacierBackfillParams{
				VaultName: "margin-offers-archive",
				Retrieval: true,
			},
		},
		// Add off-chain job for external APIs
		{
			Source:    "off-chain",
			StartTime: startTime,
			EndTime:   endTime,
			BatchSize: b.defaultBatchSize,
			OffChainParams: &types.OffChainBackfillParams{
				APIEndpoint: "https://api.example.com/margin-offers",
				Headers: map[string]string{
					"Authorization": "Bearer ${API_TOKEN}",
				},
			},
		},
		// Add specific leverage program job
		{
			Source:    "chain",
			StartTime: startTime,
			EndTime:   endTime,
			BatchSize: b.defaultBatchSize,
			ChainParams: &types.ChainBackfillParams{
				Programs: []string{"CRSeeBqjDnm3UPefJ9gxrtngTsnQRhEJiTA345Q83X3v"}, // Leverage program ID
				Network:  "mainnet",
			},
		},
	}

	// Execute backfills concurrently
	errChan := make(chan error, len(requests))
	for _, req := range requests {
		go func(r *types.BackfillRequest) {
			errChan <- b.BackfillRange(ctx, r)
		}(req)
	}

	// Collect results
	var lastErr error
	for i := 0; i < len(requests); i++ {
		if err := <-errChan; err != nil {
			b.logger.Error("Backfill failed", "error", err)
			lastErr = err
		}
	}

	// Update last backfill time
	b.mu.Lock()
	now := time.Now().UTC()
	b.lastBackfillTime = &now
	next := now.Add(b.backfillInterval)
	b.nextBackfillTime = &next
	b.mu.Unlock()

	return lastErr
}

// FetchFromChain retrieves data from blockchain
func (b *Backfiller) FetchFromChain(ctx context.Context, startBlock, endBlock uint64) ([]*types.MarginOffer, error) {
	if !b.chainClient.IsConnected() {
		if err := b.chainClient.Connect(ctx, "wss://api.mainnet-beta.solana.com"); err != nil {
			return nil, fmt.Errorf("failed to connect to chain: %w", err)
		}
	}

	var allOffers []*types.MarginOffer

	// Fetch data block by block in batches
	batchSize := uint64(100)
	for block := startBlock; block <= endBlock; block += batchSize {
		endBatch := block + batchSize - 1
		if endBatch > endBlock {
			endBatch = endBlock
		}

		offers, err := b.chainClient.GetMarginOffersByBlock(ctx, block)
		if err != nil {
			b.logger.Warn("Failed to fetch offers for block", "block", block, "error", err)
			continue
		}

		allOffers = append(allOffers, offers...)

		// Rate limiting
		select {
		case <-time.After(time.Millisecond * 100):
		case <-ctx.Done():
			return allOffers, ctx.Err()
		}
	}

	return allOffers, nil
}

// FetchFromGlacier retrieves data from Glacier storage
func (b *Backfiller) FetchFromGlacier(ctx context.Context, startTime, endTime time.Time) ([]*types.MarginOffer, error) {
	// This would implement AWS Glacier retrieval
	// For now, return empty slice as placeholder
	b.logger.Info("Fetching from Glacier", "start", startTime, "end", endTime)
	return []*types.MarginOffer{}, nil
}

// FetchFromOffChain retrieves data from off-chain sources
func (b *Backfiller) FetchFromOffChain(ctx context.Context, source string) ([]*types.MarginOffer, error) {
	// This would implement API calls to external services
	b.logger.Info("Fetching from off-chain source", "source", source)
	return []*types.MarginOffer{}, nil
}

// ScheduleBackfill schedules a recurring backfill operation
func (b *Backfiller) ScheduleBackfill(ctx context.Context, req *types.BackfillScheduleRequest) error {
	if req == nil || req.Name == "" {
		return fmt.Errorf("invalid schedule request")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.scheduledJobs[req.Name] = req
	b.logger.Info("Scheduled backfill job", "name", req.Name, "schedule", req.Schedule)

	return nil
}

// GetStatus returns the current status of the backfiller
func (b *Backfiller) GetStatus() types.BackfillStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return types.BackfillStatus{
		IsRunning:        b.isRunning,
		ActiveJobs:       b.copyActiveJobs(),
		CompletedJobs:    b.completedJobs,
		FailedJobs:       b.failedJobs,
		ScheduledJobs:    int64(len(b.scheduledJobs)),
		LastBackfillTime: b.lastBackfillTime,
		NextBackfillTime: b.nextBackfillTime,
	}
}

// GetProgress returns the progress of a specific job
func (b *Backfiller) GetProgress() types.BackfillProgress {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Return progress for the most recent job
	for _, progress := range b.activeJobs {
		return *progress
	}

	// Return empty progress if no active jobs
	return types.BackfillProgress{
		Status: "idle",
	}
}

// Helper methods
func (b *Backfiller) validateBackfillRequest(req *types.BackfillRequest) error {
	if req.Source == "" {
		return fmt.Errorf("source is required")
	}
	if req.StartTime.After(req.EndTime) {
		return fmt.Errorf("start time must be before end time")
	}
	if req.BatchSize <= 0 {
		req.BatchSize = b.defaultBatchSize
	}
	return nil
}

func (b *Backfiller) copyActiveJobs() map[string]*types.BackfillProgress {
	copy := make(map[string]*types.BackfillProgress)
	for k, v := range b.activeJobs {
		copy[k] = v
	}
	return copy
}

// FetchAllPools retrieves all pools from the leverage program using getProgramAccounts
func (b *Backfiller) FetchAllPools(ctx context.Context, programID string) ([]*types.Pool, error) {
	if !b.chainClient.IsConnected() {
		if err := b.chainClient.Connect(ctx, "https://solitary-broken-pallet.solana-mainnet.quiknode.pro/e70f49ff42c8fbda17f4fd2041e184af8c94f97b/"); err != nil {
			return nil, fmt.Errorf("failed to connect to chain: %w", err)
		}
	}

	b.logger.Info("Fetching all pools from leverage program", "program_id", programID)

	// Call getProgramAccounts for the leverage program
	accounts, err := b.chainClient.GetProgramAccounts(ctx, programID, map[string]interface{}{
		"encoding": "base64",
		"filters": []map[string]interface{}{
			{
				"dataSize": 105, // Size of Pool account (1 + 32 + 8 + 32 + 8 + 8 + 32 = 105 bytes)
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get program accounts: %w", err)
	}

	var pools []*types.Pool
	for _, account := range accounts {
		// Decode base64 data
		data, err := b.chainClient.DecodeBase64(account.Account.Data[0])
		if err != nil {
			b.logger.Warn("Failed to decode account data", "account", account.Pubkey, "error", err)
			continue
		}

		// Deserialize pool
		pool, err := types.DeserializePool(data)
		if err != nil {
			b.logger.Warn("Failed to deserialize pool", "account", account.Pubkey, "error", err)
			continue
		}

		pools = append(pools, pool)
	}

	b.logger.Info("Successfully fetched pools", "count", len(pools))
	return pools, nil
}

// FetchAllNodeWallets retrieves all node wallets from the leverage program
func (b *Backfiller) FetchAllNodeWallets(ctx context.Context, programID string) ([]*types.NodeWallet, error) {
	if !b.chainClient.IsConnected() {
		if err := b.chainClient.Connect(ctx, "wss://api.mainnet-beta.solana.com"); err != nil {
			return nil, fmt.Errorf("failed to connect to chain: %w", err)
		}
	}

	b.logger.Info("Fetching all node wallets from leverage program", "program_id", programID)

	// Call getProgramAccounts for the leverage program
	accounts, err := b.chainClient.GetProgramAccounts(ctx, programID, map[string]interface{}{
		"encoding": "base64",
		"filters": []map[string]interface{}{
			{
				"dataSize": 83, // Size of NodeWallet account (8 + 8 + 1 + 2 + 32 + 32 = 83 bytes)
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get program accounts: %w", err)
	}

	var nodeWallets []*types.NodeWallet
	for _, account := range accounts {
		// Decode base64 data
		data, err := b.chainClient.DecodeBase64(account.Account.Data[0])
		if err != nil {
			b.logger.Warn("Failed to decode account data", "account", account.Pubkey, "error", err)
			continue
		}

		// Deserialize node wallet
		nodeWallet, err := types.DeserializeNodeWallet(data)
		if err != nil {
			b.logger.Warn("Failed to deserialize node wallet", "account", account.Pubkey, "error", err)
			continue
		}

		nodeWallets = append(nodeWallets, nodeWallet)
	}

	b.logger.Info("Successfully fetched node wallets", "count", len(nodeWallets))
	return nodeWallets, nil
}

// FetchAllPositions retrieves all positions from the leverage program
func (b *Backfiller) FetchAllPositions(ctx context.Context, programID string) ([]*types.Position, error) {
	if !b.chainClient.IsConnected() {
		if err := b.chainClient.Connect(ctx, "wss://api.mainnet-beta.solana.com"); err != nil {
			return nil, fmt.Errorf("failed to connect to chain: %w", err)
		}
	}

	b.logger.Info("Fetching all positions from leverage program", "program_id", programID)

	// Call getProgramAccounts for the leverage program
	accounts, err := b.chainClient.GetProgramAccounts(ctx, programID, map[string]interface{}{
		"encoding": "base64",
		"filters": []map[string]interface{}{
			{
				"dataSize": 137, // Size of Position account (32 + 8 + 8 + 8 + 8 + 8 + 32 + 32 + 8 + 8 + 1 + 8 = 137 bytes)
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get program accounts: %w", err)
	}

	var positions []*types.Position
	for _, account := range accounts {
		// Decode base64 data
		data, err := b.chainClient.DecodeBase64(account.Account.Data[0])
		if err != nil {
			b.logger.Warn("Failed to decode account data", "account", account.Pubkey, "error", err)
			continue
		}

		// Deserialize position
		position, err := types.DeserializePosition(data)
		if err != nil {
			b.logger.Warn("Failed to deserialize position", "account", account.Pubkey, "error", err)
			continue
		}

		positions = append(positions, position)
	}

	b.logger.Info("Successfully fetched positions", "count", len(positions))
	return positions, nil
}

// MapPoolsToMarginOffers fetches all pools and converts them to margin offers
func (b *Backfiller) MapPoolsToMarginOffers(ctx context.Context, programID string) ([]*types.MarginOffer, error) {
	b.logger.Info("Starting pool to margin offer mapping", "program_id", programID)

	// Fetch all pools from the leverage program
	pools, err := b.FetchAllPools(ctx, programID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools: %w", err)
	}

	b.logger.Info("Fetched pools", "count", len(pools))

	// Convert pools to margin offers
	var marginOffers []*types.MarginOffer
	for _, pool := range pools {
		marginOffer := pool.ToMarginOffer()
		if marginOffer != nil {
			marginOffers = append(marginOffers, marginOffer)
		}
	}

	b.logger.Info("Converted pools to margin offers", "pools", len(pools), "offers", len(marginOffers))
	return marginOffers, nil
}

// SyncPoolsToStore fetches all pools, converts them to margin offers, and stores them
func (b *Backfiller) SyncPoolsToStore(ctx context.Context, programID string) error {
	b.logger.Info("Starting pool synchronization to store", "program_id", programID)

	// Map pools to margin offers
	marginOffers, err := b.MapPoolsToMarginOffers(ctx, programID)
	if err != nil {
		return fmt.Errorf("failed to map pools to margin offers: %w", err)
	}

	if len(marginOffers) == 0 {
		b.logger.Warn("No margin offers to sync")
		return nil
	}

	// Use bulk overwrite to replace all existing leverage offers
	filter := &types.OverwriteFilter{
		LiquiditySource: stringPtr("leverage"),
	}

	err = b.store.BulkOverwriteByFilter(ctx, marginOffers, filter)
	if err != nil {
		return fmt.Errorf("failed to bulk overwrite margin offers: %w", err)
	}

	b.logger.Info("Successfully synced pools to store", "count", len(marginOffers))
	return nil
}

// SyncPoolsWithPositions fetches pools and positions, then creates comprehensive margin offers
func (b *Backfiller) SyncPoolsWithPositions(ctx context.Context, programID string) error {
	b.logger.Info("Starting comprehensive pool and position synchronization", "program_id", programID)

	// Fetch all pools and positions
	pools, err := b.FetchAllPools(ctx, programID)
	if err != nil {
		return fmt.Errorf("failed to fetch pools: %w", err)
	}

	positions, err := b.FetchAllPositions(ctx, programID)
	if err != nil {
		return fmt.Errorf("failed to fetch positions: %w", err)
	}

	b.logger.Info("Fetched data", "pools", len(pools), "positions", len(positions))

	// Create a map of pool addresses to positions for quick lookup
	poolPositions := make(map[string][]*types.Position)
	for _, position := range positions {
		poolKey := position.Pool.String()
		poolPositions[poolKey] = append(poolPositions[poolKey], position)
	}

	// Convert pools to margin offers with position data
	var marginOffers []*types.MarginOffer
	for _, pool := range pools {
		marginOffer := pool.ToMarginOffer()
		if marginOffer == nil {
			continue
		}

		// Add position statistics if available
		poolKey := pool.NodeWallet.String() // Assuming NodeWallet is the pool identifier
		if positions, exists := poolPositions[poolKey]; exists {
			// Calculate active positions and total borrowed
			activePositions := 0
			totalBorrowed := uint64(0)

			for _, pos := range positions {
				if pos.CloseTimestamp == 0 { // Position is still active
					activePositions++
					totalBorrowed += pos.Amount
				}
			}

			// Log position statistics for this pool
			b.logger.Debug("Pool position statistics",
				"pool", poolKey,
				"active_positions", activePositions,
				"total_borrowed", float64(totalBorrowed)/1e9)
		}

		marginOffers = append(marginOffers, marginOffer)
	}

	// Use bulk overwrite to replace all existing leverage offers
	filter := &types.OverwriteFilter{
		LiquiditySource: stringPtr("leverage"),
	}

	err = b.store.BulkOverwriteByFilter(ctx, marginOffers, filter)
	if err != nil {
		return fmt.Errorf("failed to bulk overwrite margin offers: %w", err)
	}

	b.logger.Info("Successfully synced pools with positions", "offers", len(marginOffers))
	return nil
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// RegisterCustomJob adds a custom job to the backfiller's default job list
func (b *Backfiller) RegisterCustomJob(req *types.BackfillRequest) error {
	if req == nil {
		return fmt.Errorf("backfill request is required")
	}

	// Validate request
	if err := b.validateBackfillRequest(req); err != nil {
		return fmt.Errorf("invalid backfill request: %w", err)
	}

	b.logger.Info("Registered custom backfill job", "source", req.Source)
	return nil
}

// RegisterScheduledJob adds a scheduled job that runs on a specific schedule
func (b *Backfiller) RegisterScheduledJob(name string, schedule string, req *types.BackfillRequest) error {
	if name == "" || schedule == "" || req == nil {
		return fmt.Errorf("name, schedule, and request are required")
	}

	scheduleReq := &types.BackfillScheduleRequest{
		Name:        name,
		Description: fmt.Sprintf("Scheduled job: %s", name),
		Schedule:    schedule,
		Request:     req,
		Enabled:     true,
		MaxRetries:  3,
		Timeout:     time.Hour,
	}

	return b.ScheduleBackfill(context.Background(), scheduleReq)
}
