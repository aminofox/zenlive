package analytics

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ReportType represents the type of report
type ReportType string

const (
	// ReportTypeStream is for stream-specific reports
	ReportTypeStream ReportType = "stream"
	// ReportTypeViewer is for viewer analytics reports
	ReportTypeViewer ReportType = "viewer"
	// ReportTypePerformance is for performance reports
	ReportTypePerformance ReportType = "performance"
	// ReportTypeSystem is for system metrics reports
	ReportTypeSystem ReportType = "system"
)

// ReportFormat represents the output format for reports
type ReportFormat string

const (
	// ReportFormatJSON is for JSON output
	ReportFormatJSON ReportFormat = "json"
	// ReportFormatCSV is for CSV output
	ReportFormatCSV ReportFormat = "csv"
)

// Report represents a generated report
type Report struct {
	ID          string                 // Unique report identifier
	Type        ReportType             // Type of report
	Title       string                 // Report title
	Description string                 // Report description
	StartTime   time.Time              // Report period start
	EndTime     time.Time              // Report period end
	GeneratedAt time.Time              // When the report was generated
	Data        map[string]interface{} // Report data
	Metadata    map[string]interface{} // Additional metadata
}

// NewReport creates a new report
func NewReport(reportType ReportType, title, description string, startTime, endTime time.Time) *Report {
	return &Report{
		ID:          fmt.Sprintf("%s_%d", reportType, time.Now().Unix()),
		Type:        reportType,
		Title:       title,
		Description: description,
		StartTime:   startTime,
		EndTime:     endTime,
		GeneratedAt: time.Now(),
		Data:        make(map[string]interface{}),
		Metadata:    make(map[string]interface{}),
	}
}

