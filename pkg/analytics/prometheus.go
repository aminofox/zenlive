package analytics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// PrometheusExporter exports metrics in Prometheus format
type PrometheusExporter struct {
	registry *MetricsRegistry
	mu       sync.RWMutex
}

// NewPrometheusExporter creates a new Prometheus exporter
func NewPrometheusExporter(registry *MetricsRegistry) *PrometheusExporter {
	return &PrometheusExporter{
		registry: registry,
	}
}

// ServeHTTP serves metrics in Prometheus format
func (pe *PrometheusExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics := pe.collectMetrics()
	output := pe.formatPrometheusMetrics(metrics)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(output))
}

// collectMetrics collects all metrics from the registry
func (pe *PrometheusExporter) collectMetrics() []Metric {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var allMetrics []Metric

	if pe.registry != nil {
		snapshots := pe.registry.GetAllSnapshots()
		for _, snapshot := range snapshots {
			for _, metric := range snapshot.GetAll() {
				allMetrics = append(allMetrics, metric)
			}
		}
	}

	return allMetrics
}

// formatPrometheusMetrics formats metrics in Prometheus exposition format
func (pe *PrometheusExporter) formatPrometheusMetrics(metrics []Metric) string {
	var sb strings.Builder

	// Group metrics by name
	metricsByName := make(map[string][]Metric)
	for _, metric := range metrics {
		metricsByName[metric.Name] = append(metricsByName[metric.Name], metric)
	}

	// Sort metric names for consistent output
	names := make([]string, 0, len(metricsByName))
	for name := range metricsByName {
		names = append(names, name)
	}
	sort.Strings(names)

	// Format each metric group
	for _, name := range names {
		metricsGroup := metricsByName[name]
		if len(metricsGroup) == 0 {
			continue
		}

		// Write HELP line (if available)
		if metricsGroup[0].Help != "" {
			sb.WriteString(fmt.Sprintf("# HELP %s %s\n", name, metricsGroup[0].Help))
		}

		// Write TYPE line
		prometheusType := pe.convertMetricType(metricsGroup[0].Type)
		sb.WriteString(fmt.Sprintf("# TYPE %s %s\n", name, prometheusType))

		// Write metric lines
		for _, metric := range metricsGroup {
			sb.WriteString(pe.formatMetricLine(metric))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// convertMetricType converts internal metric type to Prometheus type
func (pe *PrometheusExporter) convertMetricType(metricType MetricType) string {
	switch metricType {
	case MetricTypeCounter:
		return "counter"
	case MetricTypeGauge:
		return "gauge"
	case MetricTypeHistogram:
		return "histogram"
	case MetricTypeSummary:
		return "summary"
	default:
		return "untyped"
	}
}

// formatMetricLine formats a single metric line in Prometheus format
func (pe *PrometheusExporter) formatMetricLine(metric Metric) string {
	var sb strings.Builder

	// Metric name
	sb.WriteString(metric.Name)

	// Labels
	if len(metric.Labels) > 0 {
		sb.WriteString("{")

		// Sort labels for consistent output
		labelKeys := make([]string, 0, len(metric.Labels))
		for k := range metric.Labels {
			labelKeys = append(labelKeys, k)
		}
		sort.Strings(labelKeys)

		first := true
		for _, k := range labelKeys {
			if !first {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("%s=\"%s\"", k, escapeString(metric.Labels[k])))
			first = false
		}

		sb.WriteString("}")
	}

	// Value
	sb.WriteString(fmt.Sprintf(" %v", metric.Value))

	// Timestamp (optional, in milliseconds)
	if !metric.Timestamp.IsZero() {
		sb.WriteString(fmt.Sprintf(" %d", metric.Timestamp.UnixMilli()))
	}

	sb.WriteString("\n")

	// For histogram/summary, also output additional metrics
	if metric.Type == MetricTypeHistogram && metric.Metadata != nil {
		if count, ok := metric.Metadata["count"].(int); ok {
			sb.WriteString(fmt.Sprintf("%s_count", metric.Name))
			if len(metric.Labels) > 0 {
				sb.WriteString(pe.formatLabels(metric.Labels))
			}
			sb.WriteString(fmt.Sprintf(" %d\n", count))
		}

		if sum, ok := metric.Metadata["sum"].(float64); ok {
			sb.WriteString(fmt.Sprintf("%s_sum", metric.Name))
			if len(metric.Labels) > 0 {
				sb.WriteString(pe.formatLabels(metric.Labels))
			}
			sb.WriteString(fmt.Sprintf(" %v\n", sum))
		}
	}

	return sb.String()
}

// formatLabels formats labels in Prometheus format
func (pe *PrometheusExporter) formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("{")

	// Sort labels for consistent output
	labelKeys := make([]string, 0, len(labels))
	for k := range labels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)

	first := true
	for _, k := range labelKeys {
		if !first {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("%s=\"%s\"", k, escapeString(labels[k])))
		first = false
	}

	sb.WriteString("}")
	return sb.String()
}

// escapeString escapes special characters in label values
func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// PrometheusHandler returns an HTTP handler for Prometheus metrics
func (pe *PrometheusExporter) PrometheusHandler() http.Handler {
	return http.HandlerFunc(pe.ServeHTTP)
}

// StreamMetricsExporter exports stream metrics to Prometheus
type StreamMetricsExporter struct {
	streamCollector *StreamMetricsCollector
	collector       MetricsCollector
}

// NewStreamMetricsExporter creates a new stream metrics exporter
func NewStreamMetricsExporter(streamCollector *StreamMetricsCollector, collector MetricsCollector) *StreamMetricsExporter {
	return &StreamMetricsExporter{
		streamCollector: streamCollector,
		collector:       collector,
	}
}

// Export exports current stream metrics to the metrics collector
func (sme *StreamMetricsExporter) Export() {
	streams := sme.streamCollector.GetAllStreams()

	totalLiveStreams := 0
	totalViewers := 0

	for streamID, metrics := range streams {
		if !metrics.IsLive() {
			continue
		}

		totalLiveStreams++

		snapshot := metrics.GetSnapshot()
		labels := map[string]string{"stream_id": streamID}

		// Viewer metrics
		sme.collector.RecordGauge("stream_viewers_current", float64(snapshot.CurrentViewers), labels)
		sme.collector.RecordGauge("stream_viewers_peak", float64(snapshot.PeakViewers), labels)
		sme.collector.RecordGauge("stream_viewers_total", float64(snapshot.TotalViewers), labels)
		totalViewers += snapshot.CurrentViewers

		// Video quality metrics
		sme.collector.RecordGauge("stream_bitrate_current_bps", snapshot.CurrentBitrate, labels)
		sme.collector.RecordGauge("stream_bitrate_average_bps", snapshot.AverageBitrate, labels)
		sme.collector.RecordGauge("stream_bitrate_peak_bps", snapshot.PeakBitrate, labels)

		sme.collector.RecordGauge("stream_fps_current", snapshot.CurrentFPS, labels)
		sme.collector.RecordGauge("stream_fps_average", snapshot.AverageFPS, labels)
		sme.collector.RecordGauge("stream_fps_target", snapshot.TargetFPS, labels)

		// Frame metrics
		sme.collector.RecordCounter("stream_frames_dropped_total", float64(snapshot.DroppedFrames), labels)
		sme.collector.RecordCounter("stream_frames_total", float64(snapshot.TotalFrames), labels)
		sme.collector.RecordGauge("stream_frame_drop_rate_percent", snapshot.DropRate, labels)

		// Resolution
		sme.collector.RecordGauge("stream_resolution_width", float64(snapshot.Width), labels)
		sme.collector.RecordGauge("stream_resolution_height", float64(snapshot.Height), labels)

		// Audio metrics
		sme.collector.RecordGauge("stream_audio_bitrate_bps", snapshot.AudioBitrate, labels)
		sme.collector.RecordGauge("stream_audio_sample_rate_hz", float64(snapshot.AudioSampleRate), labels)

		// Network metrics
		sme.collector.RecordCounter("stream_bytes_sent_total", float64(snapshot.BytesSent), labels)
		sme.collector.RecordCounter("stream_bytes_received_total", float64(snapshot.BytesReceived), labels)
		sme.collector.RecordCounter("stream_packets_lost_total", float64(snapshot.PacketsLost), labels)
		sme.collector.RecordGauge("stream_jitter_ms", snapshot.Jitter, labels)
		sme.collector.RecordGauge("stream_rtt_ms", snapshot.RTT, labels)

		// Duration
		duration := time.Duration(0)
		if snapshot.EndTime.IsZero() {
			duration = time.Since(snapshot.StartTime)
		} else {
			duration = snapshot.EndTime.Sub(snapshot.StartTime)
		}
		sme.collector.RecordGauge("stream_duration_seconds", duration.Seconds(), labels)
	}

	// Global metrics (no labels)
	sme.collector.RecordGauge("streams_live_total", float64(totalLiveStreams), nil)
	sme.collector.RecordGauge("streams_viewers_total", float64(totalViewers), nil)
}

// StartPeriodicExport starts periodic export of metrics
func (sme *StreamMetricsExporter) StartPeriodicExport(interval int) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				sme.Export()
			case <-stop:
				return
			}
		}
	}()

	return stop
}
