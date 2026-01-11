package analytics

import (
	"fmt"
	"sync"
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	// HealthStatusHealthy indicates the component is healthy
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusDegraded indicates the component is degraded but functional
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusUnhealthy indicates the component is unhealthy
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	// HealthStatusUnknown indicates the health status is unknown
	HealthStatusUnknown HealthStatus = "unknown"
)

// HealthCheck represents a health check result
type HealthCheck struct {
	Name      string                 // Name of the component being checked
	Status    HealthStatus           // Current health status
	Message   string                 // Human-readable status message
	Timestamp time.Time              // When the check was performed
	Duration  time.Duration          // How long the check took
	Metadata  map[string]interface{} // Additional metadata
	Error     error                  // Error if check failed
}

// HealthChecker is the interface for performing health checks
type HealthChecker interface {
	// Check performs the health check and returns the result
	Check() HealthCheck

	// Name returns the name of the health check
	Name() string
}

// SimpleHealthChecker is a simple implementation of HealthChecker
type SimpleHealthChecker struct {
	name      string
	checkFunc func() error
}

// NewSimpleHealthChecker creates a new simple health checker
func NewSimpleHealthChecker(name string, checkFunc func() error) *SimpleHealthChecker {
	return &SimpleHealthChecker{
		name:      name,
		checkFunc: checkFunc,
	}
}

// Check performs the health check
func (shc *SimpleHealthChecker) Check() HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name:      shc.name,
		Timestamp: start,
		Metadata:  make(map[string]interface{}),
	}

	err := shc.checkFunc()
	check.Duration = time.Since(start)

	if err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("Check failed: %v", err)
		check.Error = err
	} else {
		check.Status = HealthStatusHealthy
		check.Message = "OK"
	}

	return check
}

// Name returns the name of the health check
func (shc *SimpleHealthChecker) Name() string {
	return shc.name
}

// HealthMonitor monitors the health of various components
type HealthMonitor struct {
	checkers map[string]HealthChecker
	results  map[string]HealthCheck
	mu       sync.RWMutex

	// Configuration
	checkInterval time.Duration
	stopChan      chan struct{}

	// Metrics collector
	collector MetricsCollector
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(collector MetricsCollector) *HealthMonitor {
	return &HealthMonitor{
		checkers:      make(map[string]HealthChecker),
		results:       make(map[string]HealthCheck),
		checkInterval: 30 * time.Second, // Default check every 30 seconds
		collector:     collector,
	}
}

// RegisterChecker registers a health checker
func (hm *HealthMonitor) RegisterChecker(checker HealthChecker) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.checkers[checker.Name()] = checker
}

// UnregisterChecker unregisters a health checker
func (hm *HealthMonitor) UnregisterChecker(name string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	delete(hm.checkers, name)
	delete(hm.results, name)
}

// CheckAll performs all health checks
func (hm *HealthMonitor) CheckAll() map[string]HealthCheck {
	hm.mu.RLock()
	checkers := make([]HealthChecker, 0, len(hm.checkers))
	for _, checker := range hm.checkers {
		checkers = append(checkers, checker)
	}
	hm.mu.RUnlock()

	results := make(map[string]HealthCheck)
	for _, checker := range checkers {
		result := checker.Check()
		results[checker.Name()] = result

		// Record metrics
		if hm.collector != nil {
			labels := map[string]string{
				"component": checker.Name(),
				"status":    string(result.Status),
			}

			hm.collector.RecordGauge("health_check_status", hm.statusToFloat(result.Status), labels)
			hm.collector.RecordHistogram("health_check_duration_ms", float64(result.Duration.Milliseconds()), map[string]string{
				"component": checker.Name(),
			})
		}
	}

	hm.mu.Lock()
	hm.results = results
	hm.mu.Unlock()

	return results
}

// statusToFloat converts health status to a numeric value for metrics
func (hm *HealthMonitor) statusToFloat(status HealthStatus) float64 {
	switch status {
	case HealthStatusHealthy:
		return 1.0
	case HealthStatusDegraded:
		return 0.5
	case HealthStatusUnhealthy:
		return 0.0
	case HealthStatusUnknown:
		return -1.0
	default:
		return -1.0
	}
}

// GetResult retrieves the latest health check result for a component
func (hm *HealthMonitor) GetResult(name string) (HealthCheck, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result, exists := hm.results[name]
	return result, exists
}

