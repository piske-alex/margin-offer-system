package types

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
)

// Pool represents a trading pool account from the leverage program
type Pool struct {
	InterestRate    uint8
	CollateralType  solana.PublicKey
	MaxBorrow       uint64
	NodeWallet      solana.PublicKey
	MaxExposure     uint64
	CurrentExposure uint64
	QtType          solana.PublicKey
}

// NodeWallet represents a node wallet account from the leverage program
type NodeWallet struct {
	TotalFunds     uint64
	TotalBorrowed  uint64
	MaintenanceLtv uint8
	LiquidationLtv uint16
	NodeOperator   solana.PublicKey
	Mint           solana.PublicKey
}

// Position represents a trading position account from the leverage program
type Position struct {
	Pool                       solana.PublicKey
	CloseStatusRecallTimestamp uint64
	Amount                     uint64
	UserPaid                   uint64
	CollateralAmount           uint64
	Timestamp                  int64
	Trader                     solana.PublicKey
	Seed                       solana.PublicKey
	CloseTimestamp             int64
	ClosingPositionSize        uint64
	InterestRate               uint8
	LastInterestCollect        int64
}

// Delegate represents a delegate account from the leverage program
type Delegate struct {
	DelegateType     uint8
	Field1           uint64
	Field2           uint64
	Field3           uint64
	Field4           solana.PublicKey
	Field5           solana.PublicKey
	OriginalOperator solana.PublicKey
	DelegateOperator solana.PublicKey
	Account          solana.PublicKey
}

// PositionCloseType represents how a position was closed
type PositionCloseType uint8

const (
	PositionCloseTypeClosedByUser PositionCloseType = iota
	PositionCloseTypeLiquidated
)

func (pct PositionCloseType) String() string {
	switch pct {
	case PositionCloseTypeClosedByUser:
		return "closed_by_user"
	case PositionCloseTypeLiquidated:
		return "liquidated"
	default:
		return "unknown"
	}
}

// LendingErrors represents errors from the lending program
type LendingErrors uint8

const (
	LendingErrorAddressMismatch LendingErrors = iota
	LendingErrorProgramMismatch
	LendingErrorMissingRepay
	LendingErrorIncorrectOwner
	LendingErrorIncorrectProgramAuthority
	LendingErrorCannotBorrowBeforeRepay
	LendingErrorUnknownInstruction
	LendingErrorExpectedCollateralNotEnough
)

func (le LendingErrors) String() string {
	switch le {
	case LendingErrorAddressMismatch:
		return "address_mismatch"
	case LendingErrorProgramMismatch:
		return "program_mismatch"
	case LendingErrorMissingRepay:
		return "missing_repay"
	case LendingErrorIncorrectOwner:
		return "incorrect_owner"
	case LendingErrorIncorrectProgramAuthority:
		return "incorrect_program_authority"
	case LendingErrorCannotBorrowBeforeRepay:
		return "cannot_borrow_before_repay"
	case LendingErrorUnknownInstruction:
		return "unknown_instruction"
	case LendingErrorExpectedCollateralNotEnough:
		return "expected_collateral_not_enough"
	default:
		return "unknown"
	}
}

// ErrorCode represents various error codes
type ErrorCode uint8

const (
	ErrorCodeInvalidSignature ErrorCode = iota
	ErrorCodeInvalidOracle
	ErrorCodeInvalidSplitRatio
	ErrorCodePositionsNotMergeable
	ErrorCodeOnlyDelegateOperator
	ErrorCodeAddressMismatch
	ErrorCodeInvalidDelegateType
)

func (ec ErrorCode) String() string {
	switch ec {
	case ErrorCodeInvalidSignature:
		return "invalid_signature"
	case ErrorCodeInvalidOracle:
		return "invalid_oracle"
	case ErrorCodeInvalidSplitRatio:
		return "invalid_split_ratio"
	case ErrorCodePositionsNotMergeable:
		return "positions_not_mergeable"
	case ErrorCodeOnlyDelegateOperator:
		return "only_delegate_operator"
	case ErrorCodeAddressMismatch:
		return "address_mismatch"
	case ErrorCodeInvalidDelegateType:
		return "invalid_delegate_type"
	default:
		return "unknown"
	}
}

// DeserializePool deserializes a pool account from raw bytes
func DeserializePool(data []byte) (*Pool, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("data too short for pool account")
	}

	// Check discriminator (first 8 bytes)
	discriminator := data[:8]
	expectedDiscriminator := []byte{0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f, 0x90, 0x91} // Example discriminator
	if !bytesEqual(discriminator, expectedDiscriminator) {
		return nil, fmt.Errorf("invalid pool account discriminator")
	}

	offset := 8
	if offset+1 > len(data) {
		return nil, fmt.Errorf("data too short for interest rate")
	}
	interestRate := data[offset]
	offset++

	if offset+32 > len(data) {
		return nil, fmt.Errorf("data too short for collateral type")
	}
	collateralType := solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for max borrow")
	}
	maxBorrow := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset+32 > len(data) {
		return nil, fmt.Errorf("data too short for node wallet")
	}
	nodeWallet := solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for max exposure")
	}
	maxExposure := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for current exposure")
	}
	currentExposure := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset+32 > len(data) {
		return nil, fmt.Errorf("data too short for qt type")
	}
	qtType := solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	return &Pool{
		InterestRate:    interestRate,
		CollateralType:  collateralType,
		MaxBorrow:       maxBorrow,
		NodeWallet:      nodeWallet,
		MaxExposure:     maxExposure,
		CurrentExposure: currentExposure,
		QtType:          qtType,
	}, nil
}

