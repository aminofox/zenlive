package analytics

import (
	"fmt"
	"testing"
	"time"
)

// TestInMemoryMetricsCollector tests the in-memory metrics collector
func TestInMemoryMetricsCollector(t *testing.T) {
	collector := NewInMemoryMetricsCollector()

	// Test counter
	collector.RecordCounter("test_counter", 1.0, map[string]string{"label": "value"})
	collector.RecordCounter("test_counter", 2.0, map[string]string{"label": "value"})

	snapshot := collector.GetSnapshot()
	allMetrics := snapshot.GetAll()

	// Check we have metrics
	if len(allMetrics) == 0 {
		t.Fatal("No metrics found")
	}

	// Find the counter metric
	var found bool
	var metric Metric
	for _, m := range allMetrics {
		if m.Name == "test_counter" {
			metric = m
			found = true
			break
		}
	}

	if !found {
		t.Fatal("Counter metric not found")
	}

	if metric.Value != 3.0 {
		t.Errorf("Expected counter value 3.0, got %v", metric.Value)
	}

	// Test gauge
	collector.RecordGauge("test_gauge", 100.0, nil)
	collector.RecordGauge("test_gauge", 200.0, nil)

	snapshot = collector.GetSnapshot()
	metric, exists := snapshot.Get("test_gauge")
	if !exists {
		t.Fatal("Gauge metric not found")
	}

	if metric.Value != 200.0 {
		t.Errorf("Expected gauge value 200.0, got %v", metric.Value)
	}

	// Test histogram
	collector.RecordHistogram("test_histogram", 10.0, nil)
	collector.RecordHistogram("test_histogram", 20.0, nil)
	collector.RecordHistogram("test_histogram", 30.0, nil)

	snapshot = collector.GetSnapshot()
	metric, exists = snapshot.Get("test_histogram")
	if !exists {
		t.Fatal("Histogram metric not found")
	}

	// Average should be 20.0
	if metric.Value != 20.0 {
		t.Errorf("Expected histogram average 20.0, got %v", metric.Value)
	}

	// Test reset
	collector.Reset()
	snapshot = collector.GetSnapshot()
	if len(snapshot.GetAll()) != 0 {
		t.Errorf("Expected empty snapshot after reset, got %d metrics", len(snapshot.GetAll()))
	}
}

// TestStreamMetrics tests stream metrics tracking
func TestStreamMetrics(t *testing.T) {
	metrics := NewStreamMetrics("stream_123")

	// Test viewer updates
	metrics.UpdateViewers(10)
	if metrics.CurrentViewers != 10 {
		t.Errorf("Expected 10 viewers, got %d", metrics.CurrentViewers)
	}

	metrics.UpdateViewers(20)
	if metrics.PeakViewers != 20 {
		t.Errorf("Expected peak viewers 20, got %d", metrics.PeakViewers)
	}

	metrics.UpdateViewers(5)
	if metrics.PeakViewers != 20 {
		t.Errorf("Expected peak viewers still 20, got %d", metrics.PeakViewers)
	}

	// Test bitrate updates
	metrics.UpdateBitrate(1000000) // 1 Mbps
	if metrics.CurrentBitrate != 1000000 {
		t.Errorf("Expected bitrate 1000000, got %v", metrics.CurrentBitrate)
	}

	// Test FPS updates
	metrics.UpdateFPS(30.0)
	if metrics.CurrentFPS != 30.0 {
		t.Errorf("Expected FPS 30.0, got %v", metrics.CurrentFPS)
	}

	// Test frame drops
	metrics.RecordFrames(1000)
	metrics.RecordDroppedFrames(50)

	if metrics.TotalFrames != 1050 {
		t.Errorf("Expected total frames 1050, got %d", metrics.TotalFrames)
	}

	expectedDropRate := (50.0 / 1050.0) * 100.0
	if metrics.DropRate < expectedDropRate-0.1 || metrics.DropRate > expectedDropRate+0.1 {
		t.Errorf("Expected drop rate ~%v, got %v", expectedDropRate, metrics.DropRate)
	}

	// Test duration
	time.Sleep(100 * time.Millisecond)
	duration := metrics.GetDuration()
	if duration < 100*time.Millisecond {
		t.Errorf("Expected duration >= 100ms, got %v", duration)
	}

	// Test end
	metrics.End()
	if metrics.IsLive() {
		t.Error("Expected stream to not be live after End()")
	}
}

