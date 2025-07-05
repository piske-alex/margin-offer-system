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
	mu            sync.RWMutex
	store         types.MarginOfferStore
	chainClient   types.ChainClient
	logger        types.Logger
	metrics       types.MetricsCollector
	
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
	defaultBatchSize    int32
	defaultTimeout      time.Duration
	maxConcurrentJobs   int
	backfillInterval    time.Duration
	retentionPeriod     time.Duration
	
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
		store:               store,
		chainClient:         chainClient,
		logger:              logger,
		metrics:             metrics,
		activeJobs:          make(map[string]*types.BackfillProgress),
		scheduledJobs:       make(map[string]*types.BackfillScheduleRequest),
		defaultBatchSize:    1000,
		defaultTimeout:      time.Hour,
		maxConcurrentJobs:   5,
		backfillInterval:    time.Hour * 6, // Every 6 hours
		retentionPeriod:     time.Hour * 24 * 7, // 7 days
		jobQueue:            make(chan *backfillJob, 100),
		stopChan:            make(chan struct{}),
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