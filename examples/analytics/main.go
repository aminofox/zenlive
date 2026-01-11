package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminofox/zenlive/pkg/analytics"
)

func main() {
	fmt.Println("=== ZenLive Phase 9: Analytics & Monitoring Example ===\n")

	// Run examples
	runMetricsExample()
	runStreamMetricsExample()
	runViewerAnalyticsExample()
	runPerformanceMonitoringExample()
	runHealthCheckExample()
	runReportingExample()

	// Start HTTP server for Prometheus metrics
	runPrometheusServer()
}

// runMetricsExample demonstrates basic metrics collection
func runMetricsExample() {
	fmt.Println("1. Metrics Collection Example")
	fmt.Println("   -------------------------------")

	// Create metrics collector
	collector := analytics.NewInMemoryMetricsCollector()

	// Record various metrics
	collector.RecordCounter("http_requests_total", 100, map[string]string{
		"method": "GET",
		"path":   "/api/streams",
	})

	collector.RecordGauge("active_connections", 42, nil)
	collector.RecordHistogram("request_duration_ms", 150, map[string]string{
		"endpoint": "/api/streams",
	})

	// Get snapshot
	snapshot := collector.GetSnapshot()
	fmt.Printf("   Collected %d metrics\n", len(snapshot.GetAll()))

	for _, metric := range snapshot.GetAll() {
		fmt.Printf("   - %s (%s): %v\n", metric.Name, metric.Type, metric.Value)
	}

	fmt.Println()
}

// runStreamMetricsExample demonstrates stream metrics tracking
func runStreamMetricsExample() {
	fmt.Println("2. Stream Metrics Example")
	fmt.Println("   -------------------------------")

	// Create stream metrics collector
	collector := analytics.NewInMemoryMetricsCollector()
	streamCollector := analytics.NewStreamMetricsCollector(collector)

	// Start tracking a stream
	metrics := streamCollector.StartStream("stream_001")

	// Simulate stream activity
	metrics.UpdateViewers(50)
	metrics.UpdateBitrate(2500000) // 2.5 Mbps
	metrics.UpdateFPS(30.0)
	metrics.UpdateResolution(1920, 1080)
	metrics.RecordFrames(900)      // 30 seconds at 30fps
	metrics.RecordDroppedFrames(5) // 5 dropped frames

	// Add more viewers
	metrics.UpdateViewers(120) // Peak viewers
	metrics.IncrementTotalViewers()

	// Get snapshot
	snapshot := metrics.GetSnapshot()
	fmt.Printf("   Stream ID: %s\n", snapshot.StreamID)
	fmt.Printf("   Current Viewers: %d\n", snapshot.CurrentViewers)
	fmt.Printf("   Peak Viewers: %d\n", snapshot.PeakViewers)
	fmt.Printf("   Current Bitrate: %.2f Mbps\n", snapshot.CurrentBitrate/1000000)
	fmt.Printf("   Current FPS: %.1f\n", snapshot.CurrentFPS)
	fmt.Printf("   Resolution: %dx%d\n", snapshot.Width, snapshot.Height)
	fmt.Printf("   Drop Rate: %.2f%%\n", snapshot.DropRate)

	// Calculate duration
	duration := time.Duration(0)
	if snapshot.EndTime.IsZero() {
		duration = time.Since(snapshot.StartTime)
	} else {
		duration = snapshot.EndTime.Sub(snapshot.StartTime)
	}
	fmt.Printf("   Duration: %v\n", duration)
	fmt.Printf("   Is Live: %v\n", snapshot.EndTime.IsZero())

	// Collect metrics
	streamCollector.CollectMetrics()

	fmt.Println()
}

// runViewerAnalyticsExample demonstrates viewer analytics
func runViewerAnalyticsExample() {
	fmt.Println("3. Viewer Analytics Example")
	fmt.Println("   -------------------------------")

	// Create viewer analytics
	collector := analytics.NewInMemoryMetricsCollector()
	viewerAnalytics := analytics.NewViewerAnalytics(collector)

	// Simulate viewer sessions
	session1 := viewerAnalytics.StartSession("session_001", "stream_001", "user_001")
	session1.SetDevice("desktop", "Windows", "Chrome", "Mozilla/5.0...")
	session1.SetGeographic("US", "CA", "San Francisco", 37.7749, -122.4194)

	session2 := viewerAnalytics.StartSession("session_002", "stream_001", "user_002")
	session2.SetDevice("mobile", "iOS", "Safari", "Mozilla/5.0...")
	session2.SetGeographic("JP", "Tokyo", "Tokyo", 35.6762, 139.6503)

	session3 := viewerAnalytics.StartSession("session_003", "stream_001", "user_003")
	session3.SetDevice("tablet", "Android", "Chrome", "Mozilla/5.0...")
	session3.SetGeographic("US", "NY", "New York", 40.7128, -74.0060)

	// Get analytics
	activeCount := viewerAnalytics.GetActiveViewerCount("stream_001")
	uniqueCount := viewerAnalytics.GetUniqueViewerCount("stream_001")

	fmt.Printf("   Active Viewers: %d\n", activeCount)
	fmt.Printf("   Unique Viewers: %d\n", uniqueCount)

	// Get distributions
	deviceDist := viewerAnalytics.GetDeviceDistribution("stream_001")
	fmt.Printf("   Device Distribution:\n")
	for device, count := range deviceDist {
		fmt.Printf("     - %s: %d\n", device, count)
	}

	geoDist := viewerAnalytics.GetGeographicDistribution("stream_001")
	fmt.Printf("   Geographic Distribution:\n")
	for country, count := range geoDist {
		fmt.Printf("     - %s: %d\n", country, count)
	}

	// Get viewer stats
	stats := viewerAnalytics.GetViewerStats("stream_001")
	fmt.Printf("   Viewer Stats:\n")
	fmt.Printf("     - Active: %d\n", stats.ActiveViewers)
	fmt.Printf("     - Unique: %d\n", stats.UniqueViewers)
	fmt.Printf("     - Total Watch Time: %v\n", stats.TotalWatchTime)

	fmt.Println()
}

