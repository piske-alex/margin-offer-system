package store

import (
	"fmt"
	"sync"
	"time"

	"sync/atomic"

	"github.com/google/uuid"
	pb "github.com/piske-alex/margin-offer-system/proto/gen/go/proto"
	"github.com/piske-alex/margin-offer-system/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ChangeType represents the type of change to a margin offer
type ChangeType int

const (
	ChangeTypeUnknown ChangeType = iota
	ChangeTypeCreated
	ChangeTypeUpdated
	ChangeTypeDeleted
	ChangeTypeOverwritten
)

func (ct ChangeType) String() string {
	switch ct {
	case ChangeTypeCreated:
		return "created"
	case ChangeTypeUpdated:
		return "updated"
	case ChangeTypeDeleted:
		return "deleted"
	case ChangeTypeOverwritten:
		return "overwritten"
	default:
		return "unknown"
	}
}

// ChangeEvent represents a margin offer change event
type ChangeEvent struct {
	ChangeType  ChangeType
	Offer       *types.MarginOffer
	Timestamp   time.Time
	Source      string
	OperationID string
	Metadata    map[string]string
}

// Subscription represents a client subscription
type Subscription struct {
	ID           string
	ClientID     string
	SessionID    string
	Filters      *pb.SubscribeMarginOffersRequest
	Channel      chan *pb.SubscribeMarginOffersResponse
	CreatedAt    time.Time
	LastActivity time.Time
}

// NotificationService handles margin offer change notifications and subscriptions
type NotificationService struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription
	logger        types.Logger

	// Configuration
	maxSubscriptions  int
	heartbeatInterval time.Duration
	cleanupInterval   time.Duration

	// Channels
	eventChan chan *ChangeEvent
	stopChan  chan struct{}

	// Metrics
	eventsProcessed    int64
	eventsDropped      int64
	subscriptionsCount int64
}

// NewNotificationService creates a new notification service
func NewNotificationService(logger types.Logger) *NotificationService {
	ns := &NotificationService{
		subscriptions:     make(map[string]*Subscription),
		logger:            logger,
		maxSubscriptions:  1000,
		heartbeatInterval: time.Minute * 5,
		cleanupInterval:   time.Minute * 10,
		eventChan:         make(chan *ChangeEvent, 10000), // Increased from 1000 to 10000
		stopChan:          make(chan struct{}),
	}

	// Start background goroutines
	go ns.eventProcessor()
	go ns.heartbeatProcessor()
	go ns.cleanupProcessor()

	return ns
}

// Start begins the notification service
func (ns *NotificationService) Start() error {
	ns.logger.Info("Starting notification service")
	return nil
}

// Stop gracefully stops the notification service
func (ns *NotificationService) Stop() error {
	ns.logger.Info("Stopping notification service")
	close(ns.stopChan)
	return nil
}

// Subscribe creates a new subscription for margin offer changes
func (ns *NotificationService) Subscribe(req *pb.SubscribeMarginOffersRequest) (*Subscription, error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Check subscription limit
	if len(ns.subscriptions) >= ns.maxSubscriptions {
		return nil, fmt.Errorf("maximum number of subscriptions reached")
	}

	// Create subscription
	subscription := &Subscription{
		ID:           uuid.New().String(),
		ClientID:     req.ClientId,
		SessionID:    req.SessionId,
		Filters:      req,
		Channel:      make(chan *pb.SubscribeMarginOffersResponse, 1000), // Increased from 100 to 1000
		CreatedAt:    time.Now().UTC(),
		LastActivity: time.Now().UTC(),
	}

	ns.subscriptions[subscription.ID] = subscription
	atomic.AddInt64(&ns.subscriptionsCount, 1)

	ns.logger.Info("Created subscription",
		"subscription_id", subscription.ID,
		"client_id", subscription.ClientID,
		"session_id", subscription.SessionID)

	return subscription, nil
}

// Unsubscribe removes a subscription
func (ns *NotificationService) Unsubscribe(subscriptionID string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	subscription, exists := ns.subscriptions[subscriptionID]
	if !exists {
		return fmt.Errorf("subscription not found")
	}

	close(subscription.Channel)
	delete(ns.subscriptions, subscriptionID)
	atomic.AddInt64(&ns.subscriptionsCount, -1)

	ns.logger.Info("Removed subscription", "subscription_id", subscriptionID)
	return nil
}

// NotifyChange sends a change notification to all relevant subscribers
func (ns *NotificationService) NotifyChange(event *ChangeEvent) {
	select {
	case ns.eventChan <- event:
		// Event sent successfully
	default:
		ns.logger.Warn("Event channel full, dropping change notification")
		atomic.AddInt64(&ns.eventsDropped, 1)
	}
}

