package security

import (
	"errors"
	"net"
	"sync"
	"time"
)

// IPAction defines the action for an IP address
type IPAction string

const (
	// IPActionAllow allows the IP
	IPActionAllow IPAction = "allow"
	// IPActionBlock blocks the IP
	IPActionBlock IPAction = "block"
	// IPActionRateLimit applies rate limiting
	IPActionRateLimit IPAction = "ratelimit"
)

// IPRule represents a firewall rule for an IP or CIDR range
type IPRule struct {
	ID        string
	Pattern   string // IP address or CIDR notation
	Action    IPAction
	Reason    string
	CreatedAt time.Time
	ExpiresAt time.Time // Zero value means no expiration
	Priority  int       // Higher priority rules are checked first
}

// ConnectionLimitConfig defines connection limit settings
type ConnectionLimitConfig struct {
	// MaxConnectionsPerIP limits concurrent connections per IP
	MaxConnectionsPerIP int
	// MaxConnectionsGlobal limits total concurrent connections
	MaxConnectionsGlobal int
	// MaxConnectionRate limits new connections per second
	MaxConnectionRate int
	// BanDuration is how long to ban IPs that exceed limits
	BanDuration time.Duration
}

// Firewall manages IP-based access control
type Firewall struct {
	mu               sync.RWMutex
	rules            map[string]*IPRule
	rulesList        []*IPRule // Sorted by priority
	whitelist        map[string]bool
	blacklist        map[string]bool
	connectionCounts map[string]int
	globalConnCount  int
	connLimit        *ConnectionLimitConfig
	onBlock          func(ip string, reason string)
	cleanupInterval  time.Duration
	stopCleanup      chan struct{}
}

// ConnectionTracker tracks active connections
type ConnectionTracker struct {
	mu          sync.RWMutex
	connections map[string]map[string]*Connection
	firewall    *Firewall
}

// Connection represents an active connection
type Connection struct {
	ID        string
	IP        string
	UserID    string
	StartTime time.Time
	LastSeen  time.Time
	BytesSent int64
	BytesRecv int64
}

var (
	// ErrIPBlocked is returned when an IP is blocked
	ErrIPBlocked = errors.New("IP address is blocked")
	// ErrConnectionLimitExceeded is returned when connection limit is exceeded
	ErrConnectionLimitExceeded = errors.New("connection limit exceeded")
	// ErrInvalidIPPattern is returned for invalid IP patterns
	ErrInvalidIPPattern = errors.New("invalid IP pattern")
)

// NewFirewall creates a new firewall
func NewFirewall(config *ConnectionLimitConfig) *Firewall {
	if config == nil {
		config = &ConnectionLimitConfig{
			MaxConnectionsPerIP:  100,
			MaxConnectionsGlobal: 10000,
			MaxConnectionRate:    100,
			BanDuration:          1 * time.Hour,
		}
	}

	fw := &Firewall{
		rules:            make(map[string]*IPRule),
		rulesList:        make([]*IPRule, 0),
		whitelist:        make(map[string]bool),
		blacklist:        make(map[string]bool),
		connectionCounts: make(map[string]int),
		connLimit:        config,
		cleanupInterval:  5 * time.Minute,
		stopCleanup:      make(chan struct{}),
	}

	// Add default whitelist for localhost
	fw.whitelist["127.0.0.1"] = true
	fw.whitelist["::1"] = true

	go fw.cleanup()

	return fw
}