// ToJSON exports the report as JSON
func (r *Report) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToCSV exports the report as CSV
func (r *Report) ToCSV() (string, error) {
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	// Write header
	header := []string{"Metric", "Value"}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	// Write metadata
	metadataRow := []string{"Report ID", r.ID}
	if err := writer.Write(metadataRow); err != nil {
		return "", err
	}

	metadataRow = []string{"Type", string(r.Type)}
	if err := writer.Write(metadataRow); err != nil {
		return "", err
	}

	metadataRow = []string{"Title", r.Title}
	if err := writer.Write(metadataRow); err != nil {
		return "", err
	}

	metadataRow = []string{"Generated At", r.GeneratedAt.Format(time.RFC3339)}
	if err := writer.Write(metadataRow); err != nil {
		return "", err
	}

	// Empty row
	if err := writer.Write([]string{"", ""}); err != nil {
		return "", err
	}

	// Write data
	for key, value := range r.Data {
		row := []string{key, fmt.Sprintf("%v", value)}
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return sb.String(), nil
}

// ReportGenerator generates reports from analytics data
type ReportGenerator struct {
	streamCollector *StreamMetricsCollector
	viewerAnalytics *ViewerAnalytics
	perfMonitor     *PerformanceMonitor
	timeSeriesStore *TimeSeriesStore
	mu              sync.RWMutex
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(
	streamCollector *StreamMetricsCollector,
	viewerAnalytics *ViewerAnalytics,
	perfMonitor *PerformanceMonitor,
	timeSeriesStore *TimeSeriesStore,
) *ReportGenerator {
	return &ReportGenerator{
		streamCollector: streamCollector,
		viewerAnalytics: viewerAnalytics,
		perfMonitor:     perfMonitor,
		timeSeriesStore: timeSeriesStore,
	}
}

// GenerateStreamReport generates a report for a specific stream
func (rg *ReportGenerator) GenerateStreamReport(streamID string, startTime, endTime time.Time) (*Report, error) {
	report := NewReport(
		ReportTypeStream,
		fmt.Sprintf("Stream Report: %s", streamID),
		fmt.Sprintf("Analytics report for stream %s", streamID),
		startTime,
		endTime,
	)

	// Get stream metrics
	if rg.streamCollector != nil {
		metrics, exists := rg.streamCollector.GetStream(streamID)
		if exists {
			snapshot := metrics.GetSnapshot()
			report.Data["stream_id"] = snapshot.StreamID
			duration := time.Duration(0)
			if snapshot.EndTime.IsZero() {
				duration = time.Since(snapshot.StartTime)
			} else {
				duration = snapshot.EndTime.Sub(snapshot.StartTime)
			}
			report.Data["duration_seconds"] = duration.Seconds()
			report.Data["current_viewers"] = snapshot.CurrentViewers
			report.Data["peak_viewers"] = snapshot.PeakViewers
			report.Data["total_viewers"] = snapshot.TotalViewers
			report.Data["average_bitrate_bps"] = snapshot.AverageBitrate
			report.Data["peak_bitrate_bps"] = snapshot.PeakBitrate
			report.Data["average_fps"] = snapshot.AverageFPS
			report.Data["drop_rate_percent"] = snapshot.DropRate
			report.Data["dropped_frames"] = snapshot.DroppedFrames
			report.Data["total_frames"] = snapshot.TotalFrames
			report.Data["is_live"] = snapshot.EndTime.IsZero()
		}
	}

	// Get viewer analytics
	if rg.viewerAnalytics != nil {
		stats := rg.viewerAnalytics.GetViewerStats(streamID)
		report.Data["unique_viewers"] = stats.UniqueViewers
		report.Data["total_watch_time_seconds"] = stats.TotalWatchTime.Seconds()
		report.Data["average_watch_time_seconds"] = stats.AverageWatchTime.Seconds()
		report.Data["geographic_distribution"] = stats.GeographicDist
		report.Data["device_distribution"] = stats.DeviceDist
		report.Data["platform_distribution"] = stats.PlatformDist
		report.Data["total_buffering_events"] = stats.TotalBufferingEvents
	}

	return report, nil
}

// GenerateViewerReport generates a viewer analytics report
func (rg *ReportGenerator) GenerateViewerReport(startTime, endTime time.Time) (*Report, error) {
	report := NewReport(
		ReportTypeViewer,
		"Viewer Analytics Report",
		"Comprehensive viewer analytics across all streams",
		startTime,
		endTime,
	)

	if rg.viewerAnalytics == nil {
		return report, nil
	}

	// Get all active sessions
	activeSessions := rg.viewerAnalytics.GetActiveSessions()
	report.Data["active_sessions"] = len(activeSessions)

	// Aggregate by stream
	streamStats := make(map[string]int)
	for _, session := range activeSessions {
		streamStats[session.StreamID]++
	}
	report.Data["viewers_by_stream"] = streamStats

	// Device distribution across all streams
	deviceDist := make(map[string]int)
	geoDist := make(map[string]int)
	platformDist := make(map[string]int)

	for _, session := range activeSessions {
		if session.DeviceType != "" {
			deviceDist[session.DeviceType]++
		}
		if session.Country != "" {
			geoDist[session.Country]++
		}
		if session.OS != "" {
			platformDist[session.OS]++
		}
	}

	report.Data["device_distribution"] = deviceDist
	report.Data["geographic_distribution"] = geoDist
	report.Data["platform_distribution"] = platformDist

	return report, nil
}

// GeneratePerformanceReport generates a performance report
func (rg *ReportGenerator) GeneratePerformanceReport(startTime, endTime time.Time) (*Report, error) {
	report := NewReport(
		ReportTypePerformance,
		"Performance Report",
		"System performance metrics and latency statistics",
		startTime,
		endTime,
	)

	if rg.perfMonitor == nil {
		return report, nil
	}

	// Get latency metrics
	latencyMetrics := rg.perfMonitor.GetAllLatencyMetrics()
	latencyData := make(map[string]interface{})

	for operation, m := range latencyMetrics {
		latencyData[operation] = map[string]interface{}{
			"min_ms":       m.Min.Milliseconds(),
			"max_ms":       m.Max.Milliseconds(),
			"avg_ms":       m.Average.Milliseconds(),
			"p50_ms":       m.P50.Milliseconds(),
			"p95_ms":       m.P95.Milliseconds(),
			"p99_ms":       m.P99.Milliseconds(),
			"sample_count": m.SampleCount,
		}
	}
	report.Data["latency_metrics"] = latencyData

	// Get error metrics
	errorMetrics := rg.perfMonitor.GetAllErrorMetrics()
	errorData := make(map[string]interface{})

	for operation, m := range errorMetrics {
		errorData[operation] = map[string]interface{}{
			"total_requests": m.TotalRequests,
			"error_count":    m.ErrorCount,
			"error_rate":     m.ErrorRate,
			"errors_by_type": m.ErrorsByType,
		}
	}
	report.Data["error_metrics"] = errorData

	return report, nil
}

// AggregateTimeSeries aggregates time series data
func (rg *ReportGenerator) AggregateTimeSeries(
	metricName string,
	labels map[string]string,
	startTime, endTime time.Time,
	bucketSize time.Duration,
) ([]TimeSeriesDataPoint, error) {
	if rg.timeSeriesStore == nil {
		return nil, fmt.Errorf("time series store not available")
	}

	// Get time series
	ts, exists := rg.timeSeriesStore.Get(metricName, labels)
	if !exists {
		return nil, fmt.Errorf("time series not found: %s", metricName)
	}

	// Get data points in range
	dataPoints := ts.GetRange(startTime, endTime)
	if len(dataPoints) == 0 {
		return nil, nil
	}

	// Aggregate by bucket
	buckets := make(map[int64][]float64)

	for _, dp := range dataPoints {
		bucketTimestamp := dp.Timestamp.Unix() / int64(bucketSize.Seconds())
		buckets[bucketTimestamp] = append(buckets[bucketTimestamp], dp.Value)
	}

	// Calculate aggregated values
	aggregated := make([]TimeSeriesDataPoint, 0, len(buckets))
	for bucketTS, values := range buckets {
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		avg := sum / float64(len(values))

		aggregated = append(aggregated, TimeSeriesDataPoint{
			Timestamp: time.Unix(bucketTS*int64(bucketSize.Seconds()), 0),
			Value:     avg,
		})
	}

	return aggregated, nil
}

// ExportReport exports a report in the specified format
func ExportReport(report *Report, format ReportFormat) ([]byte, error) {
	switch format {
	case ReportFormatJSON:
		return report.ToJSON()
	case ReportFormatCSV:
		csvStr, err := report.ToCSV()
		if err != nil {
			return nil, err
		}
		return []byte(csvStr), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// ScheduledReport represents a scheduled report configuration
type ScheduledReport struct {
	ID         string       // Report identifier
	Type       ReportType   // Type of report
	Schedule   string       // Cron-like schedule (e.g., "daily", "weekly")
	Format     ReportFormat // Output format
	Recipients []string     // Email recipients or webhook URLs
	Enabled    bool         // Whether the schedule is enabled
	LastRun    time.Time    // Last time the report was generated
	NextRun    time.Time    // Next scheduled run
}

// ReportScheduler schedules and generates reports
type ReportScheduler struct {
	reports   map[string]*ScheduledReport
	generator *ReportGenerator
	mu        sync.RWMutex
	stopChan  chan struct{}
}

// NewReportScheduler creates a new report scheduler
func NewReportScheduler(generator *ReportGenerator) *ReportScheduler {
	return &ReportScheduler{
		reports:   make(map[string]*ScheduledReport),
		generator: generator,
	}
}

// ScheduleReport schedules a report
func (rs *ReportScheduler) ScheduleReport(report *ScheduledReport) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.reports[report.ID] = report
}

// UnscheduleReport removes a scheduled report
func (rs *ReportScheduler) UnscheduleReport(reportID string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	delete(rs.reports, reportID)
}

// GetScheduledReports returns all scheduled reports
func (rs *ReportScheduler) GetScheduledReports() []*ScheduledReport {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	reports := make([]*ScheduledReport, 0, len(rs.reports))
	for _, report := range rs.reports {
		reports = append(reports, report)
	}
	return reports
}

// Start starts the report scheduler
func (rs *ReportScheduler) Start() {
	rs.mu.Lock()
	if rs.stopChan != nil {
		rs.mu.Unlock()
		return // Already started
	}
	rs.stopChan = make(chan struct{})
	rs.mu.Unlock()

	go rs.runScheduler()
}

// Stop stops the report scheduler
func (rs *ReportScheduler) Stop() {
	rs.mu.Lock()
	if rs.stopChan != nil {
		close(rs.stopChan)
		rs.stopChan = nil
	}
	rs.mu.Unlock()
}

// runScheduler runs the report scheduler loop
func (rs *ReportScheduler) runScheduler() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rs.checkAndRunReports()
		case <-rs.stopChan:
			return
		}
	}
}

