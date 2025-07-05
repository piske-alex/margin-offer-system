package store

import (
	"context"

	"github.com/piske-alex/margin-offer-system/types"
)

// ListByLender retrieves margin offers for a specific lender
func (ms *MemoryStore) ListByLender(ctx context.Context, lenderAddress string, req *types.ListRequest) ([]*types.MarginOffer, error) {
	if lenderAddress == "" {
		return nil, types.ErrInvalidEventData
	}
	
	if req == nil {
		req = &types.ListRequest{Limit: 100, Offset: 0}
	}
	
	// Add lender filter to request
	req.LenderAddress = &lenderAddress
	
	return ms.List(ctx, req)
}

// ListByToken retrieves margin offers for a specific token pair
func (ms *MemoryStore) ListByToken(ctx context.Context, collateralToken, borrowToken string, req *types.ListRequest) ([]*types.MarginOffer, error) {
	if collateralToken == "" || borrowToken == "" {
		return nil, types.ErrInvalidEventData
	}
	
	if req == nil {
		req = &types.ListRequest{Limit: 100, Offset: 0}
	}
	
	// Add token filters to request
	req.CollateralToken = &collateralToken
	req.BorrowToken = &borrowToken
	
	return ms.List(ctx, req)
}

// ListByOfferType retrieves margin offers of a specific type
func (ms *MemoryStore) ListByOfferType(ctx context.Context, offerType types.OfferType, req *types.ListRequest) ([]*types.MarginOffer, error) {
	if req == nil {
		req = &types.ListRequest{Limit: 100, Offset: 0}
	}
	
	// Add offer type filter to request
	req.OfferType = &offerType
	
	return ms.List(ctx, req)
}

// GetOffersByLenderFromIndex uses the lender index for faster lookups
func (ms *MemoryStore) GetOffersByLenderFromIndex(ctx context.Context, lenderAddress string) ([]*types.MarginOffer, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	offerIDs, exists := ms.lenderIndex[lenderAddress]
	if !exists {
		return []*types.MarginOffer{}, nil
	}
	
	offers := make([]*types.MarginOffer, 0, len(offerIDs))
	for _, id := range offerIDs {
		if offer, exists := ms.offers[id]; exists {
			offers = append(offers, offer.Clone())
		}
	}
	
	return offers, nil
}

// GetOffersByTokenFromIndex uses the token index for faster lookups
func (ms *MemoryStore) GetOffersByTokenFromIndex(ctx context.Context, collateralToken, borrowToken string) ([]*types.MarginOffer, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	tokenPair := collateralToken + "|" + borrowToken
	offerIDs, exists := ms.tokenIndex[tokenPair]
	if !exists {
		return []*types.MarginOffer{}, nil
	}
	
	offers := make([]*types.MarginOffer, 0, len(offerIDs))
	for _, id := range offerIDs {
		if offer, exists := ms.offers[id]; exists {
			offers = append(offers, offer.Clone())
		}
	}
	
	return offers, nil
}

// GetOffersByTypeFromIndex uses the offer type index for faster lookups
func (ms *MemoryStore) GetOffersByTypeFromIndex(ctx context.Context, offerType types.OfferType) ([]*types.MarginOffer, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	offerIDs, exists := ms.offerTypeIndex[offerType]
	if !exists {
		return []*types.MarginOffer{}, nil
	}
	
	offers := make([]*types.MarginOffer, 0, len(offerIDs))
	for _, id := range offerIDs {
		if offer, exists := ms.offers[id]; exists {
			offers = append(offers, offer.Clone())
		}
	}
	
	return offers, nil
}