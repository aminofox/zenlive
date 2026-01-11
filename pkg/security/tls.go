package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"
)

// TLSConfig represents TLS/SSL configuration for secure connections
type TLSConfig struct {
	// CertFile is the path to the TLS certificate file
	CertFile string
	// KeyFile is the path to the TLS private key file
	KeyFile string
	// MinVersion is the minimum TLS version (default: TLS 1.2)
	MinVersion uint16
	// MaxVersion is the maximum TLS version (default: TLS 1.3)
	MaxVersion uint16
	// CipherSuites is the list of allowed cipher suites
	CipherSuites []uint16
	// InsecureSkipVerify skips certificate verification (for testing only)
	InsecureSkipVerify bool
	// ClientAuth defines the client authentication policy
	ClientAuth tls.ClientAuthType
	// RootCAs is the pool of root CAs for client verification
	RootCAs *x509.CertPool
}

// CertificateManager manages TLS certificates with auto-renewal
type CertificateManager struct {
	mu            sync.RWMutex
	cert          *tls.Certificate
	config        *TLSConfig
	expiryTime    time.Time
	autoRenew     bool
	renewBefore   time.Duration // Renew certificate this duration before expiry
	stopRenewChan chan struct{}
	onRenew       func(*tls.Certificate)
	onRenewError  func(error)
}

// CertificateInfo contains information about a certificate
type CertificateInfo struct {
	Subject      pkix.Name
	Issuer       pkix.Name
	SerialNumber *big.Int
	NotBefore    time.Time
	NotAfter     time.Time
	DNSNames     []string
	IPAddresses  []string
	IsCA         bool
}

// DefaultTLSConfig returns a secure default TLS configuration
func DefaultTLSConfig() *TLSConfig {
	return &TLSConfig{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
		ClientAuth: tls.NoClientCert,
	}
}

// NewCertificateManager creates a new certificate manager
func NewCertificateManager(config *TLSConfig) (*CertificateManager, error) {
	if config == nil {
		config = DefaultTLSConfig()
	}

	cm := &CertificateManager{
		config:        config,
		autoRenew:     false,
		renewBefore:   30 * 24 * time.Hour, // 30 days before expiry
		stopRenewChan: make(chan struct{}),
	}

	// Load certificate if paths are provided
	if config.CertFile != "" && config.KeyFile != "" {
		if err := cm.LoadCertificate(config.CertFile, config.KeyFile); err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}
	}

	return cm, nil
}

// LoadCertificate loads a certificate from files
func (cm *CertificateManager) LoadCertificate(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load key pair: %w", err)
	}

	// Parse certificate to get expiry time
	if len(cert.Certificate) > 0 {
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}
		cm.mu.Lock()
		cm.expiryTime = x509Cert.NotAfter
		cm.mu.Unlock()
	}

	cm.mu.Lock()
	cm.cert = &cert
	cm.mu.Unlock()

	return nil
}

// GetCertificate returns the current certificate (safe for concurrent use)
func (cm *CertificateManager) GetCertificate() *tls.Certificate {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cert
}

// GetTLSConfig returns a tls.Config object
func (cm *CertificateManager) GetTLSConfig() *tls.Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	tlsConfig := &tls.Config{
		MinVersion:         cm.config.MinVersion,
		MaxVersion:         cm.config.MaxVersion,
		CipherSuites:       cm.config.CipherSuites,
		InsecureSkipVerify: cm.config.InsecureSkipVerify,
		ClientAuth:         cm.config.ClientAuth,
		RootCAs:            cm.config.RootCAs,
	}

	if cm.cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cm.cert}
		// Dynamic certificate getter for auto-renewal
		tlsConfig.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cm.GetCertificate(), nil
		}
	}

	return tlsConfig
}

// GetCertificateInfo returns information about the current certificate
func (cm *CertificateManager) GetCertificateInfo() (*CertificateInfo, error) {
	cm.mu.RLock()
	cert := cm.cert
	cm.mu.RUnlock()

	if cert == nil || len(cert.Certificate) == 0 {
		return nil, errors.New("no certificate loaded")
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	var ipAddresses []string
	for _, ip := range x509Cert.IPAddresses {
		ipAddresses = append(ipAddresses, ip.String())
	}

	return &CertificateInfo{
		Subject:      x509Cert.Subject,
		Issuer:       x509Cert.Issuer,
		SerialNumber: x509Cert.SerialNumber,
		NotBefore:    x509Cert.NotBefore,
		NotAfter:     x509Cert.NotAfter,
		DNSNames:     x509Cert.DNSNames,
		IPAddresses:  ipAddresses,
		IsCA:         x509Cert.IsCA,
	}, nil
}

// ExpiresIn returns the duration until certificate expiry
func (cm *CertificateManager) ExpiresIn() time.Duration {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return time.Until(cm.expiryTime)
}

// NeedsRenewal checks if the certificate needs renewal
func (cm *CertificateManager) NeedsRenewal() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return time.Until(cm.expiryTime) < cm.renewBefore
}

// EnableAutoRenew enables automatic certificate renewal
func (cm *CertificateManager) EnableAutoRenew(renewBefore time.Duration, onRenew func(*tls.Certificate), onError func(error)) {
	cm.mu.Lock()
	cm.autoRenew = true
	cm.renewBefore = renewBefore
	cm.onRenew = onRenew
	cm.onRenewError = onError
	cm.mu.Unlock()

	go cm.renewalLoop()
}

// DisableAutoRenew disables automatic certificate renewal
func (cm *CertificateManager) DisableAutoRenew() {
	cm.mu.Lock()
	cm.autoRenew = false
	cm.mu.Unlock()
	close(cm.stopRenewChan)
}

// renewalLoop periodically checks and renews certificates
func (cm *CertificateManager) renewalLoop() {
	ticker := time.NewTicker(24 * time.Hour) // Check daily
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if cm.NeedsRenewal() {
				// In production, this would call ACME/Let's Encrypt or certificate provider
				// For now, we'll just trigger the callback
				if cm.onRenewError != nil {
					cm.onRenewError(errors.New("certificate renewal not implemented - manual renewal required"))
				}
			}
		case <-cm.stopRenewChan:
			return
		}
	}
}

// GenerateSelfSignedCert generates a self-signed certificate (for testing/development only)
func GenerateSelfSignedCert(certFile, keyFile string, hosts []string, validFor time.Duration) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"ZenLive Development"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(validFor),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	for _, h := range hosts {
		template.DNSNames = append(template.DNSNames, h)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate
	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("failed to open cert file for writing: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write private key
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return fmt.Errorf("failed to open key file for writing: %w", err)
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	return nil
}

// ValidateCertificate validates a certificate file
func ValidateCertificate(certFile string) error {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return errors.New("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check if expired
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate not yet valid (valid from: %v)", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate expired (expired on: %v)", cert.NotAfter)
	}

	return nil
}
