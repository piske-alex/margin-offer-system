package etl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/piske-alex/margin-offer-system/types"
)

// Processor implements the ETL pipeline for processing blockchain events
type Processor struct {
	mu            sync.RWMutex
	store         types.MarginOfferStore
	chainClient   types.ChainClient
	logger        types.Logger
	metrics       types.MetricsCollector
	
	// Pipeline state
	isRunning     bool
	startTime     time.Time
	lastEventTime *time.Time
	errorCount    int64
	restartCount  int32
	
	// Event channels
	marginOfferEvents chan *types.MarginOfferEvent
	liquidityEvents   chan *types.LiquidityEvent
	interestEvents    chan *types.InterestRateEvent
	stopChan          chan struct{}
	
	// Configuration
	batchSize        int
	processingDelay  time.Duration
	maxRetries       int
	connectionTimeout time.Duration
}

// NewProcessor creates a new ETL processor
func NewProcessor(store types.MarginOfferStore, chainClient types.ChainClient, logger types.Logger, metrics types.MetricsCollector) *Processor {
	return &Processor{
		store:             store,
		chainClient:       chainClient,
		logger:            logger,
		metrics:           metrics,
		marginOfferEvents: make(chan *types.MarginOfferEvent, 1000),
		liquidityEvents:   make(chan *types.LiquidityEvent, 1000),
		interestEvents:    make(chan *types.InterestRateEvent, 1000),
		stopChan:          make(chan struct{}),
		batchSize:         100,
		processingDelay:   time.Millisecond * 100,
		maxRetries:        3,
		connectionTimeout: time.Second * 30,
	}
}

// Start begins the ETL pipeline
func (p *Processor) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.isRunning {
		return fmt.Errorf("processor is already running")
	}
	
	p.logger.Info("Starting ETL processor")
	p.startTime = time.Now().UTC()
	p.isRunning = true
	
	// Connect to blockchain
	if err := p.connectToChain(ctx); err != nil {
		p.isRunning = false
		return fmt.Errorf("failed to connect to chain: %w", err)
	}
	
	// Start event subscription goroutines
	go p.subscribeToEvents(ctx)
	
	// Start event processing goroutines
	go p.processMarginOfferEvents(ctx)
	go p.processLiquidityEvents(ctx)
	go p.processInterestRateEvents(ctx)
	
	// Start health monitoring
	go p.monitorHealth(ctx)
	
	p.logger.Info("ETL processor started successfully")
	return nil
}

// Stop gracefully stops the ETL pipeline
func (p *Processor) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.isRunning {
		return fmt.Errorf("processor is not running")
	}
	
	p.logger.Info("Stopping ETL processor")
	p.isRunning = false
	
	// Signal all goroutines to stop
	close(p.stopChan)
	
	// Disconnect from blockchain
	if err := p.chainClient.Disconnect(); err != nil {
		p.logger.Error("Failed to disconnect from chain", "error", err)
	}
	
	p.logger.Info("ETL processor stopped")
	return nil
}

// IsRunning returns whether the processor is currently running
func (p *Processor) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isRunning
}

// GetStatus returns the current status of the ETL processor
func (p *Processor) GetStatus() types.ETLStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	connectionStatus := "disconnected"
	if p.chainClient.IsConnected() {
		connectionStatus = "connected"
	}
	
	return types.ETLStatus{
		IsRunning:        p.isRunning,
		StartTime:        p.startTime,
		LastEventTime:    p.lastEventTime,
		ConnectionStatus: connectionStatus,
		Subscriptions:    []string{"margin_offers", "liquidity", "interest_rates"},
		ErrorCount:       p.errorCount,
		RestartCount:     p.restartCount,
	}
}