// runPerformanceMonitoringExample demonstrates performance monitoring
func runPerformanceMonitoringExample() {
	fmt.Println("4. Performance Monitoring Example")
	fmt.Println("   -------------------------------")

	// Create performance monitor
	collector := analytics.NewInMemoryMetricsCollector()
	perfMonitor := analytics.NewPerformanceMonitor(collector)

	// Simulate API calls with latency
	perfMonitor.RecordRequest("get_stream")
	perfMonitor.RecordLatency("get_stream", 45*time.Millisecond)

	perfMonitor.RecordRequest("get_stream")
	perfMonitor.RecordLatency("get_stream", 52*time.Millisecond)

	perfMonitor.RecordRequest("get_stream")
	perfMonitor.RecordLatency("get_stream", 38*time.Millisecond)

	// Simulate errors
	perfMonitor.RecordRequest("create_stream")
	perfMonitor.RecordLatency("create_stream", 150*time.Millisecond)

	perfMonitor.RecordRequest("create_stream")
	perfMonitor.RecordError("create_stream", "validation_error")

	// Get latency metrics
	latencyMetrics, _ := perfMonitor.GetLatencyMetrics("get_stream")
	fmt.Printf("   Operation: get_stream\n")
	fmt.Printf("   Latency:\n")
	fmt.Printf("     - Min: %v\n", latencyMetrics.Min)
	fmt.Printf("     - Max: %v\n", latencyMetrics.Max)
	fmt.Printf("     - Avg: %v\n", latencyMetrics.Average)
	fmt.Printf("     - P50: %v\n", latencyMetrics.P50)
	fmt.Printf("     - P95: %v\n", latencyMetrics.P95)
	fmt.Printf("     - Samples: %d\n", latencyMetrics.SampleCount)

	// Get error metrics
	errorMetrics, _ := perfMonitor.GetErrorMetrics("create_stream")
	fmt.Printf("   Operation: create_stream\n")
	fmt.Printf("   Errors:\n")
	fmt.Printf("     - Total Requests: %d\n", errorMetrics.TotalRequests)
	fmt.Printf("     - Error Count: %d\n", errorMetrics.ErrorCount)
	fmt.Printf("     - Error Rate: %.2f%%\n", errorMetrics.ErrorRate)

	fmt.Println()
}

// runHealthCheckExample demonstrates health checking
func runHealthCheckExample() {
	fmt.Println("5. Health Check Example")
	fmt.Println("   -------------------------------")

	// Create health monitor
	collector := analytics.NewInMemoryMetricsCollector()
	healthMonitor := analytics.NewHealthMonitor(collector)

	// Register health checkers
	databaseChecker := analytics.NewSimpleHealthChecker("database", func() error {
		// Simulate database check
		return nil // Healthy
	})
	healthMonitor.RegisterChecker(databaseChecker)

	redisChecker := analytics.NewSimpleHealthChecker("redis", func() error {
		// Simulate redis check
		return nil // Healthy
	})
	healthMonitor.RegisterChecker(redisChecker)

	storageChecker := analytics.NewSimpleHealthChecker("storage", func() error {
		// Simulate storage check
		return nil // Healthy
	})
	healthMonitor.RegisterChecker(storageChecker)

	// Perform health checks
	results := healthMonitor.CheckAll()

	fmt.Printf("   Health Check Results:\n")
	for name, result := range results {
		fmt.Printf("   - %s: %s (%v)\n", name, result.Status, result.Message)
		fmt.Printf("     Duration: %v\n", result.Duration)
	}

	overallStatus := healthMonitor.GetOverallStatus()
	fmt.Printf("   Overall Status: %s\n", overallStatus)

	// Create alert manager
	alertManager := analytics.NewAlertManager()

	// Register alert handler
	alertManager.RegisterHandler(func(alert *analytics.Alert) {
		if alert.Resolved {
			fmt.Printf("   [ALERT RESOLVED] %s: %s\n", alert.Name, alert.Message)
		} else {
			fmt.Printf("   [ALERT TRIGGERED] %s (%s): %s\n", alert.Name, alert.Level, alert.Message)
		}
	})

	// Trigger a test alert
	alertManager.TriggerAlert(
		"high_latency",
		analytics.AlertLevelWarning,
		"API latency exceeded threshold",
		"api_server",
	)

	// Get active alerts
	activeAlerts := alertManager.GetActiveAlerts()
	fmt.Printf("   Active Alerts: %d\n", len(activeAlerts))

	// Resolve alert
	alertManager.ResolveAlert("high_latency")

	fmt.Println()
}

