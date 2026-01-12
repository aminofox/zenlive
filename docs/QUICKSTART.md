# ZenLive Quick Start

Integrate ZenLive SDK into your project in 5 minutes.

## 📦 Installation

```bash
go get github.com/aminofox/zenlive
```

**Requirements:** Go 1.23+

## 🚀 3 Steps to Integration

### Step 1: Create SDK Instance

```go
package main

import (
    "log"
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    // Create default config
    cfg := config.DefaultConfig()
    
    // Create SDK
    sdk, err := zenlive.New(cfg)
    if err != nil {
        log.Fatal(err)
    }

    // Start SDK
    if err := sdk.Start(); err != nil {
        log.Fatal(err)
    }
    defer sdk.Stop()

    log.Println("✅ ZenLive is running!")
    select {} // Keep program running
}
```

### Step 2: Publish Stream

Use OBS or FFmpeg to stream:

```bash
# With FFmpeg
ffmpeg -re -i video.mp4 -c copy -f flv rtmp://localhost:1935/live/mystream

# With OBS Studio
# Server: rtmp://localhost:1935/live
# Stream Key: mystream
```

### Step 3: Watch Stream

**On Web (HLS):**
```html
<video controls>
    <source src="http://localhost:8080/live/mystream/index.m3u8" 
            type="application/x-mpegURL">
</video>
```

**With VLC or FFplay:**
```bash
ffplay rtmp://localhost:1935/live/mystream
# or
ffplay http://localhost:8080/live/mystream/index.m3u8
```

## 🎯 Common Use Cases

### 1. Livestream Platform (like Twitch)

```go
cfg := config.DefaultConfig()

// Enable streaming
cfg.Streaming.EnableRTMP = true  // Receive from OBS
cfg.Streaming.EnableHLS = true   // Deliver to viewers

// Enable chat
cfg.Chat.Enabled = true

// Save recordings
cfg.Storage.Type = "local"
cfg.Storage.BasePath = "./recordings"

sdk, _ := zenlive.New(cfg)
sdk.Start()
```

### 2. Video Call 1-1 (like Zoom)

```go
cfg := config.DefaultConfig()

// Only WebRTC needed
cfg.Streaming.EnableRTMP = false
cfg.Streaming.EnableHLS = false
cfg.Streaming.EnableWebRTC = true

// No chat, analytics, recording
cfg.Chat.Enabled = false
cfg.Analytics.Enabled = false

sdk, _ := zenlive.New(cfg)
sdk.Start()
```

### 3. Video Conference (Group)

```go
cfg := config.DefaultConfig()

// WebRTC for low latency
cfg.Streaming.EnableWebRTC = true
cfg.Streaming.WebRTC.STUNServers = []string{
    "stun:stun.l.google.com:19302",
}

// Optional: Chat
cfg.Chat.Enabled = true

sdk, _ := zenlive.New(cfg)
sdk.Start()
```

## 🎨 Add Features

### Add Authentication

```go
import "github.com/aminofox/zenlive/pkg/auth"

// Create authenticator
authenticator := auth.NewJWTAuthenticator(&auth.JWTConfig{
    SecretKey: "your-secret-key-here",
})

// Generate token for publisher
token, _ := authenticator.GenerateToken(&auth.User{
    ID:    "user123",
    Roles: []string{"publisher"},
})

// Use this token when publishing stream
```

### Add Chat

```go
import "github.com/aminofox/zenlive/pkg/chat"

// Chat server starts automatically when cfg.Chat.Enabled = true
chatServer := sdk.GetChatServer()

// Create room for stream
room := chatServer.CreateRoom("stream-123")

// Send message
room.Broadcast(&chat.Message{
    UserID:  "user456",
    Content: "Hello viewers!",
})

// Save chat to YOUR DATABASE
chatServer.OnMessage(func(msg *chat.Message) {
    // SDK broadcasts real-time
    chatServer.Broadcast(msg)
    
    // YOU save to database
    myDB.SaveMessage(msg)
})
```

### Add Recording

```go
cfg.Storage.Type = "local"  // or "s3"
cfg.Storage.BasePath = "./recordings"

// With S3
cfg.Storage.Type = "s3"
cfg.Storage.S3 = config.S3Config{
    Region: "us-east-1",
    Bucket: "my-streams",
    AccessKeyID: os.Getenv("AWS_ACCESS_KEY"),
    SecretAccessKey: os.Getenv("AWS_SECRET_KEY"),
}
```

### Add Analytics

```go
cfg.Analytics.Enabled = true
cfg.Analytics.EnablePrometheus = true
cfg.Analytics.PrometheusPort = 9090

// Access metrics at http://localhost:9090/metrics
```

## 📊 Configuration Templates

### Development (Simplest)

