package store

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/piske-alex/margin-offer-system/types"
)

// MemoryStore implements an in-memory storage for margin offers
type MemoryStore struct {
	mu      sync.RWMutex
	offers  map[string]*types.MarginOffer
	logger  types.Logger
	metrics types.MetricsCollector

	// Indexes for faster queries
	lenderIndex    map[string][]string          // lender -> offer IDs
	tokenIndex     map[string][]string          // token pair -> offer IDs
	offerTypeIndex map[types.OfferType][]string // offer type -> offer IDs

	// Metrics
	createdAt  time.Time
	lastUpdate time.Time

	// Notification service for change events
	notificationService *NotificationService
}

// NewMemoryStore creates a new memory-based margin offer store
func NewMemoryStore(logger types.Logger, metrics types.MetricsCollector) *MemoryStore {
	store := &MemoryStore{
		offers:              make(map[string]*types.MarginOffer),
		lenderIndex:         make(map[string][]string),
		tokenIndex:          make(map[string][]string),
		offerTypeIndex:      make(map[types.OfferType][]string),
		logger:              logger,
		metrics:             metrics,
		notificationService: NewNotificationService(logger),
	}

	// Start the notification service
	if err := store.notificationService.Start(); err != nil {
		logger.Error("Failed to start notification service", "error", err)
	}

	return store
}