// AddRule adds a firewall rule
func (fw *Firewall) AddRule(rule *IPRule) error {
	// Validate IP pattern
	if !fw.isValidIPPattern(rule.Pattern) {
		return ErrInvalidIPPattern
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	rule.CreatedAt = time.Now()
	fw.rules[rule.ID] = rule
	fw.rebuildRulesList()

	// Update whitelist/blacklist for quick lookup
	if rule.Action == IPActionAllow {
		fw.whitelist[rule.Pattern] = true
	} else if rule.Action == IPActionBlock {
		fw.blacklist[rule.Pattern] = true
	}

	return nil
}

// RemoveRule removes a firewall rule
func (fw *Firewall) RemoveRule(id string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if rule, exists := fw.rules[id]; exists {
		delete(fw.rules, id)
		delete(fw.whitelist, rule.Pattern)
		delete(fw.blacklist, rule.Pattern)
		fw.rebuildRulesList()
	}
}

// CheckIP checks if an IP address is allowed
func (fw *Firewall) CheckIP(ip string) error {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	// Quick check whitelist
	if fw.whitelist[ip] {
		return nil
	}

	// Quick check blacklist
	if fw.blacklist[ip] {
		if fw.onBlock != nil {
			fw.onBlock(ip, "IP is blacklisted")
		}
		return ErrIPBlocked
	}

	// Check rules in priority order
	for _, rule := range fw.rulesList {
		// Check if rule is expired
		if !rule.ExpiresAt.IsZero() && time.Now().After(rule.ExpiresAt) {
			continue
		}

		if fw.matchesPattern(ip, rule.Pattern) {
			if rule.Action == IPActionBlock {
				if fw.onBlock != nil {
					fw.onBlock(ip, rule.Reason)
				}
				return ErrIPBlocked
			}
			if rule.Action == IPActionAllow {
				return nil
			}
		}
	}

	return nil
}

// CheckConnectionLimit checks if a new connection from IP is allowed
func (fw *Firewall) CheckConnectionLimit(ip string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Check global connection limit
	if fw.globalConnCount >= fw.connLimit.MaxConnectionsGlobal {
		return ErrConnectionLimitExceeded
	}

	// Check per-IP connection limit
	count := fw.connectionCounts[ip]
	if count >= fw.connLimit.MaxConnectionsPerIP {
		// Auto-ban IP temporarily
		fw.banIP(ip, "Exceeded connection limit", fw.connLimit.BanDuration)
		return ErrConnectionLimitExceeded
	}

	return nil
}

// IncrementConnection increments connection count for an IP
func (fw *Firewall) IncrementConnection(ip string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.connectionCounts[ip]++
	fw.globalConnCount++
}

// DecrementConnection decrements connection count for an IP
func (fw *Firewall) DecrementConnection(ip string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.connectionCounts[ip] > 0 {
		fw.connectionCounts[ip]--
	}
	if fw.globalConnCount > 0 {
		fw.globalConnCount--
	}

	// Cleanup if no connections
	if fw.connectionCounts[ip] == 0 {
		delete(fw.connectionCounts, ip)
	}
}

// BanIP bans an IP address temporarily or permanently
func (fw *Firewall) BanIP(ip, reason string, duration time.Duration) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.banIP(ip, reason, duration)
}

// Internal ban function (must be called with lock held)
func (fw *Firewall) banIP(ip, reason string, duration time.Duration) {
	rule := &IPRule{
		ID:        "ban_" + ip + "_" + time.Now().Format("20060102150405"),
		Pattern:   ip,
		Action:    IPActionBlock,
		Reason:    reason,
		CreatedAt: time.Now(),
		Priority:  1000, // High priority
	}

	if duration > 0 {
		rule.ExpiresAt = time.Now().Add(duration)
	}

	fw.rules[rule.ID] = rule
	fw.blacklist[ip] = true
	fw.rebuildRulesList()

	if fw.onBlock != nil {
		fw.onBlock(ip, reason)
	}
}

// UnbanIP removes a ban for an IP address
func (fw *Firewall) UnbanIP(ip string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	delete(fw.blacklist, ip)

	// Remove all ban rules for this IP
	for id, rule := range fw.rules {
		if rule.Pattern == ip && rule.Action == IPActionBlock {
			delete(fw.rules, id)
		}
	}

	fw.rebuildRulesList()
}

// GetStats returns firewall statistics
func (fw *Firewall) GetStats() map[string]interface{} {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	return map[string]interface{}{
		"total_rules":        len(fw.rules),
		"whitelisted_ips":    len(fw.whitelist),
		"blacklisted_ips":    len(fw.blacklist),
		"active_connections": fw.globalConnCount,
		"max_connections":    fw.connLimit.MaxConnectionsGlobal,
		"connection_per_ip":  len(fw.connectionCounts),
	}
}

