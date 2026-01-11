# Configuration Guide

## Overview

ZenLive is highly configurable to support various deployment scenarios. This guide covers all configuration options.

## Configuration Methods

### 1. Configuration File (JSON)

Create `config.json` in your project root:

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "read_timeout": "30s",
    "write_timeout": "30s",
    "max_connections": 10000
  },
  "auth": {
    "jwt_secret": "your-secret-key-change-in-production",
    "token_expiration": "24h",
    "refresh_token_expiration": "168h",
    "enable_rbac": true
  },
  "storage": {
    "type": "local",
    "base_path": "./storage"
  },
  "streaming": {
    "enable_rtmp": true,
    "enable_hls": true,
    "enable_webrtc": true,
    "rtmp": {
      "port": 1935,
      "chunk_size": 4096,
      "enable_ssl": false
    },
    "hls": {
      "segment_duration": "6s",
      "playlist_length": 5,
      "enable_abr": true
    },
    "webrtc": {
      "stun_servers": ["stun:stun.l.google.com:19302"]
    }
  },
  "chat": {
    "enabled": true,
    "max_message_length": 500,
    "rate_limit_per_second": 5,
    "enable_persistence": false
  },
  "analytics": {
    "enabled": false,
    "enable_prometheus": false,
    "prometheus_port": 9090
  },
  "cluster": {
    "enabled": false,
    "node_id": "",
    "discovery_type": "inmemory",
    "virtual_nodes": 150
  },
  "redis": {
    "enabled": false,
    "host": "localhost",
    "port": 6379,
    "password": "",
    "db": 0,
    "pool_size": 10,
    "max_retries": 3,
    "session_ttl": "24h"
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output_path": "stdout"
  }
}
```

Load configuration in your code:

```go
import (
    "encoding/json"
    "os"
    "github.com/aminofox/zenlive/pkg/config"
)

// Option 1: Use defaults
cfg := config.DefaultConfig()

// Option 2: Load from JSON file
file, _ := os.Open("config.json")
var cfg config.Config
json.NewDecoder(file).Decode(&cfg)
```

### 2. Programmatic Configuration

Configure in code:

```go
package main

import "github.com/aminofox/zenlive/pkg/config"

func main() {
    cfg := &config.Config{
        RTMP: config.RTMPConfig{
            Port:           1935,
            EnableAuth:     true,
            MaxConnections: 1000,
        },
        HLS: config.HLSConfig{
            Port:            8080,
            SegmentDuration: 4,
            PlaylistSize:    5,
        },
        // ... more configuration
    }
    
    // Use configuration
}
```

## Configuration Sections

### Server Configuration

```go
type ServerConfig struct {
    Host           string        `json:"host"`
    Port           int           `json:"port"`
    ReadTimeout    time.Duration `json:"read_timeout"`
    WriteTimeout   time.Duration `json:"write_timeout"`
    MaxConnections int           `json:"max_connections"`
}
```

**Key Options**:
- `host`: Server listen address (default: "0.0.0.0")
- `port`: Server port (default: 8080)
- `read_timeout`: Request read timeout (default: 30s)
- `write_timeout`: Response write timeout (default: 30s)
- `max_connections`: Maximum concurrent connections (default: 10000)

### RTMP Configuration

```go
type RTMPConfig struct {
    Port      int  `json:"port"`
    ChunkSize int  `json:"chunk_size"`
    EnableSSL bool `json:"enable_ssl"`
}
```

**Key Options**:
- `port`: RTMP listening port (default: 1935)
- `chunk_size`: RTMP chunk size (default: 4096)
- `enable_ssl`: Enable RTMPS (default: false)

### HLS Configuration

```go
type HLSConfig struct {
    SegmentDuration time.Duration `json:"segment_duration"`
    PlaylistLength  int           `json:"playlist_length"`
    EnableABR       bool          `json:"enable_abr"`
}
```

**Key Options**:
- `segment_duration`: Length of each TS segment (default: 6s)
- `playlist_length`: Number of segments in playlist (default: 5)
- `enable_abr`: Enable adaptive bitrate streaming (default: true)

### WebRTC Configuration

```go
type WebRTCConfig struct {
    STUNServers []string     `json:"stun_servers"`
    TURNServers []TURNServer `json:"turn_servers"`
}

