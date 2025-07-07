package etl

import (
	"context"
	"time"

	"github.com/piske-alex/margin-offer-system/types"
)

// processMarginOfferEvents processes margin offer events from the channel
func (p *Processor) processMarginOfferEvents(ctx context.Context) {
	p.logger.Info("Starting margin offer event processor")

	for {
		select {
		case event := <-p.marginOfferEvents:
			start := time.Now()
			result := p.ProcessMarginOfferEvent(ctx, event)
			duration := time.Since(start)

			p.metrics.RecordDuration("event_processing_duration", duration, map[string]string{
				"type":   "margin_offer",
				"status": p.getStatusString(result.Success),
			})

			if !result.Success {
				p.handleProcessingError(event, result.Error)
			}

		case <-p.stopChan:
			p.logger.Info("Stopping margin offer event processor")
			return
		case <-ctx.Done():
			return
		}
	}
}

// processLiquidityEvents processes liquidity events from the channel
func (p *Processor) processLiquidityEvents(ctx context.Context) {
	p.logger.Info("Starting liquidity event processor")

	for {
		select {
		case event := <-p.liquidityEvents:
			start := time.Now()
			result := p.ProcessLiquidityEvent(ctx, event)
			duration := time.Since(start)

			p.metrics.RecordDuration("event_processing_duration", duration, map[string]string{
				"type":   "liquidity",
				"status": p.getStatusString(result.Success),
			})

			if !result.Success {
				p.handleLiquidityProcessingError(event, result.Error)
			}

		case <-p.stopChan:
			p.logger.Info("Stopping liquidity event processor")
			return
		case <-ctx.Done():
			return
		}
	}
}

// processInterestRateEvents processes interest rate events from the channel
func (p *Processor) processInterestRateEvents(ctx context.Context) {
	p.logger.Info("Starting interest rate event processor")

	for {
		select {
		case event := <-p.interestEvents:
			start := time.Now()
			result := p.ProcessInterestRateEvent(ctx, event)
			duration := time.Since(start)

			p.metrics.RecordDuration("event_processing_duration", duration, map[string]string{
				"type":   "interest_rate",
				"status": p.getStatusString(result.Success),
			})

			if !result.Success {
				p.handleInterestRateProcessingError(event, result.Error)
			}

		case <-p.stopChan:
			p.logger.Info("Stopping interest rate event processor")
			return
		case <-ctx.Done():
			return
		}
	}
}

// ProcessMarginOfferEvent processes a single margin offer event
func (p *Processor) ProcessMarginOfferEvent(ctx context.Context, event *types.MarginOfferEvent) *types.ProcessingResult {
	result := &types.ProcessingResult{
		EventID:     event.GetEventKey(),
		ProcessedAt: time.Now().UTC(),
	}

	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()

	// Validate event
	if err := event.Validate(); err != nil {
		result.Success = false
		result.Error = err.Error()
		p.metrics.IncCounter("events_validation_failed", map[string]string{"type": "margin_offer"})
		return result
	}

	// Process based on event type - use CreateOrUpdate for all events to consolidate logic
	var err error
	switch event.EventType {
	case types.EventTypeMarginOfferCreated:
		err = p.handleMarginOfferCreatedOrUpdated(ctx, event)
	case types.EventTypeMarginOfferUpdated:
		err = p.handleMarginOfferCreatedOrUpdated(ctx, event)
	case types.EventTypeMarginOfferDeleted:
		err = p.handleMarginOfferDeleted(ctx, event)
	default:
		err = types.ErrInvalidEventData
		p.logger.Warn("Unknown margin offer event type", "type", event.EventType)
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		p.metrics.IncCounter("events_processing_failed", map[string]string{"type": "margin_offer"})
	} else {
		result.Success = true
		p.metrics.IncCounter("events_processed_success", map[string]string{"type": "margin_offer"})
	}

	return result
}