// GetAllResults returns all latest health check results
func (hm *HealthMonitor) GetAllResults() map[string]HealthCheck {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	results := make(map[string]HealthCheck)
	for k, v := range hm.results {
		results[k] = v
	}
	return results
}

// GetOverallStatus returns the overall health status
func (hm *HealthMonitor) GetOverallStatus() HealthStatus {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if len(hm.results) == 0 {
		return HealthStatusUnknown
	}

	hasUnhealthy := false
	hasDegraded := false

	for _, result := range hm.results {
		switch result.Status {
		case HealthStatusUnhealthy:
			hasUnhealthy = true
		case HealthStatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return HealthStatusUnhealthy
	} else if hasDegraded {
		return HealthStatusDegraded
	}

	return HealthStatusHealthy
}

// Start starts periodic health checks
func (hm *HealthMonitor) Start() {
	hm.mu.Lock()
	if hm.stopChan != nil {
		hm.mu.Unlock()
		return // Already started
	}
	hm.stopChan = make(chan struct{})
	hm.mu.Unlock()

	go hm.runPeriodicChecks()
}

// Stop stops periodic health checks
func (hm *HealthMonitor) Stop() {
	hm.mu.Lock()
	if hm.stopChan != nil {
		close(hm.stopChan)
		hm.stopChan = nil
	}
	hm.mu.Unlock()
}

// runPeriodicChecks runs health checks periodically
func (hm *HealthMonitor) runPeriodicChecks() {
	ticker := time.NewTicker(hm.checkInterval)
	defer ticker.Stop()

	// Perform initial check
	hm.CheckAll()

	for {
		select {
		case <-ticker.C:
			hm.CheckAll()
		case <-hm.stopChan:
			return
		}
	}
}

// AlertLevel represents the severity level of an alert
type AlertLevel string

const (
	// AlertLevelInfo is for informational alerts
	AlertLevelInfo AlertLevel = "info"
	// AlertLevelWarning is for warning alerts
	AlertLevelWarning AlertLevel = "warning"
	// AlertLevelError is for error alerts
	AlertLevelError AlertLevel = "error"
	// AlertLevelCritical is for critical alerts
	AlertLevelCritical AlertLevel = "critical"
)

// Alert represents an alert
type Alert struct {
	ID         string                 // Unique alert identifier
	Name       string                 // Alert name
	Level      AlertLevel             // Severity level
	Message    string                 // Alert message
	Component  string                 // Component that triggered the alert
	Timestamp  time.Time              // When the alert was triggered
	Resolved   bool                   // Whether the alert has been resolved
	ResolvedAt time.Time              // When the alert was resolved
	Metadata   map[string]interface{} // Additional metadata
}

// AlertRule defines conditions for triggering an alert
type AlertRule struct {
	Name      string                              // Rule name
	Level     AlertLevel                          // Alert level
	Condition func(metrics *MetricsSnapshot) bool // Condition function
	Message   string                              // Alert message template
}

// AlertManager manages alerts and alert rules
type AlertManager struct {
	rules        map[string]*AlertRule
	activeAlerts map[string]*Alert
	alertHistory []*Alert
	mu           sync.RWMutex

	// Alert handlers
	handlers []AlertHandler

	// Configuration
	maxHistory int
}

// AlertHandler is called when an alert is triggered or resolved
type AlertHandler func(alert *Alert)

// NewAlertManager creates a new alert manager
func NewAlertManager() *AlertManager {
	return &AlertManager{
		rules:        make(map[string]*AlertRule),
		activeAlerts: make(map[string]*Alert),
		alertHistory: make([]*Alert, 0),
		maxHistory:   1000, // Keep last 1000 alerts
		handlers:     make([]AlertHandler, 0),
	}
}

// RegisterRule registers an alert rule
func (am *AlertManager) RegisterRule(rule *AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.rules[rule.Name] = rule
}

// UnregisterRule unregisters an alert rule
func (am *AlertManager) UnregisterRule(name string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	delete(am.rules, name)
}

// RegisterHandler registers an alert handler
func (am *AlertManager) RegisterHandler(handler AlertHandler) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.handlers = append(am.handlers, handler)
}

