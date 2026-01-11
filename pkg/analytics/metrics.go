package analytics

import (
	"sync"
	"time"
)

// MetricType represents the type of metric being collected
type MetricType string

const (
	// MetricTypeCounter represents a monotonically increasing counter
	MetricTypeCounter MetricType = "counter"
	// MetricTypeGauge represents a value that can go up and down
	MetricTypeGauge MetricType = "gauge"
	// MetricTypeHistogram represents a distribution of values
	MetricTypeHistogram MetricType = "histogram"
	// MetricTypeSummary represents a summary of observations
	MetricTypeSummary MetricType = "summary"
)

// Metric represents a single metric data point
type Metric struct {
	Name      string                 // Metric name (e.g., "stream_viewers")
	Type      MetricType             // Type of metric
	Value     float64                // Current value
	Labels    map[string]string      // Labels for metric dimensions
	Timestamp time.Time              // When the metric was recorded
	Help      string                 // Description of the metric
	Metadata  map[string]interface{} // Additional metadata
}

// MetricsSnapshot represents a collection of metrics at a point in time
type MetricsSnapshot struct {
	Timestamp time.Time         // Snapshot timestamp
	Metrics   map[string]Metric // Map of metric name to metric
	mu        sync.RWMutex
}

// NewMetricsSnapshot creates a new metrics snapshot
func NewMetricsSnapshot() *MetricsSnapshot {
	return &MetricsSnapshot{
		Timestamp: time.Now(),
		Metrics:   make(map[string]Metric),
	}
}

// Add adds a metric to the snapshot
func (s *MetricsSnapshot) Add(metric Metric) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}
	s.Metrics[metric.Name] = metric
}

// Get retrieves a metric by name
func (s *MetricsSnapshot) Get(name string) (Metric, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metric, exists := s.Metrics[name]
	return metric, exists
}

// GetAll returns all metrics in the snapshot
func (s *MetricsSnapshot) GetAll() map[string]Metric {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]Metric, len(s.Metrics))
	for k, v := range s.Metrics {
		result[k] = v
	}
	return result
}

// MetricsCollector is the interface for collecting metrics
type MetricsCollector interface {
	// RecordCounter increments a counter metric
	RecordCounter(name string, value float64, labels map[string]string)

	// RecordGauge sets a gauge metric to a specific value
	RecordGauge(name string, value float64, labels map[string]string)

	// RecordHistogram records a value in a histogram
	RecordHistogram(name string, value float64, labels map[string]string)

	// RecordSummary records a value in a summary
	RecordSummary(name string, value float64, labels map[string]string)

	// GetSnapshot returns a snapshot of current metrics
	GetSnapshot() *MetricsSnapshot

	// Reset clears all metrics
	Reset()
}

// InMemoryMetricsCollector is an in-memory implementation of MetricsCollector
type InMemoryMetricsCollector struct {
	metrics map[string]*Metric
	mu      sync.RWMutex
}

// NewInMemoryMetricsCollector creates a new in-memory metrics collector
func NewInMemoryMetricsCollector() *InMemoryMetricsCollector {
	return &InMemoryMetricsCollector{
		metrics: make(map[string]*Metric),
	}
}

// RecordCounter increments a counter metric
func (c *InMemoryMetricsCollector) RecordCounter(name string, value float64, labels map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.metricKey(name, labels)
	metric, exists := c.metrics[key]
	if !exists {
		metric = &Metric{
			Name:      name,
			Type:      MetricTypeCounter,
			Value:     0,
			Labels:    labels,
			Timestamp: time.Now(),
		}
		c.metrics[key] = metric
	}

	metric.Value += value
	metric.Timestamp = time.Now()
}

// RecordGauge sets a gauge metric to a specific value
func (c *InMemoryMetricsCollector) RecordGauge(name string, value float64, labels map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.metricKey(name, labels)
	metric, exists := c.metrics[key]
	if !exists {
		metric = &Metric{
			Name:      name,
			Type:      MetricTypeGauge,
			Value:     value,
			Labels:    labels,
			Timestamp: time.Now(),
		}
		c.metrics[key] = metric
	} else {
		metric.Value = value
		metric.Timestamp = time.Now()
	}
}

// RecordHistogram records a value in a histogram
func (c *InMemoryMetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.metricKey(name, labels)
	metric, exists := c.metrics[key]
	if !exists {
		metric = &Metric{
			Name:      name,
			Type:      MetricTypeHistogram,
			Value:     value,
			Labels:    labels,
			Timestamp: time.Now(),
			Metadata:  make(map[string]interface{}),
		}
		metric.Metadata["count"] = 1
		metric.Metadata["sum"] = value
		c.metrics[key] = metric
	} else {
		count := metric.Metadata["count"].(int) + 1
		sum := metric.Metadata["sum"].(float64) + value
		metric.Metadata["count"] = count
		metric.Metadata["sum"] = sum
		metric.Value = sum / float64(count) // Average
		metric.Timestamp = time.Now()
	}
}

// RecordSummary records a value in a summary
func (c *InMemoryMetricsCollector) RecordSummary(name string, value float64, labels map[string]string) {
	// For simplicity, treat summary similar to histogram
	c.RecordHistogram(name, value, labels)
}

