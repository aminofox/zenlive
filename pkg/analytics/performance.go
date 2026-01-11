package analytics

import (
	"sync"
	"time"
)

// LatencyMetrics tracks latency measurements
type LatencyMetrics struct {
	Name        string        // Metric name
	Min         time.Duration // Minimum latency
	Max         time.Duration // Maximum latency
	Average     time.Duration // Average latency
	P50         time.Duration // 50th percentile
	P95         time.Duration // 95th percentile
	P99         time.Duration // 99th percentile
	SampleCount int           // Number of samples
	mu          sync.RWMutex
	samples     []time.Duration
}

// NewLatencyMetrics creates a new latency metrics tracker
func NewLatencyMetrics(name string) *LatencyMetrics {
	return &LatencyMetrics{
		Name:    name,
		samples: make([]time.Duration, 0),
	}
}

// Record records a latency measurement
func (lm *LatencyMetrics) Record(latency time.Duration) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.samples = append(lm.samples, latency)
	lm.SampleCount++

	// Update min/max
	if lm.SampleCount == 1 {
		lm.Min = latency
		lm.Max = latency
	} else {
		if latency < lm.Min {
			lm.Min = latency
		}
		if latency > lm.Max {
			lm.Max = latency
		}
	}

	// Update average
	total := time.Duration(0)
	for _, sample := range lm.samples {
		total += sample
	}
	lm.Average = total / time.Duration(lm.SampleCount)

	// Calculate percentiles (simple implementation)
	if lm.SampleCount >= 2 {
		sorted := make([]time.Duration, len(lm.samples))
		copy(sorted, lm.samples)

		// Simple bubble sort for small samples
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i] > sorted[j] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		lm.P50 = sorted[len(sorted)*50/100]
		lm.P95 = sorted[len(sorted)*95/100]
		lm.P99 = sorted[len(sorted)*99/100]
	}
}

// Reset resets the latency metrics
func (lm *LatencyMetrics) Reset() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.Min = 0
	lm.Max = 0
	lm.Average = 0
	lm.P50 = 0
	lm.P95 = 0
	lm.P99 = 0
	lm.SampleCount = 0
	lm.samples = make([]time.Duration, 0)
}

// LatencyMetricsSnapshot is a snapshot without locks
type LatencyMetricsSnapshot struct {
	Name        string
	Min         time.Duration
	Max         time.Duration
	Average     time.Duration
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	SampleCount int
}

// GetSnapshot returns a snapshot of current metrics
func (lm *LatencyMetrics) GetSnapshot() LatencyMetricsSnapshot {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	return LatencyMetricsSnapshot{
		Name:        lm.Name,
		Min:         lm.Min,
		Max:         lm.Max,
		Average:     lm.Average,
		P50:         lm.P50,
		P95:         lm.P95,
		P99:         lm.P99,
		SampleCount: lm.SampleCount,
	}
}

// ErrorRateMetricsSnapshot is a snapshot without locks
type ErrorRateMetricsSnapshot struct {
	TotalRequests int64
	ErrorCount    int64
	ErrorRate     float64
	ErrorsByType  map[string]int64
}

// ErrorRateMetrics tracks error rates
type ErrorRateMetrics struct {
	TotalRequests int64   // Total number of requests
	ErrorCount    int64   // Number of errors
	ErrorRate     float64 // Error rate (percentage)
	mu            sync.RWMutex

	// Error breakdown by type
	ErrorsByType map[string]int64
}

// NewErrorRateMetrics creates a new error rate metrics tracker
func NewErrorRateMetrics() *ErrorRateMetrics {
	return &ErrorRateMetrics{
		ErrorsByType: make(map[string]int64),
	}
}

// RecordRequest records a request
func (erm *ErrorRateMetrics) RecordRequest() {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	erm.TotalRequests++
	erm.updateErrorRate()
}

// RecordError records an error
func (erm *ErrorRateMetrics) RecordError(errorType string) {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	erm.ErrorCount++
	erm.ErrorsByType[errorType]++
	erm.updateErrorRate()
}

// updateErrorRate updates the error rate (must be called with lock held)
func (erm *ErrorRateMetrics) updateErrorRate() {
	if erm.TotalRequests > 0 {
		erm.ErrorRate = float64(erm.ErrorCount) / float64(erm.TotalRequests) * 100.0
	}
}

