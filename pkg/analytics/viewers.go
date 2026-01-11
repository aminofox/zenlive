package analytics

import (
	"sync"
	"time"
)

// ViewerSession represents a single viewer session
type ViewerSession struct {
	SessionID    string    // Unique session identifier
	StreamID     string    // Stream being watched
	UserID       string    // User identifier (empty for anonymous)
	Username     string    // Username (empty for anonymous)
	StartTime    time.Time // When the session started
	EndTime      time.Time // When the session ended (zero if active)
	LastActivity time.Time // Last activity timestamp

	// Geographic information
	Country   string  // Country code (e.g., "US")
	Region    string  // Region/state
	City      string  // City name
	Latitude  float64 // Geographic latitude
	Longitude float64 // Geographic longitude

	// Device information
	DeviceType string // Device type (mobile, tablet, desktop)
	OS         string // Operating system
	Browser    string // Browser name
	UserAgent  string // Full user agent string

	// Connection information
	IPAddress      string // IP address
	ConnectionType string // Connection type (wifi, cellular, etc.)

	// Watch metrics
	TotalWatchTime time.Duration // Total time watched
	BufferingTime  time.Duration // Total time spent buffering
	BufferingCount int           // Number of buffering events

	// Quality metrics
	AverageQuality string // Average quality level watched
	QualityChanges int    // Number of quality changes

	Metadata map[string]interface{} // Additional session metadata
	mu       sync.RWMutex
}

// NewViewerSession creates a new viewer session
func NewViewerSession(sessionID, streamID, userID string) *ViewerSession {
	return &ViewerSession{
		SessionID:    sessionID,
		StreamID:     streamID,
		UserID:       userID,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		Metadata:     make(map[string]interface{}),
	}
}

// UpdateActivity updates the last activity timestamp
func (vs *ViewerSession) UpdateActivity() {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.LastActivity = time.Now()
}

// End ends the viewer session
func (vs *ViewerSession) End() {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.EndTime = time.Now()
	vs.TotalWatchTime = vs.EndTime.Sub(vs.StartTime)
}

// IsActive returns whether the session is currently active
func (vs *ViewerSession) IsActive(timeout time.Duration) bool {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if !vs.EndTime.IsZero() {
		return false
	}

	return time.Since(vs.LastActivity) < timeout
}

// GetWatchTime returns the current watch time
func (vs *ViewerSession) GetWatchTime() time.Duration {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if !vs.EndTime.IsZero() {
		return vs.TotalWatchTime
	}
	return time.Since(vs.StartTime)
}

// RecordBuffering records a buffering event
func (vs *ViewerSession) RecordBuffering(duration time.Duration) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.BufferingTime += duration
	vs.BufferingCount++
}

// SetGeographic sets geographic information
func (vs *ViewerSession) SetGeographic(country, region, city string, lat, lon float64) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.Country = country
	vs.Region = region
	vs.City = city
	vs.Latitude = lat
	vs.Longitude = lon
}

// SetDevice sets device information
func (vs *ViewerSession) SetDevice(deviceType, os, browser, userAgent string) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.DeviceType = deviceType
	vs.OS = os
	vs.Browser = browser
	vs.UserAgent = userAgent
}

// ViewerSessionSnapshot is a snapshot of a viewer session without locks
type ViewerSessionSnapshot struct {
	SessionID      string
	StreamID       string
	UserID         string
	Username       string
	StartTime      time.Time
	EndTime        time.Time
	LastActivity   time.Time
	Country        string
	Region         string
	City           string
	Latitude       float64
	Longitude      float64
	DeviceType     string
	OS             string
	Browser        string
	UserAgent      string
	IPAddress      string
	ConnectionType string
	TotalWatchTime time.Duration
	BufferingTime  time.Duration
	BufferingCount int
	AverageQuality string
	QualityChanges int
	Metadata       map[string]interface{}
}