```go
cfg := config.DefaultConfig()
cfg.Server.Port = 8080
cfg.Logging.Level = "debug"
```

### Production (Basic)

```go
cfg := config.DefaultConfig()

// Server
cfg.Server.Port = 8080
cfg.Server.MaxConnections = 10000

// Auth
cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")

// Storage
cfg.Storage.Type = "s3"
cfg.Storage.S3.Region = "us-east-1"
cfg.Storage.S3.Bucket = "my-streams"
cfg.Storage.S3.AccessKeyID = os.Getenv("AWS_ACCESS_KEY")
cfg.Storage.S3.SecretAccessKey = os.Getenv("AWS_SECRET_KEY")

// Logging
cfg.Logging.Level = "info"
cfg.Logging.Format = "json"
```

### Production (Full Features)

```go
cfg := config.DefaultConfig()

// Streaming
cfg.Streaming.EnableRTMP = true
cfg.Streaming.EnableHLS = true
cfg.Streaming.EnableWebRTC = true

// Features
cfg.Chat.Enabled = true
cfg.Analytics.Enabled = true

// Storage
cfg.Storage.Type = "s3"

// Logging
cfg.Logging.Level = "info"
```

### Cluster (Multi-server)

```go
cfg := config.DefaultConfig()

// Cluster mode
cfg.Cluster.Enabled = true
cfg.Cluster.NodeID = "node-1"  // Unique per server

// Redis (REQUIRED when cluster enabled)
cfg.Redis.Enabled = true
cfg.Redis.Host = "redis.example.com"
cfg.Redis.Port = 6379

// Shared storage
cfg.Storage.Type = "s3"
```

## ⚠️ Important Notes

### 1. SDK Does NOT Manage Database

**SDK only does:** Real-time delivery (streaming, chat)
**YOU must do:** Save data to database (chat history, user info, stream metadata)

```go
// ❌ WRONG - Expecting SDK to save chat
cfg.Chat.EnablePersistence = true  // Just in-memory buffer!

// ✅ CORRECT - You save to database
chatServer.OnMessage(func(msg *chat.Message) {
    myDB.Exec("INSERT INTO messages ...")  // YOU do this
})
```

### 2. Redis Only for Cluster Mode

```go
// ❌ WRONG - Single server doesn't need Redis
cfg.Cluster.Enabled = false
cfg.Redis.Enabled = true  // Waste!

// ✅ CORRECT - Redis only when cluster
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true  // Required
```

### 3. Chat is Optional

```go
// Livestream - with chat
cfg.Chat.Enabled = true

// Video call - no chat needed
cfg.Chat.Enabled = false
```

## 📁 Project Structure

```
myapp/
├── main.go                 # Entry point
├── go.mod
├── handlers/
│   ├── stream.go          # Stream logic
│   └── chat.go            # Chat logic (if needed)
├── database/
│   └── db.go              # YOUR DATABASE
└── recordings/            # If using local storage
```

## 🔧 Troubleshooting

### Port already in use

```bash
# Check port
lsof -i :1935  # RTMP
lsof -i :8080  # HTTP/HLS

# Kill process
kill -9 <PID>
```

### Stream not showing

```bash
# Check logs
cfg.Logging.Level = "debug"

# Check if stream is publishing
curl http://localhost:8080/api/streams
```

### Out of memory

```go
// Reduce max connections
cfg.Server.MaxConnections = 1000

// Reduce HLS playlist size
cfg.Streaming.HLS.PlaylistLength = 3
```

## 📚 Next Steps

- **[ARCHITECTURE.md](ARCHITECTURE.md)** - Understand how SDK works
- **[Examples](../examples/)** - See 11+ complete examples
- **[Config Examples](../examples/config/)** - Configuration templates

## 💡 Best Practices

1. **Start simple** - Use `DefaultConfig()`, add features gradually
2. **Use environment variables** - For secrets (JWT, AWS keys)
3. **Production logging** - `level: "info"`, `format: "json"`
4. **S3 for production** - Local storage only for dev/test
5. **Test configuration** - Before deployment

## ❓ FAQ

**Q: Does SDK have built-in database?**  
A: No. You manage your own database.

**Q: Where is chat history stored?**  
A: SDK only broadcasts real-time. YOU save to YOUR database.

**Q: When do I need Redis?**  
A: Only when `Cluster.Enabled = true` (multi-server).

**Q: Can I use MongoDB/PostgreSQL?**  
A: Yes! Use any database - SDK doesn't care.

**Q: Is chat required?**  
A: No. Disable with `Chat.Enabled = false` for video calls.

## 🆘 Support

- **Issues**: [github.com/aminofox/zenlive/issues](https://github.com/aminofox/zenlive/issues)
- **Examples**: [github.com/aminofox/zenlive/examples](https://github.com/aminofox/zenlive/tree/main/examples)
