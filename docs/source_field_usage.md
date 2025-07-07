# Source Field Usage Guide

## Overview

The `source` field has been added to the `MarginOffer` message to track the origin of margin offers. This field is optional and can be used to identify which system or process created the offer (e.g., "marginfi", "jupiter", "manual").

## Field Definition

### MarginOffer
- **Field**: `source` (optional string)
- **Description**: Identifies the source/origin of the margin offer
- **Examples**: "marginfi", "jupiter", "manual", "api", "backfiller"

### OverwriteFilter
- **Field**: `source` (optional string)
- **Description**: Filters margin offers by source when performing bulk overwrite operations

## Usage Examples

### 1. Creating Margin Offers with Source

```go
// Create a MarginFi offer
marginfiSource := "marginfi"
marginfiOffer := &pb.MarginOffer{
    Id:                    "marginfi_sol_usdc_001",
    OfferType:             "V1",
    CollateralToken:       "SOL",
    BorrowToken:           "USDC",
    AvailableBorrowAmount: 1000000.0,
    MaxOpenLtv:            0.75,
    LiquidationLtv:        0.85,
    InterestRate:          0.05,
    InterestModel:         "floating",
    LiquiditySource:       "marginfi",
    Source:                &marginfiSource,  // Set the source
    CreatedTimestamp:      timestamppb.Now(),
    UpdatedTimestamp:      timestamppb.Now(),
}
```

### 2. Filtering Offers by Source

```go
// List only MarginFi offers
marginfiFilter := "marginfi"
listResp, err := client.ListMarginOffers(context.Background(), &pb.ListMarginOffersRequest{
    Limit:  10,
    Source: &marginfiFilter,
})
```

### 3. Bulk Overwrite by Source

```go
// Overwrite only MarginFi offers
overwriteResp, err := client.BulkOverwriteMarginOffers(context.Background(), &pb.BulkOverwriteMarginOffersRequest{
    Offers: newMarginfiOffers,
    Filter: &pb.OverwriteFilter{
        Source: &marginfiSource,  // Only affect MarginFi offers
    },
})
```

### 4. Preview Overwrite by Source

```go
// Preview what would be affected
previewResp, err := client.OverwritePreview(context.Background(), &pb.OverwritePreviewRequest{
    Filter: &pb.OverwriteFilter{
        Source: &jupiterSource,  // Preview Jupiter offers
    },
})
```

## TypeScript Usage

### Creating Offers with Source

```typescript
const marginfiOffer: MarginOffer = {
    id: "marginfi_sol_usdc_001",
    offerType: "V1",
    collateralToken: "SOL",
    borrowToken: "USDC",
    availableBorrowAmount: 1000000.0,
    maxOpenLtv: 0.75,
    liquidationLtv: 0.85,
    interestRate: 0.05,
    interestModel: "floating",
    liquiditySource: "marginfi",
    source: "marginfi",  // Set the source
    createdTimestamp: new Date(),
    updatedTimestamp: new Date(),
};
```

### Converting Banks to Margin Offers

```typescript
private convertBanksToMarginOffers(banks: MarginFiBank[]): MarginOffer[] {
    const now = new Date();
    
    return banks.map(bank => ({
        id: `marginfi_${bank.address}`,
        offerType: 'V1',
        collateralToken: bank.collateralToken,
        borrowToken: bank.borrowToken,
        availableBorrowAmount: bank.availableLiquidity,
        maxOpenLtv: bank.maxLtv,
        liquidationLtv: bank.liquidationLtv,
        interestRate: bank.interestRate,
        interestModel: bank.interestModel,
        liquiditySource: 'marginfi',
        source: 'marginfi',  // Set source for MarginFi offers
        createdTimestamp: now,
        updatedTimestamp: now,
    }));
}
```

## Go Types

The Go `MarginOffer` struct has been updated to include the `Source` field:

```go
type MarginOffer struct {
    // ... existing fields ...
    Source *string `json:"source,omitempty" bson:"source,omitempty"`
}
```

The `Clone()` method has also been updated to handle the `Source` field properly.

## Subscription Filtering

The `SubscribeMarginOffersRequest` also supports filtering by source:

```go
// Subscribe to MarginFi offer changes only
subscribeReq := &pb.SubscribeMarginOffersRequest{
    Source: &marginfiSource,
    SubscribeCreated: true,
    SubscribeUpdated: true,
    SubscribeOverwritten: true,
    ClientId: "my-client",
    SessionId: "session-123",
}
```

## Best Practices

1. **Consistent Naming**: Use consistent source names across your system (e.g., "marginfi", "jupiter", "manual")

2. **Source Identification**: Always set the source when creating offers to track their origin

3. **Filtered Operations**: Use source filters when performing bulk operations to avoid affecting offers from other sources

4. **Monitoring**: Use source information for monitoring and analytics to understand which sources are most active

5. **Testing**: Test source filtering with different combinations to ensure proper isolation

## Migration Notes

- The `source` field is optional, so existing offers without this field will continue to work
- When reading offers, always check if `Source` is nil before accessing its value
- The field is available in all filtering operations (List, Overwrite, Subscribe)

## Example Implementation

See `examples/source_field_example.go` for a complete working example that demonstrates:
- Creating offers with different sources
- Filtering offers by source
- Bulk overwrite operations with source filters
- Preview operations
- Verification of results 