// EvaluateRules evaluates all alert rules against current metrics
func (am *AlertManager) EvaluateRules(metrics *MetricsSnapshot) {
	am.mu.RLock()
	rules := make([]*AlertRule, 0, len(am.rules))
	for _, rule := range am.rules {
		rules = append(rules, rule)
	}
	am.mu.RUnlock()

	for _, rule := range rules {
		if rule.Condition(metrics) {
			am.TriggerAlert(rule.Name, rule.Level, rule.Message, "system")
		} else {
			am.ResolveAlert(rule.Name)
		}
	}
}

// TriggerAlert triggers a new alert
func (am *AlertManager) TriggerAlert(name string, level AlertLevel, message, component string) {
	am.mu.Lock()

	// Check if alert already exists
	if existingAlert, exists := am.activeAlerts[name]; exists && !existingAlert.Resolved {
		am.mu.Unlock()
		return
	}

	alert := &Alert{
		ID:        fmt.Sprintf("%s_%d", name, time.Now().Unix()),
		Name:      name,
		Level:     level,
		Message:   message,
		Component: component,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	am.activeAlerts[name] = alert
	am.addToHistory(alert)

	handlers := make([]AlertHandler, len(am.handlers))
	copy(handlers, am.handlers)

	am.mu.Unlock()

	// Call handlers
	for _, handler := range handlers {
		handler(alert)
	}
}

// ResolveAlert resolves an active alert
func (am *AlertManager) ResolveAlert(name string) {
	am.mu.Lock()

	alert, exists := am.activeAlerts[name]
	if !exists || alert.Resolved {
		am.mu.Unlock()
		return
	}

	alert.Resolved = true
	alert.ResolvedAt = time.Now()
	delete(am.activeAlerts, name)

	handlers := make([]AlertHandler, len(am.handlers))
	copy(handlers, am.handlers)

	am.mu.Unlock()

	// Call handlers
	for _, handler := range handlers {
		handler(alert)
	}
}

// GetActiveAlerts returns all active alerts
func (am *AlertManager) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]*Alert, 0, len(am.activeAlerts))
	for _, alert := range am.activeAlerts {
		alerts = append(alerts, alert)
	}
	return alerts
}

// GetAlertHistory returns alert history
func (am *AlertManager) GetAlertHistory(limit int) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if limit <= 0 || limit > len(am.alertHistory) {
		limit = len(am.alertHistory)
	}

	// Return most recent alerts
	start := len(am.alertHistory) - limit
	if start < 0 {
		start = 0
	}

	alerts := make([]*Alert, limit)
	copy(alerts, am.alertHistory[start:])
	return alerts
}

// addToHistory adds an alert to history (must be called with lock held)
func (am *AlertManager) addToHistory(alert *Alert) {
	am.alertHistory = append(am.alertHistory, alert)

	// Trim history if needed
	if len(am.alertHistory) > am.maxHistory {
		am.alertHistory = am.alertHistory[len(am.alertHistory)-am.maxHistory:]
	}
}

// HealthSummary represents an overall health summary
type HealthSummary struct {
	OverallStatus  HealthStatus           // Overall health status
	Timestamp      time.Time              // When the summary was generated
	Components     map[string]HealthCheck // Health check results by component
	ActiveAlerts   int                    // Number of active alerts
	TotalChecks    int                    // Total number of checks performed
	HealthyCount   int                    // Number of healthy components
	DegradedCount  int                    // Number of degraded components
	UnhealthyCount int                    // Number of unhealthy components
}

// GetHealthSummary returns a comprehensive health summary
func GetHealthSummary(monitor *HealthMonitor, alertManager *AlertManager) HealthSummary {
	summary := HealthSummary{
		Timestamp:  time.Now(),
		Components: make(map[string]HealthCheck),
	}

	// Get health check results
	if monitor != nil {
		results := monitor.GetAllResults()
		summary.Components = results
		summary.TotalChecks = len(results)
		summary.OverallStatus = monitor.GetOverallStatus()

		// Count statuses
		for _, result := range results {
			switch result.Status {
			case HealthStatusHealthy:
				summary.HealthyCount++
			case HealthStatusDegraded:
				summary.DegradedCount++
			case HealthStatusUnhealthy:
				summary.UnhealthyCount++
			}
		}
	}

	// Get active alerts
	if alertManager != nil {
		summary.ActiveAlerts = len(alertManager.GetActiveAlerts())
	}

	return summary
}