// Reset resets the error rate metrics
func (erm *ErrorRateMetrics) Reset() {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	erm.TotalRequests = 0
	erm.ErrorCount = 0
	erm.ErrorRate = 0
	erm.ErrorsByType = make(map[string]int64)
}

// GetSnapshot returns a snapshot of current metrics
func (erm *ErrorRateMetrics) GetSnapshot() ErrorRateMetricsSnapshot {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	errorsByType := make(map[string]int64)
	for k, v := range erm.ErrorsByType {
		errorsByType[k] = v
	}

	return ErrorRateMetricsSnapshot{
		TotalRequests: erm.TotalRequests,
		ErrorCount:    erm.ErrorCount,
		ErrorRate:     erm.ErrorRate,
		ErrorsByType:  errorsByType,
	}
}

// PerformanceMonitor monitors system performance
type PerformanceMonitor struct {
	// Latency tracking
	latencyMetrics map[string]*LatencyMetrics

	// Error rate tracking
	errorMetrics map[string]*ErrorRateMetrics

	// Throughput tracking
	requestsPerSecond float64
	lastRequestTime   time.Time
	requestCount      int64

	mu        sync.RWMutex
	collector MetricsCollector
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(collector MetricsCollector) *PerformanceMonitor {
	return &PerformanceMonitor{
		latencyMetrics: make(map[string]*LatencyMetrics),
		errorMetrics:   make(map[string]*ErrorRateMetrics),
		collector:      collector,
	}
}

// RecordLatency records a latency measurement
func (pm *PerformanceMonitor) RecordLatency(operation string, latency time.Duration) {
	pm.mu.Lock()
	metrics, exists := pm.latencyMetrics[operation]
	if !exists {
		metrics = NewLatencyMetrics(operation)
		pm.latencyMetrics[operation] = metrics
	}
	pm.mu.Unlock()

	metrics.Record(latency)

	// Also record to collector
	if pm.collector != nil {
		pm.collector.RecordHistogram("operation_latency_ms", float64(latency.Milliseconds()), map[string]string{
			"operation": operation,
		})
	}
}

// RecordRequest records a request for throughput tracking
func (pm *PerformanceMonitor) RecordRequest(operation string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.requestCount++
	pm.lastRequestTime = time.Now()

	// Update error metrics
	metrics, exists := pm.errorMetrics[operation]
	if !exists {
		metrics = NewErrorRateMetrics()
		pm.errorMetrics[operation] = metrics
	}
	metrics.RecordRequest()

	// Record to collector
	if pm.collector != nil {
		pm.collector.RecordCounter("requests_total", 1, map[string]string{
			"operation": operation,
		})
	}
}

// RecordError records an error
func (pm *PerformanceMonitor) RecordError(operation, errorType string) {
	pm.mu.Lock()
	metrics, exists := pm.errorMetrics[operation]
	if !exists {
		metrics = NewErrorRateMetrics()
		pm.errorMetrics[operation] = metrics
	}
	pm.mu.Unlock()

	metrics.RecordError(errorType)

	// Record to collector
	if pm.collector != nil {
		pm.collector.RecordCounter("errors_total", 1, map[string]string{
			"operation":  operation,
			"error_type": errorType,
		})
	}
}

// GetLatencyMetrics retrieves latency metrics for an operation
func (pm *PerformanceMonitor) GetLatencyMetrics(operation string) (*LatencyMetrics, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metrics, exists := pm.latencyMetrics[operation]
	return metrics, exists
}

// GetErrorMetrics retrieves error metrics for an operation
func (pm *PerformanceMonitor) GetErrorMetrics(operation string) (*ErrorRateMetrics, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metrics, exists := pm.errorMetrics[operation]
	return metrics, exists
}

// GetAllLatencyMetrics returns all latency metrics
func (pm *PerformanceMonitor) GetAllLatencyMetrics() map[string]LatencyMetricsSnapshot {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]LatencyMetricsSnapshot)
	for k, v := range pm.latencyMetrics {
		result[k] = v.GetSnapshot()
	}
	return result
}

// GetAllErrorMetrics returns all error metrics
func (pm *PerformanceMonitor) GetAllErrorMetrics() map[string]ErrorRateMetricsSnapshot {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]ErrorRateMetricsSnapshot)
	for k, v := range pm.errorMetrics {
		result[k] = v.GetSnapshot()
	}
	return result
}

