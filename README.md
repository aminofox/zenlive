# ZenLive - Go Live Streaming SDK

[![Go Version](https://img.shields.io/badge/Go-1.23-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/aminofox/zenlive)](https://goreportcard.com/report/github.com/aminofox/zenlive)
[![GoDoc](https://godoc.org/github.com/aminofox/zenlive?status.svg)](https://godoc.org/github.com/aminofox/zenlive)

Production-ready Go SDK for building live streaming platforms. Similar to LiveKit and Agora, ZenLive provides RTMP, HLS, WebRTC streaming with real-time chat, recording, and analytics.

## 🚀 Quick Start (5 Minutes)

### 1. Install

```bash
go get github.com/aminofox/zenlive
```

### 2. Create Server

```go
package main

import (
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    cfg := config.DefaultConfig()
    sdk, _ := zenlive.New(cfg)
    sdk.Start()
    defer sdk.Stop()
    
    select {} // Keep running
}
```

### 3. Stream & Watch

```bash
# Publish with OBS or FFmpeg
ffmpeg -re -i video.mp4 -c copy -f flv rtmp://localhost:1935/live/mystream

# Watch in browser
open http://localhost:8080/live/mystream/index.m3u8
```

**That's it!** Your streaming server is running.

📖 **[Full Documentation →](docs/QUICKSTART.md)**

## ✨ Features

### Streaming Protocols
- **RTMP** - Ingest from OBS, FFmpeg, Streamlabs
- **HLS** - Deliver to web/mobile with adaptive bitrate
- **WebRTC** - Ultra-low latency (<1s) for video calls

### Additional Features
- **Real-time Chat** - WebSocket chat with moderation
- **Recording** - Save to local storage or S3
- **Analytics** - Real-time metrics & Prometheus export
- **Authentication** - JWT & role-based access control
- **Scalable** - Horizontal scaling with Redis

## 📦 Use Cases

| Use Case | Protocols | Config |
|----------|-----------|--------|
| **Live Streaming Platform** | RTMP + HLS | [Example](examples/basic/) |
| **Video Call (1-1)** | WebRTC | [Example](examples/webrtc/) |
| **Video Conference** | WebRTC + Chat | [Example](examples/chat/) |
| **Recording Server** | RTMP + HLS + Storage | [Example](examples/storage/) |

## 🎯 SDK Philosophy

**ZenLive focuses on REAL-TIME DELIVERY - not data persistence.**

✅ **SDK Does:**
- Real-time streaming (RTMP, HLS, WebRTC)
- Real-time chat delivery
- Session management
- Recording to local/S3

❌ **SDK Does NOT:**
- Database persistence (you handle this)
- Chat history storage (you save to your DB)
- User account management (your responsibility)

💡 **Your Responsibility:** The SDK delivers real-time. YOU decide what to save to YOUR database.

## 📚 Documentation

- **[Quick Start](docs/QUICKSTART.md)** - Get started in 5 minutes
- **[Architecture](docs/ARCHITECTURE.md)** - Understand how ZenLive works
- **[Examples](examples/)** - 11+ working code examples
- **[API Reference](https://pkg.go.dev/github.com/aminofox/zenlive)** - Complete API docs

## 💻 Examples

```bash
# Basic streaming server
cd examples/basic && go run main.go

# With authentication
cd examples/auth && go run main.go

# WebRTC video call
cd examples/webrtc && go run main.go

# With chat
cd examples/chat && go run main.go

# With recording
cd examples/storage && go run main.go
```

See [examples/](examples/) for all 11+ examples.

## 🔧 Configuration

### Simple (Development)
```go
cfg := config.DefaultConfig()
cfg.Streaming.EnableRTMP = true
cfg.Streaming.EnableHLS = true
```

### With Chat
```go
cfg.Chat.Enabled = true

// Save chat to YOUR database
chatServer.OnMessage(func(msg *chat.Message) {
    myDB.SaveMessage(msg) // Your code
})
```

### Production
```go
cfg := config.DefaultConfig()
cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")
cfg.Storage.Type = "s3"
cfg.Analytics.Enabled = true
```

### Multi-Server (Cluster)
```go
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true // Required for cluster
```

## 🏗️ Architecture

```
Publishers (OBS/FFmpeg)
    ↓ RTMP
ZenLive SDK
    ├── RTMP Server
    ├── HLS Server
    ├── WebRTC Server
    ├── Chat Server
    └── Storage
    ↓
Viewers (Web/Mobile)
```

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

## 📊 Performance

| Metric | Single Server | Cluster (3 nodes) |
|--------|---------------|-------------------|
| Concurrent Streams | ~1,000 | ~10,000+ |
| Concurrent Viewers | ~10,000 | ~100,000+ |
| Latency (HLS) | 10-30s | 10-30s |
| Latency (WebRTC) | <1s | <1s |

## 🔐 Security

- JWT authentication
- Role-based access control (RBAC)
- TLS/HTTPS encryption
- Rate limiting
- Stream key rotation
- Audit logging

## 🤝 Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md).

## 📄 License

MIT License - see [LICENSE](LICENSE) file.

## 🆘 Support

- **Documentation**: [docs/](docs/)
- **Examples**: [examples/](examples/)
- **Issues**: [GitHub Issues](https://github.com/aminofox/zenlive/issues)
- **Discussions**: [GitHub Discussions](https://github.com/aminofox/zenlive/discussions)

## 🌟 Star Us!

If you find ZenLive useful, please give us a star! ⭐

---

**Made with ❤️ by the ZenLive Team**
- Viewer analytics and session tracking
- Prometheus metrics export
- Health check endpoints
- Performance monitoring
- **Config**: Set `Analytics.Enabled = false` to disable

### 🎨 Interactive Features *(Optional)*

- Live polling (single/multiple choice)
- Virtual gifts and currency system
- Real-time reactions and emojis
- Co-streaming support

### ⚡ Scalability *(Optional - for distributed deployments)*

- Horizontal scaling with load balancing
- Distributed session management (requires Redis)
- Service discovery
- CDN integration
- **Config**: Set `Cluster.Enabled = true` and `Redis.Enabled = true` to enable

## 🎯 Use Cases

### ✅ Perfect For

- **Livestreaming Platforms** (Twitch-like, YouTube Live)
  - RTMP ingestion from OBS/StreamLabs
  - HLS playback for viewers
  - Real-time chat and interactions
  
- **Video Conferencing** (Zoom-like, Google Meet)
  - WebRTC peer-to-peer connections
  - Low latency (<500ms)
  - Optional recording
  
- **Audio Calling** (Discord-like voice channels)
  - WebRTC audio-only streams
  - Group voice channels
  
- **Hybrid Platforms**
  - Combine streaming + video calls
  - Switch between protocols dynamically

### ⚙️ Flexible Configuration

**Simple Single-Server Livestream:**
```go
cfg := config.DefaultConfig()
cfg.Streaming.EnableRTMP = true
cfg.Streaming.EnableHLS = true
cfg.Chat.Enabled = true
cfg.Analytics.Enabled = false  // Disable analytics
cfg.Cluster.Enabled = false    // Single server
```

**Video Call Only:**
```go
cfg := config.DefaultConfig()
cfg.Streaming.EnableRTMP = false
cfg.Streaming.EnableHLS = false
cfg.Streaming.EnableWebRTC = true
cfg.Chat.Enabled = false       // Not needed for 1-1 calls
```

**Production Multi-Server:**
```go
cfg := config.DefaultConfig()
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true       // For distributed sessions
cfg.Analytics.Enabled = true
cfg.Storage.Type = "s3"
```

📖 **See [examples/config/](examples/config/)** for complete configuration examples.

## 📚 Documentation

- **[Configuration Summary](docs/CONFIGURATION_SUMMARY.md)** - Quick reference and philosophy
- **[SDK Philosophy](docs/SDK_PHILOSOPHY.md)** - Design principles and data responsibility
- **[Getting Started Guide](docs/getting-started.md)** - Installation and first stream
- **[Architecture Analysis](docs/architecture-analysis.md)** - Component requirements and use cases
- **[Architecture Overview](docs/architecture.md)** - System design and components
- **[Configuration Guide](docs/configuration.md)** - Configuration options
- **[Configuration Examples](examples/config/)** - Sample configs for different scenarios
- **[API Reference](https://pkg.go.dev/github.com/aminofox/zenlive)** - Full API documentation
- **[Tutorials](docs/tutorials/)** - Step-by-step guides
- **[Troubleshooting](docs/troubleshooting.md)** - Common issues and solutions
- **[Migration Guide](docs/migration.md)** - Migrate from other platforms

## 💡 Usage Examples

### RTMP Streaming Server

```go
package main

import (
    "github.com/aminofox/zenlive/pkg/logger"
    "github.com/aminofox/zenlive/pkg/streaming/rtmp"
)

func main() {
    log := logger.NewDefaultLogger(logger.InfoLevel, "text")
    
    server := rtmp.NewServer(":1935", log)
    
    server.SetOnPublish(func(streamKey string, metadata map[string]interface{}) error {
        log.Info("Stream started", logger.String("key", streamKey))
        return nil
    })
    
    server.Start()
}
```

**Publish with OBS**: `rtmp://localhost:1935/live/your-stream-key`

### WebRTC Low-Latency Streaming

```go
package main

import (
    "github.com/aminofox/zenlive/pkg/logger"
    "github.com/aminofox/zenlive/pkg/streaming/webrtc"
)

func main() {
    log := logger.NewDefaultLogger(logger.InfoLevel, "json")
    
    sfuConfig := webrtc.DefaultSFUConfig()
    sfu, _ := webrtc.NewSFU(sfuConfig, log)
    
    signalingConfig := webrtc.DefaultSignalingServerConfig()
    signalingConfig.Address = ":8081"
    
    server, _ := webrtc.NewSignalingServer(signalingConfig, sfu, log)
    server.Start()
}
```

### Stream Management

```go
package main

import (
    "context"
    "github.com/aminofox/zenlive/pkg/sdk"
    "github.com/aminofox/zenlive/pkg/logger"
)

func main() {
    log := logger.NewDefaultLogger(logger.InfoLevel, "text")
    manager := sdk.NewStreamManager(log)
    ctx := context.Background()
    
    // Create a stream
    stream, _ := manager.CreateStream(ctx, &sdk.CreateStreamRequest{
        UserID:      "user-123",
        Title:       "My Gaming Stream",
        Description: "Playing Minecraft",
        Protocol:    sdk.ProtocolRTMP,
    })
    
    // Start stream
    controller := sdk.NewStreamController(manager, nil, log)
    controller.StartStream(ctx, stream.ID)
    
    // Get popular streams
    popular, _ := manager.GetPopularStreams(ctx, 10)
}
```

### Real-time Chat

```go
package main

import (
    "github.com/aminofox/zenlive/pkg/chat"
    "github.com/aminofox/zenlive/pkg/logger"
)

func main() {
    log := logger.NewDefaultLogger(logger.InfoLevel, "text")
    
    // SDK only handles real-time delivery
    // Users handle their own message persistence
    server := chat.NewServer(chat.DefaultServerConfig(), log)
    
    // Create chat room for stream
    room, _ := server.CreateRoom(ctx, "stream-123", "My Stream Chat")
    
    // Setup WebSocket endpoint
    http.HandleFunc("/ws", server.HandleWebSocket)
    http.ListenAndServe(":8080", nil)
}
```

**More examples in [`examples/`](examples/) directory** (11 complete examples)

## 🏗️ Architecture

```
┌─────────────────────────────────────────┐
│           Your Application              │
│   (import github.com/aminofox/zenlive)  │
└───────────────┬─────────────────────────┘
                │
       ┌────────▼────────┐
       │   ZenLive SDK   │
       └────────┬────────┘
                │
    ┌───────────┴───────────┐
    │                       │
┌───▼────┐          ┌──────▼──────┐
│Streaming│          │   Features  │
│Protocols│          │             │
├────────┤          ├─────────────┤
│ RTMP   │          │ Chat        │
│ HLS    │          │ Recording   │
│ WebRTC │          │ Analytics   │
└────────┘          │ Auth        │
                    │ Security    │
                    └─────────────┘
```

**ZenLive is a library/SDK** - you import it into your Go application. It's not a standalone service.

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/streaming/rtmp/...

# Run benchmarks
go test -bench=. ./tests/performance/...
```

**Test Coverage**: 85%+ across all packages

## 📈 Performance

- **RTMP**: 1000+ concurrent publishers
- **HLS**: 10,000+ concurrent viewers per node  
- **WebRTC**: Sub-second latency (200-500ms)
- **Chat**: 10,000+ messages/second per room
- **Horizontal Scaling**: Tested up to 10 nodes

## 🛠️ Development

```bash
# Clone repository
git clone https://github.com/aminofox/zenlive.git
cd zenlive

# Install dependencies
go mod download

# Run tests
go test ./...

# Build examples
go build -o bin/basic ./examples/basic
```

## 📋 Requirements

- **Go**: 1.24.0 or later
- **Optional**: Redis (for distributed sessions), S3-compatible storage (for cloud recording)

## 🗺️ Roadmap

### ✅ v1.0.0 (Current - January 2026)
- Multi-protocol streaming (RTMP, HLS, WebRTC)
- Real-time chat
- Recording & storage
- Authentication & security
- Analytics & monitoring
- Interactive features

### 🔮 Future Versions
- **v1.1.0**: Enhanced WebRTC (simulcast, SVC)
- **v1.2.0**: AI-powered analytics and insights
- **v1.3.0**: Multi-region deployment
- **v2.0.0**: GraphQL API, gRPC support

## 🤝 Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)  
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 💬 Community & Support

- **GitHub Issues**: [Report bugs and request features](https://github.com/aminofox/zenlive/issues)
- **GitHub Discussions**: [Ask questions and share ideas](https://github.com/aminofox/zenlive/discussions)
- **Documentation**: [Complete guides](https://github.com/aminofox/zenlive/tree/main/docs)

## 🙏 Acknowledgments

Built with ❤️ for the livestreaming community.

Special thanks to:
- [Pion WebRTC](https://github.com/pion/webrtc) - Excellent WebRTC implementation
- [Gorilla WebSocket](https://github.com/gorilla/websocket) - Reliable WebSocket library
- [AWS SDK for Go](https://github.com/aws/aws-sdk-go-v2) - S3 integration

## 🔗 Related Projects

- [LiveKit](https://github.com/livekit/livekit) - Open source WebRTC infrastructure
- [Agora](https://www.agora.io/) - Real-time engagement platform
- [Ant Media Server](https://github.com/ant-media/Ant-Media-Server) - Live streaming engine

---

**Current Version**: v1.0.0  
**Release Date**: January 11, 2026  
**Status**: ✅ Production Ready

**Get Started**: `go get github.com/aminofox/zenlive@v1.0.0`
