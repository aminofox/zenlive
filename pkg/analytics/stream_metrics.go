package analytics

import (
	"sync"
	"time"
)

// StreamMetrics represents metrics for a single stream
type StreamMetrics struct {
	StreamID  string    // Unique stream identifier
	StartTime time.Time // When the stream started
	EndTime   time.Time // When the stream ended (zero if still live)

	// Viewer metrics
	CurrentViewers int // Current number of viewers
	PeakViewers    int // Maximum concurrent viewers
	TotalViewers   int // Total unique viewers

	// Video quality metrics
	CurrentBitrate float64 // Current bitrate in bps
	AverageBitrate float64 // Average bitrate over stream duration
	PeakBitrate    float64 // Maximum bitrate recorded

	CurrentFPS float64 // Current frames per second
	AverageFPS float64 // Average FPS over stream duration
	TargetFPS  float64 // Target FPS for the stream

	// Frame drop metrics
	DroppedFrames int64   // Total number of dropped frames
	TotalFrames   int64   // Total number of frames
	DropRate      float64 // Percentage of dropped frames

	// Resolution metrics
	Width  int // Current video width
	Height int // Current video height

	// Audio metrics
	AudioBitrate    float64 // Current audio bitrate in bps
	AudioSampleRate int     // Audio sample rate in Hz

	// Network metrics
	BytesSent     int64   // Total bytes sent
	BytesReceived int64   // Total bytes received
	PacketsLost   int64   // Total packets lost
	Jitter        float64 // Network jitter in ms
	RTT           float64 // Round-trip time in ms

	mu sync.RWMutex
}

// NewStreamMetrics creates a new stream metrics instance
func NewStreamMetrics(streamID string) *StreamMetrics {
	return &StreamMetrics{
		StreamID:  streamID,
		StartTime: time.Now(),
		TargetFPS: 30.0, // Default target FPS
	}
}

// UpdateViewers updates the viewer count
func (sm *StreamMetrics) UpdateViewers(count int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.CurrentViewers = count
	if count > sm.PeakViewers {
		sm.PeakViewers = count
	}
}

// IncrementTotalViewers increments the total unique viewers
func (sm *StreamMetrics) IncrementTotalViewers() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.TotalViewers++
}

// UpdateBitrate updates the bitrate metrics
func (sm *StreamMetrics) UpdateBitrate(bitrate float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.CurrentBitrate = bitrate

	// Update average bitrate
	if sm.AverageBitrate == 0 {
		sm.AverageBitrate = bitrate
	} else {
		// Exponential moving average
		sm.AverageBitrate = 0.9*sm.AverageBitrate + 0.1*bitrate
	}

	if bitrate > sm.PeakBitrate {
		sm.PeakBitrate = bitrate
	}
}

// UpdateFPS updates the FPS metrics
func (sm *StreamMetrics) UpdateFPS(fps float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.CurrentFPS = fps

	// Update average FPS
	if sm.AverageFPS == 0 {
		sm.AverageFPS = fps
	} else {
		// Exponential moving average
		sm.AverageFPS = 0.9*sm.AverageFPS + 0.1*fps
	}
}

// RecordDroppedFrames records dropped frames
func (sm *StreamMetrics) RecordDroppedFrames(dropped int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.DroppedFrames += dropped
	sm.TotalFrames += dropped // Also count as total frames
	sm.updateDropRate()
}

// RecordFrames records successfully transmitted frames
func (sm *StreamMetrics) RecordFrames(count int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.TotalFrames += count
	sm.updateDropRate()
}

// updateDropRate updates the drop rate (must be called with lock held)
func (sm *StreamMetrics) updateDropRate() {
	if sm.TotalFrames > 0 {
		sm.DropRate = float64(sm.DroppedFrames) / float64(sm.TotalFrames) * 100.0
	}
}

// UpdateResolution updates the video resolution
func (sm *StreamMetrics) UpdateResolution(width, height int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.Width = width
	sm.Height = height
}

