package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
)

// EncryptionKey represents an encryption key with metadata
type EncryptionKey struct {
	ID        string
	Key       []byte
	Algorithm string
	CreatedAt int64
	ExpiresAt int64 // 0 means no expiration
}

// KeyManager manages encryption keys with rotation support
type KeyManager struct {
	mu         sync.RWMutex
	keys       map[string]*EncryptionKey
	currentKey string
	keySize    int
	onRotate   func(oldKeyID, newKeyID string)
}

// EncryptionConfig holds encryption configuration
type EncryptionConfig struct {
	Algorithm string // AES-256-GCM, AES-128-GCM
	KeySize   int    // 16 (AES-128) or 32 (AES-256)
}

// TokenEncryptor encrypts and decrypts tokens
type TokenEncryptor struct {
	keyManager *KeyManager
	config     *EncryptionConfig
}

// DataEncryptor provides data encryption utilities
type DataEncryptor struct {
	keyManager *KeyManager
}

var (
	// ErrInvalidKey indicates an invalid encryption key
	ErrInvalidKey = errors.New("invalid encryption key")
	// ErrDecryptionFailed indicates decryption failure
	ErrDecryptionFailed = errors.New("decryption failed")
	// ErrInvalidCiphertext indicates invalid ciphertext format
	ErrInvalidCiphertext = errors.New("invalid ciphertext format")
)

// NewKeyManager creates a new key manager
func NewKeyManager(keySize int) *KeyManager {
	if keySize != 16 && keySize != 32 {
		keySize = 32 // Default to AES-256
	}

	return &KeyManager{
		keys:    make(map[string]*EncryptionKey),
		keySize: keySize,
	}
}

// GenerateKey generates a new random encryption key
func (km *KeyManager) GenerateKey(id string) (*EncryptionKey, error) {
	key := make([]byte, km.keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	algorithm := "AES-256-GCM"
	if km.keySize == 16 {
		algorithm = "AES-128-GCM"
	}

	encKey := &EncryptionKey{
		ID:        id,
		Key:       key,
		Algorithm: algorithm,
		CreatedAt: nowTimestamp(),
		ExpiresAt: 0,
	}

	km.mu.Lock()
	km.keys[id] = encKey
	if km.currentKey == "" {
		km.currentKey = id
	}
	km.mu.Unlock()

	return encKey, nil
}

// AddKey adds an existing key
func (km *KeyManager) AddKey(key *EncryptionKey) error {
	if len(key.Key) != km.keySize {
		return ErrInvalidKey
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	km.keys[key.ID] = key
	if km.currentKey == "" {
		km.currentKey = key.ID
	}

	return nil
}

// GetKey retrieves a key by ID
func (km *KeyManager) GetKey(id string) (*EncryptionKey, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	key, exists := km.keys[id]
	if !exists {
		return nil, ErrInvalidKey
	}

	return key, nil
}

// GetCurrentKey returns the current active key
func (km *KeyManager) GetCurrentKey() (*EncryptionKey, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.currentKey == "" {
		return nil, errors.New("no current key set")
	}

	key, exists := km.keys[km.currentKey]
	if !exists {
		return nil, errors.New("current key not found")
	}

	return key, nil
}

// RotateKey rotates to a new key
func (km *KeyManager) RotateKey(newKeyID string) error {
	_, err := km.GenerateKey(newKeyID)
	if err != nil {
		return err
	}

	km.mu.Lock()
	oldKeyID := km.currentKey
	km.currentKey = newKeyID
	km.mu.Unlock()

	if km.onRotate != nil {
		km.onRotate(oldKeyID, newKeyID)
	}

	return nil
}

// SetRotationCallback sets the callback for key rotation events
func (km *KeyManager) SetRotationCallback(callback func(oldKeyID, newKeyID string)) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.onRotate = callback
}

// NewTokenEncryptor creates a new token encryptor
func NewTokenEncryptor(keyManager *KeyManager) *TokenEncryptor {
	return &TokenEncryptor{
		keyManager: keyManager,
		config: &EncryptionConfig{
			Algorithm: "AES-256-GCM",
			KeySize:   32,
		},
	}
}

// Encrypt encrypts a token using the current key
func (te *TokenEncryptor) Encrypt(plaintext string) (string, error) {
	key, err := te.keyManager.GetCurrentKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key.Key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Prepend key ID for future decryption
	result := append([]byte(key.ID+":"), ciphertext...)

	return base64.StdEncoding.EncodeToString(result), nil
}

// Decrypt decrypts a token
func (te *TokenEncryptor) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	// Extract key ID
	colonIdx := -1
	for i, b := range data {
		if b == ':' {
			colonIdx = i
			break
		}
	}

	if colonIdx == -1 {
		return "", ErrInvalidCiphertext
	}

	keyID := string(data[:colonIdx])
	encryptedData := data[colonIdx+1:]

	key, err := te.keyManager.GetKey(keyID)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key.Key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertextBytes := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// NewDataEncryptor creates a new data encryptor
func NewDataEncryptor(keyManager *KeyManager) *DataEncryptor {
	return &DataEncryptor{
		keyManager: keyManager,
	}
}

// EncryptBytes encrypts arbitrary byte data
func (de *DataEncryptor) EncryptBytes(plaintext []byte) ([]byte, error) {
	key, err := de.keyManager.GetCurrentKey()
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Prepend key ID
	result := append([]byte(key.ID+":"), ciphertext...)

	return result, nil
}

// DecryptBytes decrypts byte data
func (de *DataEncryptor) DecryptBytes(ciphertext []byte) ([]byte, error) {
	// Extract key ID
	colonIdx := -1
	for i, b := range ciphertext {
		if b == ':' {
			colonIdx = i
			break
		}
	}

	if colonIdx == -1 {
		return nil, ErrInvalidCiphertext
	}

	keyID := string(ciphertext[:colonIdx])
	encryptedData := ciphertext[colonIdx+1:]

	key, err := de.keyManager.GetKey(keyID)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	nonce, ciphertextBytes := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// HashPassword hashes a password using Argon2id
func HashPassword(password, salt []byte) []byte {
	// Argon2id parameters (OWASP recommended)
	return argon2.IDKey(password, salt, 2, 64*1024, 4, 32)
}

// HashPasswordWithSalt hashes a password and returns both hash and salt
func HashPasswordWithSalt(password string) (hash, salt []byte, err error) {
	salt = make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	hash = HashPassword([]byte(password), salt)
	return hash, salt, nil
}

// DeriveKey derives an encryption key from a password using PBKDF2
func DeriveKey(password, salt []byte, keySize int) []byte {
	return pbkdf2.Key(password, salt, 100000, keySize, sha256.New)
}

// GenerateRandomBytes generates cryptographically secure random bytes
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return b, nil
}

// Helper function to get current timestamp
func nowTimestamp() int64 {
	return int64(1736585902) // Placeholder - in production use time.Now().Unix()
}