// GetSnapshot returns a snapshot of current metrics
func (c *InMemoryMetricsCollector) GetSnapshot() *MetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := NewMetricsSnapshot()
	for _, metric := range c.metrics {
		snapshot.Add(*metric)
	}

	return snapshot
}

// Reset clears all metrics
func (c *InMemoryMetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics = make(map[string]*Metric)
}

// metricKey generates a unique key for a metric based on name and labels
func (c *InMemoryMetricsCollector) metricKey(name string, labels map[string]string) string {
	key := name
	if labels != nil {
		for k, v := range labels {
			key += "_" + k + ":" + v
		}
	}
	return key
}

// MetricsRegistry holds multiple collectors for different subsystems
type MetricsRegistry struct {
	collectors map[string]MetricsCollector
	mu         sync.RWMutex
}

// NewMetricsRegistry creates a new metrics registry
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		collectors: make(map[string]MetricsCollector),
	}
}

// Register registers a collector with a name
func (r *MetricsRegistry) Register(name string, collector MetricsCollector) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.collectors[name] = collector
}

// Get retrieves a collector by name
func (r *MetricsRegistry) Get(name string) (MetricsCollector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	collector, exists := r.collectors[name]
	return collector, exists
}

// GetAllSnapshots returns snapshots from all collectors
func (r *MetricsRegistry) GetAllSnapshots() map[string]*MetricsSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshots := make(map[string]*MetricsSnapshot)
	for name, collector := range r.collectors {
		snapshots[name] = collector.GetSnapshot()
	}

	return snapshots
}

// TimeSeries represents a time series of metric values
type TimeSeries struct {
	Name       string                // Metric name
	Labels     map[string]string     // Metric labels
	DataPoints []TimeSeriesDataPoint // Data points over time
	mu         sync.RWMutex
}

// TimeSeriesDataPoint represents a single data point in a time series
type TimeSeriesDataPoint struct {
	Timestamp time.Time // When the value was recorded
	Value     float64   // The value at this time
}

// NewTimeSeries creates a new time series
func NewTimeSeries(name string, labels map[string]string) *TimeSeries {
	return &TimeSeries{
		Name:       name,
		Labels:     labels,
		DataPoints: make([]TimeSeriesDataPoint, 0),
	}
}

// Add adds a data point to the time series
func (ts *TimeSeries) Add(timestamp time.Time, value float64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.DataPoints = append(ts.DataPoints, TimeSeriesDataPoint{
		Timestamp: timestamp,
		Value:     value,
	})
}

// GetRange returns data points within a time range
func (ts *TimeSeries) GetRange(start, end time.Time) []TimeSeriesDataPoint {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []TimeSeriesDataPoint
	for _, dp := range ts.DataPoints {
		if (dp.Timestamp.Equal(start) || dp.Timestamp.After(start)) &&
			(dp.Timestamp.Equal(end) || dp.Timestamp.Before(end)) {
			result = append(result, dp)
		}
	}

	return result
}

// Prune removes data points older than the specified duration
func (ts *TimeSeries) Prune(maxAge time.Duration) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	newDataPoints := make([]TimeSeriesDataPoint, 0)

	for _, dp := range ts.DataPoints {
		if dp.Timestamp.After(cutoff) {
			newDataPoints = append(newDataPoints, dp)
		}
	}

	ts.DataPoints = newDataPoints
}

// TimeSeriesStore stores multiple time series
type TimeSeriesStore struct {
	series map[string]*TimeSeries
	mu     sync.RWMutex
}

// NewTimeSeriesStore creates a new time series store
func NewTimeSeriesStore() *TimeSeriesStore {
	return &TimeSeriesStore{
		series: make(map[string]*TimeSeries),
	}
}

// Record records a data point for a time series
func (s *TimeSeriesStore) Record(name string, labels map[string]string, timestamp time.Time, value float64) {
	key := s.seriesKey(name, labels)

	s.mu.Lock()
	ts, exists := s.series[key]
	if !exists {
		ts = NewTimeSeries(name, labels)
		s.series[key] = ts
	}
	s.mu.Unlock()

	ts.Add(timestamp, value)
}

// Get retrieves a time series by name and labels
func (s *TimeSeriesStore) Get(name string, labels map[string]string) (*TimeSeries, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.seriesKey(name, labels)
	ts, exists := s.series[key]
	return ts, exists
}

// GetAll returns all time series
func (s *TimeSeriesStore) GetAll() map[string]*TimeSeries {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*TimeSeries)
	for k, v := range s.series {
		result[k] = v
	}
	return result
}

// PruneAll removes old data points from all time series
func (s *TimeSeriesStore) PruneAll(maxAge time.Duration) {
	s.mu.RLock()
	series := make([]*TimeSeries, 0, len(s.series))
	for _, ts := range s.series {
		series = append(series, ts)
	}
	s.mu.RUnlock()

	for _, ts := range series {
		ts.Prune(maxAge)
	}
}

// seriesKey generates a unique key for a time series
func (s *TimeSeriesStore) seriesKey(name string, labels map[string]string) string {
	key := name
	if labels != nil {
		for k, v := range labels {
			key += "_" + k + ":" + v
		}
	}
	return key
}