// CalculateThroughput calculates requests per second
func (pm *PerformanceMonitor) CalculateThroughput(window time.Duration) float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.requestCount == 0 {
		return 0
	}

	return float64(pm.requestCount) / window.Seconds()
}

// CollectMetrics collects current performance metrics
func (pm *PerformanceMonitor) CollectMetrics() {
	if pm.collector == nil {
		return
	}

	// Collect latency metrics
	latencyMetrics := pm.GetAllLatencyMetrics()
	for operation, m := range latencyMetrics {
		labels := map[string]string{"operation": operation}

		pm.collector.RecordGauge("latency_min_ms", float64(m.Min.Milliseconds()), labels)
		pm.collector.RecordGauge("latency_max_ms", float64(m.Max.Milliseconds()), labels)
		pm.collector.RecordGauge("latency_avg_ms", float64(m.Average.Milliseconds()), labels)
		pm.collector.RecordGauge("latency_p50_ms", float64(m.P50.Milliseconds()), labels)
		pm.collector.RecordGauge("latency_p95_ms", float64(m.P95.Milliseconds()), labels)
		pm.collector.RecordGauge("latency_p99_ms", float64(m.P99.Milliseconds()), labels)
	}

	// Collect error metrics
	errorMetrics := pm.GetAllErrorMetrics()
	for operation, m := range errorMetrics {
		labels := map[string]string{"operation": operation}

		pm.collector.RecordGauge("error_rate_percent", m.ErrorRate, labels)
		pm.collector.RecordCounter("errors_by_operation_total", float64(m.ErrorCount), labels)

		// Error breakdown by type
		for errorType, count := range m.ErrorsByType {
			typeLabels := map[string]string{
				"operation":  operation,
				"error_type": errorType,
			}
			pm.collector.RecordCounter("errors_by_type_total", float64(count), typeLabels)
		}
	}
}

// SystemMetrics tracks system-level metrics
type SystemMetrics struct {
	CPUUsage      float64   // CPU usage percentage
	MemoryUsage   float64   // Memory usage in bytes
	MemoryPercent float64   // Memory usage percentage
	DiskUsage     float64   // Disk usage in bytes
	DiskPercent   float64   // Disk usage percentage
	NetworkIn     int64     // Network bytes received
	NetworkOut    int64     // Network bytes sent
	Timestamp     time.Time // When metrics were collected
}

// SystemMonitor monitors system resources
type SystemMonitor struct {
	metrics   *SystemMetrics
	mu        sync.RWMutex
	collector MetricsCollector
}

// NewSystemMonitor creates a new system monitor
func NewSystemMonitor(collector MetricsCollector) *SystemMonitor {
	return &SystemMonitor{
		metrics:   &SystemMetrics{},
		collector: collector,
	}
}

// UpdateMetrics updates system metrics (to be called periodically)
func (sm *SystemMonitor) UpdateMetrics(cpu, memUsage, memPercent, diskUsage, diskPercent float64, netIn, netOut int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.metrics.CPUUsage = cpu
	sm.metrics.MemoryUsage = memUsage
	sm.metrics.MemoryPercent = memPercent
	sm.metrics.DiskUsage = diskUsage
	sm.metrics.DiskPercent = diskPercent
	sm.metrics.NetworkIn = netIn
	sm.metrics.NetworkOut = netOut
	sm.metrics.Timestamp = time.Now()

	// Record to collector
	if sm.collector != nil {
		sm.collector.RecordGauge("system_cpu_usage_percent", cpu, nil)
		sm.collector.RecordGauge("system_memory_usage_bytes", memUsage, nil)
		sm.collector.RecordGauge("system_memory_usage_percent", memPercent, nil)
		sm.collector.RecordGauge("system_disk_usage_bytes", diskUsage, nil)
		sm.collector.RecordGauge("system_disk_usage_percent", diskPercent, nil)
		sm.collector.RecordCounter("system_network_in_bytes_total", float64(netIn), nil)
		sm.collector.RecordCounter("system_network_out_bytes_total", float64(netOut), nil)
	}
}

// GetMetrics returns current system metrics
func (sm *SystemMonitor) GetMetrics() SystemMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return *sm.metrics
}
