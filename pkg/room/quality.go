package room

import (
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// NetworkQuality represents the network quality for a participant
type NetworkQuality struct {
	// ParticipantID is the participant identifier
	ParticipantID string `json:"participant_id"`

	// Quality is the overall quality level
	Quality QualityLevel `json:"quality"`

	// PacketLoss is the packet loss percentage (0-100)
	PacketLoss float64 `json:"packet_loss"`

	// Jitter is the jitter in milliseconds
	Jitter time.Duration `json:"jitter"`

	// RTT is the round-trip time in milliseconds
	RTT time.Duration `json:"rtt"`

	// AvailableBandwidth is the estimated available bandwidth in bps
	AvailableBandwidth int `json:"available_bandwidth"`

	// Timestamp is when the measurement was taken
	Timestamp time.Time `json:"timestamp"`

	// Score is a numeric quality score (0-100)
	Score int `json:"score"`
}

// CalculateQualityLevel determines the quality level based on metrics
func (nq *NetworkQuality) CalculateQualityLevel() QualityLevel {
	score := nq.CalculateScore()

	if score >= 80 {
		return QualityHigh
	} else if score >= 50 {
		return QualityMedium
	}
	return QualityLow
}

// CalculateScore calculates a numeric quality score (0-100)
func (nq *NetworkQuality) CalculateScore() int {
	// Packet loss score (0-40 points)
	packetLossScore := 0
	if nq.PacketLoss < 1.0 {
		packetLossScore = 40
	} else if nq.PacketLoss < 3.0 {
		packetLossScore = 30
	} else if nq.PacketLoss < 5.0 {
		packetLossScore = 20
	} else if nq.PacketLoss < 10.0 {
		packetLossScore = 10
	}

	// RTT score (0-30 points)
	rttScore := 0
	rttMs := nq.RTT.Milliseconds()
	if rttMs < 100 {
		rttScore = 30
	} else if rttMs < 200 {
		rttScore = 20
	} else if rttMs < 400 {
		rttScore = 10
	}

	// Jitter score (0-20 points)
	jitterScore := 0
	jitterMs := nq.Jitter.Milliseconds()
	if jitterMs < 20 {
		jitterScore = 20
	} else if jitterMs < 50 {
		jitterScore = 15
	} else if jitterMs < 100 {
		jitterScore = 10
	}

	// Bandwidth score (0-10 points)
	bandwidthScore := 0
	if nq.AvailableBandwidth > 3_000_000 { // > 3 Mbps
		bandwidthScore = 10
	} else if nq.AvailableBandwidth > 1_000_000 { // > 1 Mbps
		bandwidthScore = 7
	} else if nq.AvailableBandwidth > 500_000 { // > 500 Kbps
		bandwidthScore = 4
	}

	score := packetLossScore + rttScore + jitterScore + bandwidthScore
	nq.Score = score

	return score
}

// NetworkQualityMonitor monitors network quality for participants
type NetworkQualityMonitor struct {
	// roomID is the room identifier
	roomID string

	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// qualities maps participant ID to their network quality
	qualities map[string]*NetworkQuality

	// history maps participant ID to quality history
	history map[string][]*NetworkQuality

	// maxHistorySize is the maximum number of quality measurements to keep
	maxHistorySize int

	// onQualityChange callback for quality changes
	onQualityChange func(participantID string, quality *NetworkQuality)

	// thresholds for quality change notifications
	thresholds NetworkQualityThresholds
}

// NetworkQualityThresholds defines thresholds for quality notifications
type NetworkQualityThresholds struct {
	// PacketLossWarning is the packet loss percentage to trigger a warning
	PacketLossWarning float64

	// PacketLossCritical is the packet loss percentage to trigger critical alert
	PacketLossCritical float64

	// RTTWarning is the RTT to trigger a warning
	RTTWarning time.Duration

	// RTTCritical is the RTT to trigger critical alert
	RTTCritical time.Duration

	// JitterWarning is the jitter to trigger a warning
	JitterWarning time.Duration

	// JitterCritical is the jitter to trigger critical alert
	JitterCritical time.Duration

	// MinBandwidth is the minimum acceptable bandwidth
	MinBandwidth int
}

// DefaultNetworkQualityThresholds returns default thresholds
func DefaultNetworkQualityThresholds() NetworkQualityThresholds {
	return NetworkQualityThresholds{
		PacketLossWarning:  3.0,  // 3% packet loss
		PacketLossCritical: 10.0, // 10% packet loss
		RTTWarning:         200 * time.Millisecond,
		RTTCritical:        500 * time.Millisecond,
		JitterWarning:      50 * time.Millisecond,
		JitterCritical:     100 * time.Millisecond,
		MinBandwidth:       500_000, // 500 Kbps
	}
}

// NewNetworkQualityMonitor creates a new network quality monitor
func NewNetworkQualityMonitor(roomID string, log logger.Logger) *NetworkQualityMonitor {
	return &NetworkQualityMonitor{
		roomID:         roomID,
		logger:         log,
		qualities:      make(map[string]*NetworkQuality),
		history:        make(map[string][]*NetworkQuality),
		maxHistorySize: 30, // Keep last 30 measurements
		thresholds:     DefaultNetworkQualityThresholds(),
	}
}

// OnQualityChange sets the callback for quality changes
func (nqm *NetworkQualityMonitor) OnQualityChange(callback func(participantID string, quality *NetworkQuality)) {
	nqm.mu.Lock()
	defer nqm.mu.Unlock()
	nqm.onQualityChange = callback
}

// UpdateQuality updates the network quality for a participant
func (nqm *NetworkQualityMonitor) UpdateQuality(participantID string, packetLoss float64, jitter, rtt time.Duration, bandwidth int) {
	nqm.mu.Lock()
	defer nqm.mu.Unlock()

	quality := &NetworkQuality{
		ParticipantID:      participantID,
		PacketLoss:         packetLoss,
		Jitter:             jitter,
		RTT:                rtt,
		AvailableBandwidth: bandwidth,
		Timestamp:          time.Now(),
	}

	// Calculate quality level and score
	quality.Quality = quality.CalculateQualityLevel()
	quality.CalculateScore()

	// Store current quality
	previousQuality := nqm.qualities[participantID]
	nqm.qualities[participantID] = quality

	// Add to history
	if nqm.history[participantID] == nil {
		nqm.history[participantID] = make([]*NetworkQuality, 0, nqm.maxHistorySize)
	}

	nqm.history[participantID] = append(nqm.history[participantID], quality)

	// Trim history if too large
	if len(nqm.history[participantID]) > nqm.maxHistorySize {
		nqm.history[participantID] = nqm.history[participantID][1:]
	}

	// Check for quality degradation
	nqm.checkThresholds(participantID, quality)

	// Notify if quality level changed
	if previousQuality != nil && previousQuality.Quality != quality.Quality {
		nqm.logger.Info("Network quality changed",
			logger.String("room_id", nqm.roomID),
			logger.String("participant_id", participantID),
			logger.String("previous_quality", string(previousQuality.Quality)),
			logger.String("new_quality", string(quality.Quality)),
			logger.Int("score", quality.Score),
		)

		if nqm.onQualityChange != nil {
			go nqm.onQualityChange(participantID, quality)
		}
	}
}

// checkThresholds checks if quality metrics exceed thresholds
func (nqm *NetworkQualityMonitor) checkThresholds(participantID string, quality *NetworkQuality) {
	// Check packet loss
	if quality.PacketLoss >= nqm.thresholds.PacketLossCritical {
		nqm.logger.Error("Critical packet loss detected",
			logger.String("room_id", nqm.roomID),
			logger.String("participant_id", participantID),
			logger.Field{Key: "packet_loss", Value: quality.PacketLoss},
		)
	} else if quality.PacketLoss >= nqm.thresholds.PacketLossWarning {
		nqm.logger.Warn("High packet loss detected",
			logger.String("room_id", nqm.roomID),
			logger.String("participant_id", participantID),
			logger.Field{Key: "packet_loss", Value: quality.PacketLoss},
		)
	}

	// Check RTT
	if quality.RTT >= nqm.thresholds.RTTCritical {
		nqm.logger.Error("Critical RTT detected",
			logger.String("room_id", nqm.roomID),
			logger.String("participant_id", participantID),
			logger.Int64("rtt_ms", quality.RTT.Milliseconds()),
		)
	} else if quality.RTT >= nqm.thresholds.RTTWarning {
		nqm.logger.Warn("High RTT detected",
			logger.String("room_id", nqm.roomID),
			logger.String("participant_id", participantID),
			logger.Int64("rtt_ms", quality.RTT.Milliseconds()),
		)
	}

	// Check jitter
	if quality.Jitter >= nqm.thresholds.JitterCritical {
		nqm.logger.Error("Critical jitter detected",
			logger.String("room_id", nqm.roomID),
			logger.String("participant_id", participantID),
			logger.Int64("jitter_ms", quality.Jitter.Milliseconds()),
		)
	} else if quality.Jitter >= nqm.thresholds.JitterWarning {
		nqm.logger.Warn("High jitter detected",
			logger.String("room_id", nqm.roomID),
			logger.String("participant_id", participantID),
			logger.Int64("jitter_ms", quality.Jitter.Milliseconds()),
		)
	}

	// Check bandwidth
	if quality.AvailableBandwidth < nqm.thresholds.MinBandwidth {
		nqm.logger.Warn("Low bandwidth detected",
			logger.String("room_id", nqm.roomID),
			logger.String("participant_id", participantID),
			logger.Int("bandwidth_bps", quality.AvailableBandwidth),
		)
	}
}

// GetQuality returns the current network quality for a participant
func (nqm *NetworkQualityMonitor) GetQuality(participantID string) (*NetworkQuality, bool) {
	nqm.mu.RLock()
	defer nqm.mu.RUnlock()

	quality, exists := nqm.qualities[participantID]
	return quality, exists
}

// GetQualityHistory returns the quality history for a participant
func (nqm *NetworkQualityMonitor) GetQualityHistory(participantID string, limit int) []*NetworkQuality {
	nqm.mu.RLock()
	defer nqm.mu.RUnlock()

	history, exists := nqm.history[participantID]
	if !exists {
		return nil
	}

	if limit > 0 && len(history) > limit {
		return history[len(history)-limit:]
	}

	return history
}

// GetAverageQuality calculates average quality metrics over a time period
func (nqm *NetworkQualityMonitor) GetAverageQuality(participantID string, duration time.Duration) *NetworkQuality {
	nqm.mu.RLock()
	defer nqm.mu.RUnlock()

	history, exists := nqm.history[participantID]
	if !exists || len(history) == 0 {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	var packetLossSum, bandwidthSum float64
	var jitterSum, rttSum int64
	count := 0

	for i := len(history) - 1; i >= 0; i-- {
		quality := history[i]
		if quality.Timestamp.Before(cutoff) {
			break
		}

		packetLossSum += quality.PacketLoss
		jitterSum += quality.Jitter.Milliseconds()
		rttSum += quality.RTT.Milliseconds()
		bandwidthSum += float64(quality.AvailableBandwidth)
		count++
	}

	if count == 0 {
		return nil
	}

	avgQuality := &NetworkQuality{
		ParticipantID:      participantID,
		PacketLoss:         packetLossSum / float64(count),
		Jitter:             time.Duration(jitterSum/int64(count)) * time.Millisecond,
		RTT:                time.Duration(rttSum/int64(count)) * time.Millisecond,
		AvailableBandwidth: int(bandwidthSum / float64(count)),
		Timestamp:          time.Now(),
	}

	avgQuality.Quality = avgQuality.CalculateQualityLevel()
	avgQuality.CalculateScore()

	return avgQuality
}

// RemoveParticipant removes quality data for a participant
func (nqm *NetworkQualityMonitor) RemoveParticipant(participantID string) {
	nqm.mu.Lock()
	defer nqm.mu.Unlock()

	delete(nqm.qualities, participantID)
	delete(nqm.history, participantID)

	nqm.logger.Debug("Removed participant from quality monitoring",
		logger.String("room_id", nqm.roomID),
		logger.String("participant_id", participantID),
	)
}

// GetAllQualities returns current quality for all participants
func (nqm *NetworkQualityMonitor) GetAllQualities() map[string]*NetworkQuality {
	nqm.mu.RLock()
	defer nqm.mu.RUnlock()

	result := make(map[string]*NetworkQuality, len(nqm.qualities))
	for id, quality := range nqm.qualities {
		result[id] = quality
	}

	return result
}

// GetRoomAverageQuality calculates average quality for the entire room
func (nqm *NetworkQualityMonitor) GetRoomAverageQuality() *NetworkQuality {
	nqm.mu.RLock()
	defer nqm.mu.RUnlock()

	if len(nqm.qualities) == 0 {
		return nil
	}

	var packetLossSum, bandwidthSum float64
	var jitterSum, rttSum int64
	count := 0

	for _, quality := range nqm.qualities {
		packetLossSum += quality.PacketLoss
		jitterSum += quality.Jitter.Milliseconds()
		rttSum += quality.RTT.Milliseconds()
		bandwidthSum += float64(quality.AvailableBandwidth)
		count++
	}

	avgQuality := &NetworkQuality{
		ParticipantID:      "room_average",
		PacketLoss:         packetLossSum / float64(count),
		Jitter:             time.Duration(jitterSum/int64(count)) * time.Millisecond,
		RTT:                time.Duration(rttSum/int64(count)) * time.Millisecond,
		AvailableBandwidth: int(bandwidthSum / float64(count)),
		Timestamp:          time.Now(),
	}

	avgQuality.Quality = avgQuality.CalculateQualityLevel()
	avgQuality.CalculateScore()

	return avgQuality
}
