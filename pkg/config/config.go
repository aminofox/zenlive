package config

import (
	"time"
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
	Host string `json:"host"`

	// Port is the server port
	Port int `json:"port"`

	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration `json:"read_timeout"`

	// WriteTimeout is the maximum duration before timing out writes
	WriteTimeout time.Duration `json:"write_timeout"`

	// MaxConnections is the maximum number of concurrent connections
	MaxConnections int `json:"max_connections"`
}

// AuthConfig holds authentication-related configuration
type AuthConfig struct {
	// JWTSecret is the secret key for JWT tokens
	JWTSecret string `json:"jwt_secret"`

	// TokenExpiration is the duration before tokens expire
	TokenExpiration time.Duration `json:"token_expiration"`

	// RefreshTokenExpiration is the duration before refresh tokens expire
	RefreshTokenExpiration time.Duration `json:"refresh_token_expiration"`

	// EnableRBAC enables role-based access control
	EnableRBAC bool `json:"enable_rbac"`
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
	Enabled bool `json:"enabled"`

	// Host is the Redis server host
	Host string `json:"host"`

	// Port is the Redis server port
	Port int `json:"port"`

	// Password is the Redis password (optional)
	Password string `json:"password"`

	// DB is the Redis database number
	DB int `json:"db"`

	// PoolSize is the maximum number of connections
	PoolSize int `json:"pool_size"`

	// MaxRetries is the maximum number of retries
	MaxRetries int `json:"max_retries"`

	// SessionTTL is the session time-to-live duration
	SessionTTL time.Duration `json:"session_ttl"`
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
			Port:           8080,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxConnections: 10000,
		},
		Auth: AuthConfig{
			JWTSecret:              "change-me-in-production",
			TokenExpiration:        24 * time.Hour,
			RefreshTokenExpiration: 7 * 24 * time.Hour,
			EnableRBAC:             true,
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
