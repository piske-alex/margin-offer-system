package store

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/piske-alex/margin-offer-system/pkg/logger"
	"github.com/piske-alex/margin-offer-system/pkg/metrics"
	pb "github.com/piske-alex/margin-offer-system/proto/gen/go/proto"
	"github.com/piske-alex/margin-offer-system/types"
)

func TestBulkOperationsSendNotifications(t *testing.T) {
	// Create store with notification service
	log := logger.NewLogger("test")
	metrics := metrics.NewMetricsCollector()
	store := NewMemoryStore(log, metrics)

	// Create test offers
	offers := []*types.MarginOffer{
		{
			ID:                    "offer1",
			OfferType:             types.OfferTypeV1,
			CollateralToken:       "SOL",
			BorrowToken:           "USDC",
			AvailableBorrowAmount: 1000.0,
			MaxOpenLTV:            0.8,
			LiquidationLTV:        0.9,
			InterestRate:          0.05,
			InterestModel:         types.InterestModelFixed,
			LiquiditySource:       "marginfi",
		},
		{
			ID:                    "offer2",
			OfferType:             types.OfferTypeBP,
			CollateralToken:       "ETH",
			BorrowToken:           "USDT",
			AvailableBorrowAmount: 500.0,
			MaxOpenLTV:            0.7,
			LiquidationLTV:        0.85,
			InterestRate:          0.06,
			InterestModel:         types.InterestModelFloating,
			LiquiditySource:       "jupiter",
		},
	}

	// Test BulkCreate notifications
	t.Run("BulkCreate sends notifications", func(t *testing.T) {
		// Subscribe to created events
		subscription, err := store.notificationService.Subscribe(&pb.SubscribeMarginOffersRequest{
			ClientId:         "test_client",
			SessionId:        "test_session",
			SubscribeCreated: true,
		})
		if err != nil {
			t.Fatalf("Failed to create subscription: %v", err)
		}
		defer store.notificationService.Unsubscribe(subscription.ID)

		// Create a channel to receive notifications
		notifications := make(chan *pb.SubscribeMarginOffersResponse, 10)
		go func() {
			for response := range subscription.Channel {
				notifications <- response
			}
		}()

		// Perform bulk create
		err = store.BulkCreate(context.Background(), offers)
		if err != nil {
			t.Fatalf("BulkCreate failed: %v", err)
		}

		// Wait for notifications
		timeout := time.After(2 * time.Second)
		receivedCount := 0
		for {
			select {
			case notification := <-notifications:
				if change := notification.GetChange(); change != nil {
					if change.ChangeType == pb.MarginOfferChange_CREATED {
						receivedCount++
					}
				}
			case <-timeout:
				break
			}
			if receivedCount >= len(offers) {
				break
			}
		}

		if receivedCount != len(offers) {
			t.Errorf("Expected %d notifications, got %d", len(offers), receivedCount)
		}
	})

	// Test BulkUpdate notifications
	t.Run("BulkUpdate sends notifications", func(t *testing.T) {
		// Subscribe to updated events
		subscription, err := store.notificationService.Subscribe(&pb.SubscribeMarginOffersRequest{
			ClientId:         "test_client",
			SessionId:        "test_session",
			SubscribeUpdated: true,
		})
		if err != nil {
			t.Fatalf("Failed to create subscription: %v", err)
		}
		defer store.notificationService.Unsubscribe(subscription.ID)

		// Create a channel to receive notifications
		notifications := make(chan *pb.SubscribeMarginOffersResponse, 10)
		go func() {
			for response := range subscription.Channel {
				notifications <- response
			}
		}()

		// Update offers
		for _, offer := range offers {
			offer.InterestRate += 0.01
		}

		// Perform bulk update
		err = store.BulkUpdate(context.Background(), offers)
		if err != nil {
			t.Fatalf("BulkUpdate failed: %v", err)
		}

		// Wait for notifications
		timeout := time.After(2 * time.Second)
		receivedCount := 0
		for {
			select {
			case notification := <-notifications:
				if change := notification.GetChange(); change != nil {
					if change.ChangeType == pb.MarginOfferChange_UPDATED {
						receivedCount++
					}
				}
			case <-timeout:
				break
			}
			if receivedCount >= len(offers) {
				break
			}
		}

		if receivedCount != len(offers) {
			t.Errorf("Expected %d notifications, got %d", len(offers), receivedCount)
		}
	})

	// Test BulkCreateOrUpdate notifications
	t.Run("BulkCreateOrUpdate sends notifications", func(t *testing.T) {
		// Clear store first
		store.Close()
		store = NewMemoryStore(log, metrics)

		// Subscribe to both created and updated events
		subscription, err := store.notificationService.Subscribe(&pb.SubscribeMarginOffersRequest{
			ClientId:         "test_client",
			SessionId:        "test_session",
			SubscribeCreated: true,
			SubscribeUpdated: true,
		})
		if err != nil {
			t.Fatalf("Failed to create subscription: %v", err)
		}
		defer store.notificationService.Unsubscribe(subscription.ID)

		// Create a channel to receive notifications
		notifications := make(chan *pb.SubscribeMarginOffersResponse, 10)
		go func() {
			for response := range subscription.Channel {
				notifications <- response
			}
		}()

		// Perform bulk create or update (should create since store is empty)
		err = store.BulkCreateOrUpdate(context.Background(), offers)
		if err != nil {
			t.Fatalf("BulkCreateOrUpdate failed: %v", err)
		}

		// Wait for notifications
		timeout := time.After(2 * time.Second)
		receivedCount := 0
		for {
			select {
			case notification := <-notifications:
				if change := notification.GetChange(); change != nil {
					if change.ChangeType == pb.MarginOfferChange_CREATED {
						receivedCount++
					}
				}
			case <-timeout:
				break
			}
			if receivedCount >= len(offers) {
				break
			}
		}

		if receivedCount != len(offers) {
			t.Errorf("Expected %d notifications, got %d", len(offers), receivedCount)
		}
	})
}

