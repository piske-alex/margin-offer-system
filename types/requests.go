package types

import "time"

// ListRequest represents a request for listing margin offers with pagination and filtering
type ListRequest struct {
	// Pagination
	Limit  int32 `json:"limit" validate:"min=1,max=1000"`
	Offset int32 `json:"offset" validate:"min=0"`

	// Sorting
	SortBy    string `json:"sort_by,omitempty"`
	SortOrder string `json:"sort_order,omitempty"` // "asc" or "desc"

	// Filtering
	OfferType       *OfferType     `json:"offer_type,omitempty"`
	CollateralToken *string        `json:"collateral_token,omitempty"`
	BorrowToken     *string        `json:"borrow_token,omitempty"`
	InterestModel   *InterestModel `json:"interest_model,omitempty"`
	LiquiditySource *string        `json:"liquidity_source,omitempty"`
	LenderAddress   *string        `json:"lender_address,omitempty"`
	Source          *string        `json:"source,omitempty"`

	// Range filters
	MinBorrowAmount *float64 `json:"min_borrow_amount,omitempty"`
	MaxBorrowAmount *float64 `json:"max_borrow_amount,omitempty"`
	MinInterestRate *float64 `json:"min_interest_rate,omitempty"`
	MaxInterestRate *float64 `json:"max_interest_rate,omitempty"`
	MinMaxOpenLTV   *float64 `json:"min_max_open_ltv,omitempty"`
	MaxMaxOpenLTV   *float64 `json:"max_max_open_ltv,omitempty"`

	// Time filters
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	UpdatedAfter  *time.Time `json:"updated_after,omitempty"`
	UpdatedBefore *time.Time `json:"updated_before,omitempty"`

	// Status filters
	IncludeExpired bool `json:"include_expired"`
	ActiveOnly     bool `json:"active_only"`
}

// BackfillRequest represents a request for backfilling data
type BackfillRequest struct {
	Source    string    `json:"source" validate:"required"`
	StartTime time.Time `json:"start_time" validate:"required"`
	EndTime   time.Time `json:"end_time" validate:"required"`
	BatchSize int32     `json:"batch_size" validate:"min=1,max=10000"`
	DryRun    bool      `json:"dry_run"`

	// Source-specific parameters
	ChainParams    *ChainBackfillParams    `json:"chain_params,omitempty"`
	GlacierParams  *GlacierBackfillParams  `json:"glacier_params,omitempty"`
	OffChainParams *OffChainBackfillParams `json:"off_chain_params,omitempty"`
}

// ChainBackfillParams contains parameters specific to blockchain backfilling
type ChainBackfillParams struct {
	StartBlock uint64   `json:"start_block"`
	EndBlock   uint64   `json:"end_block"`
	Programs   []string `json:"programs"`
	Network    string   `json:"network"`
}

// GlacierBackfillParams contains parameters for Glacier storage backfilling
type GlacierBackfillParams struct {
	VaultName string `json:"vault_name"`
	ArchiveID string `json:"archive_id,omitempty"`
	JobID     string `json:"job_id,omitempty"`
	Retrieval bool   `json:"retrieval"`
}

// OffChainBackfillParams contains parameters for off-chain data backfilling
type OffChainBackfillParams struct {
	APIEndpoint string            `json:"api_endpoint"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"query_params,omitempty"`
	AuthToken   string            `json:"auth_token,omitempty"`
}

// BackfillScheduleRequest represents a request to schedule a backfill operation
type BackfillScheduleRequest struct {
	Name        string           `json:"name" validate:"required"`
	Description string           `json:"description"`
	Schedule    string           `json:"schedule" validate:"required"` // Cron expression
	Request     *BackfillRequest `json:"request" validate:"required"`
	Enabled     bool             `json:"enabled"`
	MaxRetries  int32            `json:"max_retries" validate:"min=0,max=10"`
	Timeout     time.Duration    `json:"timeout"`
}

// BulkCreateRequest represents a request for bulk creating margin offers
type BulkCreateRequest struct {
	Offers          []*MarginOffer `json:"offers" validate:"required,min=1,max=1000"`
	IgnoreConflicts bool           `json:"ignore_conflicts"`
	ValidateAll     bool           `json:"validate_all"`
}

// BulkUpdateRequest represents a request for bulk updating margin offers
type BulkUpdateRequest struct {
	Offers        []*MarginOffer `json:"offers" validate:"required,min=1,max=1000"`
	Upsert        bool           `json:"upsert"`
	PartialUpdate bool           `json:"partial_update"`
}

// BulkCreateOrUpdateRequest represents a request for bulk upsert operations
type BulkCreateOrUpdateRequest struct {
	Offers      []*MarginOffer `json:"offers" validate:"required,min=1,max=1000"`
	ValidateAll bool           `json:"validate_all"`
}

// BulkOverwriteRequest represents a request for bulk overwrite operations
type BulkOverwriteRequest struct {
	Offers      []*MarginOffer   `json:"offers" validate:"required,min=0,max=1000"`
	Filter      *OverwriteFilter `json:"filter,omitempty"`
	ValidateAll bool             `json:"validate_all"`
	DryRun      bool             `json:"dry_run"` // Preview operation without executing
}

// BulkDeleteRequest represents a request for bulk deleting margin offers
type BulkDeleteRequest struct {
	IDs            []string `json:"ids" validate:"required,min=1,max=1000"`
	IgnoreNotFound bool     `json:"ignore_not_found"`
}

// StatsRequest represents a request for statistics
type StatsRequest struct {
	GroupBy     []string   `json:"group_by,omitempty"`
	Metrics     []string   `json:"metrics,omitempty"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Granularity string     `json:"granularity,omitempty"` // "hour", "day", "week", "month"
}

// OverwritePreviewRequest represents a request to preview an overwrite operation
type OverwritePreviewRequest struct {
	Filter *OverwriteFilter `json:"filter,omitempty"`
}
