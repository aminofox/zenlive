# ZenLive Examples

Complete, runnable examples demonstrating ZenLive features. Each example is a standalone application.

## 🚀 Quick Start

```bash
# Clone the repo
git clone https://github.com/aminofox/zenlive
cd zenlive/examples

# Run any example
cd basic
go run main.go
```

## 📚 Examples

### 1. Basic - Simple Streaming Server ⭐

**Path**: [`basic/`](basic/)  
**Level**: Beginner  
**Demonstrates**:
- RTMP server setup
- HLS streaming
- Basic stream management

**Run**:
```bash
cd basic && go run main.go
```

**Test**:
```bash
# Publish
ffmpeg -re -i test.mp4 -c copy -f flv rtmp://localhost:1935/live/test

# Watch
ffplay http://localhost:8080/live/test/index.m3u8
```

---

### 2. Authentication - JWT & RBAC

**Path**: [`auth/`](auth/)  
**Level**: Intermediate  
**Demonstrates**:
- JWT authentication
- Role-based access control
- Stream key validation
- Session management

**Run**:
```bash
cd auth && go run main.go
```

**Test**:
```bash
# Login
curl -X POST http://localhost:8080/auth/login \
  -d '{"username":"admin","password":"admin"}'

# Create stream with token
curl -X POST http://localhost:8080/api/streams \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"name":"mystream"}'
```

---

### 3. Chat - Real-time Chat

**Path**: [`chat/`](chat/)  
**Level**: Intermediate  
**Demonstrates**:
- WebSocket chat server
- Chat rooms (one per stream)
- Message broadcasting
- **Important**: Chat history persistence (YOUR responsibility)

**Run**:
```bash
cd chat && go run main.go
```

**Note**: This example shows how to receive chat messages in real-time. **YOU must save messages to YOUR database** if you need chat history.

```go
// Example: Save to your database
chatServer.OnMessage(func(msg *chat.Message) {
    // SDK broadcasts in real-time
    chatServer.Broadcast(msg)
    
    // YOU save to YOUR database
    myDB.Exec("INSERT INTO messages ...", msg)
})
```

---

### 4. Storage - Recording Streams

**Path**: [`storage/`](storage/)  
**Level**: Intermediate  
**Demonstrates**:
- Recording to local storage
- Recording to S3
- Thumbnail generation
- **Important**: Stream metadata persistence (YOUR responsibility)

**Run**:
```bash
cd storage && go run main.go
```

**Note**: SDK records video files to storage. **YOU must save stream metadata to YOUR database**.

```go
// Example: Save metadata to your database
sdk.OnStreamEnd(func(stream *types.Stream) {
    // SDK saved video file
    
    // YOU save metadata to YOUR database
    myDB.Exec("INSERT INTO streams ...", stream)
})
```

---

### 5. WebRTC - Ultra-Low Latency

**Path**: [`webrtc/`](webrtc/)  
**Level**: Advanced  
**Demonstrates**:
- WebRTC signaling
- SFU (Selective Forwarding Unit)
- Sub-second latency
- Bandwidth adaptation

**Run**:
```bash
cd webrtc && go run main.go
```

**Test**:
```bash
# Open publisher
open http://localhost:8443/publish.html

# Open player
open http://localhost:8443/play.html
```

---

### 6. Analytics - Metrics & Monitoring

**Path**: [`analytics/`](analytics/)  
**Level**: Intermediate  
**Demonstrates**:
- Real-time stream metrics
- Viewer tracking
- Prometheus export
- **Important**: Long-term analytics (YOUR responsibility)

**Run**:
```bash
cd analytics && go run main.go

# Check Prometheus metrics
curl http://localhost:9090/metrics
```

**Note**: SDK provides real-time metrics. **YOU aggregate and store to YOUR database** for long-term analytics.

---

### 7. Scalability - Multi-Server Cluster

**Path**: [`scalability/`](scalability/)  
**Level**: Advanced  
**Demonstrates**:
- Cluster mode setup
- Redis for distributed sessions
- Load balancing
- Multi-node deployment

**Run**:
```bash
# Start Redis first
docker run -d -p 6379:6379 redis

# Start node 1
cd scalability
NODE_ID=node-1 go run main.go

# Start node 2 (different terminal)
NODE_ID=node-2 PORT=8081 go run main.go
```