// Create adds a new margin offer to the store
func (ms *MemoryStore) Create(ctx context.Context, offer *types.MarginOffer) error {
	if offer == nil {
		return types.ErrInvalidEventData
	}

	if err := offer.Validate(); err != nil {
		return err
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Check if offer already exists
	if _, exists := ms.offers[offer.ID]; exists {
		return types.ErrOfferAlreadyExists
	}

	// Set timestamps
	now := time.Now().UTC()
	offer.CreatedTimestamp = now
	offer.UpdatedTimestamp = now

	// Store the offer
	ms.offers[offer.ID] = offer.Clone()

	// Update indexes
	ms.addToIndexes(offer)
	ms.lastUpdate = now

	// Send notification
	if ms.notificationService != nil {
		ms.notificationService.NotifyChange(&ChangeEvent{
			ChangeType:  ChangeTypeCreated,
			Offer:       offer,
			Timestamp:   now,
			Source:      "api",
			OperationID: uuid.New().String(),
			Metadata:    map[string]string{"method": "Create"},
		})
	}

	return nil
}

// GetByID retrieves a margin offer by its ID
func (ms *MemoryStore) GetByID(ctx context.Context, id string) (*types.MarginOffer, error) {
	if id == "" {
		return nil, types.ErrMissingID
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	offer, exists := ms.offers[id]
	if !exists {
		return nil, types.ErrOfferNotFound
	}

	return offer.Clone(), nil
}

// Update modifies an existing margin offer
func (ms *MemoryStore) Update(ctx context.Context, offer *types.MarginOffer) error {
	if offer == nil {
		return types.ErrInvalidEventData
	}

	if err := offer.Validate(); err != nil {
		return err
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Check if offer exists
	oldOffer, exists := ms.offers[offer.ID]
	if !exists {
		return types.ErrOfferNotFound
	}

	// Remove from old indexes
	ms.removeFromIndexes(oldOffer)

	// Update timestamp and store
	offer.UpdateTimestamp()
	ms.offers[offer.ID] = offer.Clone()

	// Add to new indexes
	ms.addToIndexes(offer)
	ms.lastUpdate = time.Now().UTC()

	// Send notification
	if ms.notificationService != nil {
		ms.notificationService.NotifyChange(&ChangeEvent{
			ChangeType:  ChangeTypeUpdated,
			Offer:       offer,
			Timestamp:   time.Now().UTC(),
			Source:      "api",
			OperationID: uuid.New().String(),
			Metadata:    map[string]string{"method": "Update"},
		})
	}

	return nil
}

// Delete removes a margin offer from the store
func (ms *MemoryStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return types.ErrMissingID
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	offer, exists := ms.offers[id]
	if !exists {
		return types.ErrOfferNotFound
	}

	// Remove from indexes
	ms.removeFromIndexes(offer)

	// Delete from main storage
	delete(ms.offers, id)
	ms.lastUpdate = time.Now().UTC()

	// Send notification
	if ms.notificationService != nil {
		ms.notificationService.NotifyChange(&ChangeEvent{
			ChangeType:  ChangeTypeDeleted,
			Offer:       offer,
			Timestamp:   time.Now().UTC(),
			Source:      "api",
			OperationID: uuid.New().String(),
			Metadata:    map[string]string{"method": "Delete"},
		})
	}

	return nil
}

// List retrieves margin offers with pagination and filtering
func (ms *MemoryStore) List(ctx context.Context, req *types.ListRequest) ([]*types.MarginOffer, error) {
	if req == nil {
		req = &types.ListRequest{Limit: 100, Offset: 0}
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Get all offers that match filters
	var offers []*types.MarginOffer
	for _, offer := range ms.offers {
		if ms.matchesFilters(offer, req) {
			offers = append(offers, offer.Clone())
		}
	}

	// Sort offers
	ms.sortOffers(offers, req.SortBy, req.SortOrder)

	// Apply pagination
	start := int(req.Offset)
	end := start + int(req.Limit)

	if start >= len(offers) {
		return []*types.MarginOffer{}, nil
	}

	if end > len(offers) {
		end = len(offers)
	}

	return offers[start:end], nil
}

// Helper methods for indexes
func (ms *MemoryStore) addToIndexes(offer *types.MarginOffer) {
	// Lender index
	if offer.LenderAddress != nil {
		lender := *offer.LenderAddress
		ms.lenderIndex[lender] = append(ms.lenderIndex[lender], offer.ID)
	}

	// Token pair index
	tokenPair := offer.CollateralToken + "|" + offer.BorrowToken
	ms.tokenIndex[tokenPair] = append(ms.tokenIndex[tokenPair], offer.ID)

	// Offer type index
	ms.offerTypeIndex[offer.OfferType] = append(ms.offerTypeIndex[offer.OfferType], offer.ID)
}

func (ms *MemoryStore) removeFromIndexes(offer *types.MarginOffer) {
	// Lender index
	if offer.LenderAddress != nil {
		lender := *offer.LenderAddress
		ms.lenderIndex[lender] = ms.removeFromSlice(ms.lenderIndex[lender], offer.ID)
		if len(ms.lenderIndex[lender]) == 0 {
			delete(ms.lenderIndex, lender)
		}
	}

	// Token pair index
	tokenPair := offer.CollateralToken + "|" + offer.BorrowToken
	ms.tokenIndex[tokenPair] = ms.removeFromSlice(ms.tokenIndex[tokenPair], offer.ID)
	if len(ms.tokenIndex[tokenPair]) == 0 {
		delete(ms.tokenIndex, tokenPair)
	}

	// Offer type index
	ms.offerTypeIndex[offer.OfferType] = ms.removeFromSlice(ms.offerTypeIndex[offer.OfferType], offer.ID)
	if len(ms.offerTypeIndex[offer.OfferType]) == 0 {
		delete(ms.offerTypeIndex, offer.OfferType)
	}
}

func (ms *MemoryStore) removeFromSlice(slice []string, item string) []string {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// HealthCheck performs a health check on the store
func (ms *MemoryStore) HealthCheck(ctx context.Context) error {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Basic health checks
	if ms.offers == nil {
		return types.ErrStoreNotInitialized
	}

	return nil
}

// Close cleans up the store
func (ms *MemoryStore) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.offers = nil
	ms.lenderIndex = nil
	ms.tokenIndex = nil
	ms.offerTypeIndex = nil

	return nil
}
