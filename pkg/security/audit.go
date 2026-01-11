package security

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// AuditEventType defines the type of audit event
type AuditEventType string

const (
	// AuditEventAuth represents authentication events
	AuditEventAuth AuditEventType = "auth"
	// AuditEventStream represents stream lifecycle events
	AuditEventStream AuditEventType = "stream"
	// AuditEventAccess represents access control events
	AuditEventAccess AuditEventType = "access"
	// AuditEventSecurity represents security events
	AuditEventSecurity AuditEventType = "security"
	// AuditEventData represents data modification events
	AuditEventData AuditEventType = "data"
	// AuditEventAdmin represents administrative actions
	AuditEventAdmin AuditEventType = "admin"
	// AuditEventCompliance represents compliance-related events
	AuditEventCompliance AuditEventType = "compliance"
)

// AuditSeverity defines the severity level of an audit event
type AuditSeverity string

const (
	// AuditSeverityInfo is for informational events
	AuditSeverityInfo AuditSeverity = "info"
	// AuditSeverityWarning is for warning events
	AuditSeverityWarning AuditSeverity = "warning"
	// AuditSeverityError is for error events
	AuditSeverityError AuditSeverity = "error"
	// AuditSeverityCritical is for critical security events
	AuditSeverityCritical AuditSeverity = "critical"
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	ID         string
	Type       AuditEventType
	Severity   AuditSeverity
	Timestamp  time.Time
	UserID     string
	IP         string
	Action     string
	Resource   string
	ResourceID string
	Status     string // success, failure, denied
	Message    string
	Metadata   map[string]interface{}
	Duration   time.Duration
}

// AuditLogger logs security and compliance events
type AuditLogger struct {
	mu          sync.RWMutex
	events      []*AuditEvent
	maxEvents   int
	persistence AuditPersistence
	filters     []AuditFilter
	onEvent     func(*AuditEvent)
}

// AuditPersistence is an interface for persisting audit logs
type AuditPersistence interface {
	Save(event *AuditEvent) error
	Query(filter *AuditQuery) ([]*AuditEvent, error)
	Delete(before time.Time) error
}

// AuditFilter is a function that determines if an event should be logged
type AuditFilter func(*AuditEvent) bool

// AuditQuery defines query parameters for audit logs
type AuditQuery struct {
	StartTime  time.Time
	EndTime    time.Time
	Types      []AuditEventType
	Severities []AuditSeverity
	UserIDs    []string
	Actions    []string
	Limit      int
	Offset     int
}

// ComplianceReport represents a compliance report
type ComplianceReport struct {
	ID               string
	GeneratedAt      time.Time
	StartTime        time.Time
	EndTime          time.Time
	TotalEvents      int
	EventsByType     map[AuditEventType]int
	EventsBySeverity map[AuditSeverity]int
	CriticalEvents   []*AuditEvent
	Violations       []string
	Recommendations  []string
}

// InMemoryPersistence implements in-memory audit persistence
type InMemoryPersistence struct {
	mu     sync.RWMutex
	events []*AuditEvent
}

var (
	// ErrInvalidQuery is returned for invalid audit queries
	ErrInvalidQuery = errors.New("invalid audit query")
)

// NewAuditLogger creates a new audit logger
func NewAuditLogger(maxEvents int, persistence AuditPersistence) *AuditLogger {
	if maxEvents <= 0 {
		maxEvents = 10000
	}

	if persistence == nil {
		persistence = NewInMemoryPersistence()
	}

	return &AuditLogger{
		events:      make([]*AuditEvent, 0, maxEvents),
		maxEvents:   maxEvents,
		persistence: persistence,
		filters:     make([]AuditFilter, 0),
	}
}

