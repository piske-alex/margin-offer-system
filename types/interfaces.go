package types

import (
	"context"
	"time"
)

// MarginOfferStore defines the interface for margin offer storage operations
type MarginOfferStore interface {
	// CRUD operations
	Create(ctx context.Context, offer *MarginOffer) error
	GetByID(ctx context.Context, id string) (*MarginOffer, error)
	Update(ctx context.Context, offer *MarginOffer) error
	Delete(ctx context.Context, id string) error

	// CreateOrUpdate operations (upsert)
	CreateOrUpdate(ctx context.Context, offer *MarginOffer) error

	// Query operations
	List(ctx context.Context, req *ListRequest) ([]*MarginOffer, error)
	ListByLender(ctx context.Context, lenderAddress string, req *ListRequest) ([]*MarginOffer, error)
	ListByToken(ctx context.Context, collateralToken, borrowToken string, req *ListRequest) ([]*MarginOffer, error)
	ListByOfferType(ctx context.Context, offerType OfferType, req *ListRequest) ([]*MarginOffer, error)

	// Bulk operations
	BulkCreate(ctx context.Context, offers []*MarginOffer) error
	BulkUpdate(ctx context.Context, offers []*MarginOffer) error
	BulkDelete(ctx context.Context, ids []string) error

	// Bulk CreateOrUpdate operations (upsert)
	BulkCreateOrUpdate(ctx context.Context, offers []*MarginOffer) error

	// Bulk overwrite operations - replaces entire dataset
	BulkOverwrite(ctx context.Context, offers []*MarginOffer) error
	BulkOverwriteByFilter(ctx context.Context, offers []*MarginOffer, filter *OverwriteFilter) error

	// Stats and aggregation
	Count(ctx context.Context) (int64, error)
	CountByOfferType(ctx context.Context, offerType OfferType) (int64, error)
	GetTotalLiquidity(ctx context.Context, borrowToken string) (float64, error)
	GetOverwriteStats(ctx context.Context, filter *OverwriteFilter) (int64, error)

	// Health and maintenance
	HealthCheck(ctx context.Context) error
	Close() error
}

// OverwriteFilter defines criteria for partial overwrite operations
type OverwriteFilter struct {
	// Filter criteria - offers matching these will be deleted before inserting new ones
	OfferType       *OfferType `json:"offer_type,omitempty"`
	CollateralToken *string    `json:"collateral_token,omitempty"`
	BorrowToken     *string    `json:"borrow_token,omitempty"`
	LenderAddress   *string    `json:"lender_address,omitempty"`
	LiquiditySource *string    `json:"liquidity_source,omitempty"`
	Source          *string    `json:"source,omitempty"`

	// Time-based filters
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	UpdatedAfter  *time.Time `json:"updated_after,omitempty"`
	UpdatedBefore *time.Time `json:"updated_before,omitempty"`
}

// ETLProcessor defines the interface for processing blockchain events
type ETLProcessor interface {
	// Event processing
	ProcessMarginOfferEvent(ctx context.Context, event *MarginOfferEvent) error
	ProcessLiquidityEvent(ctx context.Context, event *LiquidityEvent) error
	ProcessInterestRateEvent(ctx context.Context, event *InterestRateEvent) error

	// Pipeline control
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool

	// Health monitoring
	GetStatus() ETLStatus
	GetMetrics() ETLMetrics
}

// Backfiller defines the interface for backfilling operations
type Backfiller interface {
	// Backfill operations
	BackfillRange(ctx context.Context, req *BackfillRequest) error
	BackfillLatest(ctx context.Context) error

	// Data source operations
	FetchFromChain(ctx context.Context, startBlock, endBlock uint64) ([]*MarginOffer, error)
	FetchFromGlacier(ctx context.Context, startTime, endTime time.Time) ([]*MarginOffer, error)
	FetchFromOffChain(ctx context.Context, source string) ([]*MarginOffer, error)

	// Control operations
	Start(ctx context.Context) error
	Stop() error
	ScheduleBackfill(ctx context.Context, req *BackfillScheduleRequest) error

	// Status monitoring
	GetStatus() BackfillStatus
	GetProgress() BackfillProgress
}

// ProgramAccount represents a program account from Solana
type ProgramAccount struct {
	Pubkey  string `json:"pubkey"`
	Account struct {
		Data       []string `json:"data"`
		Executable bool     `json:"executable"`
		Lamports   uint64   `json:"lamports"`
		Owner      string   `json:"owner"`
		RentEpoch  uint64   `json:"rentEpoch"`
	} `json:"account"`
}

// ChainClient defines the interface for blockchain interactions
type ChainClient interface {
	// Connection management
	Connect(ctx context.Context, endpoint string) error
	Disconnect() error
	IsConnected() bool

	// Event subscription
	SubscribeToMarginOfferEvents(ctx context.Context, callback func(*MarginOfferEvent)) error
	SubscribeToLiquidityEvents(ctx context.Context, callback func(*LiquidityEvent)) error
	Unsubscribe(subscriptionID string) error

	// Data fetching
	GetMarginOffersByBlock(ctx context.Context, blockNumber uint64) ([]*MarginOffer, error)
	GetLatestBlock(ctx context.Context) (uint64, error)
	GetBlockTimestamp(ctx context.Context, blockNumber uint64) (time.Time, error)

	// Program account operations
	GetProgramAccounts(ctx context.Context, programID string, filters map[string]interface{}) ([]ProgramAccount, error)
	DecodeBase64(data string) ([]byte, error)
}

// Logger defines the interface for logging operations
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
}

// MetricsCollector defines the interface for metrics collection
type MetricsCollector interface {
	// Counter metrics
	IncCounter(name string, tags map[string]string)
	AddCounter(name string, value float64, tags map[string]string)

	// Gauge metrics
	SetGauge(name string, value float64, tags map[string]string)

	// Histogram metrics
	RecordDuration(name string, duration time.Duration, tags map[string]string)
	RecordValue(name string, value float64, tags map[string]string)

	// Health metrics
	SetHealthy(service string)
	SetUnhealthy(service string, reason string)
}
