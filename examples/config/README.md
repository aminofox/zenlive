# Configuration Examples

Example configuration files for different deployment scenarios.

## 🎯 SDK Philosophy

**ZenLive SDK handles real-time communication ONLY.**

✅ **SDK delivers**: Streams and messages in real-time  
❌ **SDK does NOT**: Persist application data to database  
💾 **Your responsibility**: Save data to YOUR database  
🔴 **Redis**: ONLY for cluster mode (distributed sessions)  
💬 **Chat**: Real-time delivery only (you handle history storage)

## 📁 Configuration Files

### 1. `config.example.json`
Complete configuration with all available options and default values.

**Use for**: Understanding all configuration options

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

**Important**: Chat messages are delivered in real-time only. **YOU must save to YOUR database** for chat history.

**Use cases:**
- Small livestreaming platform
- Single server deployment
- Development/testing

**Run:**
```bash
./zenlive --config=examples/config/livestream-simple.json
```

**Save chat to database:**
```go
chatServer.OnMessage(func(msg *chat.Message) {
    // SDK broadcasts real-time
    chatServer.Broadcast(msg)
    
    // YOU save to YOUR database
    myDB.Exec("INSERT INTO messages ...", msg)
})
```

---

### 3. `video-call.json`
Configuration for 1-1 video/audio calling.

**Features:**
- ✅ WebRTC only
- ✅ Auth enabled
- ❌ RTMP disabled
- ❌ HLS disabled
- ❌ Chat disabled (not needed for simple calls)
- ❌ Recording disabled

**Use cases:**
- Video conferencing
- 1-1 video calls
- Audio calls

**Run:**
```bash
./zenlive --config=examples/config/video-call.json
```

---

### 4. `production-distributed.json`
Production-ready configuration for distributed multi-server deployment.

**Features:**
- ✅ All protocols enabled
- ✅ Cluster mode
- ✅ Redis for distributed sessions (REQUIRED)
- ✅ S3 storage
- ✅ Analytics & Prometheus
- ✅ High availability

**Important Notes:**
- Redis is **REQUIRED** when `Cluster.Enabled = true`
- SDK handles real-time delivery
- **YOU handle database persistence** for application data
- Configure your own PostgreSQL/MySQL/MongoDB separately

**Use cases:**
- Production deployments
- Multi-region
- High traffic
- Scalability

**Run:**
```bash
# Start Redis first
docker run -d -p 6379:6379 redis

# Start multiple nodes
NODE_ID=node-1 ./zenlive --config=examples/config/production-distributed.json
NODE_ID=node-2 PORT=8081 ./zenlive --config=examples/config/production-distributed.json
```

**Database strategy:**
```go
// Example: PostgreSQL for application data
db, _ := sql.Open("postgres", "...")

// Save stream metadata
sdk.OnStreamEnd(func(stream *types.Stream) {
    db.Exec("INSERT INTO streams ...", stream)
})

// Save chat messages
chatServer.OnMessage(func(msg *chat.Message) {
    db.Exec("INSERT INTO messages ...", msg)
})

// Save viewer analytics
sdk.OnViewerJoin(func(viewer *types.Viewer) {
    db.Exec("INSERT INTO analytics ...", viewer)
})
```

---

## 📊 Configuration Comparison

| Feature | Simple | Video Call | Production |
|---------|--------|-----------|-----------|
| RTMP | ✅ | ❌ | ✅ |
| HLS | ✅ | ❌ | ✅ |
| WebRTC | ❌ | ✅ | ✅ |
| Chat | ✅ (real-time) | ❌ | ✅ (real-time) |
| Analytics | ❌ | ❌ | ✅ |
| Redis | ❌ | ❌ | ✅ (required) |
| Cluster | ❌ | ❌ | ✅ |
| Your DB | Optional | Optional | **REQUIRED** |

## 💡 Key Points

### 1. Database is YOUR Responsibility

```go
// ❌ WRONG - SDK does NOT save to database
cfg.Chat.EnablePersistence = true  // Just in-memory buffer!

// ✅ CORRECT - YOU save to YOUR database
chatServer.OnMessage(func(msg *Message) {
    myDB.SaveMessage(msg)  // Your responsibility
})
```

### 2. Redis Only for Cluster

```go
// ❌ WRONG - Waste resources
cfg.Cluster.Enabled = false
cfg.Redis.Enabled = true  // Not needed!

// ✅ CORRECT - Redis only when cluster
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true  // Required
```

### 3. Chat is Optional

```go
// Livestream - enable chat
cfg.Chat.Enabled = true

// Video call - disable chat
cfg.Chat.Enabled = false
```

## 🚀 Quick Start

### Development
```bash
cp config.example.json my-config.json
# Edit my-config.json
./zenlive --config=my-config.json
```

### Production
```bash
# Use environment variables for secrets
export JWT_SECRET=$(openssl rand -base64 32)
export AWS_ACCESS_KEY="..."
export AWS_SECRET_KEY="..."
export REDIS_HOST="redis.example.com"

# Run with config
./zenlive --config=examples/config/production-distributed.json
```

## 📖 Documentation

- **[QUICKSTART.md](../../docs/QUICKSTART.md)** - Get started in 5 minutes
- **[ARCHITECTURE.md](../../docs/ARCHITECTURE.md)** - Understand SDK architecture
- **[Examples](../)** - 11+ working code examples

## 🆘 Need Help?

1. Read [QUICKSTART.md](../../docs/QUICKSTART.md)
2. Check [Examples](../)
3. Visit [GitHub Issues](https://github.com/aminofox/zenlive/issues)

---

**Happy Streaming! 🎥**
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
