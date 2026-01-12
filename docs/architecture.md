# ZenLive Architecture

Learn how ZenLive SDK works and how to integrate it into your project.

## 🎯 SDK Philosophy

**ZenLive focuses on REAL-TIME DELIVERY - not data persistence.**

### What SDK Does
✅ Real-time streaming (RTMP, HLS, WebRTC)  
✅ Real-time chat delivery  
✅ Session management (in-memory or Redis)  
✅ Stream recording (local/S3)  
✅ Real-time metrics  

### What SDK Does NOT Do (Your Responsibility)
❌ Database persistence  
❌ Chat history storage  
❌ User account management  
❌ Application business logic  

**💡 Principle:** SDK delivers real-time, YOU decide what to save to YOUR DATABASE.

## 📊 System Overview

```
┌────────────────────────────────────────────────────┐
│           Publishers (OBS, FFmpeg, Browser)         │
└──────────────────┬────────────────────────────────┘
                   │ RTMP/WebRTC
┌──────────────────▼────────────────────────────────┐
│              ZenLive SDK                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │   RTMP   │  │    HLS   │  │  WebRTC  │        │
│  │  Server  │  │  Server  │  │  Server  │        │
│  └──────────┘  └──────────┘  └──────────┘        │
│                                                    │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │   Auth   │  │   Chat   │  │Analytics │        │
│  └──────────┘  └──────────┘  └──────────┘        │
└──────────────────┬────────────────────────────────┘
                   │
┌──────────────────▼────────────────────────────────┐
│        Storage (Local / S3) + Cache (Redis)        │
└────────────────────────────────────────────────────┘
                   │
┌──────────────────▼────────────────────────────────┐
│              Viewers (Web, Mobile, Apps)           │
└────────────────────────────────────────────────────┘
```

## 🔧 Core Components

### 1. Streaming (REQUIRED)

#### RTMP Server (`pkg/streaming/rtmp/`)
- **Purpose:** Receive streams from OBS, FFmpeg
- **Port:** 1935 (default)
- **Use case:** Publishing from desktop apps

```go
cfg.Streaming.EnableRTMP = true
cfg.Streaming.RTMP.Port = 1935
```

#### HLS Server (`pkg/streaming/hls/`)
- **Purpose:** Deliver streams via HTTP for web/mobile
- **Port:** 8080 (default)
- **Use case:** Viewers on web, mobile apps

```go
cfg.Streaming.EnableHLS = true
cfg.Streaming.HLS.SegmentDuration = 6 * time.Second
```

#### WebRTC Server (`pkg/streaming/webrtc/`)
- **Purpose:** Ultra-low latency (<1s) streaming
- **Use case:** Video calls, live interaction

```go
cfg.Streaming.EnableWebRTC = true
cfg.Streaming.WebRTC.STUNServers = []string{
    "stun:stun.l.google.com:19302",
}
```

### 2. Authentication (OPTIONAL)

**Protect streams with JWT.**

```go
import "github.com/aminofox/zenlive/pkg/auth"

auth := auth.NewJWTAuthenticator(&auth.JWTConfig{
    SecretKey: "your-secret-key",
})

token, _ := auth.GenerateToken(&auth.User{
    ID:    "user123",
    Roles: []string{"publisher"},
}))
```

**Roles:**
- `admin` - Full access
- `publisher` - Tạo/quản lý streams
- `viewer` - Xem streams
- `moderator` - Quản lý chat

### 3. Storage (OPTIONAL)

**Recording streams to local hoặc S3.**

```go
// Local storage
cfg.Storage.Type = "local"
cfg.Storage.BasePath = "./recordings"

// S3 storage
cfg.Storage.Type = "s3"
cfg.Storage.S3.Region = "us-east-1"
cfg.Storage.S3.Bucket = "my-streams"
```

### 4. Chat (OPTIONAL)

**Real-time chat delivery - BẠN tự lưu history.**

```go
cfg.Chat.Enabled = true

// Lưu vào DATABASE CỦA BẠN
chatServer.OnMessage(func(msg *chat.Message) {
    // SDK phát real-time
    chatServer.Broadcast(msg)
    
    // BẠN lưu vào database
    myDB.Exec("INSERT INTO messages ...")
})
```

**⚠️ Lưu ý:** `EnablePersistence = false` - chỉ là in-memory buffer, KHÔNG phải database!

### 5. Analytics (OPTIONAL)

**Real-time metrics (viewers, bitrate, FPS).**

```go
cfg.Analytics.Enabled = true
cfg.Analytics.EnablePrometheus = true

// Metrics tại http://localhost:9090/metrics
```

### 6. Redis (CLUSTER MODE ONLY)

**Chỉ cần khi `Cluster.Enabled = true`.**

```go
// Multi-server deployment
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true  // BẮT BUỘC
cfg.Redis.Host = "redis.example.com"
```

**Redis chỉ dùng cho:**
- Distributed session state
- Stream routing across nodes

**KHÔNG dùng cho:**
- Chat history (BẠN tự lưu)
- Application data (BẠN tự quản lý)

## 🔄 Data Flow

### Publishing Flow (RTMP → HLS)

```
OBS/FFmpeg
    ↓ RTMP (port 1935)
RTMP Server
    ↓ Authenticate
Stream Manager
    ↓ Convert
HLS Transmuxer
    ↓ Create segments
Cache
    ↓ HTTP (port 8080)
Viewers
```