type TURNServer struct {
    URLs       []string `json:"urls"`
    Username   string   `json:"username"`
    Credential string   `json:"credential"`
}
```

**Example Configuration**:
```json
{
  "webrtc": {
    "stun_servers": [
      "stun:stun.l.google.com:19302",
      "stun:stun1.l.google.com:19302"
    ],
    "turn_servers": [
      {
        "urls": ["turn:turn.example.com:3478"],
        "username": "user",
        "credential": "password"
      }
    ]
  }
}
```

### Authentication Configuration

```go
type AuthConfig struct {
    JWTSecret              string        `json:"jwt_secret"`
    TokenExpiration        time.Duration `json:"token_expiration"`
    RefreshTokenExpiration time.Duration `json:"refresh_token_expiration"`
    EnableRBAC             bool          `json:"enable_rbac"`
}
```

**Key Options**:
- `jwt_secret`: Secret key for JWT tokens (CHANGE IN PRODUCTION!)
- `token_expiration`: Token lifetime (default: 24h)
- `refresh_token_expiration`: Refresh token lifetime (default: 168h/7 days)
- `enable_rbac`: Enable role-based access control (default: true)

**Example JWT Secret Generation**:
```bash
openssl rand -base64 32
```

### Storage Configuration

```go
type StorageConfig struct {
    Type     string   `json:"type"`      // "local" or "s3"
    BasePath string   `json:"base_path"` // for local storage
    S3       S3Config `json:"s3"`        // for S3 storage
}

