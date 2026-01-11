// Package hls implements adaptive bitrate (ABR) streaming
package hls

import (
	"fmt"
	"sync"
	"time"
)

// ABRManager manages adaptive bitrate streaming
type ABRManager struct {
	// variants contains all available quality variants
	variants []*Variant

	// segmentDuration is the target segment duration in seconds
	segmentDuration int

	// bandwidthEstimator estimates available bandwidth
	bandwidthEstimator *BandwidthEstimator

	mu sync.RWMutex
}

// BandwidthEstimator estimates available bandwidth
type BandwidthEstimator struct {
	// measurements stores recent bandwidth measurements
	measurements []BandwidthMeasurement

	// maxMeasurements is the maximum number of measurements to keep
	maxMeasurements int

	// currentBandwidth is the estimated current bandwidth in bits/second
	currentBandwidth int

	mu sync.RWMutex
}

// BandwidthMeasurement represents a single bandwidth measurement
type BandwidthMeasurement struct {
	Timestamp time.Time
	Bandwidth int // bits per second
	Duration  time.Duration
	BytesRead int
}

// NewABRManager creates a new ABR manager
func NewABRManager(segmentDuration int) *ABRManager {
	return &ABRManager{
		variants:           make([]*Variant, 0),
		segmentDuration:    segmentDuration,
		bandwidthEstimator: NewBandwidthEstimator(10),
	}
}

// AddVariant adds a quality variant
func (m *ABRManager) AddVariant(variant *Variant) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.variants = append(m.variants, variant)
	m.sortVariants()
}

// RemoveVariant removes a quality variant by name
func (m *ABRManager) RemoveVariant(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, v := range m.variants {
		if v.Name == name {
			m.variants = append(m.variants[:i], m.variants[i+1:]...)
			return
		}
	}
}

// GetVariants returns all available variants
func (m *ABRManager) GetVariants() []*Variant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	variants := make([]*Variant, len(m.variants))
	copy(variants, m.variants)
	return variants
}

// sortVariants sorts variants by bandwidth (ascending)
func (m *ABRManager) sortVariants() {
	// Simple bubble sort (sufficient for small number of variants)
	n := len(m.variants)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if m.variants[j].Bandwidth > m.variants[j+1].Bandwidth {
				m.variants[j], m.variants[j+1] = m.variants[j+1], m.variants[j]
			}
		}
	}
}

// SelectVariant selects the best variant based on current bandwidth
func (m *ABRManager) SelectVariant(currentBandwidth int) (*Variant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.variants) == 0 {
		return nil, fmt.Errorf("no variants available")
	}

	// Select highest quality variant that fits within bandwidth
	// Use 0.9x bandwidth to provide buffer
	targetBandwidth := int(float64(currentBandwidth) * 0.9)

	var selected *Variant
	for _, v := range m.variants {
		if v.Bandwidth <= targetBandwidth {
			selected = v
		} else {
			break
		}
	}

	// If no variant fits, select lowest quality
	if selected == nil {
		selected = m.variants[0]
	}

	return selected, nil
}

// SelectVariantByResolution selects variant by resolution preference
func (m *ABRManager) SelectVariantByResolution(width, height int) (*Variant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.variants) == 0 {
		return nil, fmt.Errorf("no variants available")
	}

	// Find exact match first
	for _, v := range m.variants {
		if v.Width == width && v.Height == height {
			return v, nil
		}
	}

	// Find closest match (prefer lower resolution if exact match not found)
	var closest *Variant
	minDiff := int(^uint(0) >> 1) // Max int

	for _, v := range m.variants {
		diff := abs(v.Width-width) + abs(v.Height-height)
		if diff < minDiff {
			minDiff = diff
			closest = v
		}
	}

	if closest == nil {
		closest = m.variants[0]
	}

	return closest, nil
}

// SelectVariantByName selects variant by name
func (m *ABRManager) SelectVariantByName(name string) (*Variant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, v := range m.variants {
		if v.Name == name {
			return v, nil
		}
	}

	return nil, fmt.Errorf("variant %s not found", name)
}

// UpdateBandwidth updates the bandwidth estimate
func (m *ABRManager) UpdateBandwidth(bytesRead int, duration time.Duration) {
	if m.bandwidthEstimator != nil {
		m.bandwidthEstimator.AddMeasurement(bytesRead, duration)
	}
}

// GetEstimatedBandwidth returns the current bandwidth estimate
func (m *ABRManager) GetEstimatedBandwidth() int {
	if m.bandwidthEstimator != nil {
		return m.bandwidthEstimator.GetBandwidth()
	}
	return 0
}

// NewBandwidthEstimator creates a new bandwidth estimator
func NewBandwidthEstimator(maxMeasurements int) *BandwidthEstimator {
	return &BandwidthEstimator{
		measurements:     make([]BandwidthMeasurement, 0, maxMeasurements),
		maxMeasurements:  maxMeasurements,
		currentBandwidth: 0,
	}
}

