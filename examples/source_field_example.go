package main

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/piske-alex/margin-offer-system/proto/gen/go/proto"
)

func sourceFieldExample() {
	// Connect to the store service
	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewMarginOfferServiceClient(conn)

	// Example 1: Create margin offers with different sources
	fmt.Println("=== Creating margin offers with different sources ===")

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
		Source:                &marginfiSource,
		CreatedTimestamp:      timestamppb.Now(),
		UpdatedTimestamp:      timestamppb.Now(),
	}

	// Create a Jupiter offer
	jupiterSource := "jupiter"
	jupiterOffer := &pb.MarginOffer{
		Id:                    "jupiter_eth_usdc_001",
		OfferType:             "V1",
		CollateralToken:       "ETH",
		BorrowToken:           "USDC",
		AvailableBorrowAmount: 500000.0,
		MaxOpenLtv:            0.70,
		LiquidationLtv:        0.80,
		InterestRate:          0.06,
		InterestModel:         "floating",
		LiquiditySource:       "jupiter",
		Source:                &jupiterSource,
		CreatedTimestamp:      timestamppb.Now(),
		UpdatedTimestamp:      timestamppb.Now(),
	}

	// Create a manual offer
	manualSource := "manual"
	manualOffer := &pb.MarginOffer{
		Id:                    "manual_btc_usdc_001",
		OfferType:             "V1",
		CollateralToken:       "BTC",
		BorrowToken:           "USDC",
		AvailableBorrowAmount: 750000.0,
		MaxOpenLtv:            0.65,
		LiquidationLtv:        0.75,
		InterestRate:          0.04,
		InterestModel:         "fixed",
		LiquiditySource:       "manual",
		Source:                &manualSource,
		CreatedTimestamp:      timestamppb.Now(),
		UpdatedTimestamp:      timestamppb.Now(),
	}

	// Bulk create the offers
	createResp, err := client.BulkCreateMarginOffers(context.Background(), &pb.BulkCreateMarginOffersRequest{
		Offers: []*pb.MarginOffer{marginfiOffer, jupiterOffer, manualOffer},
	})
	if err != nil {
		log.Fatalf("Failed to create offers: %v", err)
	}
	fmt.Printf("Created %d offers successfully\n", createResp.SuccessCount)

	// Example 2: List all offers
	fmt.Println("\n=== Listing all offers ===")
	listResp, err := client.ListMarginOffers(context.Background(), &pb.ListMarginOffersRequest{
		Limit: 10,
	})
	if err != nil {
		log.Fatalf("Failed to list offers: %v", err)
	}
	fmt.Printf("Found %d offers:\n", len(listResp.Offers))
	for _, offer := range listResp.Offers {
		source := "unknown"
		if offer.Source != nil {
			source = *offer.Source
		}
		fmt.Printf("- %s: %s/%s (source: %s)\n", offer.Id, offer.CollateralToken, offer.BorrowToken, source)
	}

	// Example 3: Filter offers by source using ListMarginOffers
	fmt.Println("\n=== Filtering offers by source (marginfi) ===")
	marginfiFilter := "marginfi"
	marginfiListResp, err := client.ListMarginOffers(context.Background(), &pb.ListMarginOffersRequest{
		Limit:  10,
		Source: &marginfiFilter,
	})
	if err != nil {
		log.Fatalf("Failed to list MarginFi offers: %v", err)
	}
	fmt.Printf("Found %d MarginFi offers:\n", len(marginfiListResp.Offers))
	for _, offer := range marginfiListResp.Offers {
		fmt.Printf("- %s: %s/%s\n", offer.Id, offer.CollateralToken, offer.BorrowToken)
	}

	// Example 4: Use OverwriteFilter to overwrite only MarginFi offers
	fmt.Println("\n=== Overwriting only MarginFi offers ===")

	// Create new MarginFi offers
	newMarginfiSource := "marginfi"
	newMarginfiOffers := []*pb.MarginOffer{
		{
			Id:                    "marginfi_sol_usdc_002",
			OfferType:             "V1",
			CollateralToken:       "SOL",
			BorrowToken:           "USDC",
			AvailableBorrowAmount: 1200000.0, // Updated amount
			MaxOpenLtv:            0.80,      // Updated LTV
			LiquidationLtv:        0.90,
			InterestRate:          0.045,
			InterestModel:         "floating",
			LiquiditySource:       "marginfi",
			Source:                &newMarginfiSource,
			CreatedTimestamp:      timestamppb.Now(),
			UpdatedTimestamp:      timestamppb.Now(),
		},
		{
			Id:                    "marginfi_eth_usdc_002",
			OfferType:             "V1",
			CollateralToken:       "ETH",
			BorrowToken:           "USDC",
			AvailableBorrowAmount: 800000.0,
			MaxOpenLtv:            0.75,
			LiquidationLtv:        0.85,
			InterestRate:          0.055,
			InterestModel:         "floating",
			LiquiditySource:       "marginfi",
			Source:                &newMarginfiSource,
			CreatedTimestamp:      timestamppb.Now(),
			UpdatedTimestamp:      timestamppb.Now(),
		},
	}

	// Overwrite only MarginFi offers using the source filter
	overwriteResp, err := client.BulkOverwriteMarginOffers(context.Background(), &pb.BulkOverwriteMarginOffersRequest{
		Offers: newMarginfiOffers,
		Filter: &pb.OverwriteFilter{
			Source: &marginfiSource,
		},
	})
	if err != nil {
		log.Fatalf("Failed to overwrite MarginFi offers: %v", err)
	}
	fmt.Printf("Overwrite completed: %d deleted, %d created\n", overwriteResp.DeletedCount, overwriteResp.CreatedCount)

	// Example 5: Verify the results
	fmt.Println("\n=== Verifying results after overwrite ===")
	finalListResp, err := client.ListMarginOffers(context.Background(), &pb.ListMarginOffersRequest{
		Limit: 10,
	})
	if err != nil {
		log.Fatalf("Failed to list final offers: %v", err)
	}
	fmt.Printf("Final state - %d offers:\n", len(finalListResp.Offers))
	for _, offer := range finalListResp.Offers {
		source := "unknown"
		if offer.Source != nil {
			source = *offer.Source
		}
		fmt.Printf("- %s: %s/%s (source: %s, amount: %.0f)\n",
			offer.Id, offer.CollateralToken, offer.BorrowToken, source, offer.AvailableBorrowAmount)
	}

	// Example 6: Preview what would be affected by an overwrite
	fmt.Println("\n=== Previewing overwrite operation ===")
	jupiterFilter := "jupiter"
	previewResp, err := client.OverwritePreview(context.Background(), &pb.OverwritePreviewRequest{
		Filter: &pb.OverwriteFilter{
			Source: &jupiterFilter,
		},
	})
	if err != nil {
		log.Fatalf("Failed to preview overwrite: %v", err)
	}
	fmt.Printf("Overwrite preview: %d offers would be affected\n", previewResp.AffectedCount)
	for _, id := range previewResp.AffectedIds {
		fmt.Printf("- Would affect: %s\n", id)
	}

	fmt.Println("\n=== Source field example completed ===")
}
