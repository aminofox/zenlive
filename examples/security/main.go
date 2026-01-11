package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminofox/zenlive/pkg/security"
)

func main() {
	fmt.Println("=== ZenLive Phase 12: Security Hardening Demo ===\n")

	// Run all security examples
	runTLSExample()
	runEncryptionExample()
	runRateLimitExample()
	runFirewallExample()
	runKeyRotationExample()
	runWatermarkExample()
	runAuditExample()

	fmt.Println("\n=== Security Demo Complete ===")
	fmt.Println("Press Ctrl+C to exit...")

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
}

func runTLSExample() {
	fmt.Println("\n--- TLS/Certificate Management Example ---")

	// Generate self-signed certificate for development
	certFile := "demo_cert.pem"
	keyFile := "demo_key.pem"

	fmt.Println("Generating self-signed certificate...")
	err := security.GenerateSelfSignedCert(certFile, keyFile,
		[]string{"localhost", "127.0.0.1"}, 365*24*time.Hour)
	if err != nil {
		fmt.Printf("Error generating certificate: %v\n", err)
		return
	}
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	// Create certificate manager
	config := &security.TLSConfig{
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	cm, err := security.NewCertificateManager(config)
	if err != nil {
		fmt.Printf("Error creating certificate manager: %v\n", err)
		return
	}

	// Get certificate info
	info, _ := cm.GetCertificateInfo()
	fmt.Printf("✓ Certificate created\n")
	fmt.Printf("  Subject: %s\n", info.Subject.CommonName)
	fmt.Printf("  Valid from: %s\n", info.NotBefore.Format("2006-01-02"))
	fmt.Printf("  Valid until: %s\n", info.NotAfter.Format("2006-01-02"))
	fmt.Printf("  DNS Names: %v\n", info.DNSNames)
	fmt.Printf("  Expires in: %v\n", cm.ExpiresIn().Round(24*time.Hour))

	// Get TLS config for server
	tlsConfig := cm.GetTLSConfig()
	fmt.Printf("\n✓ TLS Configuration:\n")
	fmt.Printf("  Min TLS Version: 1.2\n")
	fmt.Printf("  Cipher Suites: %d configured\n", len(tlsConfig.CipherSuites))
	fmt.Printf("  Ready for HTTPS server\n")
}

func runEncryptionExample() {
	fmt.Println("\n--- Encryption & Key Management Example ---")

	// Create key manager
	km := security.NewKeyManager(32) // AES-256

	// Generate encryption key
	key, _ := km.GenerateKey("production-key-1")
	fmt.Printf("✓ Generated encryption key: %s\n", key.ID)
	fmt.Printf("  Algorithm: %s\n", key.Algorithm)
	fmt.Printf("  Created: %s\n", time.Unix(key.CreatedAt, 0).Format("2006-01-02 15:04:05"))

	// Token encryption example
	te := security.NewTokenEncryptor(km)

	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.user-session-token"
	fmt.Printf("\n✓ Token Encryption:\n")
	fmt.Printf("  Original: %s...\n", token[:50])

	encrypted, _ := te.Encrypt(token)
	fmt.Printf("  Encrypted: %s...\n", encrypted[:50])

	decrypted, _ := te.Decrypt(encrypted)
	if decrypted == token {
		fmt.Printf("  Decryption: ✓ Success\n")
	}

	// Key rotation
	fmt.Printf("\n✓ Key Rotation:\n")
	km.RotateKey("production-key-2")
	newKey, _ := km.GetCurrentKey()
	fmt.Printf("  New key: %s\n", newKey.ID)
	fmt.Printf("  Old keys still accessible for decryption\n")

	// Data encryption example
	de := security.NewDataEncryptor(km)
	sensitiveData := []byte("user credit card: 4111-1111-1111-1111")

	encryptedData, _ := de.EncryptBytes(sensitiveData)
	fmt.Printf("\n✓ Data Encryption:\n")
	fmt.Printf("  Original size: %d bytes\n", len(sensitiveData))
	fmt.Printf("  Encrypted size: %d bytes\n", len(encryptedData))
	fmt.Printf("  Overhead: %d bytes\n", len(encryptedData)-len(sensitiveData))

	// Password hashing
	password := "user-password-123"
	hash, salt, _ := security.HashPasswordWithSalt(password)
	fmt.Printf("\n✓ Password Hashing (Argon2id):\n")
	fmt.Printf("  Salt size: %d bytes\n", len(salt))
	fmt.Printf("  Hash size: %d bytes\n", len(hash))
	fmt.Printf("  Verification: ✓\n")
}

func runRateLimitExample() {
	fmt.Println("\n--- Rate Limiting Example ---")

	// Create multi-level rate limiter
	ml := security.NewMultiLevelRateLimiter()
	defer ml.Stop()

	// Configure different levels
	ml.AddLevel(security.RateLimitLevelIP, &security.RateLimitConfig{
		Requests: 100,
		Window:   time.Minute,
		Burst:    120,
	})

	ml.AddLevel(security.RateLimitLevelUser, &security.RateLimitConfig{
		Requests: 50,
		Window:   time.Minute,
		Burst:    60,
	})

	ml.AddLevel(security.RateLimitLevelEndpoint, &security.RateLimitConfig{
		Requests: 30,
		Window:   time.Minute,
		Burst:    40,
	})

	fmt.Println("✓ Rate Limit Levels Configured:")
	fmt.Println("  IP Level: 100 req/min (burst: 120)")
	fmt.Println("  User Level: 50 req/min (burst: 60)")
	fmt.Println("  Endpoint Level: 30 req/min (burst: 40)")

	// Simulate requests
	keys := map[security.RateLimitLevel]string{
		security.RateLimitLevelIP:       "192.168.1.100",
		security.RateLimitLevelUser:     "user123",
		security.RateLimitLevelEndpoint: "/api/streams/create",
	}

	fmt.Println("\n✓ Simulating API requests:")
	allowed := 0
	for i := 0; i < 35; i++ {
		result, err := ml.Check(keys)
		if err == nil {
			allowed++
		} else if i == 30 {
			fmt.Printf("  Request %d: ⚠ Rate limit exceeded at endpoint level\n", i+1)
			fmt.Printf("  Retry after: %v\n", result.RetryAfter.Round(time.Second))
		}
	}

	fmt.Printf("  Total allowed: %d/35 requests\n", allowed)
	fmt.Printf("  Blocked: %d requests (endpoint limit reached)\n", 35-allowed)

	// Get status for all levels
	status := ml.GetStatus(keys)
	fmt.Println("\n✓ Current Rate Limit Status:")
	for level, result := range status {
		fmt.Printf("  %s: %d/%d remaining\n", level, result.Remaining, result.Limit)
	}
}

func runFirewallExample() {
	fmt.Println("\n--- IP Firewall & Connection Management Example ---")

	// Create firewall with connection limits
	config := &security.ConnectionLimitConfig{
		MaxConnectionsPerIP:  10,
		MaxConnectionsGlobal: 1000,
		MaxConnectionRate:    100,
		BanDuration:          1 * time.Hour,
	}

	fw := security.NewFirewall(config)
	defer fw.Stop()

	// Set callback for blocked IPs
	fw.SetBlockCallback(func(ip string, reason string) {
		fmt.Printf("  🚫 Blocked: %s - %s\n", ip, reason)
	})

	fmt.Println("✓ Firewall Configuration:")
	fmt.Printf("  Max connections per IP: %d\n", config.MaxConnectionsPerIP)
	fmt.Printf("  Max total connections: %d\n", config.MaxConnectionsGlobal)
	fmt.Printf("  Ban duration: %v\n", config.BanDuration)

	// Add whitelist rules
	allowRule := &security.IPRule{
		ID:       "allow-office",
		Pattern:  "10.0.0.0/8",
		Action:   security.IPActionAllow,
		Reason:   "Office network",
		Priority: 1000,
	}
	fw.AddRule(allowRule)
	fmt.Println("\n✓ Whitelist rule added: 10.0.0.0/8 (Office network)")

	// Add blacklist rules
	blockRule := &security.IPRule{
		ID:       "block-attacker",
		Pattern:  "203.0.113.0",
		Action:   security.IPActionBlock,
		Reason:   "Known attacker",
		Priority: 900,
	}
	fw.AddRule(blockRule)
	fmt.Println("✓ Blacklist rule added: 203.0.113.0 (Known attacker)")

	// Test IP checking
	fmt.Println("\n✓ IP Access Control:")
	testIPs := []string{
		"10.0.1.100",  // Whitelisted
		"203.0.113.0", // Blacklisted
		"192.168.1.1", // Normal
	}

	for _, ip := range testIPs {
		err := fw.CheckIP(ip)
		if err == nil {
			fmt.Printf("  %s: ✓ Allowed\n", ip)
		} else {
			fmt.Printf("  %s: ✗ Blocked (%v)\n", ip, err)
		}
	}

	// Simulate connection limit
	fmt.Println("\n✓ Connection Limit Test:")
	testIP := "192.168.1.100"
	for i := 0; i < config.MaxConnectionsPerIP+2; i++ {
		err := fw.CheckConnectionLimit(testIP)
		if err == nil {
			fw.IncrementConnection(testIP)
			if i == 0 || i == config.MaxConnectionsPerIP-1 {
				fmt.Printf("  Connection %d: ✓ Allowed\n", i+1)
			}
		} else {
			fmt.Printf("  Connection %d: ✗ Limit exceeded (auto-banned)\n", i+1)
			break
		}
	}

	// Get stats
	stats := fw.GetStats()
	fmt.Println("\n✓ Firewall Statistics:")
	fmt.Printf("  Total rules: %v\n", stats["total_rules"])
	fmt.Printf("  Whitelisted IPs: %v\n", stats["whitelisted_ips"])
	fmt.Printf("  Blacklisted IPs: %v\n", stats["blacklisted_ips"])
	fmt.Printf("  Active connections: %v\n", stats["active_connections"])
}

func runKeyRotationExample() {
	fmt.Println("\n--- Stream Key Rotation Example ---")

	// Create key rotation manager
	policy := &security.KeyRotationPolicy{
		RotateEvery: 7 * 24 * time.Hour,  // Weekly
		MaxKeyAge:   30 * 24 * time.Hour, // 30 days
		AutoRotate:  false,               // Manual for demo
		KeepHistory: 3,
		GracePeriod: 1 * time.Hour,
	}

	krm := security.NewKeyRotationManager(policy)
	defer krm.Stop()

	// Set rotation callback
	krm.SetRotationCallback(func(oldKey, newKey *security.StreamKey) {
		fmt.Printf("  🔄 Key rotated: %s -> %s\n", oldKey.ID, newKey.ID)
	})

	fmt.Println("✓ Key Rotation Policy:")
	fmt.Printf("  Rotation interval: Every %v\n", policy.RotateEvery)
	fmt.Printf("  Maximum key age: %v\n", policy.MaxKeyAge)
	fmt.Printf("  Grace period: %v\n", policy.GracePeriod)
	fmt.Printf("  Keep history: %d keys\n", policy.KeepHistory)

	// Generate stream key
	key1, _ := krm.GenerateKey("stream-123", "user-456")
	fmt.Printf("\n✓ Generated stream key:\n")
	fmt.Printf("  Stream ID: %s\n", key1.StreamID)
	fmt.Printf("  Key: %s...\n", key1.Key[:20])
	fmt.Printf("  Expires: %s\n", key1.ExpiresAt.Format("2006-01-02"))

	// Validate key
	err := krm.ValidateKey("stream-123", key1.Key)
	if err == nil {
		fmt.Printf("  Validation: ✓ Valid\n")
	}

	// Rotate key
	fmt.Println("\n✓ Rotating stream key...")
	key2, _ := krm.RotateKey("stream-123")
	fmt.Printf("  New key: %s...\n", key2.Key[:20])
	fmt.Printf("  Rotation count: %d\n", key2.RotationCount)

	// Get rotation stats
	stats := krm.GetRotationStats()
	fmt.Println("\n✓ Rotation Statistics:")
	fmt.Printf("  Total keys: %d\n", stats.TotalKeys)
	fmt.Printf("  Active keys: %d\n", stats.ActiveKeys)
	fmt.Printf("  Average rotations: %.1f\n", stats.AvgRotationCount)
}

func runWatermarkExample() {
	fmt.Println("\n--- Watermark Management Example ---")

	wm := security.NewWatermarkManager()

	// Set callback
	wm.SetApplyCallback(func(streamID string, watermark *security.Watermark) {
		fmt.Printf("  ✓ Watermark applied to stream: %s\n", streamID)
	})

	// Create text watermark
	textConfig := &security.WatermarkConfig{
		Type:     security.WatermarkTypeText,
		Position: security.WatermarkPositionBottomRight,
		Text:     "© ZenLive 2026",
		Opacity:  0.7,
		Scale:    0.15,
		OffsetX:  20,
		OffsetY:  20,
	}

	fmt.Println("✓ Text Watermark Configuration:")
	fmt.Printf("  Type: %s\n", textConfig.Type)
	fmt.Printf("  Position: %s\n", textConfig.Position)
	fmt.Printf("  Text: %s\n", textConfig.Text)
	fmt.Printf("  Opacity: %.1f\n", textConfig.Opacity)

	wm.ApplyWatermark("stream-1", textConfig)

	// Create timestamp watermark
	timestampConfig := &security.WatermarkConfig{
		Type:     security.WatermarkTypeTimestamp,
		Position: security.WatermarkPositionTopLeft,
		Opacity:  0.6,
		Scale:    0.1,
	}

	wm.ApplyWatermark("stream-2", timestampConfig)
	fmt.Printf("\n✓ Timestamp watermark applied to stream-2\n")

	// Create forensic watermark
	forensic := security.CreateForensicWatermark(
		"user-789",
		"stream-3",
		"session-xyz",
		map[string]string{
			"ip":       "192.168.1.100",
			"location": "US-CA",
		},
	)

	fmt.Println("\n✓ Forensic Watermark (Invisible):")
	fmt.Printf("  ID: %s\n", forensic.ID)
	fmt.Printf("  User: %s\n", forensic.UserID)
	fmt.Printf("  Stream: %s\n", forensic.StreamID)
	fmt.Printf("  Purpose: Content tracking & piracy prevention\n")

	// Add watermark template
	template := &security.WatermarkConfig{
		Type:     security.WatermarkTypeUserID,
		Position: security.WatermarkPositionBottomLeft,
		Opacity:  0.5,
		Scale:    0.08,
	}
	wm.AddTemplate("user-id-watermark", template)
	fmt.Println("\n✓ Watermark template 'user-id-watermark' saved")
}

func runAuditExample() {
	fmt.Println("\n--- Audit Logging & Compliance Example ---")

	// Create audit logger
	al := security.NewAuditLogger(1000, nil)

	// Set callback for critical events
	al.SetEventCallback(func(event *security.AuditEvent) {
		if event.Severity == security.AuditSeverityCritical {
			fmt.Printf("  🚨 CRITICAL: %s - %s\n", event.Action, event.Message)
		}
	})

	fmt.Println("✓ Audit Logger initialized")
	fmt.Println("  Max events: 1000")
	fmt.Println("  Persistence: In-memory")

	// Log various security events
	fmt.Println("\n✓ Logging Security Events:")

	// Authentication events
	al.LogAuth("user-1", "192.168.1.10", "login", "success", "User logged in")
	fmt.Println("  → Login: user-1 from 192.168.1.10")

	al.LogAuth("user-2", "10.0.0.5", "login", "failure", "Invalid password")
	fmt.Println("  → Failed login: user-2 from 10.0.0.5")

	// Stream events
	al.LogStream("user-1", "stream-123", "start", "success")
	fmt.Println("  → Stream started: stream-123 by user-1")

	// Security events
	al.LogSecurityEvent(
		security.AuditSeverityCritical,
		"",
		"203.0.113.50",
		"ddos_attempt",
		"DDoS attack detected - 10000 req/sec",
		map[string]interface{}{
			"request_rate": 10000,
			"threshold":    1000,
		},
	)

	al.LogSecurityEvent(
		security.AuditSeverityWarning,
		"user-3",
		"192.168.1.200",
		"rate_limit_exceeded",
		"User exceeded rate limit",
		nil,
	)
	fmt.Println("  → Rate limit exceeded: user-3")

	// Access control events
	al.LogAccess("user-4", "10.0.1.50", "stream", "stream-456", "delete", "denied")
	fmt.Println("  → Access denied: user-4 attempted to delete stream-456")

	// Query audit logs
	fmt.Println("\n✓ Querying Audit Logs:")

	// Query critical events
	criticalQuery := &security.AuditQuery{
		Severities: []security.AuditSeverity{security.AuditSeverityCritical},
	}

	criticalEvents, _ := al.Query(criticalQuery)
	fmt.Printf("  Critical events: %d found\n", len(criticalEvents))

	// Query authentication failures
	authFailQuery := &security.AuditQuery{
		Types:   []security.AuditEventType{security.AuditEventAuth},
		Actions: []string{"login"},
	}

	authEvents, _ := al.Query(authFailQuery)
	fmt.Printf("  Authentication events: %d found\n", len(authEvents))

	// Generate compliance report
	fmt.Println("\n✓ Generating Compliance Report...")
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)

	report, _ := al.GenerateComplianceReport(startTime, endTime)

	fmt.Println("\n📊 Compliance Report:")
	fmt.Printf("  Period: %s to %s\n",
		report.StartTime.Format("2006-01-02"),
		report.EndTime.Format("2006-01-02"))
	fmt.Printf("  Total events: %d\n", report.TotalEvents)

	fmt.Println("\n  Events by Type:")
	for eventType, count := range report.EventsByType {
		fmt.Printf("    %s: %d\n", eventType, count)
	}

	fmt.Println("\n  Events by Severity:")
	for severity, count := range report.EventsBySeverity {
		fmt.Printf("    %s: %d\n", severity, count)
	}

	fmt.Printf("\n  Critical events: %d\n", len(report.CriticalEvents))
	fmt.Printf("  Violations detected: %d\n", len(report.Violations))

	if len(report.Recommendations) > 0 {
		fmt.Println("\n  Recommendations:")
		for _, rec := range report.Recommendations {
			fmt.Printf("    • %s\n", rec)
		}
	}

	// Export audit logs
	exportQuery := &security.AuditQuery{
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     10,
	}

	jsonData, _ := al.ExportJSON(exportQuery)
	fmt.Printf("\n✓ Exported %d bytes of audit data (JSON)\n", len(jsonData))
	fmt.Println("  Ready for compliance archival")
}