### Chat Flow

```
User A sends message
    ↓ WebSocket
Chat Server (SDK)
    ↓ Real-time broadcast
All connected users
    ↓ Your app receives event
YOUR DATABASE
    ↓ You save message
```

**💡 Nhớ:** SDK chỉ phát real-time, BẠN quyết định lưu gì!

## 📈 Performance

### Latency

| Protocol | Latency | Use Case |
|----------|---------|----------|
| RTMP | 5-15s | Publishing |
| HLS | 10-30s | Web/mobile viewing |
| WebRTC | <1s | Video calls, live interaction |

### Capacity (Single Server)

| Metric | Estimate |
|--------|----------|
| Concurrent Streams | ~1,000 |
| Concurrent Viewers | ~10,000 |
| CPU per stream (1080p) | ~5-10% |
| Memory per stream | ~50-100MB |

### Scaling (Cluster Mode)

```
Load Balancer
    ↓
┌───┼───┐
│   │   │
Node 1  Node 2  Node 3
│   │   │
└───┼───┘
    ↓
Redis Cluster (session state)
    ↓
S3 Storage (recordings)
```

**Capacity:** 10,000+ streams, 100,000+ viewers

## 🏗️ Deployment Architectures

### 1. Development (Single Server)

```go
cfg := config.DefaultConfig()
cfg.Streaming.EnableRTMP = true
cfg.Streaming.EnableHLS = true
cfg.Storage.Type = "local"
```

**Capacity:** ~100 viewers  
**Cost:** Minimal

### 2. Production (Single Server)

```go
cfg := config.DefaultConfig()
cfg.Streaming.EnableRTMP = true
cfg.Streaming.EnableHLS = true
cfg.Storage.Type = "s3"
cfg.Analytics.Enabled = true
cfg.Logging.Level = "info"
```

**Capacity:** ~1,000 viewers  
**Cost:** EC2 + S3

### 3. Cluster (Multi-Server)

```go
cfg := config.DefaultConfig()
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true  // Required!
cfg.Storage.Type = "s3"
cfg.Analytics.Enabled = true
```

**Capacity:** 10,000+ viewers  
**Cost:** EC2 x N + Redis + S3

## 🎨 Integration Examples

### Example 1: Livestream Platform

```go
cfg := config.DefaultConfig()

// Streaming
cfg.Streaming.EnableRTMP = true  // OBS publishing
cfg.Streaming.EnableHLS = true   // Web viewing

// Features
cfg.Chat.Enabled = true
cfg.Analytics.Enabled = true
cfg.Storage.Type = "s3"

// YOUR database for chat history, user data
db := sql.Open("postgres", "...")

// Handle chat
chatServer := sdk.GetChatServer()
chatServer.OnMessage(func(msg *chat.Message) {
    // YOU save to YOUR database
    db.Exec("INSERT INTO messages ...")
})
```

### Example 2: Video Call App

```go
cfg := config.DefaultConfig()

// Only WebRTC
cfg.Streaming.EnableRTMP = false
cfg.Streaming.EnableHLS = false
cfg.Streaming.EnableWebRTC = true

// No chat, analytics, recording
cfg.Chat.Enabled = false
cfg.Analytics.Enabled = false

// YOUR database for call logs
db := sql.Open("postgres", "...")
```

### Example 3: Recording Server

```go
cfg := config.DefaultConfig()

// Streaming
cfg.Streaming.EnableRTMP = true
cfg.Streaming.EnableHLS = true

// Storage
cfg.Storage.Type = "s3"
cfg.Storage.S3.Bucket = "my-recordings"

// YOUR database for metadata
db := sql.Open("postgres", "...")

// Save stream metadata
sdk.OnStreamEnd(func(stream *types.Stream) {
    db.Exec("INSERT INTO streams ...")
})
```

## 💡 Best Practices

### 1. Database Strategy

```go
// ✅ CORRECT - You manage your database
type MyApp struct {
    sdk *zenlive.SDK
    db  *sql.DB  // PostgreSQL, MySQL, MongoDB, etc.
}

// Handle SDK events → Save to YOUR database
app.sdk.OnStreamStart(func(s *Stream) {
    app.db.Exec("INSERT INTO streams ...")
})
```

### 2. Redis Strategy

```go
// ✅ Single server - NO Redis
cfg.Cluster.Enabled = false
cfg.Redis.Enabled = false

// ✅ Multi-server - YES Redis (required)
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true
```

### 3. Chat Strategy

```go
// Livestream - enable chat
cfg.Chat.Enabled = true

chatServer.OnMessage(func(msg *Message) {
    // 1. SDK broadcasts real-time
    chatServer.Broadcast(msg)
    
    // 2. YOU save to database
    myDB.SaveMessage(msg)
})

// Video call - disable chat
cfg.Chat.Enabled = false
```

### 4. Progressive Scaling

```go
// Day 1: Simple
cfg := config.DefaultConfig()

// Week 1: Add chat
cfg.Chat.Enabled = true

// Month 1: Add analytics
cfg.Analytics.Enabled = true

// Month 3: Scale to cluster
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true
```

## 🚀 Next Steps

- **[QUICKSTART.md](QUICKSTART.md)** - Integrate SDK now (5 minutes)
- **[Examples](../examples/)** - 11+ complete code examples
- **[GitHub](https://github.com/aminofox/zenlive)** - Source code & issues