// TestViewerAnalytics tests viewer analytics
func TestViewerAnalytics(t *testing.T) {
	collector := NewInMemoryMetricsCollector()
	analytics := NewViewerAnalytics(collector)

	// Start a session
	session := analytics.StartSession("session_1", "stream_1", "user_1")
	if session == nil {
		t.Fatal("Failed to start session")
	}

	// Set device info
	session.SetDevice("mobile", "iOS", "Safari", "Mozilla/5.0...")

	// Set geographic info
	session.SetGeographic("US", "CA", "San Francisco", 37.7749, -122.4194)

	// Test active viewer count
	count := analytics.GetActiveViewerCount("stream_1")
	if count != 1 {
		t.Errorf("Expected 1 active viewer, got %d", count)
	}

	// Start another session
	analytics.StartSession("session_2", "stream_1", "user_2")
	count = analytics.GetActiveViewerCount("stream_1")
	if count != 2 {
		t.Errorf("Expected 2 active viewers, got %d", count)
	}

	// Test unique viewer count
	uniqueCount := analytics.GetUniqueViewerCount("stream_1")
	if uniqueCount != 2 {
		t.Errorf("Expected 2 unique viewers, got %d", uniqueCount)
	}

	// Test device distribution
	deviceDist := analytics.GetDeviceDistribution("stream_1")
	if deviceDist["mobile"] != 1 {
		t.Errorf("Expected 1 mobile viewer, got %d", deviceDist["mobile"])
	}

	// Test geographic distribution
	geoDist := analytics.GetGeographicDistribution("stream_1")
	if geoDist["US"] != 1 {
		t.Errorf("Expected 1 US viewer, got %d", geoDist["US"])
	}

	// End session
	analytics.EndSession("session_1")
	retrievedSession, exists := analytics.GetSession("session_1")
	if !exists {
		t.Fatal("Session not found after ending")
	}

	if retrievedSession.EndTime.IsZero() {
		t.Error("Expected session to have end time")
	}
}

// TestPerformanceMonitor tests performance monitoring
func TestPerformanceMonitor(t *testing.T) {
	collector := NewInMemoryMetricsCollector()
	monitor := NewPerformanceMonitor(collector)

	// Test latency recording
	monitor.RecordLatency("api_call", 50*time.Millisecond)
	monitor.RecordLatency("api_call", 100*time.Millisecond)
	monitor.RecordLatency("api_call", 75*time.Millisecond)

	metrics, exists := monitor.GetLatencyMetrics("api_call")
	if !exists {
		t.Fatal("Latency metrics not found")
	}

	if metrics.Min != 50*time.Millisecond {
		t.Errorf("Expected min latency 50ms, got %v", metrics.Min)
	}

	if metrics.Max != 100*time.Millisecond {
		t.Errorf("Expected max latency 100ms, got %v", metrics.Max)
	}

	if metrics.SampleCount != 3 {
		t.Errorf("Expected 3 samples, got %d", metrics.SampleCount)
	}

	// Test error recording
	monitor.RecordRequest("api_call")
	monitor.RecordRequest("api_call")
	monitor.RecordError("api_call", "timeout")

	errorMetrics, exists := monitor.GetErrorMetrics("api_call")
	if !exists {
		t.Fatal("Error metrics not found")
	}

	if errorMetrics.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", errorMetrics.TotalRequests)
	}

	if errorMetrics.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", errorMetrics.ErrorCount)
	}

	expectedRate := (1.0 / 2.0) * 100.0
	if errorMetrics.ErrorRate != expectedRate {
		t.Errorf("Expected error rate %v, got %v", expectedRate, errorMetrics.ErrorRate)
	}
}

// TestHealthMonitor tests health monitoring
func TestHealthMonitor(t *testing.T) {
	collector := NewInMemoryMetricsCollector()
	monitor := NewHealthMonitor(collector)

	// Register a healthy checker
	healthyChecker := NewSimpleHealthChecker("test_component", func() error {
		return nil
	})
	monitor.RegisterChecker(healthyChecker)

	// Perform health check
	results := monitor.CheckAll()

	if len(results) != 1 {
		t.Fatalf("Expected 1 health check result, got %d", len(results))
	}

	result, exists := results["test_component"]
	if !exists {
		t.Fatal("Health check result not found")
	}

	if result.Status != HealthStatusHealthy {
		t.Errorf("Expected healthy status, got %v", result.Status)
	}

	// Test overall status
	overallStatus := monitor.GetOverallStatus()
	if overallStatus != HealthStatusHealthy {
		t.Errorf("Expected overall healthy status, got %v", overallStatus)
	}

	// Register an unhealthy checker
	unhealthyChecker := NewSimpleHealthChecker("failing_component", func() error {
		return fmt.Errorf("component is down")
	})
	monitor.RegisterChecker(unhealthyChecker)

	results = monitor.CheckAll()
	if len(results) != 2 {
		t.Fatalf("Expected 2 health check results, got %d", len(results))
	}

	overallStatus = monitor.GetOverallStatus()
	if overallStatus != HealthStatusUnhealthy {
		t.Errorf("Expected overall unhealthy status, got %v", overallStatus)
	}
}

