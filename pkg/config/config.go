package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration for the ZenLive SDK
type Config struct {
	// Server configuration
	Server ServerConfig `json:"server"`

	// Authentication configuration
	Auth AuthConfig `json:"auth"`

	// Storage configuration
	Storage StorageConfig `json:"storage"`

	// Streaming configuration
	Streaming StreamingConfig `json:"streaming"`

	// Chat configuration
	Chat ChatConfig `json:"chat"`

	// Analytics configuration
	Analytics AnalyticsConfig `json:"analytics"`

	// Cluster configuration (optional - for distributed deployments)
	Cluster ClusterConfig `json:"cluster"`

	// Redis configuration (optional - required when Cluster.Enabled = true)
	Redis RedisConfig `json:"redis"`

	// Logging configuration
	Logging LoggingConfig `json:"logging"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	// Host is the server host address
	Host string `json:"host" yaml:"host"`

	// Port is the server port
	Port int `json:"port" yaml:"port"`

	// SignalingPort is the WebRTC signaling port
	SignalingPort int `json:"signaling_port" yaml:"signaling_port"`

	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`

	// WriteTimeout is the maximum duration before timing out writes
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`

	// MaxConnections is the maximum number of concurrent connections
	MaxConnections int `json:"max_connections" yaml:"max_connections"`

	// DevMode enables development mode
	DevMode bool `json:"dev_mode" yaml:"dev_mode"`
}

// AuthConfig holds authentication-related configuration
type AuthConfig struct {
	// JWTSecret is the secret key for JWT tokens
	JWTSecret string `json:"jwt_secret" yaml:"jwt_secret"`

	// TokenExpiration is the duration before tokens expire
	TokenExpiration time.Duration `json:"token_expiration" yaml:"token_expiration"`

	// RefreshTokenExpiration is the duration before refresh tokens expire
	RefreshTokenExpiration time.Duration `json:"refresh_token_expiration" yaml:"refresh_token_expiration"`

	// EnableRBAC enables role-based access control
	EnableRBAC bool `json:"enable_rbac" yaml:"enable_rbac"`

	// DefaultAPIKey is the default API key for development
	DefaultAPIKey string `json:"default_api_key" yaml:"default_api_key"`

	// DefaultSecretKey is the default secret key for development
	DefaultSecretKey string `json:"default_secret_key" yaml:"default_secret_key"`
}

// StorageConfig holds storage-related configuration
type StorageConfig struct {
	// Type is the storage backend type (local, s3, minio)
	Type string `json:"type"`

	// BasePath is the base path for local storage
	BasePath string `json:"base_path"`

	// S3 configuration
	S3 S3Config `json:"s3"`
}

// S3Config holds S3-compatible storage configuration
type S3Config struct {
	// Endpoint is the S3 endpoint URL
	Endpoint string `json:"endpoint"`

	// Region is the AWS region
	Region string `json:"region"`

	// Bucket is the S3 bucket name
	Bucket string `json:"bucket"`

	// AccessKeyID is the S3 access key
	AccessKeyID string `json:"access_key_id"`

	// SecretAccessKey is the S3 secret key
	SecretAccessKey string `json:"secret_access_key"`

	// UseSSL enables SSL/TLS
	UseSSL bool `json:"use_ssl"`
}

// StreamingConfig holds streaming-related configuration
type StreamingConfig struct {
	// EnableRTMP enables RTMP protocol support
	EnableRTMP bool `json:"enable_rtmp"`

	// EnableHLS enables HLS protocol support
	EnableHLS bool `json:"enable_hls"`

	// EnableWebRTC enables WebRTC protocol support
	EnableWebRTC bool `json:"enable_webrtc"`

	// RTMP configuration
	RTMP RTMPConfig `json:"rtmp"`

	// HLS configuration
	HLS HLSConfig `json:"hls"`

	// WebRTC configuration
	WebRTC WebRTCConfig `json:"webrtc"`
}

// RTMPConfig holds RTMP-specific configuration
type RTMPConfig struct {
	// Port is the RTMP server port
	Port int `json:"port"`

	// ChunkSize is the RTMP chunk size
	ChunkSize int `json:"chunk_size"`

	// EnableSSL enables RTMPS
	EnableSSL bool `json:"enable_ssl"`
}

// HLSConfig holds HLS-specific configuration
type HLSConfig struct {
	// SegmentDuration is the duration of each HLS segment
	SegmentDuration time.Duration `json:"segment_duration"`

	// PlaylistLength is the number of segments in the playlist
	PlaylistLength int `json:"playlist_length"`

	// EnableABR enables adaptive bitrate streaming
	EnableABR bool `json:"enable_abr"`
}

// WebRTCConfig holds WebRTC-specific configuration
type WebRTCConfig struct {
	// STUNServers is the list of STUN server URLs
	STUNServers []string `json:"stun_servers"`

	// TURNServers is the list of TURN server configurations
	TURNServers []TURNServer `json:"turn_servers"`
}

// TURNServer represents a TURN server configuration
type TURNServer struct {
	// URLs are the TURN server URLs
	URLs []string `json:"urls"`

	// Username for TURN authentication
	Username string `json:"username"`

	// Credential for TURN authentication
	Credential string `json:"credential"`
}

// ChatConfig holds chat-related configuration
// Note: Chat is optional. For video/audio calls, it's just an additional action.
// The SDK provides real-time chat delivery only. Users are responsible for
// persisting chat history to their own database if needed.
type ChatConfig struct {
	// Enabled enables the chat feature
	Enabled bool `json:"enabled"`

	// MaxMessageLength is the maximum length of a chat message
	MaxMessageLength int `json:"max_message_length"`

	// RateLimitPerSecond is the maximum messages per second per user
	RateLimitPerSecond int `json:"rate_limit_per_second"`

	// EnablePersistence enables in-memory message history (not database persistence)
	// Set to false for video/audio calls where chat history is not needed
	EnablePersistence bool `json:"enable_persistence"`
}

// AnalyticsConfig holds analytics-related configuration
type AnalyticsConfig struct {
	// Enabled enables analytics collection
	Enabled bool `json:"enabled"`

	// EnablePrometheus enables Prometheus metrics
	EnablePrometheus bool `json:"enable_prometheus"`

	// PrometheusPort is the Prometheus metrics port
	PrometheusPort int `json:"prometheus_port"`
}

// ClusterConfig holds cluster-related configuration (optional)
type ClusterConfig struct {
	// Enabled enables cluster mode for distributed deployments
	Enabled bool `json:"enabled"`

	// NodeID is the unique identifier for this node
	NodeID string `json:"node_id"`

	// DiscoveryType is the service discovery method (inmemory, consul, etcd)
	DiscoveryType string `json:"discovery_type"`

	// VirtualNodes is the number of virtual nodes for consistent hashing
	VirtualNodes int `json:"virtual_nodes"`
}

// RedisConfig holds Redis configuration
// Required when Cluster.Enabled = true for distributed session management
type RedisConfig struct {
	// Enabled enables Redis for distributed sessions
	// Must be true when Cluster.Enabled = true
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Address is the Redis server address (host:port)
	Address string `json:"address" yaml:"address"`

	// Host is the Redis server host
	Host string `json:"host" yaml:"host"`

	// Port is the Redis server port
	Port int `json:"port" yaml:"port"`

	// Password is the Redis password (optional)
	Password string `json:"password" yaml:"password"`

	// DB is the Redis database number
	DB int `json:"db" yaml:"db"`

	// PoolSize is the maximum number of connections
	PoolSize int `json:"pool_size" yaml:"pool_size"`

	// MaxRetries is the maximum number of retries
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// SessionTTL is the session time-to-live duration
	SessionTTL time.Duration `json:"session_ttl" yaml:"session_ttl"`
}

// LoggingConfig holds logging-related configuration
type LoggingConfig struct {
	// Level is the logging level (debug, info, warn, error)
	Level string `json:"level"`

	// Format is the log format (json, text)
	Format string `json:"format"`

	// OutputPath is the log output path
	OutputPath string `json:"output_path"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:           "0.0.0.0",
			Port:           7880,
			SignalingPort:  7881,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxConnections: 10000,
			DevMode:        false,
		},
		Auth: AuthConfig{
			JWTSecret:              "change-me-in-production",
			TokenExpiration:        24 * time.Hour,
			RefreshTokenExpiration: 7 * 24 * time.Hour,
			EnableRBAC:             true,
			DefaultAPIKey:          "",
			DefaultSecretKey:       "",
		},
		Storage: StorageConfig{
			Type:     "local",
			BasePath: "./storage",
		},
		Streaming: StreamingConfig{
			EnableRTMP:   true,
			EnableHLS:    true,
			EnableWebRTC: true,
			RTMP: RTMPConfig{
				Port:      1935,
				ChunkSize: 4096,
				EnableSSL: false,
			},
			HLS: HLSConfig{
				SegmentDuration: 6 * time.Second,
				PlaylistLength:  5,
				EnableABR:       true,
			},
			WebRTC: WebRTCConfig{
				STUNServers: []string{
					"stun:stun.l.google.com:19302",
				},
			},
		},
		Chat: ChatConfig{
			Enabled:            true, // Enable for livestream, disable for simple calls
			MaxMessageLength:   500,
			RateLimitPerSecond: 5,
			EnablePersistence:  false, // In-memory only, users handle DB persistence
		},
		Analytics: AnalyticsConfig{
			Enabled:          false, // Optional - disable by default
			EnablePrometheus: false,
			PrometheusPort:   9090,
		},
		Cluster: ClusterConfig{
			Enabled:       false, // Optional - disable by default for single-node deployments
			NodeID:        "",
			DiscoveryType: "inmemory",
			VirtualNodes:  150,
		},
		Redis: RedisConfig{
			Enabled:    false, // Only needed for cluster mode
			Address:    "localhost:6379",
			Host:       "localhost",
			Port:       6379,
			Password:   "",
			DB:         0,
			PoolSize:   10,
			MaxRetries: 3,
			SessionTTL: 24 * time.Hour,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			OutputPath: "stdout",
		},
	}
}

// Load loads configuration from a YAML file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override from environment variables
	cfg.loadFromEnv()

	return cfg, nil
}

// loadFromEnv overrides config from environment variables
func (c *Config) loadFromEnv() {
	if host := os.Getenv("ZENLIVE_HOST"); host != "" {
		c.Server.Host = host
	}
	if redisAddr := os.Getenv("REDIS_URL"); redisAddr != "" {
		c.Redis.Host = redisAddr
	}
	if redisPass := os.Getenv("REDIS_PASSWORD"); redisPass != "" {
		c.Redis.Password = redisPass
	}
}