// Log logs an audit event
func (al *AuditLogger) Log(event *AuditEvent) error {
	// Generate ID and timestamp if not set
	if event.ID == "" {
		event.ID = generateAuditID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Apply filters
	for _, filter := range al.filters {
		if !filter(event) {
			return nil // Event filtered out
		}
	}

	// Store in memory
	al.mu.Lock()
	al.events = append(al.events, event)
	if len(al.events) > al.maxEvents {
		al.events = al.events[1:] // Remove oldest
	}
	al.mu.Unlock()

	// Persist
	if err := al.persistence.Save(event); err != nil {
		return fmt.Errorf("failed to persist audit event: %w", err)
	}

	// Trigger callback
	if al.onEvent != nil {
		al.onEvent(event)
	}

	return nil
}

// LogAuth logs an authentication event
func (al *AuditLogger) LogAuth(userID, ip, action, status, message string) error {
	return al.Log(&AuditEvent{
		Type:     AuditEventAuth,
		Severity: al.getSeverityForStatus(status),
		UserID:   userID,
		IP:       ip,
		Action:   action,
		Status:   status,
		Message:  message,
	})
}

// LogStream logs a stream event
func (al *AuditLogger) LogStream(userID, streamID, action, status string) error {
	return al.Log(&AuditEvent{
		Type:       AuditEventStream,
		Severity:   AuditSeverityInfo,
		UserID:     userID,
		Action:     action,
		Resource:   "stream",
		ResourceID: streamID,
		Status:     status,
	})
}

// LogSecurityEvent logs a security event
func (al *AuditLogger) LogSecurityEvent(severity AuditSeverity, userID, ip, action, message string, metadata map[string]interface{}) error {
	return al.Log(&AuditEvent{
		Type:     AuditEventSecurity,
		Severity: severity,
		UserID:   userID,
		IP:       ip,
		Action:   action,
		Message:  message,
		Metadata: metadata,
	})
}

// LogAccess logs an access control event
func (al *AuditLogger) LogAccess(userID, ip, resource, resourceID, action, status string) error {
	return al.Log(&AuditEvent{
		Type:       AuditEventAccess,
		Severity:   al.getSeverityForStatus(status),
		UserID:     userID,
		IP:         ip,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Status:     status,
	})
}

// Query queries audit logs
func (al *AuditLogger) Query(query *AuditQuery) ([]*AuditEvent, error) {
	if query == nil {
		return nil, ErrInvalidQuery
	}

	// Try persistence first
	if al.persistence != nil {
		return al.persistence.Query(query)
	}

	// Fall back to in-memory
	al.mu.RLock()
	defer al.mu.RUnlock()

	results := make([]*AuditEvent, 0)

	for _, event := range al.events {
		if al.matchesQuery(event, query) {
			results = append(results, event)
		}
	}

	// Apply limit and offset
	if query.Offset < len(results) {
		results = results[query.Offset:]
	} else {
		results = []*AuditEvent{}
	}

	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results, nil
}

// GetRecent returns the most recent audit events
func (al *AuditLogger) GetRecent(count int) []*AuditEvent {
	al.mu.RLock()
	defer al.mu.RUnlock()

	if count > len(al.events) {
		count = len(al.events)
	}

	results := make([]*AuditEvent, count)
	startIdx := len(al.events) - count

	for i := 0; i < count; i++ {
		results[i] = al.events[startIdx+i]
	}

	return results
}

// GenerateComplianceReport generates a compliance report
func (al *AuditLogger) GenerateComplianceReport(startTime, endTime time.Time) (*ComplianceReport, error) {
	query := &AuditQuery{
		StartTime: startTime,
		EndTime:   endTime,
	}

	events, err := al.Query(query)
	if err != nil {
		return nil, err
	}

	report := &ComplianceReport{
		ID:               generateReportID(),
		GeneratedAt:      time.Now(),
		StartTime:        startTime,
		EndTime:          endTime,
		TotalEvents:      len(events),
		EventsByType:     make(map[AuditEventType]int),
		EventsBySeverity: make(map[AuditSeverity]int),
		CriticalEvents:   make([]*AuditEvent, 0),
		Violations:       make([]string, 0),
		Recommendations:  make([]string, 0),
	}

	// Analyze events
	for _, event := range events {
		report.EventsByType[event.Type]++
		report.EventsBySeverity[event.Severity]++

		if event.Severity == AuditSeverityCritical {
			report.CriticalEvents = append(report.CriticalEvents, event)
		}

		// Check for violations
		if event.Status == "failure" && event.Type == AuditEventAuth {
			report.Violations = append(report.Violations,
				fmt.Sprintf("Failed authentication attempt from %s at %s", event.IP, event.Timestamp))
		}
	}

	// Add recommendations based on findings
	if len(report.CriticalEvents) > 10 {
		report.Recommendations = append(report.Recommendations,
			"High number of critical security events detected. Review security policies.")
	}

	if report.EventsByType[AuditEventAuth] > 1000 {
		report.Recommendations = append(report.Recommendations,
			"High authentication activity. Consider implementing additional rate limiting.")
	}

	return report, nil
}

// AddFilter adds an audit filter
func (al *AuditLogger) AddFilter(filter AuditFilter) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.filters = append(al.filters, filter)
}