// DeserializeNodeWallet deserializes a node wallet account from raw bytes
func DeserializeNodeWallet(data []byte) (*NodeWallet, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("data too short for node wallet account")
	}

	// Check discriminator (first 8 bytes)
	discriminator := data[:8]
	expectedDiscriminator := []byte{0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f, 0x90, 0x92} // Example discriminator
	if !bytesEqual(discriminator, expectedDiscriminator) {
		return nil, fmt.Errorf("invalid node wallet account discriminator")
	}

	offset := 8
	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for total funds")
	}
	totalFunds := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for total borrowed")
	}
	totalBorrowed := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset+1 > len(data) {
		return nil, fmt.Errorf("data too short for maintenance ltv")
	}
	maintenanceLtv := data[offset]
	offset++

	if offset+2 > len(data) {
		return nil, fmt.Errorf("data too short for liquidation ltv")
	}
	liquidationLtv := binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	if offset+32 > len(data) {
		return nil, fmt.Errorf("data too short for node operator")
	}
	nodeOperator := solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	if offset+32 > len(data) {
		return nil, fmt.Errorf("data too short for mint")
	}
	mint := solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	return &NodeWallet{
		TotalFunds:     totalFunds,
		TotalBorrowed:  totalBorrowed,
		MaintenanceLtv: maintenanceLtv,
		LiquidationLtv: liquidationLtv,
		NodeOperator:   nodeOperator,
		Mint:           mint,
	}, nil
}

// DeserializePosition deserializes a position account from raw bytes
func DeserializePosition(data []byte) (*Position, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("data too short for position account")
	}

	// Check discriminator (first 8 bytes)
	discriminator := data[:8]
	expectedDiscriminator := []byte{0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f, 0x90, 0x93} // Example discriminator
	if !bytesEqual(discriminator, expectedDiscriminator) {
		return nil, fmt.Errorf("invalid position account discriminator")
	}

	offset := 8
	if offset+32 > len(data) {
		return nil, fmt.Errorf("data too short for pool")
	}
	pool := solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for close status recall timestamp")
	}
	closeStatusRecallTimestamp := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for amount")
	}
	amount := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for user paid")
	}
	userPaid := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for collateral amount")
	}
	collateralAmount := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for timestamp")
	}
	timestamp := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8

	if offset+32 > len(data) {
		return nil, fmt.Errorf("data too short for trader")
	}
	trader := solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	if offset+32 > len(data) {
		return nil, fmt.Errorf("data too short for seed")
	}
	seed := solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for close timestamp")
	}
	closeTimestamp := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for closing position size")
	}
	closingPositionSize := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset+1 > len(data) {
		return nil, fmt.Errorf("data too short for interest rate")
	}
	interestRate := data[offset]
	offset++

	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for last interest collect")
	}
	lastInterestCollect := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8

	return &Position{
		Pool:                       pool,
		CloseStatusRecallTimestamp: closeStatusRecallTimestamp,
		Amount:                     amount,
		UserPaid:                   userPaid,
		CollateralAmount:           collateralAmount,
		Timestamp:                  timestamp,
		Trader:                     trader,
		Seed:                       seed,
		CloseTimestamp:             closeTimestamp,
		ClosingPositionSize:        closingPositionSize,
		InterestRate:               interestRate,
		LastInterestCollect:        lastInterestCollect,
	}, nil
}

// ToMarginOffer converts a Pool to a MarginOffer
func (p *Pool) ToMarginOffer() *MarginOffer {
	offer := &MarginOffer{
		ID:                    p.NodeWallet.String(),
		OfferType:             OfferTypeLV, // Leverage pools are LV type
		CollateralToken:       p.CollateralType.String(),
		BorrowToken:           p.QtType.String(),
		AvailableBorrowAmount: float64(p.MaxBorrow-p.CurrentExposure) / 1e9, // Assuming 9 decimals
		MaxOpenLTV:            float64(80),                                  // Default LTV for leverage pools
		LiquidationLTV:        float64(90),                                  // Default liquidation LTV
		InterestRate:          float64(p.InterestRate) / 100,                // Convert from basis points
		InterestModel:         InterestModelFloating,                        // Leverage pools typically use floating rates
		LiquiditySource:       "leverage",
		CreatedTimestamp:      time.Now(), // We don't have this in the pool data
		UpdatedTimestamp:      time.Now(), // We don't have this in the pool data
	}

	return offer
}

// Helper function to compare byte slices
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