// NotifyBatch sends multiple change notifications in a batch
func (ns *NotificationService) NotifyBatch(events []*ChangeEvent) {
	if len(events) == 0 {
		return
	}

	// Determine batch size based on event count
	var batchSize int
	if len(events) <= 1000 {
		batchSize = 100 // Small batches for small operations
	} else if len(events) <= 10000 {
		batchSize = 250 // Medium batches for large operations
	} else {
		batchSize = 500 // Large batches for very large operations
	}

	droppedCount := 0

	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}

		batch := events[i:end]
		for _, event := range batch {
			ns.processChangeEvent(event)
		}

		// Add small delay between batches to prevent overwhelming
		if end < len(events) {
			time.Sleep(1 * time.Millisecond)
		}
	}

	if droppedCount > 0 {
		ns.logger.Error("Batch notification processing completed with dropped events",
			"total_events", len(events),
			"dropped_events", droppedCount)
	}
}

// NotifyLargeBatch handles very large batches by processing them in chunks
func (ns *NotificationService) NotifyLargeBatch(events []*ChangeEvent) {
	if len(events) == 0 {
		return
	}

	// For very large operations (>10k events), process in chunks
	chunkSize := 5000
	totalEvents := len(events)
	processedEvents := 0
	droppedEvents := 0

	ns.logger.Info("Starting large batch notification",
		"total_events", totalEvents,
		"chunk_size", chunkSize)

	for i := 0; i < totalEvents; i += chunkSize {
		end := i + chunkSize
		if end > totalEvents {
			end = totalEvents
		}

		chunk := events[i:end]

		// Process chunk directly (same as NotifyBatch)
		for _, event := range chunk {
			ns.processChangeEvent(event)
			processedEvents++
		}

		// Log progress for large operations
		if totalEvents > 10000 {
			progress := float64(i+len(chunk)) / float64(totalEvents) * 100
			ns.logger.Info("Large batch progress",
				"progress_percent", fmt.Sprintf("%.1f", progress),
				"processed_so_far", i+len(chunk),
				"total_events", totalEvents)
		}

		// Small delay between chunks to prevent overwhelming
		if end < totalEvents {
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Log final results
	successRate := float64(processedEvents) / float64(totalEvents) * 100
	ns.logger.Info("Large batch notification completed",
		"total_events", totalEvents,
		"processed_events", processedEvents,
		"dropped_events", droppedEvents,
		"success_rate", fmt.Sprintf("%.1f", successRate))

	if droppedEvents > 0 {
		atomic.AddInt64(&ns.eventsDropped, int64(droppedEvents))
	}
}

// eventProcessor processes change events and sends notifications to subscribers
func (ns *NotificationService) eventProcessor() {
	for {
		select {
		case event := <-ns.eventChan:
			atomic.AddInt64(&ns.eventsProcessed, 1)
			ns.processChangeEvent(event)
		case <-ns.stopChan:
			return
		}
	}
}

// processChangeEvent sends a change event to all matching subscribers
func (ns *NotificationService) processChangeEvent(event *ChangeEvent) {
	ns.mu.RLock()
	subscriptions := make([]*Subscription, 0, len(ns.subscriptions))
	for _, sub := range ns.subscriptions {
		subscriptions = append(subscriptions, sub)
	}
	ns.mu.RUnlock()

	// Convert event to protobuf message
	pbEvent := ns.convertToProtobufEvent(event)

	// Send to matching subscriptions with backpressure handling
	droppedSubscriptions := 0
	matchedSubscriptions := 0

	for _, subscription := range subscriptions {
		if ns.matchesSubscription(event, subscription) {
			matchedSubscriptions++
			select {
			case subscription.Channel <- pbEvent:
				subscription.LastActivity = time.Now().UTC()
				// Debug: Log when events are sent to subscriptions
				ns.logger.Debug("Sent event to subscription",
					"subscription_id", subscription.ID,
					"client_id", subscription.ClientID,
					"event_type", event.ChangeType.String(),
					"offer_id", event.Offer.ID)
			default:
				droppedSubscriptions++
				// Only log warning for first few dropped subscriptions to avoid spam
				if droppedSubscriptions <= 3 {
					ns.logger.Warn("Subscription channel full, dropping notification",
						"subscription_id", subscription.ID,
						"client_id", subscription.ClientID,
						"dropped_count", droppedSubscriptions)
				}
			}
		}
	}

	// Debug: Log event processing summary
	if len(subscriptions) > 0 {
		ns.logger.Debug("Event processing summary",
			"event_type", event.ChangeType.String(),
			"offer_id", event.Offer.ID,
			"total_subscriptions", len(subscriptions),
			"matched_subscriptions", matchedSubscriptions,
			"dropped_subscriptions", droppedSubscriptions)
	}

	if droppedSubscriptions > 0 {
		ns.logger.Error("Event processing completed with dropped subscriptions",
			"total_subscriptions", len(subscriptions),
			"dropped_subscriptions", droppedSubscriptions,
			"event_type", event.ChangeType.String(),
			"event_source", event.Source)
	}
}

// matchesSubscription checks if an event matches a subscription's filters
func (ns *NotificationService) matchesSubscription(event *ChangeEvent, subscription *Subscription) bool {
	filters := subscription.Filters

	// Debug: Log subscription matching attempt
	ns.logger.Debug("Checking subscription match",
		"subscription_id", subscription.ID,
		"client_id", subscription.ClientID,
		"event_type", event.ChangeType.String(),
		"offer_id", event.Offer.ID,
		"subscribe_created", filters.SubscribeCreated,
		"subscribe_updated", filters.SubscribeUpdated,
		"subscribe_deleted", filters.SubscribeDeleted,
		"subscribe_overwritten", filters.SubscribeOverwritten)

	// Check if subscription is interested in this change type
	switch event.ChangeType {
	case ChangeTypeCreated:
		if !filters.SubscribeCreated {
			ns.logger.Debug("Subscription not interested in CREATED events",
				"subscription_id", subscription.ID)
			return false
		}
	case ChangeTypeUpdated:
		if !filters.SubscribeUpdated {
			ns.logger.Debug("Subscription not interested in UPDATED events",
				"subscription_id", subscription.ID)
			return false
		}
	case ChangeTypeDeleted:
		if !filters.SubscribeDeleted {
			ns.logger.Debug("Subscription not interested in DELETED events",
				"subscription_id", subscription.ID)
			return false
		}
	case ChangeTypeOverwritten:
		if !filters.SubscribeOverwritten {
			ns.logger.Debug("Subscription not interested in OVERWRITTEN events",
				"subscription_id", subscription.ID)
			return false
		}
	}

	// Check offer type filter
	if filters.OfferType != nil && *filters.OfferType != "" && event.Offer.OfferType != types.OfferType(*filters.OfferType) {
		ns.logger.Debug("Subscription filtered by offer type",
			"subscription_id", subscription.ID,
			"want", *filters.OfferType,
			"got", event.Offer.OfferType)
		return false
	}

	// Check collateral token filter
	if filters.CollateralToken != nil && *filters.CollateralToken != "" && event.Offer.CollateralToken != *filters.CollateralToken {
		ns.logger.Debug("Subscription filtered by collateral token",
			"subscription_id", subscription.ID,
			"want", *filters.CollateralToken,
			"got", event.Offer.CollateralToken)
		return false
	}

	// Check borrow token filter
	if filters.BorrowToken != nil && *filters.BorrowToken != "" && event.Offer.BorrowToken != *filters.BorrowToken {
		ns.logger.Debug("Subscription filtered by borrow token",
			"subscription_id", subscription.ID,
			"want", *filters.BorrowToken,
			"got", event.Offer.BorrowToken)
		return false
	}

	// Check liquidity source filter
	if filters.LiquiditySource != nil && *filters.LiquiditySource != "" && event.Offer.LiquiditySource != *filters.LiquiditySource {
		ns.logger.Debug("Subscription filtered by liquidity source",
			"subscription_id", subscription.ID,
			"want", *filters.LiquiditySource,
			"got", event.Offer.LiquiditySource)
		return false
	}

	// Check lender address filter
	if filters.LenderAddress != nil && *filters.LenderAddress != "" {
		if event.Offer.LenderAddress == nil || *event.Offer.LenderAddress != *filters.LenderAddress {
			ns.logger.Debug("Subscription filtered by lender address",
				"subscription_id", subscription.ID,
				"want", *filters.LenderAddress,
				"got", event.Offer.LenderAddress)
			return false
		}
	}

	ns.logger.Debug("Subscription matches event",
		"subscription_id", subscription.ID)
	return true
}

// convertToProtobufEvent converts a ChangeEvent to a protobuf message
func (ns *NotificationService) convertToProtobufEvent(event *ChangeEvent) *pb.SubscribeMarginOffersResponse {
	// Convert change type
	var changeType pb.MarginOfferChange_ChangeType
	switch event.ChangeType {
	case ChangeTypeCreated:
		changeType = pb.MarginOfferChange_CREATED
	case ChangeTypeUpdated:
		changeType = pb.MarginOfferChange_UPDATED
	case ChangeTypeDeleted:
		changeType = pb.MarginOfferChange_DELETED
	case ChangeTypeOverwritten:
		changeType = pb.MarginOfferChange_OVERWRITTEN
	default:
		changeType = pb.MarginOfferChange_UNKNOWN
	}

	// Convert margin offer to protobuf
	pbOffer := &pb.MarginOffer{
		Id:                    event.Offer.ID,
		OfferType:             string(event.Offer.OfferType),
		CollateralToken:       event.Offer.CollateralToken,
		BorrowToken:           event.Offer.BorrowToken,
		AvailableBorrowAmount: event.Offer.AvailableBorrowAmount,
		MaxOpenLtv:            event.Offer.MaxOpenLTV,
		LiquidationLtv:        event.Offer.LiquidationLTV,
		InterestRate:          event.Offer.InterestRate,
		InterestModel:         string(event.Offer.InterestModel),
		LiquiditySource:       event.Offer.LiquiditySource,
		CreatedTimestamp:      timestamppb.New(event.Offer.CreatedTimestamp),
		UpdatedTimestamp:      timestamppb.New(event.Offer.UpdatedTimestamp),
	}

	// Add optional fields
	if event.Offer.TermDays != nil {
		pbOffer.TermDays = event.Offer.TermDays
	}
	if event.Offer.LenderAddress != nil {
		pbOffer.LenderAddress = event.Offer.LenderAddress
	}
	if event.Offer.LastBorrowedTimestamp != nil {
		pbOffer.LastBorrowedTimestamp = timestamppb.New(*event.Offer.LastBorrowedTimestamp)
	}

	// Create change event
	change := &pb.MarginOfferChange{
		ChangeType:  changeType,
		Offer:       pbOffer,
		Timestamp:   timestamppb.New(event.Timestamp),
		Source:      event.Source,
		OperationId: event.OperationID,
		Metadata:    event.Metadata,
	}

	return &pb.SubscribeMarginOffersResponse{
		Event: &pb.SubscribeMarginOffersResponse_Change{
			Change: change,
		},
	}
}

// heartbeatProcessor sends periodic heartbeats to all subscribers
func (ns *NotificationService) heartbeatProcessor() {
	ticker := time.NewTicker(ns.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ns.sendHeartbeats()
		case <-ns.stopChan:
			return
		}
	}
}

// sendHeartbeats sends heartbeats to all active subscriptions
func (ns *NotificationService) sendHeartbeats() {
	ns.mu.RLock()
	subscriptions := make([]*Subscription, 0, len(ns.subscriptions))
	for _, sub := range ns.subscriptions {
		subscriptions = append(subscriptions, sub)
	}
	ns.mu.RUnlock()

	now := time.Now().UTC()

	for _, subscription := range subscriptions {
		heartbeat := &pb.SubscribeMarginOffersResponse{
			Event: &pb.SubscribeMarginOffersResponse_Heartbeat{
				Heartbeat: &pb.SubscriptionHeartbeat{
					Timestamp: timestamppb.New(now),
					ClientId:  subscription.ClientID,
					SessionId: subscription.SessionID,
				},
			},
		}

		select {
		case subscription.Channel <- heartbeat:
			subscription.LastActivity = now
		default:
			// Channel is full, skip this heartbeat
		}
	}
}

// cleanupProcessor removes inactive subscriptions
func (ns *NotificationService) cleanupProcessor() {
	ticker := time.NewTicker(ns.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ns.cleanupInactiveSubscriptions()
		case <-ns.stopChan:
			return
		}
	}
}

