# ZenLive Examples

This directory contains working examples demonstrating various ZenLive features. Each example is a complete, runnable application.

## Quick Start

```bash
# Navigate to any example
cd examples/basic

# Run the example
go run main.go
```

## Available Examples

### 1. Basic - Simple Streaming Server

**Location**: [`basic/`](basic/)  
**Complexity**: ⭐ Beginner  
**What it demonstrates**:
- RTMP server setup
- HLS transmuxing
- Basic stream management

**Run**:
```bash
cd basic
go run main.go
```

**Test**:
```bash
# Publish with OBS or FFmpeg
ffmpeg -re -i test.mp4 -c copy -f flv rtmp://localhost:1935/live/test

# Play with FFplay
ffplay http://localhost:8080/live/test/index.m3u8
```

---

### 2. Authentication - User Auth & RBAC

**Location**: [`auth/`](auth/)  
**Complexity**: ⭐⭐ Intermediate  
**What it demonstrates**:
- JWT authentication
- Role-based access control (RBAC)
- Stream key validation
- Session management

**Run**:
```bash
cd auth
go run main.go
```

**Test**:
```bash
# Generate token
curl -X POST http://localhost:8080/auth/login \
  -d '{"username":"admin","password":"admin"}'

# Use token to create stream
curl -X POST http://localhost:8080/api/streams \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"name":"mystream"}'
```

---

### 3. RTMP - Advanced RTMP Features

**Location**: [`rtmp/`](rtmp/)  
**Complexity**: ⭐⭐ Intermediate  
**What it demonstrates**:
- RTMP handshake protocol
- Chunk handling
- AMF encoding/decoding
- Multiple concurrent streams
- Bandwidth management

**Run**:
```bash
cd rtmp
go run main.go
```

---

### 4. HLS - HTTP Live Streaming

**Location**: [`hls/`](hls/)  
**Complexity**: ⭐⭐ Intermediate  
**What it demonstrates**:
- HLS segment generation
- Adaptive bitrate streaming (ABR)
- DVR (time-shifting)
- Playlist management

**Run**:
```bash
cd hls
go run main.go
```

**Test**:
```bash
# Publish stream
ffmpeg -re -i test.mp4 -c copy -f flv rtmp://localhost:1935/live/test

# Check playlist
curl http://localhost:8080/live/test/index.m3u8

# Play in browser (needs hls.js)
open http://localhost:8080/player.html?stream=test
```

---

### 5. WebRTC - Ultra-Low Latency Streaming

**Location**: [`webrtc/`](webrtc/)  
**Complexity**: ⭐⭐⭐ Advanced  
**What it demonstrates**:
- WebRTC signaling
- SFU (Selective Forwarding Unit)
- Bandwidth estimation
- Sub-second latency

**Run**:
```bash
cd webrtc
go run main.go
```

**Test**:
```bash
# Open publisher
open http://localhost:8443/publish.html

# Open player
open http://localhost:8443/play.html
```

---

### 6. Chat - Real-time Chat Integration

**Location**: [`chat/`](chat/)  
**Complexity**: ⭐⭐ Intermediate  
**What it demonstrates**:
- WebSocket chat server
- Room-based chat
- Message moderation
- Chat persistence

**Run**:
```bash
cd chat
go run main.go
```

**Test**:
```bash
# Open chat client
open http://localhost:9000/chat.html

# Send message via API
curl -X POST http://localhost:9000/api/chat/mystream/message \
  -d '{"user":"john","message":"Hello!"}'
```

---

### 7. Storage - Recording & Cloud Storage

**Location**: [`storage/`](storage/)  
**Complexity**: ⭐⭐ Intermediate  
**What it demonstrates**:
- Automatic recording
- Local filesystem storage
- S3/MinIO cloud storage
- Thumbnail generation
- Metadata management

**Run**:
```bash
cd storage
go run main.go
```

**Test**:
```bash
# Publish and record
ffmpeg -re -i test.mp4 -c copy -f flv rtmp://localhost:1935/live/test

# List recordings
curl http://localhost:8081/api/recordings

# Download recording
curl -O http://localhost:8081/api/recordings/download/test
```

---

### 8. Analytics - Metrics & Monitoring

**Location**: [`analytics/`](analytics/)  
**Complexity**: ⭐⭐ Intermediate  
**What it demonstrates**:
- Stream metrics collection
- Viewer tracking
- Prometheus integration
- Performance monitoring
- Report generation

**Run**:
```bash
cd analytics
go run main.go
```

**Test**:
```bash
# View Prometheus metrics
curl http://localhost:9090/metrics

# Get stream stats
curl http://localhost:8080/api/stats/mystream

# View viewer analytics
curl http://localhost:8080/api/viewers/mystream
```

---

### 9. Interactive - Polls, Gifts, Reactions

**Location**: [`interactive/`](interactive/)  
**Complexity**: ⭐⭐⭐ Advanced  
**What it demonstrates**:
- Live polling system
- Virtual gifts
- Real-time reactions
- Currency management
- Co-streaming

**Run**:
```bash
cd interactive
go run main.go
```

**Test**:
```bash
# Create poll
curl -X POST http://localhost:8080/api/polls \
  -d '{"question":"Favorite color?","options":["Red","Blue","Green"]}'

# Send gift
curl -X POST http://localhost:8080/api/gifts \
  -d '{"streamId":"test","giftId":"rose","from":"alice","to":"bob"}'
```

