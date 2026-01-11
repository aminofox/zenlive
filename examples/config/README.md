# Configuration Examples

This folder contains example configuration files for different deployment scenarios.

## 🎯 Important Philosophy

**ZenLive SDK handles real-time communication only.**

- ✅ SDK delivers streams and messages in real-time
- ❌ SDK does NOT persist application data to database
- 💾 YOU are responsible for persisting data to your own database
- 🔴 Redis is ONLY for cluster mode (distributed sessions)
- 💬 Chat is real-time delivery only (you handle history storage)

**See [../../docs/SDK_PHILOSOPHY.md](../../docs/SDK_PHILOSOPHY.md) for details.**

---

## 📁 Files

### 1. `config.example.json`
Complete configuration file with all available options and default values.

**Use this for:** Understanding all configuration options

---

### 2. `livestream-simple.json`
Simple livestreaming configuration for single server deployment.

**Features:**
- ✅ RTMP ingestion
- ✅ HLS playback
- ✅ Chat enabled (real-time only)
- ❌ WebRTC disabled
- ❌ Analytics disabled
- ❌ Cluster disabled
- ❌ Redis disabled

**Note:** Chat messages are delivered in real-time only. If you need chat history, save messages to your own database.

**Use cases:**
- Small livestreaming platform
- Single server deployment
- Development/testing

**Command:**
```bash
./zenlive --config=examples/config/livestream-simple.json
```

---

### 3. `video-call.json`
Configuration for 1-1 video/audio calling.

**Features:**
- ✅ WebRTC only
- ✅ Auth enabled
- ❌ RTMP disabled
- ❌ HLS disabled
- ❌ Chat disabled (not needed)
- ❌ Recording disabled

**Use cases:**
- Video conferencing
- 1-1 video calls
- Audio calls

**Command:**
```bash
./zenlive --config=examples/config/video-call.json
```

---

### 4. `production-distributed.json`
Production-ready configuration for distributed multi-server deployment.

**Features:**
- ✅ All protocols enabled
- ✅ Cluster mode
- ✅ Redis for distributed sessions
- ✅ PostgreSQL for persistence
- ✅ S3 storage
- ✅ Analytics & Prometheus
- ✅ High availability

**Use cases:**
- Production deployments
- Multi-region
- High traffic
- Scalability
 (required for cluster mode)

**Important:**
- SDK handles real-time delivery
- YOU handle database persistence for your application data
- Configure your own PostgreSQL/MySQL/MongoDB separately
- PostgreSQL database
- S3-compatible storage

**Command:**
```bash
./zenlive --config=examples/config/production-distributed.json
```

**Important**: Set secrets via environment variables in your code:
```go
cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")
cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")
```

---

## 🔧 Using Configuration Files

### Command Line
```bash
./zenlive --config=/path/to/config.json
```

### Programmatic (Go)
```go
package main

import (
    "encoding/json"
    "os"
    "log"
    
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    // Load config from file
    file, err := os.Open("config.json")
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()
    
    var cfg config.Config
    if err := json.NewDecoder(file).Decode(&cfg); err != nil {
        log.Fatal(err)
    }
    
    // Override secrets from environment
    if secret := os.Getenv("JWT_SECRET"); secret != "" {
        cfg.Auth.JWTSecret = secret
    }
    if redisPass := os.Getenv("REDIS_PASSWORD"); redisPass != "" {
        cfg.Redis.Password = redisPass
    }
    
    // Create SDK
    sdk, err := zenlive.New(&cfg)
    if err != nil {
        log.Fatal(err)
    }
    
    // Start
    if err := sdk.Start(); err != nil {
        log.Fatal(err)
    }
    defer sdk.Stop()
    
    log.Println("ZenLive is running!")
    select {}
}
```

---

## 📊 Configuration Comparison

| Feature | Simple Livestream | Video Call | Production |
|---------|------------------|------------|------------|
| RTMP | ✅ | ❌ | ✅ |
| HLS | ✅ | ❌ | ✅ |
| WebRTC | ❌ | ✅ | ✅ |
| Chat | ✅ (real-time) | ✅ (real-time) | ✅ (real-time) |
| Analytics | ❌ | ❌ | ✅ |
| Cluster | ❌ | ❌ | ✅ |
| Redis | ❌ | ❌ | ✅ (required) |
| Storage | Local | Local | S3 |
| Max Connections | 5,000 | 1,000 | 50,000 |

**Note**: 
- Chat is real-time delivery only - you handle database persistence
- Redis is only for cluster mode (distributed sessions)
- Storage is for recordings, not application data

---

## 🚀 Quick Start

### Development
```bash
# Use default config in code
./zenlive

# Or load from file
./zenlive --config=examples/config/livestream-simple.json
```

### Production
```bash
# Copy and edit production config
cp examples/config/production-distributed.json config.production.json
nano config.production.json

# Set secrets in your code via environment:
# cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")
# cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")

# Run
./zenlive --config=config.production.json
```

---

## 📝 Configuration Validation

The SDK validates configuration on startup and will return errors for:
- Invalid port numbers
- Missing required fields (when features are enabled)
- Invalid durations
- Cluster enabled but Redis disabled

Example:
```go
cfg := config.DefaultConfig()
cfg.Server.Port = -1  // Invalid

sdk, err := zenlive.New(cfg)
// Error: invalid server port: -1
```

---

## 🔍 See Also

- [SDK Philosophy](../../docs/architecture.md) - Design principles
- [Configuration Documentation](../../docs/configuration.md) - Detailed config reference
- [Getting Started](../../docs/getting-started.md) - Tutorial