// UpdateAudio updates audio metrics
func (sm *StreamMetrics) UpdateAudio(bitrate float64, sampleRate int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.AudioBitrate = bitrate
	sm.AudioSampleRate = sampleRate
}

// UpdateNetwork updates network metrics
func (sm *StreamMetrics) UpdateNetwork(bytesSent, bytesReceived, packetsLost int64, jitter, rtt float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.BytesSent = bytesSent
	sm.BytesReceived = bytesReceived
	sm.PacketsLost = packetsLost
	sm.Jitter = jitter
	sm.RTT = rtt
}

// End marks the stream as ended
func (sm *StreamMetrics) End() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.EndTime = time.Now()
}

// GetDuration returns the stream duration
func (sm *StreamMetrics) GetDuration() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.EndTime.IsZero() {
		return time.Since(sm.StartTime)
	}
	return sm.EndTime.Sub(sm.StartTime)
}

// IsLive returns whether the stream is currently live
func (sm *StreamMetrics) IsLive() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.EndTime.IsZero()
}

// StreamMetricsSnapshot is a snapshot of stream metrics without locks
type StreamMetricsSnapshot struct {
	StreamID        string
	StartTime       time.Time
	EndTime         time.Time
	CurrentViewers  int
	PeakViewers     int
	TotalViewers    int
	CurrentBitrate  float64
	AverageBitrate  float64
	PeakBitrate     float64
	CurrentFPS      float64
	AverageFPS      float64
	TargetFPS       float64
	DroppedFrames   int64
	TotalFrames     int64
	DropRate        float64
	Width           int
	Height          int
	AudioBitrate    float64
	AudioSampleRate int
	BytesSent       int64
	BytesReceived   int64
	PacketsLost     int64
	Jitter          float64
	RTT             float64
}

// GetSnapshot returns a snapshot of current metrics
func (sm *StreamMetrics) GetSnapshot() StreamMetricsSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return StreamMetricsSnapshot{
		StreamID:        sm.StreamID,
		StartTime:       sm.StartTime,
		EndTime:         sm.EndTime,
		CurrentViewers:  sm.CurrentViewers,
		PeakViewers:     sm.PeakViewers,
		TotalViewers:    sm.TotalViewers,
		CurrentBitrate:  sm.CurrentBitrate,
		AverageBitrate:  sm.AverageBitrate,
		PeakBitrate:     sm.PeakBitrate,
		CurrentFPS:      sm.CurrentFPS,
		AverageFPS:      sm.AverageFPS,
		TargetFPS:       sm.TargetFPS,
		DroppedFrames:   sm.DroppedFrames,
		TotalFrames:     sm.TotalFrames,
		DropRate:        sm.DropRate,
		Width:           sm.Width,
		Height:          sm.Height,
		AudioBitrate:    sm.AudioBitrate,
		AudioSampleRate: sm.AudioSampleRate,
		BytesSent:       sm.BytesSent,
		BytesReceived:   sm.BytesReceived,
		PacketsLost:     sm.PacketsLost,
		Jitter:          sm.Jitter,
		RTT:             sm.RTT,
	}
}

// StreamMetricsCollector collects metrics for multiple streams
type StreamMetricsCollector struct {
	streams map[string]*StreamMetrics
	mu      sync.RWMutex

	// Global metrics collector
	collector MetricsCollector
}

// NewStreamMetricsCollector creates a new stream metrics collector
func NewStreamMetricsCollector(collector MetricsCollector) *StreamMetricsCollector {
	return &StreamMetricsCollector{
		streams:   make(map[string]*StreamMetrics),
		collector: collector,
	}
}

// StartStream starts tracking metrics for a stream
func (smc *StreamMetricsCollector) StartStream(streamID string) *StreamMetrics {
	smc.mu.Lock()
	defer smc.mu.Unlock()

	metrics := NewStreamMetrics(streamID)
	smc.streams[streamID] = metrics

	// Record stream start
	if smc.collector != nil {
		smc.collector.RecordCounter("stream_starts_total", 1, map[string]string{
			"stream_id": streamID,
		})
	}

	return metrics
}