type S3Config struct {
    Endpoint        string `json:"endpoint"`
    Region          string `json:"region"`
    Bucket          string `json:"bucket"`
    AccessKeyID     string `json:"access_key_id"`
    SecretAccessKey string `json:"secret_access_key"`
    UseSSL          bool   `json:"use_ssl"`
}
```

**Local Storage Example**:
```json
{
  "storage": {
    "type": "local",
    "base_path": "./storage"
  }
}
```

**S3 Storage Example**:
```json
{
  "storage": {
    "type": "s3",
    "s3": {
      "endpoint": "https://s3.amazonaws.com",
      "region": "us-east-1",
      "bucket": "my-bucket",
      "access_key_id": "YOUR_KEY",
      "secret_access_key": "YOUR_SECRET",
      "use_ssl": true
    }
  }
}
```

### Redis Configuration

**Note**: Redis is only required when `Cluster.Enabled = true` for distributed session management.

```go
type RedisConfig struct {
    Enabled    bool          `json:"enabled"`     // Must be true when Cluster.Enabled = true
    Host       string        `json:"host"`
    Port       int           `json:"port"`
    Password   string        `json:"password"`    // optional
    DB         int           `json:"db"`
    PoolSize   int           `json:"pool_size"`
    MaxRetries int           `json:"max_retries"`
    SessionTTL time.Duration `json:"session_ttl"`
}
```

**Example**:
```json
{
  "redis": {
    "enabled": true,
    "host": "localhost",
    "port": 6379,
    "password": "",
    "db": 0,
    "pool_size": 10,
    "max_retries": 3,
    "session_ttl": "24h"
  }
}
```

### Chat Configuration

**Note**: Chat is optional. For video/audio calls, it's just an additional feature. The SDK provides real-time chat delivery only. You are responsible for persisting chat history to your own database if needed.

```go
type ChatConfig struct {
    Enabled            bool `json:"enabled"`
    MaxMessageLength   int  `json:"max_message_length"`
    RateLimitPerSecond int  `json:"rate_limit_per_second"`
    EnablePersistence  bool `json:"enable_persistence"` // In-memory only, not database
}
```

**Key Options**:
- `enabled`: Enable chat feature (default: true for livestream, false for calls)
- `max_message_length`: Maximum message length (default: 500)
- `rate_limit_per_second`: Messages per second per user (default: 5)
- `enable_persistence`: In-memory history only (default: false)

### Analytics Configuration

**Note**: Analytics is optional and disabled by default.

```go
type AnalyticsConfig struct {
    Enabled          bool `json:"enabled"`
    EnablePrometheus bool `json:"enable_prometheus"`
    PrometheusPort   int  `json:"prometheus_port"`
}
```

**Key Options**:
- `enabled`: Enable analytics collection (default: false)
- `enable_prometheus`: Enable Prometheus metrics (default: false)
- `prometheus_port`: Prometheus metrics port (default: 9090)

### Cluster Configuration

**Note**: Cluster mode is optional and disabled by default for single-node deployments.

```go
type ClusterConfig struct {
    Enabled       bool   `json:"enabled"`       // Enable distributed mode
    NodeID        string `json:"node_id"`       // Unique node identifier
    DiscoveryType string `json:"discovery_type"` // "inmemory", "consul", "etcd"
    VirtualNodes  int    `json:"virtual_nodes"`  // For consistent hashing
}
```

### Logging Configuration

```go
type LoggingConfig struct {
    Level      string `json:"level"`       // "debug", "info", "warn", "error"
    Format     string `json:"format"`      // "json", "text"
    OutputPath string `json:"output_path"` // "stdout" or file path
}
```

## Example Configurations

### Development Environment

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "read_timeout": "30s",
    "write_timeout": "30s",
    "max_connections": 1000
  },
  "auth": {
    "jwt_secret": "dev-secret-change-in-prod",
    "token_expiration": "24h",
    "refresh_token_expiration": "168h",
    "enable_rbac": false
  },
  "storage": {
    "type": "local",
    "base_path": "./storage"
  },
  "streaming": {
    "enable_rtmp": true,
    "enable_hls": true,
    "enable_webrtc": true,
    "rtmp": {
      "port": 1935,
      "chunk_size": 4096,
      "enable_ssl": false
    },
    "hls": {
      "segment_duration": "6s",
      "playlist_length": 5,
      "enable_abr": false
    },
    "webrtc": {
      "stun_servers": ["stun:stun.l.google.com:19302"]
    }
  },
  "chat": {
    "enabled": true,
    "max_message_length": 500,
    "rate_limit_per_second": 10,
    "enable_persistence": false
  },
  "analytics": {
    "enabled": false
  },
  "cluster": {
    "enabled": false
  },
  "redis": {
    "enabled": false
  },
  "logging": {
    "level": "debug",
    "format": "text",
    "output_path": "stdout"
  }
}
```

### Production Environment

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "read_timeout": "30s",
    "write_timeout": "30s",
    "max_connections": 10000
  },
  "auth": {
    "jwt_secret": "YOUR-SECURE-SECRET-HERE",
    "token_expiration": "24h",
    "refresh_token_expiration": "168h",
    "enable_rbac": true
  },
  "storage": {
    "type": "s3",
    "s3": {
      "endpoint": "https://s3.amazonaws.com",
      "region": "us-east-1",
      "bucket": "my-prod-bucket",
      "access_key_id": "YOUR_KEY",
      "secret_access_key": "YOUR_SECRET",
      "use_ssl": true
    }
  },
  "streaming": {
    "enable_rtmp": true,
    "enable_hls": true,
    "enable_webrtc": true,
    "rtmp": {
      "port": 1935,
      "chunk_size": 4096,
      "enable_ssl": false
    },
    "hls": {
      "segment_duration": "6s",
      "playlist_length": 5,
      "enable_abr": true
    },
    "webrtc": {
      "stun_servers": [
        "stun:stun.l.google.com:19302",
        "stun:stun1.l.google.com:19302"
      ],
      "turn_servers": [
        {
          "urls": ["turn:turn.example.com:3478"],
          "username": "turnuser",
          "credential": "turnpassword"
        }
      ]
    }
  },
  "chat": {
    "enabled": true,
    "max_message_length": 500,
    "rate_limit_per_second": 5,
    "enable_persistence": false
  },
  "analytics": {
    "enabled": true,
    "enable_prometheus": true,
    "prometheus_port": 9090
  },
  "cluster": {
    "enabled": false
  },
  "redis": {
    "enabled": false
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output_path": "stdout"
  }
}
```

### High-Availability (Cluster Mode)

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "max_connections": 10000
  },
  "cluster": {
    "enabled": true,
    "node_id": "node-1",
    "discovery_type": "consul",
    "virtual_nodes": 150
  },
  "redis": {
    "enabled": true,
    "host": "redis.example.com",
    "port": 6379,
    "password": "YOUR_REDIS_PASSWORD",
    "db": 0,
    "pool_size": 50,
    "max_retries": 3,
    "session_ttl": "24h"
  },
  "analytics": {
    "enabled": true,
    "enable_prometheus": true,
    "prometheus_port": 9090
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output_path": "/var/log/zenlive/app.log"
  }
}
```

