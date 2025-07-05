package types

import (
	"time"
)

// EventType represents the type of blockchain event
type EventType string

const (
	EventTypeMarginOfferCreated   EventType = "margin_offer_created"
	EventTypeMarginOfferUpdated   EventType = "margin_offer_updated"
	EventTypeMarginOfferDeleted   EventType = "margin_offer_deleted"
	EventTypeLiquidityAdded       EventType = "liquidity_added"
	EventTypeLiquidityRemoved     EventType = "liquidity_removed"
	EventTypeInterestRateChanged  EventType = "interest_rate_changed"
	EventTypeBorrowExecuted       EventType = "borrow_executed"
	EventTypeLiquidationExecuted  EventType = "liquidation_executed"
)

// MarginOfferEvent represents a blockchain event related to margin offers
type MarginOfferEvent struct {
	EventType        EventType     `json:"event_type"`
	OfferID          string        `json:"offer_id"`
	TransactionHash  string        `json:"transaction_hash"`
	BlockNumber      uint64        `json:"block_number"`
	BlockHash        string        `json:"block_hash"`
	Timestamp        time.Time     `json:"timestamp"`
	LenderAddress    string        `json:"lender_address"`
	BorrowerAddress  *string       `json:"borrower_address,omitempty"`
	ProgramID        string        `json:"program_id"`
	
	// Event-specific data
	OfferData        *MarginOffer  `json:"offer_data,omitempty"`
	BorrowAmount     *float64      `json:"borrow_amount,omitempty"`
	LiquidityAmount  *float64      `json:"liquidity_amount,omitempty"`
	OldInterestRate  *float64      `json:"old_interest_rate,omitempty"`
	NewInterestRate  *float64      `json:"new_interest_rate,omitempty"`
	
	// Metadata
	RawData          map[string]interface{} `json:"raw_data,omitempty"`
	ProcessedAt      time.Time              `json:"processed_at"`
	RetryCount       int32                  `json:"retry_count"`
}

// LiquidityEvent represents a liquidity-related blockchain event
type LiquidityEvent struct {
	EventType       EventType     `json:"event_type"`
	OfferID         string        `json:"offer_id"`
	TransactionHash string        `json:"transaction_hash"`
	BlockNumber     uint64        `json:"block_number"`
	Timestamp       time.Time     `json:"timestamp"`
	LenderAddress   string        `json:"lender_address"`
	Amount          float64       `json:"amount"`
	Token           string        `json:"token"`
	NewTotalAmount  float64       `json:"new_total_amount"`
	ProgramID       string        `json:"program_id"`
	
	// Metadata
	RawData         map[string]interface{} `json:"raw_data,omitempty"`
	ProcessedAt     time.Time              `json:"processed_at"`
}

// InterestRateEvent represents an interest rate change event
type InterestRateEvent struct {
	EventType       EventType     `json:"event_type"`
	OfferID         string        `json:"offer_id"`
	TransactionHash string        `json:"transaction_hash"`
	BlockNumber     uint64        `json:"block_number"`
	Timestamp       time.Time     `json:"timestamp"`
	LenderAddress   string        `json:"lender_address"`
	OldRate         float64       `json:"old_rate"`
	NewRate         float64       `json:"new_rate"`
	RateModel       InterestModel `json:"rate_model"`
	ProgramID       string        `json:"program_id"`
	
	// Metadata
	RawData         map[string]interface{} `json:"raw_data,omitempty"`
	ProcessedAt     time.Time              `json:"processed_at"`
}

// ProcessingResult represents the result of processing an event
type ProcessingResult struct {
	Success     bool          `json:"success"`
	EventID     string        `json:"event_id"`
	ProcessedAt time.Time     `json:"processed_at"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	RetryCount  int32         `json:"retry_count"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// EventBatch represents a batch of events for processing
type EventBatch struct {
	BatchID     string                 `json:"batch_id"`
	Events      []interface{}          `json:"events"` // Can contain different event types
	CreatedAt   time.Time              `json:"created_at"`
	Source      string                 `json:"source"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Validate validates the margin offer event
func (e *MarginOfferEvent) Validate() error {
	if e.OfferID == "" {
		return ErrMissingID
	}
	if e.TransactionHash == "" {
		return ErrInvalidEventData
	}
	if e.LenderAddress == "" {
		return ErrInvalidEventData
	}
	return nil
}

// GetEventKey returns a unique key for the event
func (e *MarginOfferEvent) GetEventKey() string {
	return e.TransactionHash + "_" + e.OfferID
}

// IsRetryable determines if the event can be retried
func (e *MarginOfferEvent) IsRetryable() bool {
	return e.RetryCount < 3
}