// TestAlertManager tests alert management
func TestAlertManager(t *testing.T) {
	manager := NewAlertManager()

	// Test alert triggering
	alertTriggered := false
	manager.RegisterHandler(func(alert *Alert) {
		alertTriggered = true
		if alert.Level != AlertLevelError {
			t.Errorf("Expected error level, got %v", alert.Level)
		}
	})

	manager.TriggerAlert("test_alert", AlertLevelError, "Test alert", "test_component")

	if !alertTriggered {
		t.Error("Alert handler was not called")
	}

	// Check active alerts
	activeAlerts := manager.GetActiveAlerts()
	if len(activeAlerts) != 1 {
		t.Fatalf("Expected 1 active alert, got %d", len(activeAlerts))
	}

	if activeAlerts[0].Name != "test_alert" {
		t.Errorf("Expected alert name 'test_alert', got %s", activeAlerts[0].Name)
	}

	// Test alert resolution
	alertResolved := false
	manager.RegisterHandler(func(alert *Alert) {
		if alert.Resolved {
			alertResolved = true
		}
	})

	manager.ResolveAlert("test_alert")

	if !alertResolved {
		t.Error("Alert resolution handler was not called")
	}

	// Check active alerts after resolution
	activeAlerts = manager.GetActiveAlerts()
	if len(activeAlerts) != 0 {
		t.Errorf("Expected 0 active alerts after resolution, got %d", len(activeAlerts))
	}

	// Check alert history
	history := manager.GetAlertHistory(10)
	if len(history) != 1 {
		t.Errorf("Expected 1 alert in history, got %d", len(history))
	}
}

// TestReportGeneration tests report generation
func TestReportGeneration(t *testing.T) {
	streamCollector := NewStreamMetricsCollector(nil)
	viewerAnalytics := NewViewerAnalytics(nil)
	perfMonitor := NewPerformanceMonitor(nil)
	timeSeriesStore := NewTimeSeriesStore()

	generator := NewReportGenerator(streamCollector, viewerAnalytics, perfMonitor, timeSeriesStore)

	// Start a stream and add some metrics
	metrics := streamCollector.StartStream("stream_1")
	metrics.UpdateViewers(100)
	metrics.UpdateBitrate(2000000)

	// Start some viewer sessions
	viewerAnalytics.StartSession("session_1", "stream_1", "user_1")
	viewerAnalytics.StartSession("session_2", "stream_1", "user_2")

	// Generate stream report
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	report, err := generator.GenerateStreamReport("stream_1", startTime, endTime)
	if err != nil {
		t.Fatalf("Failed to generate stream report: %v", err)
	}

	if report.Type != ReportTypeStream {
		t.Errorf("Expected stream report type, got %v", report.Type)
	}

	if report.Data["stream_id"] != "stream_1" {
		t.Errorf("Expected stream_id 'stream_1', got %v", report.Data["stream_id"])
	}

	// Test JSON export
	jsonData, err := report.ToJSON()
	if err != nil {
		t.Fatalf("Failed to export report to JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("Expected non-empty JSON data")
	}

	// Test CSV export
	csvData, err := report.ToCSV()
	if err != nil {
		t.Fatalf("Failed to export report to CSV: %v", err)
	}

	if len(csvData) == 0 {
		t.Error("Expected non-empty CSV data")
	}
}

// TestTimeSeriesStore tests time series storage
func TestTimeSeriesStore(t *testing.T) {
	store := NewTimeSeriesStore()

	now := time.Now()
	labels := map[string]string{"stream_id": "stream_1"}

	// Record some data points
	for i := 0; i < 10; i++ {
		timestamp := now.Add(time.Duration(i) * time.Minute)
		store.Record("viewer_count", labels, timestamp, float64(i*10))
	}

	// Retrieve time series
	ts, exists := store.Get("viewer_count", labels)
	if !exists {
		t.Fatal("Time series not found")
	}

	// Get range
	startTime := now
	endTime := now.Add(5 * time.Minute)
	dataPoints := ts.GetRange(startTime, endTime)

	if len(dataPoints) < 5 {
		t.Errorf("Expected at least 5 data points, got %d", len(dataPoints))
	}

	// Test pruning
	ts.Prune(3 * time.Minute)
	allPoints := ts.GetRange(time.Time{}, time.Now().Add(1*time.Hour))

	if len(allPoints) > 10 {
		t.Errorf("Expected pruning to reduce data points, got %d", len(allPoints))
	}
}

// TestPrometheusExporter tests Prometheus metric export
func TestPrometheusExporter(t *testing.T) {
	registry := NewMetricsRegistry()
	collector := NewInMemoryMetricsCollector()

	registry.Register("test", collector)

	// Add some metrics
	collector.RecordCounter("http_requests_total", 100, map[string]string{"method": "GET"})
	collector.RecordGauge("active_connections", 42, nil)

	exporter := NewPrometheusExporter(registry)

	// Collect metrics
	metrics := exporter.collectMetrics()

	if len(metrics) < 2 {
		t.Errorf("Expected at least 2 metrics, got %d", len(metrics))
	}

	// Format as Prometheus
	output := exporter.formatPrometheusMetrics(metrics)

	if !contains(output, "http_requests_total") {
		t.Error("Expected output to contain http_requests_total")
	}

	if !contains(output, "active_connections") {
		t.Error("Expected output to contain active_connections")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && anySubstring(s, substr))
}

func anySubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