func TestBatchNotificationsPerformance(t *testing.T) {
	// Create store with notification service
	log := logger.NewLogger("test")
	metrics := metrics.NewMetricsCollector()
	store := NewMemoryStore(log, metrics)

	// Create a large number of test offers
	numOffers := 1000
	offers := make([]*types.MarginOffer, numOffers)

	for i := 0; i < numOffers; i++ {
		offers[i] = &types.MarginOffer{
			ID:                    fmt.Sprintf("offer_%d", i),
			OfferType:             types.OfferTypeV1,
			CollateralToken:       "SOL",
			BorrowToken:           "USDC",
			AvailableBorrowAmount: 1000.0,
			MaxOpenLTV:            0.8,
			LiquidationLTV:        0.9,
			InterestRate:          0.05,
			InterestModel:         types.InterestModelFixed,
			LiquiditySource:       "marginfi",
		}
	}

	// Subscribe to created events
	subscription, err := store.notificationService.Subscribe(&pb.SubscribeMarginOffersRequest{
		ClientId:         "test_client",
		SessionId:        "test_session",
		SubscribeCreated: true,
	})
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}
	defer store.notificationService.Unsubscribe(subscription.ID)

	// Create a channel to receive notifications
	notifications := make(chan *pb.SubscribeMarginOffersResponse, numOffers*2)
	go func() {
		for response := range subscription.Channel {
			notifications <- response
		}
	}()

	// Perform bulk create with large number of offers
	start := time.Now()
	err = store.BulkCreate(context.Background(), offers)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("BulkCreate failed: %v", err)
	}

	// Wait for notifications with timeout
	timeout := time.After(10 * time.Second)
	receivedCount := 0

	for {
		select {
		case notification := <-notifications:
			if change := notification.GetChange(); change != nil {
				if change.ChangeType == pb.MarginOfferChange_CREATED {
					receivedCount++
				}
			}
		case <-timeout:
			break
		}
		if receivedCount >= numOffers {
			break
		}
	}

	// Check metrics
	notificationMetrics := store.notificationService.GetMetrics()

	t.Logf("Performance test results:")
	t.Logf("  - Total offers processed: %d", numOffers)
	t.Logf("  - Notifications received: %d", receivedCount)
	t.Logf("  - Processing time: %v", duration)
	t.Logf("  - Events processed: %d", notificationMetrics["events_processed"])
	t.Logf("  - Events dropped: %d", notificationMetrics["events_dropped"])
	t.Logf("  - Event channel capacity: %d", store.notificationService.GetEventChannelCapacity())

	// Verify no events were dropped
	if notificationMetrics["events_dropped"] > 0 {
		t.Errorf("Expected no dropped events, got %d", notificationMetrics["events_dropped"])
	}

	// Verify all notifications were received
	if receivedCount != numOffers {
		t.Errorf("Expected %d notifications, got %d", numOffers, receivedCount)
	}

	// Verify processing time is reasonable (should be under 5 seconds for 1000 offers)
	if duration > 5*time.Second {
		t.Errorf("Processing took too long: %v", duration)
	}
}

