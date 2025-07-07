package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/piske-alex/margin-offer-system/types"
)

// CreateOrUpdate performs an upsert operation (create if doesn't exist, update if exists)
func (ms *MemoryStore) CreateOrUpdate(ctx context.Context, offer *types.MarginOffer) error {
	if offer == nil {
		return types.ErrInvalidEventData
	}

	if err := offer.Validate(); err != nil {
		return err
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	return ms.createOrUpdateInternal(offer)
}

// BulkCreateOrUpdate performs bulk upsert operations
func (ms *MemoryStore) BulkCreateOrUpdate(ctx context.Context, offers []*types.MarginOffer) error {
	if len(offers) == 0 {
		return nil
	}

	// Validate all offers first
	for _, offer := range offers {
		if offer == nil {
			return types.ErrInvalidEventData
		}
		if err := offer.Validate(); err != nil {
			return err
		}
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Process all offers
	for _, offer := range offers {
		if err := ms.createOrUpdateInternal(offer); err != nil {
			return err
		}
	}

	ms.lastUpdate = time.Now().UTC()
	return nil
}

// BulkOverwrite replaces the entire dataset with new offers
func (ms *MemoryStore) BulkOverwrite(ctx context.Context, offers []*types.MarginOffer) error {
	return ms.BulkOverwriteByFilter(ctx, offers, nil)
}

// BulkOverwriteByFilter replaces offers matching the filter criteria with new offers
func (ms *MemoryStore) BulkOverwriteByFilter(ctx context.Context, offers []*types.MarginOffer, filter *types.OverwriteFilter) error {
	// Validate new offers first
	for _, offer := range offers {
		if offer == nil {
			return types.ErrInvalidEventData
		}
		if err := offer.Validate(); err != nil {
			return err
		}
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Step 1: Identify offers to delete based on filter
	offersToDelete := ms.identifyOffersToDelete(filter)

	// Step 2: Remove identified offers
	for _, offer := range offersToDelete {
		ms.removeFromIndexes(offer)
		delete(ms.offers, offer.ID)
	}

	// Step 3: Insert new offers
	now := time.Now().UTC()
	operationID := uuid.New().String()

	for _, offer := range offers {
		// Set timestamps for new offers
		if offer.CreatedTimestamp.IsZero() {
			offer.CreatedTimestamp = now
		}
		offer.UpdatedTimestamp = now

		// Store the offer
		ms.offers[offer.ID] = offer.Clone()

		// Update indexes
		ms.addToIndexes(offer)
	}

	// Send notifications for all overwritten offers
	if ms.notificationService != nil {
		for _, offer := range offers {
			ms.notificationService.NotifyChange(&ChangeEvent{
				ChangeType:  ChangeTypeOverwritten,
				Offer:       offer,
				Timestamp:   now,
				Source:      "backfiller",
				OperationID: operationID,
				Metadata:    map[string]string{"method": "BulkOverwriteByFilter", "total_offers": fmt.Sprintf("%d", len(offers))},
			})
		}
	}

	ms.lastUpdate = now
	return nil
}

// createOrUpdateInternal performs the core upsert logic (requires lock to be held)
func (ms *MemoryStore) createOrUpdateInternal(offer *types.MarginOffer) error {
	now := time.Now().UTC()

	// Check if offer exists
	if existingOffer, exists := ms.offers[offer.ID]; exists {
		// Update existing offer

		// Remove from old indexes
		ms.removeFromIndexes(existingOffer)

		// Preserve original creation timestamp
		offer.CreatedTimestamp = existingOffer.CreatedTimestamp
		offer.UpdatedTimestamp = now

		// Store updated offer
		ms.offers[offer.ID] = offer.Clone()

		// Add to new indexes
		ms.addToIndexes(offer)
	} else {
		// Create new offer

		// Set timestamps for new offer
		if offer.CreatedTimestamp.IsZero() {
			offer.CreatedTimestamp = now
		}
		offer.UpdatedTimestamp = now

		// Store the offer
		ms.offers[offer.ID] = offer.Clone()

		// Update indexes
		ms.addToIndexes(offer)
	}

	ms.lastUpdate = now
	return nil
}

// identifyOffersToDelete returns offers that match the filter criteria
func (ms *MemoryStore) identifyOffersToDelete(filter *types.OverwriteFilter) []*types.MarginOffer {
	if filter == nil {
		// No filter means delete all offers
		var allOffers []*types.MarginOffer
		for _, offer := range ms.offers {
			allOffers = append(allOffers, offer)
		}
		return allOffers
	}

	// Apply filter criteria
	var matchingOffers []*types.MarginOffer
	for _, offer := range ms.offers {
		if ms.matchesOverwriteFilter(offer, filter) {
			matchingOffers = append(matchingOffers, offer)
		}
	}

	return matchingOffers
}

// matchesOverwriteFilter checks if an offer matches the overwrite filter criteria
func (ms *MemoryStore) matchesOverwriteFilter(offer *types.MarginOffer, filter *types.OverwriteFilter) bool {
	// Offer type filter
	if filter.OfferType != nil && offer.OfferType != *filter.OfferType {
		return false
	}

	// Token filters
	if filter.CollateralToken != nil && offer.CollateralToken != *filter.CollateralToken {
		return false
	}
	if filter.BorrowToken != nil && offer.BorrowToken != *filter.BorrowToken {
		return false
	}

	// Lender address filter
	if filter.LenderAddress != nil {
		if offer.LenderAddress == nil || *offer.LenderAddress != *filter.LenderAddress {
			return false
		}
	}

	// Liquidity source filter
	if filter.LiquiditySource != nil && offer.LiquiditySource != *filter.LiquiditySource {
		return false
	}

	// Time filters
	if filter.CreatedAfter != nil && offer.CreatedTimestamp.Before(*filter.CreatedAfter) {
		return false
	}
	if filter.CreatedBefore != nil && offer.CreatedTimestamp.After(*filter.CreatedBefore) {
		return false
	}
	if filter.UpdatedAfter != nil && offer.UpdatedTimestamp.Before(*filter.UpdatedAfter) {
		return false
	}
	if filter.UpdatedBefore != nil && offer.UpdatedTimestamp.After(*filter.UpdatedBefore) {
		return false
	}

	return true
}

// GetOverwriteStats returns statistics about what would be affected by an overwrite operation
func (ms *MemoryStore) GetOverwriteStats(ctx context.Context, filter *types.OverwriteFilter) (int64, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	offersToDelete := ms.identifyOffersToDelete(filter)
	return int64(len(offersToDelete)), nil
}

// PreviewOverwrite returns the IDs of offers that would be deleted by an overwrite operation
func (ms *MemoryStore) PreviewOverwrite(ctx context.Context, filter *types.OverwriteFilter) ([]string, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	offersToDelete := ms.identifyOffersToDelete(filter)
	ids := make([]string, len(offersToDelete))
	for i, offer := range offersToDelete {
		ids[i] = offer.ID
	}

	return ids, nil
}