// cleanupInactiveSubscriptions removes subscriptions that have been inactive for too long
func (ns *NotificationService) cleanupInactiveSubscriptions() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	cutoff := time.Now().UTC().Add(-time.Hour) // Remove subscriptions inactive for more than 1 hour

	for id, subscription := range ns.subscriptions {
		if subscription.LastActivity.Before(cutoff) {
			close(subscription.Channel)
			delete(ns.subscriptions, id)
			ns.logger.Info("Removed inactive subscription", "subscription_id", id)
		}
	}
}

// GetSubscriptionCount returns the current number of active subscriptions
func (ns *NotificationService) GetSubscriptionCount() int {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return len(ns.subscriptions)
}

// GetMetrics returns notification service metrics
func (ns *NotificationService) GetMetrics() map[string]int64 {
	return map[string]int64{
		"events_processed":     atomic.LoadInt64(&ns.eventsProcessed),
		"events_dropped":       atomic.LoadInt64(&ns.eventsDropped),
		"subscriptions_count":  atomic.LoadInt64(&ns.subscriptionsCount),
		"event_channel_length": int64(len(ns.eventChan)),
	}
}

// GetEventChannelCapacity returns the capacity of the event channel
func (ns *NotificationService) GetEventChannelCapacity() int {
	return cap(ns.eventChan)
}
