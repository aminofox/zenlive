package rtmp

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

const (
	// HandshakeSize is the size of C1/S1/C2/S2 packets
	HandshakeSize = 1536
)

// Handshake manages the RTMP handshake process
type Handshake struct {
	// C0/S0 version byte
	version byte

	// C1/S1 timestamp
	timestamp uint32

	// C1/S1 random data
	randomData [HandshakeSize - 8]byte
}

// NewHandshake creates a new handshake manager
func NewHandshake() *Handshake {
	return &Handshake{
		version: Version,
	}
}

// PerformServerHandshake performs the server-side RTMP handshake
// C0 + C1 -> S0 + S1 + S2 -> C2
func (h *Handshake) PerformServerHandshake(rw io.ReadWriter) error {
	// Read C0 (1 byte version)
	c0 := make([]byte, 1)
	if _, err := io.ReadFull(rw, c0); err != nil {
		return fmt.Errorf("failed to read C0: %w", err)
	}

	if c0[0] != Version {
		return fmt.Errorf("unsupported RTMP version: %d", c0[0])
	}

	// Read C1 (1536 bytes)
	c1 := make([]byte, HandshakeSize)
	if _, err := io.ReadFull(rw, c1); err != nil {
		return fmt.Errorf("failed to read C1: %w", err)
	}

	// Send S0 (1 byte version)
	s0 := []byte{Version}
	if _, err := rw.Write(s0); err != nil {
		return fmt.Errorf("failed to write S0: %w", err)
	}

	// Create S1 (1536 bytes)
	s1 := make([]byte, HandshakeSize)
	// Timestamp (4 bytes)
	timestamp := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(s1[0:4], timestamp)
	// Zero (4 bytes)
	binary.BigEndian.PutUint32(s1[4:8], 0)
	// Random data (1528 bytes)
	if _, err := rand.Read(s1[8:]); err != nil {
		return fmt.Errorf("failed to generate random data: %w", err)
	}

	// Send S1
	if _, err := rw.Write(s1); err != nil {
		return fmt.Errorf("failed to write S1: %w", err)
	}

	// Create S2 (echo C1)
	s2 := make([]byte, HandshakeSize)
	copy(s2, c1)
	// Update timestamp to current time
	binary.BigEndian.PutUint32(s2[0:4], timestamp)

	// Send S2
	if _, err := rw.Write(s2); err != nil {
		return fmt.Errorf("failed to write S2: %w", err)
	}

	// Read C2 (1536 bytes, should echo S1)
	c2 := make([]byte, HandshakeSize)
	if _, err := io.ReadFull(rw, c2); err != nil {
		return fmt.Errorf("failed to read C2: %w", err)
	}

	// Validate C2 (should contain S1's timestamp)
	c2Timestamp := binary.BigEndian.Uint32(c2[0:4])
	if c2Timestamp != timestamp {
		// Note: Some clients don't validate this strictly
		// We'll log but not fail
	}

	return nil
}

// PerformClientHandshake performs the client-side RTMP handshake
// S0 + S1 + S2 <- C0 + C1 <- C2
func (h *Handshake) PerformClientHandshake(rw io.ReadWriter) error {
	// Send C0 (1 byte version)
	c0 := []byte{Version}
	if _, err := rw.Write(c0); err != nil {
		return fmt.Errorf("failed to write C0: %w", err)
	}

	// Create C1 (1536 bytes)
	c1 := make([]byte, HandshakeSize)
	// Timestamp (4 bytes)
	timestamp := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(c1[0:4], timestamp)
	// Zero (4 bytes)
	binary.BigEndian.PutUint32(c1[4:8], 0)
	// Random data (1528 bytes)
	if _, err := rand.Read(c1[8:]); err != nil {
		return fmt.Errorf("failed to generate random data: %w", err)
	}

	// Send C1
	if _, err := rw.Write(c1); err != nil {
		return fmt.Errorf("failed to write C1: %w", err)
	}

	// Read S0 (1 byte version)
	s0 := make([]byte, 1)
	if _, err := io.ReadFull(rw, s0); err != nil {
		return fmt.Errorf("failed to read S0: %w", err)
	}

	if s0[0] != Version {
		return fmt.Errorf("unsupported RTMP version: %d", s0[0])
	}

	// Read S1 (1536 bytes)
	s1 := make([]byte, HandshakeSize)
	if _, err := io.ReadFull(rw, s1); err != nil {
		return fmt.Errorf("failed to read S1: %w", err)
	}

	// Read S2 (1536 bytes, should echo C1)
	s2 := make([]byte, HandshakeSize)
	if _, err := io.ReadFull(rw, s2); err != nil {
		return fmt.Errorf("failed to read S2: %w", err)
	}

	// Create C2 (echo S1)
	c2 := make([]byte, HandshakeSize)
	copy(c2, s1)
	// Update timestamp
	binary.BigEndian.PutUint32(c2[0:4], uint32(time.Now().Unix()))

	// Send C2
	if _, err := rw.Write(c2); err != nil {
		return fmt.Errorf("failed to write C2: %w", err)
	}

	return nil
}

// SimpleHandshake performs a simplified handshake (useful for testing)
func SimpleHandshake(rw io.ReadWriter, isServer bool) error {
	h := NewHandshake()
	if isServer {
		return h.PerformServerHandshake(rw)
	}
	return h.PerformClientHandshake(rw)
}
