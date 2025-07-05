package backfillers

import (
	"context"
	"time"

	"github.com/piske-alex/margin-offer-system/types"
)

// worker processes backfill jobs from the queue
func (b *Backfiller) worker(ctx context.Context, workerID int) {
	b.logger.Info("Starting backfill worker", "worker_id", workerID)
	
	for {
		select {
		case job := <-b.jobQueue:
			b.processJob(ctx, job)
		case <-b.stopChan:
			b.logger.Info("Stopping backfill worker", "worker_id", workerID)
			return
		case <-ctx.Done():
			return
		}
	}
}

// processJob executes a single backfill job
func (b *Backfiller) processJob(ctx context.Context, job *backfillJob) {
	start := time.Now()
	
	// Create progress tracking
	progress := &types.BackfillProgress{
		JobID:     job.id,
		Status:    "running",
		StartTime: start,
	}
	
	b.mu.Lock()
	b.activeJobs[job.id] = progress
	b.mu.Unlock()
	
	defer func() {
		b.mu.Lock()
		delete(b.activeJobs, job.id)
		b.mu.Unlock()
	}()
	
	b.logger.Info("Processing backfill job", "job_id", job.id, "source", job.request.Source)
	
	// Execute backfill based on source
	var err error
	switch job.request.Source {
	case "chain":
		err = b.processChainBackfill(ctx, job.request, progress)
	case "glacier":
		err = b.processGlacierBackfill(ctx, job.request, progress)
	case "off-chain":
		err = b.processOffChainBackfill(ctx, job.request, progress)
	default:
		err = types.ErrBackfillSourceUnavailable
	}
	
	// Update progress
	now := time.Now()
	progress.EndTime = &now
	progress.Progress = 1.0
	
	if err != nil {
		progress.Status = "failed"
		progress.Errors = append(progress.Errors, err.Error())
		b.mu.Lock()
		b.failedJobs++
		b.mu.Unlock()
		b.logger.Error("Backfill job failed", "job_id", job.id, "error", err)
	} else {
		progress.Status = "completed"
		b.mu.Lock()
		b.completedJobs++
		b.mu.Unlock()
		b.logger.Info("Backfill job completed", "job_id", job.id)
	}
	
	// Send result
	select {
	case job.result <- err:
	default:
		// Channel might be closed
	}
}

// processChainBackfill handles blockchain data backfill
func (b *Backfiller) processChainBackfill(ctx context.Context, req *types.BackfillRequest, progress *types.BackfillProgress) error {
	if req.ChainParams == nil {
		return types.ErrInvalidBackfillRange
	}
	
	// Fetch data from blockchain
	offers, err := b.FetchFromChain(ctx, req.ChainParams.StartBlock, req.ChainParams.EndBlock)
	if err != nil {
		return err
	}
	
	progress.TotalRecords = int64(len(offers))
	
	// Process in batches
	batchSize := int(req.BatchSize)
	for i := 0; i < len(offers); i += batchSize {
		end := i + batchSize
		if end > len(offers) {
			end = len(offers)
		}
		
		batch := offers[i:end]
		
		if req.DryRun {
			b.logger.Info("Dry run: would process batch", "size", len(batch))
		} else {
			if err := b.store.BulkCreate(ctx, batch); err != nil {
				// Try bulk update if create fails
				if err := b.store.BulkUpdate(ctx, batch); err != nil {
					progress.FailedRecords += int64(len(batch))
					continue
				}
			}
		}
		
		progress.ProcessedRecords += int64(len(batch))
		progress.SuccessRecords += int64(len(batch))
		progress.CurrentBatch++
		progress.Progress = float64(progress.ProcessedRecords) / float64(progress.TotalRecords)
		
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Rate limiting
		time.Sleep(time.Millisecond * 50)
	}
	
	return nil
}