// SetBlockCallback sets the callback for when an IP is blocked
func (fw *Firewall) SetBlockCallback(callback func(ip string, reason string)) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.onBlock = callback
}

// rebuildRulesList rebuilds the rules list sorted by priority
func (fw *Firewall) rebuildRulesList() {
	fw.rulesList = make([]*IPRule, 0, len(fw.rules))
	for _, rule := range fw.rules {
		fw.rulesList = append(fw.rulesList, rule)
	}

	// Sort by priority (descending)
	for i := 0; i < len(fw.rulesList); i++ {
		for j := i + 1; j < len(fw.rulesList); j++ {
			if fw.rulesList[j].Priority > fw.rulesList[i].Priority {
				fw.rulesList[i], fw.rulesList[j] = fw.rulesList[j], fw.rulesList[i]
			}
		}
	}
}

// isValidIPPattern checks if an IP pattern is valid
func (fw *Firewall) isValidIPPattern(pattern string) bool {
	// Check if it's a valid IP
	if net.ParseIP(pattern) != nil {
		return true
	}

	// Check if it's a valid CIDR
	_, _, err := net.ParseCIDR(pattern)
	return err == nil
}

// matchesPattern checks if an IP matches a pattern (IP or CIDR)
func (fw *Firewall) matchesPattern(ip, pattern string) bool {
	// Exact match
	if ip == pattern {
		return true
	}

	// CIDR match
	_, ipNet, err := net.ParseCIDR(pattern)
	if err != nil {
		return false
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	return ipNet.Contains(parsedIP)
}

// cleanup periodically removes expired rules
func (fw *Firewall) cleanup() {
	ticker := time.NewTicker(fw.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fw.mu.Lock()
			now := time.Now()
			for id, rule := range fw.rules {
				if !rule.ExpiresAt.IsZero() && now.After(rule.ExpiresAt) {
					delete(fw.rules, id)
					delete(fw.blacklist, rule.Pattern)
					delete(fw.whitelist, rule.Pattern)
				}
			}
			fw.rebuildRulesList()
			fw.mu.Unlock()
		case <-fw.stopCleanup:
			return
		}
	}
}

// Stop stops the firewall
func (fw *Firewall) Stop() {
	close(fw.stopCleanup)
}

// NewConnectionTracker creates a new connection tracker
func NewConnectionTracker(firewall *Firewall) *ConnectionTracker {
	return &ConnectionTracker{
		connections: make(map[string]map[string]*Connection),
		firewall:    firewall,
	}
}

// AddConnection adds a new connection
func (ct *ConnectionTracker) AddConnection(conn *Connection) error {
	// Check firewall
	if err := ct.firewall.CheckIP(conn.IP); err != nil {
		return err
	}

	// Check connection limit
	if err := ct.firewall.CheckConnectionLimit(conn.IP); err != nil {
		return err
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	if _, exists := ct.connections[conn.IP]; !exists {
		ct.connections[conn.IP] = make(map[string]*Connection)
	}

	ct.connections[conn.IP][conn.ID] = conn
	ct.firewall.IncrementConnection(conn.IP)

	return nil
}

// RemoveConnection removes a connection
func (ct *ConnectionTracker) RemoveConnection(ip, connID string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if connections, exists := ct.connections[ip]; exists {
		delete(connections, connID)
		if len(connections) == 0 {
			delete(ct.connections, ip)
		}
		ct.firewall.DecrementConnection(ip)
	}
}

// GetConnectionCount returns the number of connections for an IP
func (ct *ConnectionTracker) GetConnectionCount(ip string) int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	if connections, exists := ct.connections[ip]; exists {
		return len(connections)
	}
	return 0
}

// GetTotalConnections returns total number of active connections
func (ct *ConnectionTracker) GetTotalConnections() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	total := 0
	for _, connections := range ct.connections {
		total += len(connections)
	}
	return total
}
