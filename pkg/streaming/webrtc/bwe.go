// Package webrtc provides bandwidth estimation and congestion control for WebRTC.
package webrtc

import (
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// BandwidthEstimator estimates available bandwidth using various metrics
type BandwidthEstimator struct {
	// config is the BWE configuration
	config BWEConfig

	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// currentBitrate is the current estimated bitrate
	currentBitrate int

	// targetBitrate is the target bitrate
	targetBitrate int

	// lastUpdate is the timestamp of the last update
	lastUpdate time.Time

	// bytesSent tracks bytes sent
	bytesSent uint64

	// packetsLost tracks packets lost
	packetsLost uint64

	// rtt is the current round-trip time
	rtt time.Duration

	// lossRate is the current packet loss rate
	lossRate float64
}

// NewBandwidthEstimator creates a new bandwidth estimator
func NewBandwidthEstimator(config BWEConfig, log logger.Logger) *BandwidthEstimator {
	return &BandwidthEstimator{
		config:         config,
		logger:         log,
		currentBitrate: config.StartBitrate,
		targetBitrate:  config.StartBitrate,
		lastUpdate:     time.Now(),
	}
}

// Update updates the bandwidth estimate based on network metrics
func (bwe *BandwidthEstimator) Update(bytesSent, packetsLost uint64, rtt time.Duration) int {
	bwe.mu.Lock()
	defer bwe.mu.Unlock()

	now := time.Now()
	deltaTime := now.Sub(bwe.lastUpdate).Seconds()

	if deltaTime == 0 {
		return bwe.currentBitrate
	}

	// Calculate current bitrate
	deltaBytes := bytesSent - bwe.bytesSent
	measuredBitrate := int(float64(deltaBytes*8) / deltaTime)

	// Calculate packet loss rate
	deltaLost := packetsLost - bwe.packetsLost
	if deltaBytes > 0 {
		bwe.lossRate = float64(deltaLost) / float64(deltaBytes)
	}

	bwe.rtt = rtt
	bwe.bytesSent = bytesSent
	bwe.packetsLost = packetsLost

	// Adjust bitrate based on network conditions
	bwe.adjustBitrate(measuredBitrate)

	bwe.lastUpdate = now

	bwe.logger.Debug("Updated bandwidth estimate",
		logger.Field{Key: "current_bitrate", Value: bwe.currentBitrate},
		logger.Field{Key: "target_bitrate", Value: bwe.targetBitrate},
		logger.Field{Key: "loss_rate", Value: bwe.lossRate},
		logger.Field{Key: "rtt_ms", Value: rtt.Milliseconds()},
	)

	return bwe.currentBitrate
}

// adjustBitrate adjusts the bitrate based on network conditions
func (bwe *BandwidthEstimator) adjustBitrate(measuredBitrate int) {
	// Check if we should decrease bitrate
	if bwe.shouldDecrease() {
		// Decrease bitrate
		bwe.targetBitrate = int(float64(bwe.targetBitrate) * bwe.config.RampDownFactor)
	} else if bwe.shouldIncrease(measuredBitrate) {
		// Increase bitrate
		bwe.targetBitrate = int(float64(bwe.targetBitrate) * bwe.config.RampUpFactor)
	}

	// Clamp to min/max
	if bwe.targetBitrate < bwe.config.MinBitrate {
		bwe.targetBitrate = bwe.config.MinBitrate
	}
	if bwe.targetBitrate > bwe.config.MaxBitrate {
		bwe.targetBitrate = bwe.config.MaxBitrate
	}

	// Smoothly transition to target bitrate
	bwe.currentBitrate = bwe.smoothTransition(bwe.currentBitrate, bwe.targetBitrate)
}

// shouldDecrease determines if bitrate should be decreased
func (bwe *BandwidthEstimator) shouldDecrease() bool {
	// Decrease if packet loss is high
	if bwe.lossRate > bwe.config.LossThreshold {
		return true
	}

	// Decrease if RTT is high
	if bwe.rtt > bwe.config.RTTThreshold {
		return true
	}

	return false
}

// shouldIncrease determines if bitrate should be increased
func (bwe *BandwidthEstimator) shouldIncrease(measuredBitrate int) bool {
	// Don't increase if packet loss is present
	if bwe.lossRate > bwe.config.LossThreshold/2 {
		return false
	}

	// Don't increase if RTT is elevated
	if bwe.rtt > bwe.config.RTTThreshold/2 {
		return false
	}

	// Increase if we're using most of current bitrate
	utilizationRate := float64(measuredBitrate) / float64(bwe.currentBitrate)
	if utilizationRate > 0.9 {
		return true
	}

	return false
}

// smoothTransition smoothly transitions from current to target bitrate
func (bwe *BandwidthEstimator) smoothTransition(current, target int) int {
	diff := target - current
	step := diff / 10 // Transition over ~10 updates

	if step == 0 {
		return target
	}

	return current + step
}

// GetCurrentBitrate returns the current estimated bitrate
func (bwe *BandwidthEstimator) GetCurrentBitrate() int {
	bwe.mu.RLock()
	defer bwe.mu.RUnlock()

	return bwe.currentBitrate
}

// GetTargetBitrate returns the target bitrate
func (bwe *BandwidthEstimator) GetTargetBitrate() int {
	bwe.mu.RLock()
	defer bwe.mu.RUnlock()

	return bwe.targetBitrate
}

// GetLossRate returns the current packet loss rate
func (bwe *BandwidthEstimator) GetLossRate() float64 {
	bwe.mu.RLock()
	defer bwe.mu.RUnlock()

	return bwe.lossRate
}

// GetRTT returns the current round-trip time
func (bwe *BandwidthEstimator) GetRTT() time.Duration {
	bwe.mu.RLock()
	defer bwe.mu.RUnlock()

	return bwe.rtt
}

// SetBitrate manually sets the current bitrate
func (bwe *BandwidthEstimator) SetBitrate(bitrate int) {
	bwe.mu.Lock()
	defer bwe.mu.Unlock()

	if bitrate < bwe.config.MinBitrate {
		bitrate = bwe.config.MinBitrate
	}
	if bitrate > bwe.config.MaxBitrate {
		bitrate = bwe.config.MaxBitrate
	}

	bwe.currentBitrate = bitrate
	bwe.targetBitrate = bitrate

	bwe.logger.Info("Set bitrate manually",
		logger.Field{Key: "bitrate", Value: bitrate},
	)
}

// Reset resets the bandwidth estimator to initial state
func (bwe *BandwidthEstimator) Reset() {
	bwe.mu.Lock()
	defer bwe.mu.Unlock()

	bwe.currentBitrate = bwe.config.StartBitrate
	bwe.targetBitrate = bwe.config.StartBitrate
	bwe.lastUpdate = time.Now()
	bwe.bytesSent = 0
	bwe.packetsLost = 0
	bwe.rtt = 0
	bwe.lossRate = 0

	bwe.logger.Info("Reset bandwidth estimator")
}

// CongestionController manages congestion control for WebRTC
type CongestionController struct {
	// bwe is the bandwidth estimator
	bwe *BandwidthEstimator

	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// congestionDetected indicates if congestion is detected
	congestionDetected bool

	// lastCongestionTime is the timestamp of last detected congestion
	lastCongestionTime time.Time
}

// NewCongestionController creates a new congestion controller
func NewCongestionController(config BWEConfig, log logger.Logger) *CongestionController {
	return &CongestionController{
		bwe:    NewBandwidthEstimator(config, log),
		logger: log,
	}
}

// Update updates congestion state based on network metrics
func (cc *CongestionController) Update(bytesSent, packetsLost uint64, rtt time.Duration) {
	bitrate := cc.bwe.Update(bytesSent, packetsLost, rtt)

	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Detect congestion
	lossRate := cc.bwe.GetLossRate()
	congested := lossRate > 0.1 || rtt > 500*time.Millisecond

	if congested && !cc.congestionDetected {
		cc.congestionDetected = true
		cc.lastCongestionTime = time.Now()

		cc.logger.Warn("Congestion detected",
			logger.Field{Key: "loss_rate", Value: lossRate},
			logger.Field{Key: "rtt_ms", Value: rtt.Milliseconds()},
			logger.Field{Key: "bitrate", Value: bitrate},
		)
	} else if !congested && cc.congestionDetected {
		cc.congestionDetected = false

		cc.logger.Info("Congestion cleared",
			logger.Field{Key: "bitrate", Value: bitrate},
		)
	}
}

// IsCongested returns whether congestion is detected
func (cc *CongestionController) IsCongested() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	return cc.congestionDetected
}

// GetRecommendedBitrate returns the recommended bitrate
func (cc *CongestionController) GetRecommendedBitrate() int {
	return cc.bwe.GetCurrentBitrate()
}

// GetBandwidthEstimator returns the bandwidth estimator
func (cc *CongestionController) GetBandwidthEstimator() *BandwidthEstimator {
	return cc.bwe
}