func TestLargeBulkOperation(t *testing.T) {
	// Create store with notification service
	log := logger.NewLogger("test")
	metrics := metrics.NewMetricsCollector()
	store := NewMemoryStore(log, metrics)

	// Create 11,556 test offers (the problematic number)
	numOffers := 11556
	offers := make([]*types.MarginOffer, numOffers)

	for i := 0; i < numOffers; i++ {
		offers[i] = &types.MarginOffer{
			ID:                    fmt.Sprintf("offer_%d", i),
			OfferType:             types.OfferTypeV1,
			CollateralToken:       "SOL",
			BorrowToken:           "USDC",
			AvailableBorrowAmount: 1000.0,
			MaxOpenLTV:            0.8,
			LiquidationLTV:        0.9,
			InterestRate:          0.05,
			InterestModel:         types.InterestModelFixed,
			LiquiditySource:       "marginfi",
		}
	}

	// Subscribe to created events
	subscription, err := store.notificationService.Subscribe(&pb.SubscribeMarginOffersRequest{
		ClientId:         "test_client",
		SessionId:        "test_session",
		SubscribeCreated: true,
	})
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}
	defer store.notificationService.Unsubscribe(subscription.ID)

	// Create a channel to receive notifications
	notifications := make(chan *pb.SubscribeMarginOffersResponse, numOffers*2)
	go func() {
		for response := range subscription.Channel {
			notifications <- response
		}
	}()

	// Perform bulk create with large number of offers
	start := time.Now()
	err = store.BulkCreate(context.Background(), offers)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("BulkCreate failed: %v", err)
	}

	// Wait for notifications with longer timeout for large operation
	timeout := time.After(30 * time.Second)
	receivedCount := 0

	for {
		select {
		case notification := <-notifications:
			if change := notification.GetChange(); change != nil {
				if change.ChangeType == pb.MarginOfferChange_CREATED {
					receivedCount++
				}
			}
		case <-timeout:
			break
		}
		if receivedCount >= numOffers {
			break
		}
	}

	// Check metrics
	notificationMetrics := store.notificationService.GetMetrics()

	t.Logf("Large bulk operation test results:")
	t.Logf("  - Total offers processed: %d", numOffers)
	t.Logf("  - Notifications received: %d", receivedCount)
	t.Logf("  - Processing time: %v", duration)
	t.Logf("  - Events processed: %d", notificationMetrics["events_processed"])
	t.Logf("  - Events dropped: %d", notificationMetrics["events_dropped"])
	t.Logf("  - Event channel capacity: %d", store.notificationService.GetEventChannelCapacity())

	// Check if we received all notifications
	if receivedCount != numOffers {
		t.Errorf("Expected %d notifications, got %d (missing %d)", numOffers, receivedCount, numOffers-receivedCount)
	}

	// Check if any events were dropped
	if notificationMetrics["events_dropped"] > 0 {
		t.Errorf("Expected no dropped events, got %d", notificationMetrics["events_dropped"])
	}

	// Verify processing time is reasonable (should be under 30 seconds for 11,556 offers)
	if duration > 30*time.Second {
		t.Errorf("Processing took too long: %v", duration)
	}
}

