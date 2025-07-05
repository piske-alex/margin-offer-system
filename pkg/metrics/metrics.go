package metrics

import (
	"sync"
	"time"

	"github.com/piske-alex/margin-offer-system/types"
)

// MetricsCollector implements the types.MetricsCollector interface
type MetricsCollector struct {
	mu       sync.RWMutex
	counters map[string]float64
	gauges   map[string]float64
	health   map[string]HealthStatus
}

type HealthStatus struct {
	Healthy bool
	Reason  string
	Updated time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() types.MetricsCollector {
	return &MetricsCollector{
		counters: make(map[string]float64),
		gauges:   make(map[string]float64),
		health:   make(map[string]HealthStatus),
	}
}

// IncCounter increments a counter metric
func (m *MetricsCollector) IncCounter(name string, tags map[string]string) {
	m.AddCounter(name, 1, tags)
}

// AddCounter adds a value to a counter metric
func (m *MetricsCollector) AddCounter(name string, value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.buildKey(name, tags)
	m.counters[key] += value
}

// SetGauge sets a gauge metric value
func (m *MetricsCollector) SetGauge(name string, value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.buildKey(name, tags)
	m.gauges[key] = value
}

// RecordDuration records a duration metric
func (m *MetricsCollector) RecordDuration(name string, duration time.Duration, tags map[string]string) {
	m.SetGauge(name+"_duration_ms", float64(duration.Milliseconds()), tags)
}

// RecordValue records a histogram value
func (m *MetricsCollector) RecordValue(name string, value float64, tags map[string]string) {
	m.SetGauge(name, value, tags)
}

// SetHealthy marks a service as healthy
func (m *MetricsCollector) SetHealthy(service string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.health[service] = HealthStatus{
		Healthy: true,
		Reason:  "",
		Updated: time.Now(),
	}
}

// SetUnhealthy marks a service as unhealthy
func (m *MetricsCollector) SetUnhealthy(service string, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.health[service] = HealthStatus{
		Healthy: false,
		Reason:  reason,
		Updated: time.Now(),
	}
}

// GetCounters returns all counter metrics
func (m *MetricsCollector) GetCounters() map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]float64)
	for k, v := range m.counters {
		result[k] = v
	}
	return result
}

// GetGauges returns all gauge metrics
func (m *MetricsCollector) GetGauges() map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]float64)
	for k, v := range m.gauges {
		result[k] = v
	}
	return result
}

// GetHealth returns all health status
func (m *MetricsCollector) GetHealth() map[string]HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]HealthStatus)
	for k, v := range m.health {
		result[k] = v
	}
	return result
}

// buildKey creates a unique key from metric name and tags
func (m *MetricsCollector) buildKey(name string, tags map[string]string) string {
	key := name
	for k, v := range tags {
		key += "|" + k + ":" + v
	}
	return key
}