// EndStream ends tracking for a stream
func (smc *StreamMetricsCollector) EndStream(streamID string) {
	smc.mu.Lock()
	metrics, exists := smc.streams[streamID]
	smc.mu.Unlock()

	if !exists {
		return
	}

	metrics.End()

	// Record stream end and final metrics
	if smc.collector != nil {
		labels := map[string]string{"stream_id": streamID}

		smc.collector.RecordCounter("stream_ends_total", 1, labels)
		smc.collector.RecordHistogram("stream_duration_seconds", metrics.GetDuration().Seconds(), labels)
		smc.collector.RecordGauge("stream_peak_viewers", float64(metrics.PeakViewers), labels)
		smc.collector.RecordGauge("stream_total_viewers", float64(metrics.TotalViewers), labels)
		smc.collector.RecordGauge("stream_average_bitrate", metrics.AverageBitrate, labels)
		smc.collector.RecordGauge("stream_drop_rate_percent", metrics.DropRate, labels)
	}
}

// GetStream retrieves metrics for a specific stream
func (smc *StreamMetricsCollector) GetStream(streamID string) (*StreamMetrics, bool) {
	smc.mu.RLock()
	defer smc.mu.RUnlock()

	metrics, exists := smc.streams[streamID]
	return metrics, exists
}

// GetAllStreams returns metrics for all streams
func (smc *StreamMetricsCollector) GetAllStreams() map[string]*StreamMetrics {
	smc.mu.RLock()
	defer smc.mu.RUnlock()

	result := make(map[string]*StreamMetrics)
	for k, v := range smc.streams {
		result[k] = v
	}
	return result
}

// GetLiveStreams returns metrics for currently live streams
func (smc *StreamMetricsCollector) GetLiveStreams() map[string]*StreamMetrics {
	smc.mu.RLock()
	defer smc.mu.RUnlock()

	result := make(map[string]*StreamMetrics)
	for k, v := range smc.streams {
		if v.IsLive() {
			result[k] = v
		}
	}
	return result
}

// CollectMetrics collects current metrics for all streams
func (smc *StreamMetricsCollector) CollectMetrics() {
	smc.mu.RLock()
	streams := make([]*StreamMetrics, 0, len(smc.streams))
	for _, metrics := range smc.streams {
		if metrics.IsLive() {
			streams = append(streams, metrics)
		}
	}
	smc.mu.RUnlock()

	if smc.collector == nil {
		return
	}

	// Collect metrics for each live stream
	for _, metrics := range streams {
		snapshot := metrics.GetSnapshot()
		labels := map[string]string{"stream_id": snapshot.StreamID}

		smc.collector.RecordGauge("stream_viewers_current", float64(snapshot.CurrentViewers), labels)
		smc.collector.RecordGauge("stream_bitrate_bps", snapshot.CurrentBitrate, labels)
		smc.collector.RecordGauge("stream_fps", snapshot.CurrentFPS, labels)
		smc.collector.RecordGauge("stream_dropped_frames_total", float64(snapshot.DroppedFrames), labels)
		smc.collector.RecordGauge("stream_drop_rate_percent", snapshot.DropRate, labels)
		smc.collector.RecordGauge("stream_jitter_ms", snapshot.Jitter, labels)
		smc.collector.RecordGauge("stream_rtt_ms", snapshot.RTT, labels)
	}
}

// CleanupEndedStreams removes metrics for streams that ended more than the specified duration ago
func (smc *StreamMetricsCollector) CleanupEndedStreams(maxAge time.Duration) {
	smc.mu.Lock()
	defer smc.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for streamID, metrics := range smc.streams {
		if !metrics.IsLive() && !metrics.EndTime.IsZero() && metrics.EndTime.Before(cutoff) {
			delete(smc.streams, streamID)
		}
	}
}