// runReportingExample demonstrates report generation
func runReportingExample() {
	fmt.Println("6. Reporting Example")
	fmt.Println("   -------------------------------")

	// Create components
	streamCollector := analytics.NewStreamMetricsCollector(nil)
	viewerAnalytics := analytics.NewViewerAnalytics(nil)
	perfMonitor := analytics.NewPerformanceMonitor(nil)
	timeSeriesStore := analytics.NewTimeSeriesStore()

	// Create report generator
	generator := analytics.NewReportGenerator(
		streamCollector,
		viewerAnalytics,
		perfMonitor,
		timeSeriesStore,
	)

	// Start a stream and add metrics
	metrics := streamCollector.StartStream("stream_report_001")
	metrics.UpdateViewers(200)
	metrics.UpdateBitrate(3000000)
	metrics.UpdateFPS(60.0)

	// Add viewer sessions
	viewerAnalytics.StartSession("session_r1", "stream_report_001", "user_r1")
	viewerAnalytics.StartSession("session_r2", "stream_report_001", "user_r2")
	viewerAnalytics.StartSession("session_r3", "stream_report_001", "user_r3")

	// Generate stream report
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	report, err := generator.GenerateStreamReport("stream_report_001", startTime, endTime)
	if err != nil {
		log.Printf("   Error generating report: %v\n", err)
		return
	}

	fmt.Printf("   Report Generated:\n")
	fmt.Printf("   - ID: %s\n", report.ID)
	fmt.Printf("   - Type: %s\n", report.Type)
	fmt.Printf("   - Title: %s\n", report.Title)
	fmt.Printf("   - Generated At: %s\n", report.GeneratedAt.Format(time.RFC3339))
	fmt.Printf("   - Data Points: %d\n", len(report.Data))

	// Export to JSON
	jsonData, err := report.ToJSON()
	if err != nil {
		log.Printf("   Error exporting to JSON: %v\n", err)
	} else {
		fmt.Printf("   - JSON Export: %d bytes\n", len(jsonData))
	}

	// Export to CSV
	csvData, err := report.ToCSV()
	if err != nil {
		log.Printf("   Error exporting to CSV: %v\n", err)
	} else {
		fmt.Printf("   - CSV Export: %d bytes\n", len(csvData))
	}

	fmt.Println()
}

// runPrometheusServer starts an HTTP server with Prometheus metrics endpoint
func runPrometheusServer() {
	fmt.Println("7. Prometheus Server Example")
	fmt.Println("   -------------------------------")

	// Create metrics registry
	registry := analytics.NewMetricsRegistry()

	// Create and register collectors
	streamCollector := analytics.NewInMemoryMetricsCollector()
	registry.Register("stream", streamCollector)

	viewerCollector := analytics.NewInMemoryMetricsCollector()
	registry.Register("viewer", viewerCollector)

	// Add some sample metrics
	streamCollector.RecordGauge("live_streams_total", 5, nil)
	streamCollector.RecordGauge("total_viewers", 542, nil)

	viewerCollector.RecordCounter("sessions_started_total", 1234, nil)
	viewerCollector.RecordGauge("active_sessions", 542, nil)

	// Create Prometheus exporter
	exporter := analytics.NewPrometheusExporter(registry)

	// Create HTTP server
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle("/metrics", exporter.PrometheusHandler())

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Root endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
		<head><title>ZenLive Analytics</title></head>
		<body>
			<h1>ZenLive Analytics & Monitoring</h1>
			<ul>
				<li><a href="/metrics">Prometheus Metrics</a></li>
				<li><a href="/health">Health Check</a></li>
			</ul>
		</body>
		</html>
		`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	// Start server
	server := &http.Server{
		Addr:    ":8090",
		Handler: mux,
	}

	go func() {
		fmt.Printf("   Starting HTTP server on http://localhost:8090\n")
		fmt.Printf("   - Prometheus metrics: http://localhost:8090/metrics\n")
		fmt.Printf("   - Health check: http://localhost:8090/health\n")
		fmt.Printf("   Press Ctrl+C to stop\n\n")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n   Shutting down server...")
}
