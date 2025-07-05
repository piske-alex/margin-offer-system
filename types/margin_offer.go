package types

import (
	"time"
)

// OfferType represents the type of margin offer
type OfferType string

const (
	OfferTypeV1 OfferType = "V1"
	OfferTypeBP OfferType = "BP"
	OfferTypeLV OfferType = "LV"
)

// InterestModel represents the interest calculation model
type InterestModel string

const (
	InterestModelFixed    InterestModel = "fixed"
	InterestModelFloating InterestModel = "floating"
)

// MarginOffer represents a margin lending offer
type MarginOffer struct {
	ID                      string        `json:"id" bson:"_id"`
	OfferType              OfferType     `json:"offer_type" bson:"offer_type"`
	CollateralToken        string        `json:"collateral_token" bson:"collateral_token"`
	BorrowToken            string        `json:"borrow_token" bson:"borrow_token"`
	AvailableBorrowAmount  float64       `json:"available_borrow_amount" bson:"available_borrow_amount"`
	MaxOpenLTV             float64       `json:"max_open_ltv" bson:"max_open_ltv"`
	LiquidationLTV         float64       `json:"liquidation_ltv" bson:"liquidation_ltv"`
	InterestRate           float64       `json:"interest_rate" bson:"interest_rate"`
	InterestModel          InterestModel `json:"interest_model" bson:"interest_model"`
	TermDays               *int32        `json:"term_days,omitempty" bson:"term_days,omitempty"`
	LiquiditySource        string        `json:"liquidity_source" bson:"liquidity_source"`
	LenderAddress          *string       `json:"lender_address,omitempty" bson:"lender_address,omitempty"`
	LastBorrowedTimestamp  *time.Time    `json:"last_borrowed_timestamp,omitempty" bson:"last_borrowed_timestamp,omitempty"`
	CreatedTimestamp       time.Time     `json:"created_timestamp" bson:"created_timestamp"`
	UpdatedTimestamp       time.Time     `json:"updated_timestamp" bson:"updated_timestamp"`
}

// Validate performs basic validation on the MarginOffer
func (mo *MarginOffer) Validate() error {
	if mo.ID == "" {
		return ErrMissingID
	}
	if mo.CollateralToken == "" {
		return ErrMissingCollateralToken
	}
	if mo.BorrowToken == "" {
		return ErrMissingBorrowToken
	}
	if mo.AvailableBorrowAmount < 0 {
		return ErrInvalidBorrowAmount
	}
	if mo.MaxOpenLTV < 0 || mo.MaxOpenLTV > 1 {
		return ErrInvalidMaxOpenLTV
	}
	if mo.LiquidationLTV < 0 || mo.LiquidationLTV > 1 {
		return ErrInvalidLiquidationLTV
	}
	if mo.LiquidationLTV <= mo.MaxOpenLTV {
		return ErrInvalidLTVRatio
	}
	if mo.InterestRate < 0 {
		return ErrInvalidInterestRate
	}
	if mo.LiquiditySource == "" {
		return ErrMissingLiquiditySource
	}
	return nil
}

// UpdateTimestamp updates the UpdatedTimestamp field to the current time
func (mo *MarginOffer) UpdateTimestamp() {
	mo.UpdatedTimestamp = time.Now().UTC()
}

// IsExpired checks if the offer has expired based on term days
func (mo *MarginOffer) IsExpired() bool {
	if mo.TermDays == nil {
		return false // No expiration for offers without term
	}
	expirationTime := mo.CreatedTimestamp.AddDate(0, 0, int(*mo.TermDays))
	return time.Now().UTC().After(expirationTime)
}

// GetEffectiveLTV returns the effective LTV based on current state
func (mo *MarginOffer) GetEffectiveLTV() float64 {
	return mo.MaxOpenLTV
}

// Clone creates a deep copy of the MarginOffer
func (mo *MarginOffer) Clone() *MarginOffer {
	clone := *mo
	if mo.TermDays != nil {
		termDays := *mo.TermDays
		clone.TermDays = &termDays
	}
	if mo.LenderAddress != nil {
		lenderAddr := *mo.LenderAddress
		clone.LenderAddress = &lenderAddr
	}
	if mo.LastBorrowedTimestamp != nil {
		lastBorrowed := *mo.LastBorrowedTimestamp
		clone.LastBorrowedTimestamp = &lastBorrowed
	}
	return &clone
}