// StreamQualityAnalyzer analyzes stream quality based on metrics
type StreamQualityAnalyzer struct {
	// Quality thresholds
	MinBitrate  float64 // Minimum acceptable bitrate
	MaxDropRate float64 // Maximum acceptable drop rate (percentage)
	MinFPS      float64 // Minimum acceptable FPS
	MaxJitter   float64 // Maximum acceptable jitter (ms)
	MaxRTT      float64 // Maximum acceptable RTT (ms)
}

// NewStreamQualityAnalyzer creates a new quality analyzer with default thresholds
func NewStreamQualityAnalyzer() *StreamQualityAnalyzer {
	return &StreamQualityAnalyzer{
		MinBitrate:  500000, // 500 kbps
		MaxDropRate: 5.0,    // 5%
		MinFPS:      24.0,   // 24 fps
		MaxJitter:   50.0,   // 50 ms
		MaxRTT:      200.0,  // 200 ms
	}
}

// QualityScore represents the quality assessment of a stream
type QualityScore struct {
	Overall   float64  // Overall quality score (0-100)
	Bitrate   float64  // Bitrate score (0-100)
	FPS       float64  // FPS score (0-100)
	Stability float64  // Stability score (0-100)
	Network   float64  // Network score (0-100)
	Issues    []string // List of quality issues
}

// AnalyzeQuality analyzes the quality of a stream
func (qa *StreamQualityAnalyzer) AnalyzeQuality(metrics *StreamMetrics) QualityScore {
	snapshot := metrics.GetSnapshot()
	score := QualityScore{
		Issues: make([]string, 0),
	}

	// Bitrate score
	if snapshot.CurrentBitrate >= qa.MinBitrate*2 {
		score.Bitrate = 100
	} else if snapshot.CurrentBitrate >= qa.MinBitrate {
		score.Bitrate = 50 + (snapshot.CurrentBitrate-qa.MinBitrate)/(qa.MinBitrate)*50
	} else {
		score.Bitrate = (snapshot.CurrentBitrate / qa.MinBitrate) * 50
		score.Issues = append(score.Issues, "Low bitrate")
	}

	// FPS score
	if snapshot.CurrentFPS >= snapshot.TargetFPS {
		score.FPS = 100
	} else if snapshot.CurrentFPS >= qa.MinFPS {
		score.FPS = 50 + (snapshot.CurrentFPS-qa.MinFPS)/(snapshot.TargetFPS-qa.MinFPS)*50
	} else {
		score.FPS = (snapshot.CurrentFPS / qa.MinFPS) * 50
		score.Issues = append(score.Issues, "Low FPS")
	}

	// Stability score (based on drop rate)
	if snapshot.DropRate <= qa.MaxDropRate/2 {
		score.Stability = 100
	} else if snapshot.DropRate <= qa.MaxDropRate {
		score.Stability = 50 + (qa.MaxDropRate-snapshot.DropRate)/(qa.MaxDropRate/2)*50
	} else {
		score.Stability = 50 - ((snapshot.DropRate-qa.MaxDropRate)/qa.MaxDropRate)*50
		if score.Stability < 0 {
			score.Stability = 0
		}
		score.Issues = append(score.Issues, "High frame drop rate")
	}

	// Network score (based on jitter and RTT)
	jitterScore := 100.0
	if snapshot.Jitter > qa.MaxJitter {
		jitterScore = 50 - ((snapshot.Jitter-qa.MaxJitter)/qa.MaxJitter)*50
		if jitterScore < 0 {
			jitterScore = 0
		}
		score.Issues = append(score.Issues, "High jitter")
	}

	rttScore := 100.0
	if snapshot.RTT > qa.MaxRTT {
		rttScore = 50 - ((snapshot.RTT-qa.MaxRTT)/qa.MaxRTT)*50
		if rttScore < 0 {
			rttScore = 0
		}
		score.Issues = append(score.Issues, "High RTT")
	}

	score.Network = (jitterScore + rttScore) / 2

	// Overall score (weighted average)
	score.Overall = (score.Bitrate*0.3 + score.FPS*0.3 + score.Stability*0.25 + score.Network*0.15)

	return score
}