## Loading Configuration

### From Code (Default Config)

```go
package main

import (
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    // Use defaults
    cfg := config.DefaultConfig()
    
    // Customize if needed
    cfg.Server.Port = 8080
    cfg.Logging.Level = "debug"
    
    sdk, _ := zenlive.New(cfg)
    sdk.Start()
}
```

### From JSON File

```go
package main

import (
    "encoding/json"
    "os"
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    // Load from file
    file, err := os.Open("config.json")
    if err != nil {
        panic(err)
    }
    defer file.Close()
    
    var cfg config.Config
    if err := json.NewDecoder(file).Decode(&cfg); err != nil {
        panic(err)
    }
    
    sdk, _ := zenlive.New(&cfg)
    sdk.Start()
}
```

## Configuration Validation

Validate configuration before using:

```go
func ValidateConfig(cfg *config.Config) error {
    if cfg.Server.Port < 1024 || cfg.Server.Port > 65535 {
        return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
    }
    
    if cfg.Auth.JWTSecret == "" || cfg.Auth.JWTSecret == "change-me-in-production" {
        return fmt.Errorf("JWT secret must be set in production")
    }
    
    if cfg.Storage.Type == "s3" && cfg.Storage.S3.Bucket == "" {
        return fmt.Errorf("S3 bucket required when storage type is s3")
    }
    
    if cfg.Cluster.Enabled && !cfg.Redis.Enabled {
        return fmt.Errorf("Redis must be enabled when cluster mode is enabled")
    }
    
    return nil
}

// Usage
cfg := config.DefaultConfig()
if err := ValidateConfig(cfg); err != nil {
    log.Fatal(err)
}
```

## Best Practices

1. **Protect Secrets**: Never commit `config.json` with real secrets to git
   - Use `.gitignore` to exclude config files
   - Store secrets in environment variables or secret management systems
   - Use different configs for dev/staging/production

2. **Use Strong Secrets**: 
   ```bash
   # Generate secure JWT secret
   openssl rand -base64 32
   ```

3. **Validate on Startup**: Always validate configuration before starting server

4. **Use Defaults**: The `DefaultConfig()` provides sensible defaults for development

5. **Cluster Mode**: Only enable cluster mode and Redis when you need distributed deployment

6. **Chat Persistence**: SDK provides real-time delivery only - implement your own database persistence if needed

7. **Storage**: Start with local storage in dev, use S3 in production

## Troubleshooting

### Configuration Not Loading

```bash
# Check config file exists
ls -la config.json

# Validate JSON syntax
cat config.json | jq .
# or
python3 -m json.tool config.json
```

### Invalid Configuration

Check validation errors in logs:

```
ERROR: Configuration validation failed: JWT secret must be set in production
```

### Common Issues

1. **Invalid JSON**: Use a JSON validator to check syntax
2. **Wrong types**: Ensure durations use string format like "24h", "30s"
3. **Missing required fields**: S3 bucket required when storage type is "s3"
4. **Cluster without Redis**: Redis must be enabled when cluster mode is enabled

## Next Steps

- [Getting Started](getting-started.md)
- [Architecture](architecture.md)
- [Troubleshooting](troubleshooting.md)