func TestVeryLargeBulkOperation(t *testing.T) {
	// Create store with notification service
	log := logger.NewLogger("test")
	metrics := metrics.NewMetricsCollector()
	store := NewMemoryStore(log, metrics)

	// Create 20,000 test offers to test large batch processing
	numOffers := 20000
	offers := make([]*types.MarginOffer, numOffers)

	for i := 0; i < numOffers; i++ {
		offers[i] = &types.MarginOffer{
			ID:                    fmt.Sprintf("offer_%d", i),
			OfferType:             types.OfferTypeV1,
			CollateralToken:       "SOL",
			BorrowToken:           "USDC",
			AvailableBorrowAmount: 1000.0,
			MaxOpenLTV:            0.8,
			LiquidationLTV:        0.9,
			InterestRate:          0.05,
			InterestModel:         types.InterestModelFixed,
			LiquiditySource:       "marginfi",
		}
	}

	// Subscribe to created events
	subscription, err := store.notificationService.Subscribe(&pb.SubscribeMarginOffersRequest{
		ClientId:         "test_client",
		SessionId:        "test_session",
		SubscribeCreated: true,
	})
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}
	defer store.notificationService.Unsubscribe(subscription.ID)

	// Create a channel to receive notifications
	notifications := make(chan *pb.SubscribeMarginOffersResponse, numOffers*2)
	go func() {
		for response := range subscription.Channel {
			notifications <- response
		}
	}()

	// Perform bulk create with very large number of offers
	start := time.Now()
	err = store.BulkCreate(context.Background(), offers)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("BulkCreate failed: %v", err)
	}

	// Wait for notifications with longer timeout for very large operation
	timeout := time.After(60 * time.Second)
	receivedCount := 0

	for {
		select {
		case notification := <-notifications:
			if change := notification.GetChange(); change != nil {
				if change.ChangeType == pb.MarginOfferChange_CREATED {
					receivedCount++
				}
			}
		case <-timeout:
			break
		}
		if receivedCount >= numOffers {
			break
		}
	}

	// Check metrics
	notificationMetrics := store.notificationService.GetMetrics()

	t.Logf("Very large bulk operation test results:")
	t.Logf("  - Total offers processed: %d", numOffers)
	t.Logf("  - Notifications received: %d", receivedCount)
	t.Logf("  - Processing time: %v", duration)
	t.Logf("  - Events processed: %d", notificationMetrics["events_processed"])
	t.Logf("  - Events dropped: %d", notificationMetrics["events_dropped"])
	t.Logf("  - Event channel capacity: %d", store.notificationService.GetEventChannelCapacity())

	// Check if we received all notifications
	if receivedCount != numOffers {
		t.Errorf("Expected %d notifications, got %d (missing %d)", numOffers, receivedCount, numOffers-receivedCount)
	}

	// Check if any events were dropped
	if notificationMetrics["events_dropped"] > 0 {
		t.Errorf("Expected no dropped events, got %d", notificationMetrics["events_dropped"])
	}

	// Verify processing time is reasonable (should be under 60 seconds for 20,000 offers)
	if duration > 60*time.Second {
		t.Errorf("Processing took too long: %v", duration)
	}
}