// processGlacierBackfill handles Glacier storage backfill
func (b *Backfiller) processGlacierBackfill(ctx context.Context, req *types.BackfillRequest, progress *types.BackfillProgress) error {
	if req.GlacierParams == nil {
		return types.ErrInvalidBackfillRange
	}
	
	// Fetch data from Glacier
	offers, err := b.FetchFromGlacier(ctx, req.StartTime, req.EndTime)
	if err != nil {
		return err
	}
	
	progress.TotalRecords = int64(len(offers))
	
	// Process offers (similar to chain backfill)
	if !req.DryRun && len(offers) > 0 {
		if err := b.store.BulkCreate(ctx, offers); err != nil {
			return err
		}
	}
	
	progress.ProcessedRecords = int64(len(offers))
	progress.SuccessRecords = int64(len(offers))
	progress.Progress = 1.0
	
	return nil
}

// processOffChainBackfill handles off-chain data backfill
func (b *Backfiller) processOffChainBackfill(ctx context.Context, req *types.BackfillRequest, progress *types.BackfillProgress) error {
	if req.OffChainParams == nil {
		return types.ErrInvalidBackfillRange
	}
	
	// Fetch data from off-chain source
	offers, err := b.FetchFromOffChain(ctx, req.OffChainParams.APIEndpoint)
	if err != nil {
		return err
	}
	
	progress.TotalRecords = int64(len(offers))
	
	// Process offers
	if !req.DryRun && len(offers) > 0 {
		if err := b.store.BulkCreate(ctx, offers); err != nil {
			return err
		}
	}
	
	progress.ProcessedRecords = int64(len(offers))
	progress.SuccessRecords = int64(len(offers))
	progress.Progress = 1.0
	
	return nil
}

// scheduler handles scheduled backfill jobs
func (b *Backfiller) scheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 5) // Check every 5 minutes
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			b.checkScheduledJobs(ctx)
		case <-b.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// checkScheduledJobs checks if any scheduled jobs need to run
func (b *Backfiller) checkScheduledJobs(ctx context.Context) {
	b.mu.RLock()
	scheduledJobs := make(map[string]*types.BackfillScheduleRequest)
	for k, v := range b.scheduledJobs {
		scheduledJobs[k] = v
	}
	b.mu.RUnlock()
	
	now := time.Now()
	for name, job := range scheduledJobs {
		if !job.Enabled {
			continue
		}
		
		// Simple time-based scheduling (would use cron parser in real implementation)
		if b.shouldRunScheduledJob(job, now) {
			b.logger.Info("Running scheduled backfill job", "name", name)
			go func(req *types.BackfillRequest) {
				if err := b.BackfillRange(ctx, req); err != nil {
					b.logger.Error("Scheduled backfill failed", "name", name, "error", err)
				}
			}(job.Request)
		}
	}
}

// shouldRunScheduledJob determines if a scheduled job should run now
func (b *Backfiller) shouldRunScheduledJob(job *types.BackfillScheduleRequest, now time.Time) bool {
	// Simplified scheduling logic - would use cron parser in real implementation
	return now.Hour()%6 == 0 && now.Minute() < 5
}

// cleanup removes old job data and performs maintenance
func (b *Backfiller) cleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour) // Cleanup every hour
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			b.performCleanup()
		case <-b.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// performCleanup removes old data and performs maintenance tasks
func (b *Backfiller) performCleanup() {
	b.logger.Debug("Performing backfiller cleanup")
	
	// Clean up old active jobs (in case of crashes)
	b.mu.Lock()
	now := time.Now()
	for jobID, progress := range b.activeJobs {
		if now.Sub(progress.StartTime) > b.defaultTimeout {
			b.logger.Warn("Removing stale job", "job_id", jobID)
			delete(b.activeJobs, jobID)
		}
	}
	b.mu.Unlock()
	
	// Update metrics
	b.metrics.SetGauge("backfill_active_jobs", float64(len(b.activeJobs)), nil)
	b.metrics.SetGauge("backfill_completed_jobs", float64(b.completedJobs), nil)
	b.metrics.SetGauge("backfill_failed_jobs", float64(b.failedJobs), nil)
}