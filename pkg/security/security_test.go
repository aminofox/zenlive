package security

import (
	"os"
	"testing"
	"time"
)

// TestCertificateManager tests certificate management
func TestCertificateManager_LoadCertificate(t *testing.T) {
	// Generate test certificate
	certFile := "test_cert.pem"
	keyFile := "test_key.pem"
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	err := GenerateSelfSignedCert(certFile, keyFile, []string{"localhost", "127.0.0.1"}, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate certificate: %v", err)
	}

	// Create certificate manager
	config := &TLSConfig{
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	cm, err := NewCertificateManager(config)
	if err != nil {
		t.Fatalf("Failed to create certificate manager: %v", err)
	}

	// Verify certificate is loaded
	cert := cm.GetCertificate()
	if cert == nil {
		t.Fatal("Certificate not loaded")
	}

	// Get certificate info
	info, err := cm.GetCertificateInfo()
	if err != nil {
		t.Fatalf("Failed to get certificate info: %v", err)
	}

	if len(info.DNSNames) == 0 {
		t.Error("Expected DNS names in certificate")
	}
}

func TestCertificateManager_GetTLSConfig(t *testing.T) {
	config := DefaultTLSConfig()
	cm, _ := NewCertificateManager(config)

	tlsConfig := cm.GetTLSConfig()
	if tlsConfig == nil {
		t.Fatal("TLS config is nil")
	}

	if tlsConfig.MinVersion != config.MinVersion {
		t.Errorf("Expected MinVersion %d, got %d", config.MinVersion, tlsConfig.MinVersion)
	}
}

// TestKeyManager tests encryption key management
func TestKeyManager_GenerateKey(t *testing.T) {
	km := NewKeyManager(32)

	key, err := km.GenerateKey("key1")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	if len(key.Key) != 32 {
		t.Errorf("Expected key size 32, got %d", len(key.Key))
	}

	if key.Algorithm != "AES-256-GCM" {
		t.Errorf("Expected AES-256-GCM, got %s", key.Algorithm)
	}
}

func TestKeyManager_RotateKey(t *testing.T) {
	km := NewKeyManager(32)

	// Generate initial key
	km.GenerateKey("key1")

	// Rotate to new key
	err := km.RotateKey("key2")
	if err != nil {
		t.Fatalf("Failed to rotate key: %v", err)
	}

	// Verify current key is key2
	currentKey, _ := km.GetCurrentKey()
	if currentKey.ID != "key2" {
		t.Errorf("Expected current key to be key2, got %s", currentKey.ID)
	}

	// Old key should still be accessible
	oldKey, err := km.GetKey("key1")
	if err != nil {
		t.Error("Old key should still be accessible")
	}
	if oldKey.ID != "key1" {
		t.Errorf("Expected old key ID key1, got %s", oldKey.ID)
	}
}

// TestTokenEncryptor tests token encryption/decryption
func TestTokenEncryptor_EncryptDecrypt(t *testing.T) {
	km := NewKeyManager(32)
	km.GenerateKey("test-key")

	te := NewTokenEncryptor(km)

	plaintext := "secret-token-12345"

	// Encrypt
	ciphertext, err := te.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if ciphertext == plaintext {
		t.Error("Ciphertext should not equal plaintext")
	}

	// Decrypt
	decrypted, err := te.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Expected %s, got %s", plaintext, decrypted)
	}
}

func TestTokenEncryptor_DecryptWithRotatedKey(t *testing.T) {
	km := NewKeyManager(32)
	km.GenerateKey("key1")

	te := NewTokenEncryptor(km)

	// Encrypt with key1
	ciphertext, _ := te.Encrypt("test-data")

	// Rotate to key2
	km.RotateKey("key2")

	// Should still be able to decrypt with old key
	decrypted, err := te.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt after key rotation: %v", err)
	}

	if decrypted != "test-data" {
		t.Error("Decryption failed after key rotation")
	}
}