// SetEventCallback sets the callback for new audit events
func (al *AuditLogger) SetEventCallback(callback func(*AuditEvent)) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.onEvent = callback
}

// ExportJSON exports audit events to JSON
func (al *AuditLogger) ExportJSON(query *AuditQuery) ([]byte, error) {
	events, err := al.Query(query)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(events, "", "  ")
}

// matchesQuery checks if an event matches the query
func (al *AuditLogger) matchesQuery(event *AuditEvent, query *AuditQuery) bool {
	// Time range
	if !query.StartTime.IsZero() && event.Timestamp.Before(query.StartTime) {
		return false
	}
	if !query.EndTime.IsZero() && event.Timestamp.After(query.EndTime) {
		return false
	}

	// Types
	if len(query.Types) > 0 {
		found := false
		for _, t := range query.Types {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Severities
	if len(query.Severities) > 0 {
		found := false
		for _, s := range query.Severities {
			if event.Severity == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// UserIDs
	if len(query.UserIDs) > 0 {
		found := false
		for _, uid := range query.UserIDs {
			if event.UserID == uid {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Actions
	if len(query.Actions) > 0 {
		found := false
		for _, a := range query.Actions {
			if event.Action == a {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// getSeverityForStatus determines severity based on status
func (al *AuditLogger) getSeverityForStatus(status string) AuditSeverity {
	switch status {
	case "failure", "denied":
		return AuditSeverityWarning
	case "error":
		return AuditSeverityError
	default:
		return AuditSeverityInfo
	}
}

// NewInMemoryPersistence creates a new in-memory persistence
func NewInMemoryPersistence() *InMemoryPersistence {
	return &InMemoryPersistence{
		events: make([]*AuditEvent, 0),
	}
}

// Save saves an audit event
func (imp *InMemoryPersistence) Save(event *AuditEvent) error {
	imp.mu.Lock()
	defer imp.mu.Unlock()
	imp.events = append(imp.events, event)
	return nil
}

// Query queries audit events
func (imp *InMemoryPersistence) Query(query *AuditQuery) ([]*AuditEvent, error) {
	imp.mu.RLock()
	defer imp.mu.RUnlock()

	results := make([]*AuditEvent, 0)
	for _, event := range imp.events {
		// Time range filter
		if !query.StartTime.IsZero() && event.Timestamp.Before(query.StartTime) {
			continue
		}
		if !query.EndTime.IsZero() && event.Timestamp.After(query.EndTime) {
			continue
		}

		// Type filter
		if len(query.Types) > 0 {
			found := false
			for _, t := range query.Types {
				if event.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Severity filter
		if len(query.Severities) > 0 {
			found := false
			for _, s := range query.Severities {
				if event.Severity == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		results = append(results, event)
	}

	return results, nil
}

// Delete deletes events before a certain time
func (imp *InMemoryPersistence) Delete(before time.Time) error {
	imp.mu.Lock()
	defer imp.mu.Unlock()

	filtered := make([]*AuditEvent, 0)
	for _, event := range imp.events {
		if event.Timestamp.After(before) {
			filtered = append(filtered, event)
		}
	}

	imp.events = filtered
	return nil
}

// generateAuditID generates a unique audit event ID
func generateAuditID() string {
	return fmt.Sprintf("audit_%d", time.Now().UnixNano())
}

// generateReportID generates a unique report ID
func generateReportID() string {
	return fmt.Sprintf("report_%d", time.Now().UnixNano())
}