// AddMeasurement adds a bandwidth measurement
func (e *BandwidthEstimator) AddMeasurement(bytesRead int, duration time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if duration == 0 {
		return
	}

	// Calculate bandwidth in bits per second
	bandwidth := int(float64(bytesRead*8) / duration.Seconds())

	measurement := BandwidthMeasurement{
		Timestamp: time.Now(),
		Bandwidth: bandwidth,
		Duration:  duration,
		BytesRead: bytesRead,
	}

	e.measurements = append(e.measurements, measurement)

	// Keep only recent measurements
	if len(e.measurements) > e.maxMeasurements {
		e.measurements = e.measurements[1:]
	}

	// Recalculate current bandwidth
	e.calculateBandwidth()
}

// calculateBandwidth calculates average bandwidth from measurements
func (e *BandwidthEstimator) calculateBandwidth() {
	if len(e.measurements) == 0 {
		e.currentBandwidth = 0
		return
	}

	// Use exponential weighted moving average (EWMA)
	// More recent measurements have higher weight
	totalWeight := 0.0
	weightedSum := 0.0

	for i, m := range e.measurements {
		// Weight increases for more recent measurements
		weight := float64(i + 1)
		totalWeight += weight
		weightedSum += float64(m.Bandwidth) * weight
	}

	e.currentBandwidth = int(weightedSum / totalWeight)
}

// GetBandwidth returns the current bandwidth estimate
func (e *BandwidthEstimator) GetBandwidth() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentBandwidth
}

// GetMeasurements returns all measurements
func (e *BandwidthEstimator) GetMeasurements() []BandwidthMeasurement {
	e.mu.RLock()
	defer e.mu.RUnlock()

	measurements := make([]BandwidthMeasurement, len(e.measurements))
	copy(measurements, e.measurements)
	return measurements
}

// Reset resets all measurements
func (e *BandwidthEstimator) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.measurements = make([]BandwidthMeasurement, 0, e.maxMeasurements)
	e.currentBandwidth = 0
}

// VariantSelector provides variant selection logic
type VariantSelector interface {
	// SelectVariant selects the best variant based on current conditions
	SelectVariant(availableVariants []*Variant, currentBandwidth int) (*Variant, error)
}

// DefaultVariantSelector implements default ABR logic
type DefaultVariantSelector struct{}

// SelectVariant selects variant using default ABR algorithm
func (s *DefaultVariantSelector) SelectVariant(availableVariants []*Variant, currentBandwidth int) (*Variant, error) {
	if len(availableVariants) == 0 {
		return nil, fmt.Errorf("no variants available")
	}

	// Sort variants by bandwidth
	sortedVariants := make([]*Variant, len(availableVariants))
	copy(sortedVariants, availableVariants)

	// Simple bubble sort
	n := len(sortedVariants)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if sortedVariants[j].Bandwidth > sortedVariants[j+1].Bandwidth {
				sortedVariants[j], sortedVariants[j+1] = sortedVariants[j+1], sortedVariants[j]
			}
		}
	}

	// Select highest quality that fits (with 10% buffer)
	targetBandwidth := int(float64(currentBandwidth) * 0.9)

	var selected *Variant
	for _, v := range sortedVariants {
		if v.Bandwidth <= targetBandwidth {
			selected = v
		} else {
			break
		}
	}

	// If nothing fits, use lowest quality
	if selected == nil {
		selected = sortedVariants[0]
	}

	return selected, nil
}

// ConservativeVariantSelector implements conservative ABR logic
type ConservativeVariantSelector struct{}

// SelectVariant selects variant using conservative approach (more buffer)
func (s *ConservativeVariantSelector) SelectVariant(availableVariants []*Variant, currentBandwidth int) (*Variant, error) {
	if len(availableVariants) == 0 {
		return nil, fmt.Errorf("no variants available")
	}

	// Use 75% of bandwidth for more conservative approach
	targetBandwidth := int(float64(currentBandwidth) * 0.75)

	var selected *Variant
	for _, v := range availableVariants {
		if v.Bandwidth <= targetBandwidth {
			selected = v
		} else {
			break
		}
	}

	if selected == nil && len(availableVariants) > 0 {
		selected = availableVariants[0]
	}

	return selected, nil
}

// AggressiveVariantSelector implements aggressive ABR logic
type AggressiveVariantSelector struct{}

// SelectVariant selects variant using aggressive approach (less buffer)
func (s *AggressiveVariantSelector) SelectVariant(availableVariants []*Variant, currentBandwidth int) (*Variant, error) {
	if len(availableVariants) == 0 {
		return nil, fmt.Errorf("no variants available")
	}

	// Use 95% of bandwidth for aggressive approach
	targetBandwidth := int(float64(currentBandwidth) * 0.95)

	var selected *Variant
	for _, v := range availableVariants {
		if v.Bandwidth <= targetBandwidth {
			selected = v
		} else {
			break
		}
	}

	if selected == nil && len(availableVariants) > 0 {
		selected = availableVariants[0]
	}

	return selected, nil
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