// TestDataEncryptor tests data encryption
func TestDataEncryptor_EncryptDecryptBytes(t *testing.T) {
	km := NewKeyManager(32)
	km.GenerateKey("test-key")

	de := NewDataEncryptor(km)

	plaintext := []byte("sensitive data 123")

	// Encrypt
	ciphertext, err := de.EncryptBytes(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Decrypt
	decrypted, err := de.DecryptBytes(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Expected %s, got %s", plaintext, decrypted)
	}
}

// TestHashPassword tests password hashing
func TestHashPassword(t *testing.T) {
	password := []byte("my-password-123")
	salt := []byte("random-salt-data")

	hash1 := HashPassword(password, salt)
	hash2 := HashPassword(password, salt)

	if string(hash1) != string(hash2) {
		t.Error("Same password+salt should produce same hash")
	}

	// Different password should produce different hash
	hash3 := HashPassword([]byte("different-password"), salt)
	if string(hash1) == string(hash3) {
		t.Error("Different passwords should produce different hashes")
	}
}

// TestRateLimiter tests rate limiting
func TestRateLimiter_Allow(t *testing.T) {
	config := &RateLimitConfig{
		Requests: 5,
		Window:   time.Second,
		Burst:    5,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	key := "test-user"

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		if !rl.Allow(key) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	if rl.Allow(key) {
		t.Error("6th request should be denied")
	}
}

func TestRateLimiter_GetStatus(t *testing.T) {
	config := &RateLimitConfig{
		Requests: 10,
		Window:   time.Second,
		Burst:    10,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	key := "test-user"

	// Use 3 tokens
	for i := 0; i < 3; i++ {
		rl.Allow(key)
	}

	status := rl.GetStatus(key)
	if status.Remaining != 7 {
		t.Errorf("Expected 7 remaining, got %d", status.Remaining)
	}

	if status.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", status.Limit)
	}
}

// TestMultiLevelRateLimiter tests multi-level rate limiting
func TestMultiLevelRateLimiter_Check(t *testing.T) {
	ml := NewMultiLevelRateLimiter()
	defer ml.Stop()

	// Add levels
	ml.AddLevel(RateLimitLevelIP, &RateLimitConfig{
		Requests: 10,
		Window:   time.Second,
		Burst:    10,
	})

	ml.AddLevel(RateLimitLevelUser, &RateLimitConfig{
		Requests: 5,
		Window:   time.Second,
		Burst:    5,
	})

	keys := map[RateLimitLevel]string{
		RateLimitLevelIP:   "192.168.1.1",
		RateLimitLevelUser: "user1",
	}

	// First 5 requests should pass
	for i := 0; i < 5; i++ {
		result, err := ml.Check(keys)
		if err != nil {
			t.Errorf("Request %d should be allowed: %v", i+1, err)
		}
		if !result.Allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 6th request should fail (user limit reached)
	_, err := ml.Check(keys)
	if err == nil {
		t.Error("Expected rate limit error")
	}
}

// TestFirewall tests IP firewall
func TestFirewall_CheckIP(t *testing.T) {
	fw := NewFirewall(nil)
	defer fw.Stop()

	// Add block rule
	rule := &IPRule{
		ID:       "block1",
		Pattern:  "192.168.1.100",
		Action:   IPActionBlock,
		Reason:   "Test block",
		Priority: 100,
	}

	fw.AddRule(rule)

	// Check blocked IP
	err := fw.CheckIP("192.168.1.100")
	if err != ErrIPBlocked {
		t.Error("IP should be blocked")
	}

	// Check allowed IP
	err = fw.CheckIP("192.168.1.101")
	if err != nil {
		t.Error("IP should be allowed")
	}
}

func TestFirewall_ConnectionLimit(t *testing.T) {
	config := &ConnectionLimitConfig{
		MaxConnectionsPerIP:  2,
		MaxConnectionsGlobal: 10,
		BanDuration:          1 * time.Hour,
	}

	fw := NewFirewall(config)
	defer fw.Stop()

	ip := "192.168.1.1"

	// First 2 connections should be allowed
	for i := 0; i < 2; i++ {
		err := fw.CheckConnectionLimit(ip)
		if err != nil {
			t.Errorf("Connection %d should be allowed", i+1)
		}
		fw.IncrementConnection(ip)
	}

	// 3rd connection should be denied
	err := fw.CheckConnectionLimit(ip)
	if err != ErrConnectionLimitExceeded {
		t.Error("3rd connection should be denied")
	}
}

func TestFirewall_BanIP(t *testing.T) {
	fw := NewFirewall(nil)
	defer fw.Stop()

	ip := "10.0.0.1"

	// Initially allowed
	err := fw.CheckIP(ip)
	if err != nil {
		t.Error("IP should be allowed initially")
	}

	// Ban IP
	fw.BanIP(ip, "Test ban", 0)

	// Should be blocked
	err = fw.CheckIP(ip)
	if err != ErrIPBlocked {
		t.Error("IP should be blocked after ban")
	}

	// Unban
	fw.UnbanIP(ip)

	// Should be allowed again
	err = fw.CheckIP(ip)
	if err != nil {
		t.Error("IP should be allowed after unban")
	}
}

// TestKeyRotationManager tests stream key rotation
func TestKeyRotationManager_GenerateKey(t *testing.T) {
	policy := DefaultRotationPolicy()
	krm := NewKeyRotationManager(policy)
	defer krm.Stop()

	key, err := krm.GenerateKey("stream1", "user1")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	if key.StreamID != "stream1" {
		t.Errorf("Expected stream1, got %s", key.StreamID)
	}

	if !key.IsActive {
		t.Error("Key should be active")
	}
}

func TestKeyRotationManager_RotateKey(t *testing.T) {
	policy := DefaultRotationPolicy()
	krm := NewKeyRotationManager(policy)
	defer krm.Stop()

	// Generate initial key
	oldKey, _ := krm.GenerateKey("stream1", "user1")
	oldKeyString := oldKey.Key

	// Rotate
	newKey, err := krm.RotateKey("stream1")
	if err != nil {
		t.Fatalf("Failed to rotate key: %v", err)
	}

	if newKey.Key == oldKeyString {
		t.Error("New key should be different from old key")
	}

	if newKey.RotationCount != 1 {
		t.Errorf("Expected rotation count 1, got %d", newKey.RotationCount)
	}
}

func TestKeyRotationManager_ValidateKey(t *testing.T) {
	policy := DefaultRotationPolicy()
	policy.GracePeriod = 1 * time.Hour
	policy.AutoRotate = false // Disable auto-rotation for test
	krm := NewKeyRotationManager(policy)
	defer krm.Stop()

	// Generate key
	key, _ := krm.GenerateKey("stream1", "user1")

	// Validate current key
	err := krm.ValidateKey("stream1", key.Key)
	if err != nil {
		t.Errorf("Current key should be valid: %v", err)
	}

	// Rotate key
	newKey, _ := krm.RotateKey("stream1")

	// New key should be valid
	err = krm.ValidateKey("stream1", newKey.Key)
	if err != nil {
		t.Errorf("New key should be valid: %v", err)
	}

	// Invalid key should fail
	err = krm.ValidateKey("stream1", "invalid-key")
	if err == nil {
		t.Error("Invalid key should fail validation")
	}
}

// TestWatermarkManager tests watermark management
func TestWatermarkManager_ApplyWatermark(t *testing.T) {
	wm := NewWatermarkManager()

	config := &WatermarkConfig{
		Type:     WatermarkTypeText,
		Position: WatermarkPositionBottomRight,
		Text:     "ZenLive",
		Opacity:  0.5,
		Scale:    0.1,
	}

	watermark, err := wm.ApplyWatermark("stream1", config)
	if err != nil {
		t.Fatalf("Failed to apply watermark: %v", err)
	}

	if watermark.Config.Text != "ZenLive" {
		t.Errorf("Expected text 'ZenLive', got '%s'", watermark.Config.Text)
	}

	// Get watermark
	retrieved, err := wm.GetWatermark("stream1")
	if err != nil {
		t.Fatalf("Failed to get watermark: %v", err)
	}

	if retrieved.ID != watermark.ID {
		t.Error("Retrieved watermark ID mismatch")
	}
}

func TestWatermarkManager_RemoveWatermark(t *testing.T) {
	wm := NewWatermarkManager()

	config := DefaultWatermarkConfig()
	wm.ApplyWatermark("stream1", config)

	// Remove
	err := wm.RemoveWatermark("stream1")
	if err != nil {
		t.Fatalf("Failed to remove watermark: %v", err)
	}

	// Should not exist
	_, err = wm.GetWatermark("stream1")
	if err != ErrWatermarkNotFound {
		t.Error("Watermark should not exist after removal")
	}
}

// TestAuditLogger tests audit logging
func TestAuditLogger_Log(t *testing.T) {
	al := NewAuditLogger(100, nil)

	event := &AuditEvent{
		Type:     AuditEventAuth,
		Severity: AuditSeverityInfo,
		UserID:   "user1",
		Action:   "login",
		Status:   "success",
	}

	err := al.Log(event)
	if err != nil {
		t.Fatalf("Failed to log event: %v", err)
	}

	// Get recent events
	recent := al.GetRecent(10)
	if len(recent) != 1 {
		t.Errorf("Expected 1 event, got %d", len(recent))
	}

	if recent[0].Action != "login" {
		t.Errorf("Expected action 'login', got '%s'", recent[0].Action)
	}
}

func TestAuditLogger_Query(t *testing.T) {
	// Create fresh logger with new in-memory persistence for isolation
	al := NewAuditLogger(100, NewInMemoryPersistence())

	// Log multiple events
	for i := 0; i < 5; i++ {
		al.Log(&AuditEvent{
			Type:     AuditEventStream,
			Severity: AuditSeverityInfo,
			UserID:   "user1",
			Action:   "start_stream",
		})
	}

	for i := 0; i < 3; i++ {
		al.Log(&AuditEvent{
			Type:     AuditEventAuth,
			Severity: AuditSeverityWarning,
			UserID:   "user2",
			Action:   "login_failed",
		})
	}

	// Query by type
	query := &AuditQuery{
		Types: []AuditEventType{AuditEventAuth},
	}

	results, err := al.Query(query)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 events, got %d", len(results))
	}
}

func TestAuditLogger_GenerateComplianceReport(t *testing.T) {
	al := NewAuditLogger(100, nil)

	// Log some events
	al.Log(&AuditEvent{
		Type:     AuditEventSecurity,
		Severity: AuditSeverityCritical,
		Action:   "ddos_detected",
	})

	al.Log(&AuditEvent{
		Type:     AuditEventAuth,
		Severity: AuditSeverityWarning,
		Action:   "login",
		Status:   "failure",
		IP:       "10.0.0.1",
	})

	// Generate report
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)

	report, err := al.GenerateComplianceReport(startTime, endTime)
	if err != nil {
		t.Fatalf("Failed to generate report: %v", err)
	}

	if report.TotalEvents != 2 {
		t.Errorf("Expected 2 events, got %d", report.TotalEvents)
	}

	if len(report.CriticalEvents) != 1 {
		t.Errorf("Expected 1 critical event, got %d", len(report.CriticalEvents))
	}

	if len(report.Violations) == 0 {
		t.Error("Expected violations to be reported")
	}
}