// checkAndRunReports checks if any reports need to be run
func (rs *ReportScheduler) checkAndRunReports() {
	rs.mu.RLock()
	reports := make([]*ScheduledReport, 0)
	now := time.Now()

	for _, report := range rs.reports {
		if report.Enabled && !report.NextRun.After(now) {
			reports = append(reports, report)
		}
	}
	rs.mu.RUnlock()

	// Generate reports (outside of lock)
	for _, report := range reports {
		rs.generateScheduledReport(report)
	}
}

// generateScheduledReport generates a scheduled report
func (rs *ReportScheduler) generateScheduledReport(scheduled *ScheduledReport) {
	// This is a simplified implementation
	// In a real system, you would:
	// 1. Generate the report based on type
	// 2. Format it according to the specified format
	// 3. Send it to recipients (email, webhook, etc.)
	// 4. Update LastRun and NextRun times

	rs.mu.Lock()
	scheduled.LastRun = time.Now()
	// Calculate next run based on schedule
	// This is simplified - in reality you'd parse the schedule string
	if scheduled.Schedule == "daily" {
		scheduled.NextRun = scheduled.LastRun.Add(24 * time.Hour)
	} else if scheduled.Schedule == "weekly" {
		scheduled.NextRun = scheduled.LastRun.Add(7 * 24 * time.Hour)
	}
	rs.mu.Unlock()
}
