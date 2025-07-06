package types

import "time"

// ListResponse represents a paginated response for listing margin offers
type ListResponse struct {
	Offers     []*MarginOffer `json:"offers"`
	Total      int64          `json:"total"`
	Limit      int32          `json:"limit"`
	Offset     int32          `json:"offset"`
	HasMore    bool           `json:"has_more"`
	NextOffset *int32         `json:"next_offset,omitempty"`
}

// BulkOperationResponse represents the response for bulk operations
type BulkOperationResponse struct {
	SuccessCount int32             `json:"success_count"`
	FailureCount int32             `json:"failure_count"`
	Errors       []BulkError       `json:"errors,omitempty"`
	ProcessedIDs []string          `json:"processed_ids"`
	Duration     time.Duration     `json:"duration"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// BulkOverwriteResponse represents the response for bulk overwrite operations
type BulkOverwriteResponse struct {
	DeletedCount int32             `json:"deleted_count"`
	CreatedCount int32             `json:"created_count"`
	DeletedIDs   []string          `json:"deleted_ids"`
	CreatedIDs   []string          `json:"created_ids"`
	Errors       []BulkError       `json:"errors,omitempty"`
	Duration     time.Duration     `json:"duration"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// CreateOrUpdateResponse represents the response for upsert operations
type CreateOrUpdateResponse struct {
	Offer      *MarginOffer `json:"offer"`
	WasCreated bool         `json:"was_created"` // true if created, false if updated
}

// OverwritePreviewResponse represents the response for overwrite preview operations
type OverwritePreviewResponse struct {
	AffectedCount int32    `json:"affected_count"`
	AffectedIDs   []string `json:"affected_ids"`
}

// BulkError represents an error that occurred during bulk operations
type BulkError struct {
	Index   int32  `json:"index"`
	ID      string `json:"id,omitempty"`
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
}

// HealthResponse represents the health status of a service
type HealthResponse struct {
	Status     string            `json:"status"` // "healthy", "unhealthy", "degraded"
	Timestamp  time.Time         `json:"timestamp"`
	Uptime     time.Duration     `json:"uptime"`
	Version    string            `json:"version"`
	Checks     map[string]Check  `json:"checks"`
	Metrics    map[string]interface{} `json:"metrics,omitempty"`
}

// Check represents an individual health check result
type Check struct {
	Status    string        `json:"status"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// StatsResponse represents statistics about margin offers
type StatsResponse struct {
	TotalOffers        int64             `json:"total_offers"`
	ActiveOffers       int64             `json:"active_offers"`
	ExpiredOffers      int64             `json:"expired_offers"`
	TotalLiquidity     float64           `json:"total_liquidity"`
	AverageInterestRate float64          `json:"average_interest_rate"`
	OffersByType       map[string]int64  `json:"offers_by_type"`
	OffersByToken      map[string]int64  `json:"offers_by_token"`
	LiquidityByToken   map[string]float64 `json:"liquidity_by_token"`
	Timestamp          time.Time         `json:"timestamp"`
}

// BackfillProgress represents the progress of a backfill operation
type BackfillProgress struct {
	JobID            string        `json:"job_id"`
	Status           string        `json:"status"` // "running", "completed", "failed", "paused"
	StartTime        time.Time     `json:"start_time"`
	EndTime          *time.Time    `json:"end_time,omitempty"`
	TotalRecords     int64         `json:"total_records"`
	ProcessedRecords int64         `json:"processed_records"`
	SuccessRecords   int64         `json:"success_records"`
	FailedRecords    int64         `json:"failed_records"`
	Progress         float64       `json:"progress"` // 0.0 to 1.0
	CurrentBatch     int32         `json:"current_batch"`
	TotalBatches     int32         `json:"total_batches"`
	RemainingTime    time.Duration `json:"remaining_time"`
	Throughput       float64       `json:"throughput"` // records per second
	Errors           []string      `json:"errors,omitempty"`
}

// ETLMetrics represents metrics for the ETL pipeline
type ETLMetrics struct {
	EventsProcessed      int64         `json:"events_processed"`
	EventsPerSecond      float64       `json:"events_per_second"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	ErrorRate            float64       `json:"error_rate"`
	LastEventTimestamp   time.Time     `json:"last_event_timestamp"`
	LagSeconds           float64       `json:"lag_seconds"`
	ActiveSubscriptions  int32         `json:"active_subscriptions"`
	ConnectionStatus     string        `json:"connection_status"`
}

// BackfillStatus represents the status of the backfiller service
type BackfillStatus struct {
	IsRunning        bool                         `json:"is_running"`
	ActiveJobs       map[string]*BackfillProgress `json:"active_jobs"`
	CompletedJobs    int64                       `json:"completed_jobs"`
	FailedJobs       int64                       `json:"failed_jobs"`
	ScheduledJobs    int64                       `json:"scheduled_jobs"`
	LastBackfillTime *time.Time                  `json:"last_backfill_time,omitempty"`
	NextBackfillTime *time.Time                  `json:"next_backfill_time,omitempty"`
}

// ETLStatus represents the status of the ETL pipeline
type ETLStatus struct {
	IsRunning        bool      `json:"is_running"`
	StartTime        time.Time `json:"start_time"`
	LastEventTime    *time.Time `json:"last_event_time,omitempty"`
	ConnectionStatus string    `json:"connection_status"`
	Subscriptions    []string  `json:"subscriptions"`
	ErrorCount       int64     `json:"error_count"`
	RestartCount     int32     `json:"restart_count"`
}