---

### 10. Security - Comprehensive Security

**Location**: [`security/`](security/)  
**Complexity**: ⭐⭐⭐ Advanced  
**What it demonstrates**:
- TLS/HTTPS configuration
- Encryption (AES-256-GCM)
- Rate limiting
- IP firewall
- Stream key rotation
- Watermarking
- Audit logging

**Run**:
```bash
cd security
go run main.go
```

**Features demonstrated**:
- Certificate management
- Password hashing (Argon2id)
- Token encryption
- Multi-level rate limiting
- Compliance reporting (SOC 2, GDPR)

---

### 11. Scalability - Cluster & Load Balancing

**Location**: [`scalability/`](scalability/)  
**Complexity**: ⭐⭐⭐ Advanced  
**What it demonstrates**:
- Horizontal scaling
- Load balancing
- Service discovery
- Distributed session management
- Stream routing

**Run**:
```bash
# Start multiple nodes
cd scalability
go run main.go -node-id=1 -port=8081 &
go run main.go -node-id=2 -port=8082 &
go run main.go -node-id=3 -port=8083 &
```

**Test**:
```bash
# Check cluster status
curl http://localhost:8081/cluster/status

# Publish to load balancer
ffmpeg -re -i test.mp4 -c copy -f flv rtmp://localhost:1935/live/test
```

---

### 12. SDK - Complete SDK Usage

**Location**: [`sdk/`](sdk/)  
**Complexity**: ⭐⭐ Intermediate  
**What it demonstrates**:
- Stream lifecycle management
- Event system
- Webhooks
- Query builder
- State machine

**Run**:
```bash
cd sdk
go run main.go
```

**Features**:
- CreateStream, DeleteStream, UpdateStream
- ListStreams with filters
- Event callbacks (OnStreamStart, OnStreamEnd)
- Webhook delivery

---

## Common Setup

All examples share common dependencies:

```bash
# Install dependencies
go mod download

# For Redis examples (chat, cluster)
docker run -d -p 6379:6379 redis:alpine
```

**Note**: S3 storage is configured in code, not via environment variables. See individual example code for details.

## Building Examples

Build all examples:

```bash
# From root directory
make build-examples

# Or individually
go build -o bin/basic examples/basic/main.go
go build -o bin/auth examples/auth/main.go
# ... etc
```

Binaries will be in `bin/` directory.

## Testing Examples

Each example includes test scenarios:

```bash
cd examples/basic
go test -v
```

## Configuration

Examples use programmatic configuration (Go code):

```go
// Most examples use default config
cfg := config.DefaultConfig()
cfg.Server.Port = 8080

// Or customize specific settings
cfg.Logging.Level = "debug"
cfg.Streaming.RTMP.Port = 1935
```

See individual example code for specific configuration.

## Production Deployment

These examples are for demonstration. For production:

1. **Configure in code**:
   ```go
   cfg := config.DefaultConfig()
   
   // Use S3 storage
   cfg.Storage.Type = "s3"
   cfg.Storage.S3 = config.S3Config{
       Bucket:          "prod-recordings",
       Region:          "us-east-1",
       AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
       SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
       UseSSL:          true,
   }
   
   // Enable Redis for cluster mode
   cfg.Cluster.Enabled = true
   cfg.Redis.Enabled = true
   cfg.Redis.Host = "prod-redis"
   cfg.Redis.Port = 6379
   cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")
   
   // Enable monitoring
   cfg.Analytics.Enabled = true
   cfg.Analytics.EnablePrometheus = true
   ```

2. **Or use JSON config file**:
   ```json
   {
     "storage": {
       "type": "s3",
       "s3": {
         "bucket": "prod-recordings"
       }
     },
     "cluster": {
       "enabled": true
     },
     "redis": {
       "enabled": true,
       "host": "prod-redis",
       "port": 6379
     },
     "analytics": {
       "enabled": true,
       "enable_prometheus": true
     }
   }
   ```

## Troubleshooting

### Port already in use

```bash
# Find process using port
lsof -i :1935

# Kill process
kill -9 <PID>
```

### Permission denied

```bash
# Use higher port (> 1024) or run with sudo
sudo go run main.go
```

### Cannot connect to Redis

```bash
# Start Redis
docker run -d -p 6379:6379 redis:alpine

# Test connection
redis-cli ping
```

### FFmpeg not found

```bash
# Install FFmpeg
# macOS
brew install ffmpeg

# Ubuntu
sudo apt install ffmpeg

# Windows
# Download from https://ffmpeg.org/download.html
```

## Documentation

- [Getting Started Guide](../docs/getting-started.md)
- [Architecture Overview](../docs/architecture.md)
- [Configuration Reference](../docs/configuration.md)
- [Tutorials](../docs/tutorials/)
- [API Documentation](https://pkg.go.dev/github.com/aminofox/zenlive)

## Support

- GitHub Issues: [github.com/aminofox/zenlive/issues](https://github.com/aminofox/zenlive/issues)
- Discord: [discord.gg/zenlive](https://discord.gg/zenlive)
- Documentation: [zenlive.dev/docs](https://zenlive.dev/docs)

## License

All examples are licensed under MIT License. See [LICENSE](../LICENSE) for details.