// GetSnapshot returns a snapshot of the session
func (vs *ViewerSession) GetSnapshot() ViewerSessionSnapshot {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	// Copy metadata map
	metadata := make(map[string]interface{})
	if vs.Metadata != nil {
		for k, v := range vs.Metadata {
			metadata[k] = v
		}
	}

	return ViewerSessionSnapshot{
		SessionID:      vs.SessionID,
		StreamID:       vs.StreamID,
		UserID:         vs.UserID,
		Username:       vs.Username,
		StartTime:      vs.StartTime,
		EndTime:        vs.EndTime,
		LastActivity:   vs.LastActivity,
		Country:        vs.Country,
		Region:         vs.Region,
		City:           vs.City,
		Latitude:       vs.Latitude,
		Longitude:      vs.Longitude,
		DeviceType:     vs.DeviceType,
		OS:             vs.OS,
		Browser:        vs.Browser,
		UserAgent:      vs.UserAgent,
		IPAddress:      vs.IPAddress,
		ConnectionType: vs.ConnectionType,
		TotalWatchTime: vs.TotalWatchTime,
		BufferingTime:  vs.BufferingTime,
		BufferingCount: vs.BufferingCount,
		AverageQuality: vs.AverageQuality,
		QualityChanges: vs.QualityChanges,
		Metadata:       metadata,
	}
}

// ViewerAnalytics tracks viewer analytics across streams
type ViewerAnalytics struct {
	sessions map[string]*ViewerSession // Map of session ID to session
	mu       sync.RWMutex

	// Configuration
	sessionTimeout time.Duration // How long before a session is considered inactive

	// Metrics collector
	collector MetricsCollector
}

// NewViewerAnalytics creates a new viewer analytics tracker
func NewViewerAnalytics(collector MetricsCollector) *ViewerAnalytics {
	return &ViewerAnalytics{
		sessions:       make(map[string]*ViewerSession),
		sessionTimeout: 5 * time.Minute, // Default 5 minutes timeout
		collector:      collector,
	}
}

// StartSession starts tracking a new viewer session
func (va *ViewerAnalytics) StartSession(sessionID, streamID, userID string) *ViewerSession {
	va.mu.Lock()
	defer va.mu.Unlock()

	session := NewViewerSession(sessionID, streamID, userID)
	va.sessions[sessionID] = session

	// Record session start
	if va.collector != nil {
		va.collector.RecordCounter("viewer_sessions_started_total", 1, map[string]string{
			"stream_id": streamID,
		})
	}

	return session
}

// EndSession ends a viewer session
func (va *ViewerAnalytics) EndSession(sessionID string) {
	va.mu.Lock()
	session, exists := va.sessions[sessionID]
	va.mu.Unlock()

	if !exists {
		return
	}

	session.End()

	// Record session metrics
	if va.collector != nil {
		labels := map[string]string{
			"stream_id": session.StreamID,
		}

		va.collector.RecordCounter("viewer_sessions_ended_total", 1, labels)
		va.collector.RecordHistogram("viewer_watch_time_seconds", session.TotalWatchTime.Seconds(), labels)

		if session.BufferingCount > 0 {
			va.collector.RecordHistogram("viewer_buffering_events", float64(session.BufferingCount), labels)
			va.collector.RecordHistogram("viewer_buffering_time_seconds", session.BufferingTime.Seconds(), labels)
		}
	}
}

// GetSession retrieves a viewer session
func (va *ViewerAnalytics) GetSession(sessionID string) (*ViewerSession, bool) {
	va.mu.RLock()
	defer va.mu.RUnlock()

	session, exists := va.sessions[sessionID]
	return session, exists
}

// GetActiveSessions returns all active sessions
func (va *ViewerAnalytics) GetActiveSessions() []*ViewerSession {
	va.mu.RLock()
	defer va.mu.RUnlock()

	var active []*ViewerSession
	for _, session := range va.sessions {
		if session.IsActive(va.sessionTimeout) {
			active = append(active, session)
		}
	}

	return active
}