**Important**: Redis is **ONLY** for cluster mode. Single server doesn't need Redis.

---

### 8. Security - Advanced Security

**Path**: [`security/`](security/)  
**Level**: Advanced  
**Demonstrates**:
- TLS/HTTPS
- Rate limiting
- IP filtering
- Audit logging
- Stream key rotation

**Run**:
```bash
cd security && go run main.go
```

---

### 9. Interactive - Polls & Gifts

**Path**: [`interactive/`](interactive/)  
**Level**: Intermediate  
**Demonstrates**:
- Live polls
- Virtual gifts
- Real-time reactions
- Currency management

**Run**:
```bash
cd interactive && go run main.go
```

---

### 10. HLS - HTTP Live Streaming

**Path**: [`hls/`](hls/)  
**Level**: Intermediate  
**Demonstrates**:
- HLS segment generation
- Adaptive bitrate streaming (ABR)
- DVR (time-shifting)
- Playlist management

**Run**:
```bash
cd hls && go run main.go
```

---

### 11. RTMP - Advanced RTMP

**Path**: [`rtmp/`](rtmp/)  
**Level**: Advanced  
**Demonstrates**:
- RTMP handshake protocol
- Chunk handling
- AMF encoding/decoding
- Multiple concurrent streams

**Run**:
```bash
cd rtmp && go run main.go
```

---

### 12. SDK - Client SDK Usage

**Path**: [`sdk/`](sdk/)  
**Level**: Intermediate  
**Demonstrates**:
- Stream management API
- Event system
- Webhook delivery
- State machine

**Run**:
```bash
cd sdk && go run main.go
```

---

## 💡 Important Notes

### Database Persistence

**ZenLive SDK does NOT persist application data to database.**

✅ **SDK handles**: Real-time delivery (streaming, chat, metrics)  
❌ **SDK does NOT handle**: Database storage

**YOUR responsibility**:
- Save chat messages to YOUR database
- Save stream metadata to YOUR database
- Save user data to YOUR database
- Design YOUR own database schema

**Example**:
```go
// Chat - YOU save to database
chatServer.OnMessage(func(msg *Message) {
    myDB.SaveMessage(msg) // Your code
})

// Stream - YOU save metadata
sdk.OnStreamEnd(func(stream *Stream) {
    myDB.SaveStream(stream) // Your code
})

// Analytics - YOU aggregate data
sdk.OnViewerJoin(func(viewer *Viewer) {
    myDB.LogViewerAction(viewer) // Your code
})
```

### Redis Usage

**Redis is ONLY for cluster mode** (multi-server deployments).

- ✅ **Cluster mode**: Redis required for distributed sessions
- ❌ **Single server**: Redis NOT needed

```go
// Single server - NO Redis
cfg.Cluster.Enabled = false
cfg.Redis.Enabled = false

// Multi-server - YES Redis (required)
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true
```

### Chat Philosophy

**Chat is real-time delivery only.**

- SDK broadcasts messages to connected clients in real-time
- **YOU save to YOUR database** for chat history
- Chat is optional (disable for simple video calls)

```go
// Livestream - enable chat
cfg.Chat.Enabled = true

// Video call - disable chat
cfg.Chat.Enabled = false
```

## 🎯 Use Case Examples

### Livestream Platform (like Twitch)
```bash
cd basic  # RTMP + HLS
# or
cd chat   # RTMP + HLS + Chat
```

### Video Call (1-1)
```bash
cd webrtc  # WebRTC only
```

### Video Conference (Multi-user)
```bash
cd webrtc  # WebRTC + optional chat
```

### Recording Server
```bash
cd storage  # RTMP + HLS + Recording
```

### Production Deployment
```bash
cd scalability  # Cluster mode
```

## 📖 Documentation

- **[Quick Start](../docs/QUICKSTART.md)** - Get started in 5 minutes
- **[Architecture](../docs/ARCHITECTURE.md)** - Understand how ZenLive works
- **[Configuration](config/)** - Config examples for different scenarios

## 🆘 Need Help?

1. Check the example code
2. Read [QUICKSTART.md](../docs/QUICKSTART.md)
3. Visit [GitHub Issues](https://github.com/aminofox/zenlive/issues)

---

**Happy Coding! 🎉**

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
