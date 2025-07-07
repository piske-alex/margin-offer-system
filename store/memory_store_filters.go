package store

import (
	"sort"
	"strings"

	"github.com/piske-alex/margin-offer-system/types"
)

// matchesFilters checks if an offer matches the given filters
func (ms *MemoryStore) matchesFilters(offer *types.MarginOffer, req *types.ListRequest) bool {
	// Offer type filter
	if req.OfferType != nil && offer.OfferType != *req.OfferType {
		return false
	}

	// Token filters
	if req.CollateralToken != nil && offer.CollateralToken != *req.CollateralToken {
		return false
	}
	if req.BorrowToken != nil && offer.BorrowToken != *req.BorrowToken {
		return false
	}

	// Interest model filter
	if req.InterestModel != nil && offer.InterestModel != *req.InterestModel {
		return false
	}

	// Liquidity source filter
	if req.LiquiditySource != nil && offer.LiquiditySource != *req.LiquiditySource {
		return false
	}

	// Lender address filter
	if req.LenderAddress != nil {
		if offer.LenderAddress == nil || *offer.LenderAddress != *req.LenderAddress {
			return false
		}
	}

	// Range filters
	if req.MinBorrowAmount != nil && offer.AvailableBorrowAmount < *req.MinBorrowAmount {
		return false
	}
	if req.MaxBorrowAmount != nil && offer.AvailableBorrowAmount > *req.MaxBorrowAmount {
		return false
	}

	if req.MinInterestRate != nil && offer.InterestRate < *req.MinInterestRate {
		return false
	}
	if req.MaxInterestRate != nil && offer.InterestRate > *req.MaxInterestRate {
		return false
	}

	if req.MinMaxOpenLTV != nil && offer.MaxOpenLTV < *req.MinMaxOpenLTV {
		return false
	}
	if req.MaxMaxOpenLTV != nil && offer.MaxOpenLTV > *req.MaxMaxOpenLTV {
		return false
	}

	// Time filters
	if req.CreatedAfter != nil && offer.CreatedTimestamp.Before(*req.CreatedAfter) {
		return false
	}
	if req.CreatedBefore != nil && offer.CreatedTimestamp.After(*req.CreatedBefore) {
		return false
	}
	if req.UpdatedAfter != nil && offer.UpdatedTimestamp.Before(*req.UpdatedAfter) {
		return false
	}
	if req.UpdatedBefore != nil && offer.UpdatedTimestamp.After(*req.UpdatedBefore) {
		return false
	}

	// Status filters
	if !req.IncludeExpired && offer.IsExpired() {
		return false
	}

	if req.ActiveOnly {
		// Consider an offer active if it has available borrow amount and is not expired
		if offer.AvailableBorrowAmount <= 0 || offer.IsExpired() {
			return false
		}
	}

	return true
}

// sortOffers sorts the offers based on the given criteria
func (ms *MemoryStore) sortOffers(offers []*types.MarginOffer, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "created_timestamp"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	ascending := strings.ToLower(sortOrder) == "asc"

	sort.Slice(offers, func(i, j int) bool {
		var less bool

		switch strings.ToLower(sortBy) {
		case "created_timestamp":
			less = offers[i].CreatedTimestamp.Before(offers[j].CreatedTimestamp)
		case "updated_timestamp":
			less = offers[i].UpdatedTimestamp.Before(offers[j].UpdatedTimestamp)
		case "interest_rate":
			less = offers[i].InterestRate < offers[j].InterestRate
		case "available_borrow_amount":
			less = offers[i].AvailableBorrowAmount < offers[j].AvailableBorrowAmount
		case "max_open_ltv":
			less = offers[i].MaxOpenLTV < offers[j].MaxOpenLTV
		case "liquidation_ltv":
			less = offers[i].LiquidationLTV < offers[j].LiquidationLTV
		case "offer_type":
			less = string(offers[i].OfferType) < string(offers[j].OfferType)
		case "collateral_token":
			less = offers[i].CollateralToken < offers[j].CollateralToken
		case "borrow_token":
			less = offers[i].BorrowToken < offers[j].BorrowToken
		default:
			// Default to ID sorting
			less = offers[i].ID < offers[j].ID
		}

		if ascending {
			return less
		}
		return !less
	})
}