// GetMetrics returns processing metrics
func (p *Processor) GetMetrics() types.ETLMetrics {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	connectionStatus := "disconnected"
	if p.chainClient.IsConnected() {
		connectionStatus = "connected"
	}
	
	var lagSeconds float64
	if p.lastEventTime != nil {
		lagSeconds = time.Since(*p.lastEventTime).Seconds()
	}
	
	return types.ETLMetrics{
		EventsProcessed:       0, // Would be tracked in implementation
		EventsPerSecond:       0, // Would be calculated based on recent events
		AverageProcessingTime: p.processingDelay,
		ErrorRate:             0, // Would be calculated
		LastEventTimestamp:    time.Now().UTC(),
		LagSeconds:            lagSeconds,
		ActiveSubscriptions:   3,
		ConnectionStatus:      connectionStatus,
	}
}

// connectToChain establishes connection to the blockchain
func (p *Processor) connectToChain(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, p.connectionTimeout)
	defer cancel()
	
	// This would be configured from environment or config file
	endpoint := "wss://api.mainnet-beta.solana.com"
	
	if err := p.chainClient.Connect(ctx, endpoint); err != nil {
		p.metrics.SetUnhealthy("chain_connection", err.Error())
		return err
	}
	
	p.metrics.SetHealthy("chain_connection")
	return nil
}

// subscribeToEvents sets up event subscriptions
func (p *Processor) subscribeToEvents(ctx context.Context) {
	p.logger.Info("Setting up event subscriptions")
	
	// Subscribe to margin offer events
	if err := p.chainClient.SubscribeToMarginOfferEvents(ctx, p.handleMarginOfferEvent); err != nil {
		p.logger.Error("Failed to subscribe to margin offer events", "error", err)
		p.metrics.SetUnhealthy("margin_offer_subscription", err.Error())
	} else {
		p.metrics.SetHealthy("margin_offer_subscription")
	}
	
	// Subscribe to liquidity events
	if err := p.chainClient.SubscribeToLiquidityEvents(ctx, p.handleLiquidityEvent); err != nil {
		p.logger.Error("Failed to subscribe to liquidity events", "error", err)
		p.metrics.SetUnhealthy("liquidity_subscription", err.Error())
	} else {
		p.metrics.SetHealthy("liquidity_subscription")
	}
}

// Event handlers
func (p *Processor) handleMarginOfferEvent(event *types.MarginOfferEvent) {
	select {
	case p.marginOfferEvents <- event:
		p.updateLastEventTime(event.Timestamp)
	case <-p.stopChan:
		return
	default:
		p.logger.Warn("Margin offer event channel full, dropping event", "event_id", event.GetEventKey())
		p.metrics.IncCounter("events_dropped", map[string]string{"type": "margin_offer"})
	}
}

func (p *Processor) handleLiquidityEvent(event *types.LiquidityEvent) {
	select {
	case p.liquidityEvents <- event:
		p.updateLastEventTime(event.Timestamp)
	case <-p.stopChan:
		return
	default:
		p.logger.Warn("Liquidity event channel full, dropping event")
		p.metrics.IncCounter("events_dropped", map[string]string{"type": "liquidity"})
	}
}

func (p *Processor) updateLastEventTime(timestamp time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastEventTime = &timestamp
}

// monitorHealth performs periodic health checks
func (p *Processor) monitorHealth(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.performHealthCheck()
		case <-p.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (p *Processor) performHealthCheck() {
	// Check chain connection
	if !p.chainClient.IsConnected() {
		p.logger.Warn("Chain connection lost, attempting reconnect")
		p.metrics.SetUnhealthy("chain_connection", "connection lost")
		
		// Attempt reconnection
		ctx, cancel := context.WithTimeout(context.Background(), p.connectionTimeout)
		if err := p.connectToChain(ctx); err != nil {
			p.logger.Error("Failed to reconnect to chain", "error", err)
		}
		cancel()
	}
	
	// Check store health
	if err := p.store.HealthCheck(context.Background()); err != nil {
		p.logger.Error("Store health check failed", "error", err)
		p.metrics.SetUnhealthy("store", err.Error())
	} else {
		p.metrics.SetHealthy("store")
	}
}