// GetStreamSessions returns all sessions for a specific stream
func (va *ViewerAnalytics) GetStreamSessions(streamID string) []*ViewerSession {
	va.mu.RLock()
	defer va.mu.RUnlock()

	var sessions []*ViewerSession
	for _, session := range va.sessions {
		if session.StreamID == streamID {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// GetActiveViewerCount returns the count of active viewers for a stream
func (va *ViewerAnalytics) GetActiveViewerCount(streamID string) int {
	va.mu.RLock()
	defer va.mu.RUnlock()

	count := 0
	for _, session := range va.sessions {
		if session.StreamID == streamID && session.IsActive(va.sessionTimeout) {
			count++
		}
	}

	return count
}

// GetUniqueViewerCount returns the count of unique viewers for a stream
func (va *ViewerAnalytics) GetUniqueViewerCount(streamID string) int {
	va.mu.RLock()
	defer va.mu.RUnlock()

	uniqueUsers := make(map[string]bool)
	for _, session := range va.sessions {
		if session.StreamID == streamID {
			if session.UserID != "" {
				uniqueUsers[session.UserID] = true
			} else {
				// Count anonymous sessions separately
				uniqueUsers[session.SessionID] = true
			}
		}
	}

	return len(uniqueUsers)
}

// GetGeographicDistribution returns the geographic distribution of viewers
func (va *ViewerAnalytics) GetGeographicDistribution(streamID string) map[string]int {
	va.mu.RLock()
	defer va.mu.RUnlock()

	distribution := make(map[string]int)
	for _, session := range va.sessions {
		if session.StreamID == streamID && session.IsActive(va.sessionTimeout) {
			if session.Country != "" {
				distribution[session.Country]++
			}
		}
	}

	return distribution
}

// GetDeviceDistribution returns the device type distribution of viewers
func (va *ViewerAnalytics) GetDeviceDistribution(streamID string) map[string]int {
	va.mu.RLock()
	defer va.mu.RUnlock()

	distribution := make(map[string]int)
	for _, session := range va.sessions {
		if session.StreamID == streamID && session.IsActive(va.sessionTimeout) {
			deviceType := session.DeviceType
			if deviceType == "" {
				deviceType = "unknown"
			}
			distribution[deviceType]++
		}
	}

	return distribution
}

// GetPlatformDistribution returns the OS/platform distribution of viewers
func (va *ViewerAnalytics) GetPlatformDistribution(streamID string) map[string]int {
	va.mu.RLock()
	defer va.mu.RUnlock()

	distribution := make(map[string]int)
	for _, session := range va.sessions {
		if session.StreamID == streamID && session.IsActive(va.sessionTimeout) {
			os := session.OS
			if os == "" {
				os = "unknown"
			}
			distribution[os]++
		}
	}

	return distribution
}

// CalculateTotalWatchTime returns the total watch time for a stream
func (va *ViewerAnalytics) CalculateTotalWatchTime(streamID string) time.Duration {
	va.mu.RLock()
	defer va.mu.RUnlock()

	var total time.Duration
	for _, session := range va.sessions {
		if session.StreamID == streamID {
			total += session.GetWatchTime()
		}
	}

	return total
}

// CalculateAverageWatchTime returns the average watch time per viewer
func (va *ViewerAnalytics) CalculateAverageWatchTime(streamID string) time.Duration {
	va.mu.RLock()
	defer va.mu.RUnlock()

	var total time.Duration
	count := 0

	for _, session := range va.sessions {
		if session.StreamID == streamID {
			total += session.GetWatchTime()
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return total / time.Duration(count)
}

// CleanupInactiveSessions removes inactive sessions older than maxAge
func (va *ViewerAnalytics) CleanupInactiveSessions(maxAge time.Duration) {
	va.mu.Lock()
	defer va.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for sessionID, session := range va.sessions {
		if !session.IsActive(va.sessionTimeout) && session.LastActivity.Before(cutoff) {
			delete(va.sessions, sessionID)
		}
	}
}

// CollectMetrics collects current viewer metrics
func (va *ViewerAnalytics) CollectMetrics() {
	if va.collector == nil {
		return
	}

	// Group sessions by stream
	streamSessions := make(map[string][]*ViewerSession)
	va.mu.RLock()
	for _, session := range va.sessions {
		if session.IsActive(va.sessionTimeout) {
			streamSessions[session.StreamID] = append(streamSessions[session.StreamID], session)
		}
	}
	va.mu.RUnlock()

	// Collect metrics for each stream
	for streamID, sessions := range streamSessions {
		labels := map[string]string{"stream_id": streamID}

		// Active viewer count
		va.collector.RecordGauge("stream_active_viewers", float64(len(sessions)), labels)

		// Device distribution
		deviceDist := make(map[string]int)
		for _, session := range sessions {
			deviceType := session.DeviceType
			if deviceType == "" {
				deviceType = "unknown"
			}
			deviceDist[deviceType]++
		}

		for deviceType, count := range deviceDist {
			deviceLabels := map[string]string{
				"stream_id":   streamID,
				"device_type": deviceType,
			}
			va.collector.RecordGauge("stream_viewers_by_device", float64(count), deviceLabels)
		}

		// Geographic distribution
		geoDist := make(map[string]int)
		for _, session := range sessions {
			if session.Country != "" {
				geoDist[session.Country]++
			}
		}

		for country, count := range geoDist {
			geoLabels := map[string]string{
				"stream_id": streamID,
				"country":   country,
			}
			va.collector.RecordGauge("stream_viewers_by_country", float64(count), geoLabels)
		}
	}
}

// ViewerStats represents aggregated viewer statistics
type ViewerStats struct {
	StreamID             string         // Stream identifier
	ActiveViewers        int            // Current active viewers
	UniqueViewers        int            // Total unique viewers
	PeakViewers          int            // Peak concurrent viewers
	TotalWatchTime       time.Duration  // Total watch time across all viewers
	AverageWatchTime     time.Duration  // Average watch time per viewer
	GeographicDist       map[string]int // Geographic distribution
	DeviceDist           map[string]int // Device distribution
	PlatformDist         map[string]int // Platform/OS distribution
	TotalBufferingEvents int            // Total buffering events
	AverageBufferingTime time.Duration  // Average buffering time
}

// GetViewerStats returns aggregated viewer statistics for a stream
func (va *ViewerAnalytics) GetViewerStats(streamID string) ViewerStats {
	va.mu.RLock()
	defer va.mu.RUnlock()

	stats := ViewerStats{
		StreamID:       streamID,
		GeographicDist: make(map[string]int),
		DeviceDist:     make(map[string]int),
		PlatformDist:   make(map[string]int),
	}

	uniqueUsers := make(map[string]bool)
	peakViewers := 0
	totalBufferingTime := time.Duration(0)
	sessionCount := 0

	for _, session := range va.sessions {
		if session.StreamID != streamID {
			continue
		}

		sessionCount++

		// Unique viewers
		if session.UserID != "" {
			uniqueUsers[session.UserID] = true
		} else {
			uniqueUsers[session.SessionID] = true
		}

		// Active viewers (approximation - just count all sessions)
		if session.IsActive(va.sessionTimeout) {
			stats.ActiveViewers++

			// Geographic distribution
			if session.Country != "" {
				stats.GeographicDist[session.Country]++
			}

			// Device distribution
			deviceType := session.DeviceType
			if deviceType == "" {
				deviceType = "unknown"
			}
			stats.DeviceDist[deviceType]++

			// Platform distribution
			os := session.OS
			if os == "" {
				os = "unknown"
			}
			stats.PlatformDist[os]++
		}

		// Watch time
		stats.TotalWatchTime += session.GetWatchTime()

		// Buffering
		stats.TotalBufferingEvents += session.BufferingCount
		totalBufferingTime += session.BufferingTime
	}

	stats.UniqueViewers = len(uniqueUsers)
	stats.PeakViewers = peakViewers // This would need historical tracking

	if sessionCount > 0 {
		stats.AverageWatchTime = stats.TotalWatchTime / time.Duration(sessionCount)
		stats.AverageBufferingTime = totalBufferingTime / time.Duration(sessionCount)
	}

	return stats
}
