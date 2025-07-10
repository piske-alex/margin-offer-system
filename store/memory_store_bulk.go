package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	operationID := uuid.New().String()

	// Collect all events for batch notification
	var events []*ChangeEvent

	// Add all offers
	for _, offer := range offers {
		// Set timestamps
		offer.CreatedTimestamp = now
		offer.UpdatedTimestamp = now

		// Store the offer
		ms.offers[offer.ID] = offer.Clone()

		// Update indexes
		ms.addToIndexes(offer)

		// Prepare notification event
		events = append(events, &ChangeEvent{
			ChangeType:  ChangeTypeCreated,
			Offer:       offer,
			Timestamp:   now,
			Source:      "bulk_create",
			OperationID: operationID,
			Metadata:    map[string]string{"method": "BulkCreate", "total_offers": fmt.Sprintf("%d", len(offers))},
		})
	}

	// Send notifications in batch
	if ms.notificationService != nil && len(events) > 0 {
		if len(events) > 10000 {
			// Use large batch method for very large operations
			ms.notificationService.NotifyLargeBatch(events)
		} else {
			// Use regular batch method for smaller operations
			ms.notificationService.NotifyBatch(events)
		}
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
	operationID := uuid.New().String()

	// Collect all events for batch notification
	var events []*ChangeEvent

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

		// Prepare notification event
		events = append(events, &ChangeEvent{
			ChangeType:  ChangeTypeUpdated,
			Offer:       offer,
			Timestamp:   now,
			Source:      "bulk_update",
			OperationID: operationID,
			Metadata:    map[string]string{"method": "BulkUpdate", "total_offers": fmt.Sprintf("%d", len(offers))},
		})
	}

	// Send notifications in batch
	if ms.notificationService != nil && len(events) > 0 {
		if len(events) > 10000 {
			// Use large batch method for very large operations
			ms.notificationService.NotifyLargeBatch(events)
		} else {
			// Use regular batch method for smaller operations
			ms.notificationService.NotifyBatch(events)
		}
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

	now := time.Now().UTC()
	operationID := uuid.New().String()

	// Collect all events for batch notification
	var events []*ChangeEvent

	// Delete all offers
	for i, id := range ids {
		// Remove from indexes
		ms.removeFromIndexes(offersToDelete[i])

		// Delete from main storage
		delete(ms.offers, id)

		// Prepare notification event
		events = append(events, &ChangeEvent{
			ChangeType:  ChangeTypeDeleted,
			Offer:       offersToDelete[i],
			Timestamp:   now,
			Source:      "bulk_delete",
			OperationID: operationID,
			Metadata:    map[string]string{"method": "BulkDelete", "total_offers": fmt.Sprintf("%d", len(ids))},
		})
	}

	// Send notifications in batch
	if ms.notificationService != nil && len(events) > 0 {
		if len(events) > 10000 {
			// Use large batch method for very large operations
			ms.notificationService.NotifyLargeBatch(events)
		} else {
			// Use regular batch method for smaller operations
			ms.notificationService.NotifyBatch(events)
		}
	}

	ms.lastUpdate = now
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

// GetTotalLiquidity calculates the total liquidity for a specific borrow token
func (s *MemoryStore) GetTotalLiquidity(ctx context.Context, borrowToken string) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalLiquidity float64
	for _, offer := range s.offers {
		if offer.BorrowToken == borrowToken {
			totalLiquidity += offer.AvailableBorrowAmount
		}
	}

	return totalLiquidity, nil
}