func TestGRPCSubscription(t *testing.T) {
	// Create store with notification service
	logger := logger.NewLogger("test")
	metrics := metrics.NewMetricsCollector()
	store := NewMemoryStore(logger, metrics)
	store.notificationService = NewNotificationService(logger)

	// Create gRPC server
	server := NewGRPCServer(store)

	// Start server in background with a specific port
	go func() {
		if err := server.Start("localhost:0"); err != nil {
			t.Errorf("Failed to start server: %v", err)
		}
	}()
	defer server.Stop()

	// Give server time to start
	time.Sleep(500 * time.Millisecond)

	// Create gRPC client - use a different port
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewMarginOfferServiceClient(conn)

	// Create subscription request
	req := &pb.SubscribeMarginOffersRequest{
		SubscribeCreated:     true,
		SubscribeUpdated:     true,
		SubscribeDeleted:     true,
		SubscribeOverwritten: true,
		ClientId:             "test_client",
		SessionId:            "test_session",
	}

	// Create subscription stream
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.SubscribeMarginOffers(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	// Wait for initial response
	initialResponse, err := stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive initial response: %v", err)
	}

	if initialResponse.GetChange() == nil {
		t.Fatal("Expected initial response to have change event")
	}

	if initialResponse.GetChange().ChangeType != pb.MarginOfferChange_UNKNOWN {
		t.Fatalf("Expected UNKNOWN change type, got %s", initialResponse.GetChange().ChangeType)
	}

	// Create some offers to trigger notifications
	lender1 := "test_lender_1"
	lender2 := "test_lender_2"
	offers := []*pb.MarginOffer{
		{
			Id:              "test_offer_1",
			CollateralToken: "SOL",
			BorrowToken:     "USDC",
			LiquiditySource: "marginfi",
			LenderAddress:   &lender1,
		},
		{
			Id:              "test_offer_2",
			CollateralToken: "SOL",
			BorrowToken:     "USDC",
			LiquiditySource: "marginfi",
			LenderAddress:   &lender2,
		},
	}

	// Bulk create offers
	bulkReq := &pb.BulkCreateOrUpdateMarginOffersRequest{
		Offers: offers,
	}

	_, err = client.BulkCreateOrUpdateMarginOffers(ctx, bulkReq)
	if err != nil {
		t.Fatalf("Failed to bulk create offers: %v", err)
	}

	// Wait for notifications
	notificationCount := 0
	timeout := time.After(3 * time.Second)

	for {
		select {
		case <-timeout:
			if notificationCount == 0 {
				t.Fatal("No notifications received within timeout")
			}
			t.Logf("Received %d notifications", notificationCount)
			return
		default:
			response, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					t.Logf("Stream ended, received %d notifications", notificationCount)
					return
				}
				t.Fatalf("Failed to receive notification: %v", err)
			}

			if response.GetChange() != nil {
				notificationCount++
				t.Logf("Received notification %d: type=%s, offer_id=%s",
					notificationCount,
					response.GetChange().ChangeType,
					response.GetChange().Offer.Id)
			}
		}
	}
}

func TestSimpleSubscriptionFlow(t *testing.T) {
	// Create store with notification service
	logger := logger.NewLogger("test")
	metrics := metrics.NewMetricsCollector()
	store := NewMemoryStore(logger, metrics)
	store.notificationService = NewNotificationService(logger)

	// Create subscription
	req := &pb.SubscribeMarginOffersRequest{
		SubscribeCreated:     true,
		SubscribeUpdated:     true,
		SubscribeDeleted:     true,
		SubscribeOverwritten: true,
		ClientId:             "test_client",
		SessionId:            "test_session",
	}

	subscription, err := store.notificationService.Subscribe(req)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}
	defer store.notificationService.Unsubscribe(subscription.ID)

	t.Logf("Created subscription: %s", subscription.ID)

	// Create a single offer to trigger notification
	offer := &types.MarginOffer{
		ID:              "test_offer_1",
		CollateralToken: "SOL",
		BorrowToken:     "USDC",
		LiquiditySource: "marginfi",
	}

	// Trigger a single notification
	event := &ChangeEvent{
		ChangeType:  ChangeTypeCreated,
		Offer:       offer,
		Timestamp:   time.Now().UTC(),
		Source:      "test",
		OperationID: "test_operation",
	}

	store.notificationService.NotifyChange(event)

	// Wait for notification
	select {
	case response := <-subscription.Channel:
		if response.GetChange() == nil {
			t.Fatal("Expected notification to have change event")
		}
		change := response.GetChange()
		if change.ChangeType != pb.MarginOfferChange_CREATED {
			t.Fatalf("Expected CREATED change type, got %s", change.ChangeType)
		}
		if change.Offer.Id != "test_offer_1" {
			t.Fatalf("Expected offer ID test_offer_1, got %s", change.Offer.Id)
		}
		t.Logf("Received notification: type=%s, offer_id=%s", change.ChangeType, change.Offer.Id)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for notification")
	}
}
