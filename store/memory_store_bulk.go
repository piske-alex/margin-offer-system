package store

import (
	"context"
	"time"

	"github.com/piske-alex/margin-offer-system/types"
)

// BulkCreate adds multiple margin offers to the store (fails if any already exist)
func (ms *MemoryStore) BulkCreate(ctx context.Context, offers []*types.MarginOffer) error {
	if len(offers) == 0 {
		return nil
	}
	
	// Validate all offers first
	for i, offer := range offers {
		if offer == nil {
			return types.ErrInvalidEventData
		}
		if err := offer.Validate(); err != nil {
			return err
		}
		// Check for duplicates within the batch
		for j := i + 1; j < len(offers); j++ {
			if offers[j] != nil && offer.ID == offers[j].ID {
				return types.ErrOfferAlreadyExists
			}
		}
	}
	
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	// Check for existing offers
	for _, offer := range offers {
		if _, exists := ms.offers[offer.ID]; exists {
			return types.ErrOfferAlreadyExists
		}
	}
	
	now := time.Now().UTC()
	
	// Add all offers
	for _, offer := range offers {
		// Set timestamps
		offer.CreatedTimestamp = now
		offer.UpdatedTimestamp = now
		
		// Store the offer
		ms.offers[offer.ID] = offer.Clone()
		
		// Update indexes
		ms.addToIndexes(offer)
	}
	
	ms.lastUpdate = now
	return nil
}

// BulkUpdate modifies multiple existing margin offers (fails if any don't exist)
func (ms *MemoryStore) BulkUpdate(ctx context.Context, offers []*types.MarginOffer) error {
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
	
	// Check that all offers exist
	oldOffers := make([]*types.MarginOffer, len(offers))
	for i, offer := range offers {
		oldOffer, exists := ms.offers[offer.ID]
		if !exists {
			return types.ErrOfferNotFound
		}
		oldOffers[i] = oldOffer
	}
	
	now := time.Now().UTC()
	
	// Update all offers
	for i, offer := range offers {
		// Remove from old indexes
		ms.removeFromIndexes(oldOffers[i])
		
		// Preserve creation timestamp and update timestamp
		offer.CreatedTimestamp = oldOffers[i].CreatedTimestamp
		offer.UpdatedTimestamp = now
		
		// Store the updated offer
		ms.offers[offer.ID] = offer.Clone()
		
		// Add to new indexes
		ms.addToIndexes(offer)
	}
	
	ms.lastUpdate = now
	return nil
}

// BulkDelete removes multiple margin offers from the store
func (ms *MemoryStore) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	// Check that all offers exist and collect them
	offersToDelete := make([]*types.MarginOffer, len(ids))
	for i, id := range ids {
		if id == "" {
			return types.ErrMissingID
		}
		offer, exists := ms.offers[id]
		if !exists {
			return types.ErrOfferNotFound
		}
		offersToDelete[i] = offer
	}
	
	// Delete all offers
	for i, id := range ids {
		// Remove from indexes
		ms.removeFromIndexes(offersToDelete[i])
		
		// Delete from main storage
		delete(ms.offers, id)
	}
	
	ms.lastUpdate = time.Now().UTC()
	return nil
}

// Count returns the total number of offers in the store
func (ms *MemoryStore) Count(ctx context.Context) (int64, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	return int64(len(ms.offers)), nil
}

// CountByOfferType returns the number of offers by offer type
func (ms *MemoryStore) CountByOfferType(ctx context.Context, offerType types.OfferType) (int64, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	ids, exists := ms.offerTypeIndex[offerType]
	if !exists {
		return 0, nil
	}
	
	return int64(len(ids)), nil
}

// GetTotalLiquidity returns the total available liquidity for a given borrow token
func (ms *MemoryStore) GetTotalLiquidity(ctx context.Context, borrowToken string) (float64, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	var totalLiquidity float64
	for _, offer := range ms.offers {
		if offer.BorrowToken == borrowToken && !offer.IsExpired() {
			totalLiquidity += offer.AvailableBorrowAmount
		}
	}
	
	return totalLiquidity, nil
}