// ProcessLiquidityEvent processes a single liquidity event
func (p *Processor) ProcessLiquidityEvent(ctx context.Context, event *types.LiquidityEvent) *types.ProcessingResult {
	result := &types.ProcessingResult{
		EventID:     event.TransactionHash + "_" + event.OfferID,
		ProcessedAt: time.Now().UTC(),
	}

	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()

	// Get existing offer
	offer, err := p.store.GetByID(ctx, event.OfferID)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	// Update liquidity based on event type
	switch event.EventType {
	case types.EventTypeLiquidityAdded:
		offer.AvailableBorrowAmount = event.NewTotalAmount
	case types.EventTypeLiquidityRemoved:
		offer.AvailableBorrowAmount = event.NewTotalAmount
	default:
		result.Success = false
		result.Error = "unknown liquidity event type"
		return result
	}

	// Use CreateOrUpdate to handle the update
	if err := p.store.CreateOrUpdate(ctx, offer); err != nil {
		result.Success = false
		result.Error = err.Error()
		p.metrics.IncCounter("events_processing_failed", map[string]string{"type": "liquidity"})
	} else {
		result.Success = true
		p.metrics.IncCounter("events_processed_success", map[string]string{"type": "liquidity"})
	}

	return result
}

// ProcessInterestRateEvent processes a single interest rate event
func (p *Processor) ProcessInterestRateEvent(ctx context.Context, event *types.InterestRateEvent) *types.ProcessingResult {
	result := &types.ProcessingResult{
		EventID:     event.TransactionHash + "_" + event.OfferID,
		ProcessedAt: time.Now().UTC(),
	}

	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()

	// Get existing offer
	offer, err := p.store.GetByID(ctx, event.OfferID)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	// Update interest rate
	offer.InterestRate = event.NewRate
	offer.InterestModel = event.RateModel

	// Use CreateOrUpdate to handle the update
	if err := p.store.CreateOrUpdate(ctx, offer); err != nil {
		result.Success = false
		result.Error = err.Error()
		p.metrics.IncCounter("events_processing_failed", map[string]string{"type": "interest_rate"})
	} else {
		result.Success = true
		p.metrics.IncCounter("events_processed_success", map[string]string{"type": "interest_rate"})
	}

	return result
}

// Event type handlers - consolidated create/update logic
func (p *Processor) handleMarginOfferCreatedOrUpdated(ctx context.Context, event *types.MarginOfferEvent) error {
	if event.OfferData == nil {
		return types.ErrInvalidEventData
	}

	// Ensure the offer has a valid ID
	if event.OfferData.ID == "" {
		event.OfferData.ID = event.OfferID
	}

	// Set timestamps from event
	if event.OfferData.CreatedTimestamp.IsZero() {
		event.OfferData.CreatedTimestamp = event.Timestamp
	}
	event.OfferData.UpdatedTimestamp = event.Timestamp

	// Use CreateOrUpdate for consolidated logic
	return p.store.CreateOrUpdate(ctx, event.OfferData)
}

func (p *Processor) handleMarginOfferDeleted(ctx context.Context, event *types.MarginOfferEvent) error {
	return p.store.Delete(ctx, event.OfferID)
}

// Error handling helpers
func (p *Processor) handleProcessingError(event *types.MarginOfferEvent, errorMsg string) {
	p.mu.Lock()
	p.errorCount++
	p.mu.Unlock()

	p.logger.Error("Failed to process margin offer event",
		"event_id", event.GetEventKey(),
		"event_type", event.EventType,
		"error", errorMsg,
		"retry_count", event.RetryCount)

	// Implement retry logic if event is retryable
	if event.IsRetryable() {
		event.RetryCount++
		// Re-queue event for retry (simplified)
		go func() {
			time.Sleep(time.Second * time.Duration(event.RetryCount))
			select {
			case p.marginOfferEvents <- event:
			case <-p.stopChan:
			}
		}()
	}
}

func (p *Processor) handleLiquidityProcessingError(event *types.LiquidityEvent, errorMsg string) {
	p.mu.Lock()
	p.errorCount++
	p.mu.Unlock()

	p.logger.Error("Failed to process liquidity event",
		"offer_id", event.OfferID,
		"event_type", event.EventType,
		"error", errorMsg)
}

func (p *Processor) handleInterestRateProcessingError(event *types.InterestRateEvent, errorMsg string) {
	p.mu.Lock()
	p.errorCount++
	p.mu.Unlock()

	p.logger.Error("Failed to process interest rate event",
		"offer_id", event.OfferID,
		"event_type", event.EventType,
		"error", errorMsg)
}

func (p *Processor) getStatusString(success bool) string {
	if success {
		return "success"
	}
	return "failed"
}
