# ZenLive Documentation

Welcome to ZenLive - a production-ready video conferencing and livestreaming platform built in Go.

## 📚 Table of Contents

### Getting Started

- **[Quickstart Guide](quickstart.md)** - Get up and running in 10 minutes
- **[Installation](quickstart.md#installation)** - Install ZenLive
- **[Your First Room](quickstart.md#create-your-first-room)** - Create a video conference room

### Core Concepts

- **[Architecture](architecture.md)** - Understand ZenLive's design
- **[Room System](architecture.md#1-room-system-pkgroom)** - How rooms work
- **[WebRTC SFU](architecture.md#3-webrtc-sfu-pkgstreamingwebrtc)** - Video/audio streaming
- **[API Overview](architecture.md#2-api-layer-pkgapi)** - REST & WebSocket APIs

### Guides

- **[Configuration](configuration.md)** - Configure your server
- **[Testing](testing.md)** - Run tests
- **[Migration](migration.md)** - Migrate from other platforms
- **[Troubleshooting](troubleshooting.md)** - Common issues and solutions

### Tutorials

Step-by-step tutorials for common scenarios:

1. **[First Streaming Server](tutorials/01-first-streaming-server.md)** - RTMP + HLS streaming
2. **[Recording Streams](tutorials/02-recording-streams.md)** - Record and save videos
3. **[WebRTC Streaming](tutorials/03-webrtc-streaming.md)** - Real-time video conferencing

### API Reference

- **[REST API](api-reference.md#rest-api)** - HTTP endpoints
- **[WebSocket API](api-reference.md#websocket-api)** - Real-time signaling
- **[SDK Methods](api-reference.md#sdk-methods)** - Go SDK API
- **[Events](api-reference.md#events)** - Event callbacks

## 🎯 Quick Links

### Common Tasks

| Task | Link |
|------|------|
| Create a video room | [Quickstart](quickstart.md#create-your-first-room) |
| Join a room via WebSocket | [WebSocket Example](quickstart.md#websocket-client-example) |
| Record a room session | [Recording Tutorial](tutorials/02-recording-streams.md) |
| Deploy to production | [Deployment Guide](deployment.md) |
| Setup clustering | [Clustering](architecture.md#10-clustering-pkgcluster) |

### Code Examples

Browse working examples in the [`examples/`](../examples/) directory:

- **[examples/room/](../examples/room/)** - Basic room management
- **[examples/video-conference/](../examples/video-conference/)** - Full video conference server
- **[examples/websocket/](../examples/websocket/)** - WebSocket client
- **[examples/api/](../examples/api/)** - REST API usage
- **[examples/auth/](../examples/auth/)** - Authentication examples

## 🏗️ Architecture Overview

ZenLive is built with a modular architecture:

```
┌─────────────────────────────────────────┐
│         Client Applications              │
│  (Web, Mobile, OBS, Browser)            │
└─────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│          API Layer                       │
│  REST API | WebSocket | RTMP | WebRTC   │
└─────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│         Core Services                    │
│  Room Manager | Session Manager | Auth  │
└─────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│      Supporting Services                 │
│  Recording | Analytics | Storage         │
└─────────────────────────────────────────┘
```

**[Read full architecture guide →](architecture.md)**

## 🚀 Features

### Video Conferencing

- ✅ Room-based video calls
- ✅ Multi-participant support (100+ per room)
- ✅ Participant roles (Host, Speaker, Attendee)
- ✅ Permissions system
- ✅ Screen sharing
- ✅ Multi-room participation

### Streaming

- ✅ WebRTC (ultra-low latency <1s)
- ✅ RTMP ingress (OBS, FFmpeg)
- ✅ HLS playback (adaptive bitrate)
- ✅ Simulcast (multiple quality layers)

### Platform Features

- ✅ REST API for room management
- ✅ WebSocket for real-time signaling
- ✅ JWT authentication
- ✅ Rate limiting
- ✅ Recording to local/S3
- ✅ Analytics & metrics
- ✅ Clustering support
- ✅ Health monitoring

## 📖 Documentation Structure

```
docs/
├── README.md                    # This file
├── quickstart.md                # Quick start guide
├── architecture.md              # Architecture deep dive
├── configuration.md             # Configuration reference
├── api-reference.md             # API documentation
├── deployment.md                # Deployment guide
├── testing.md                   # Testing guide
├── troubleshooting.md           # Common issues
├── migration.md                 # Migration from other platforms
└── tutorials/                   # Step-by-step tutorials
    ├── 01-first-streaming-server.md
    ├── 02-recording-streams.md
    └── 03-webrtc-streaming.md
```

## 🎓 Learning Path

### For Beginners

1. Read the [Quickstart Guide](quickstart.md)
2. Run the [basic example](../examples/basic/)
3. Explore [room management](../examples/room/)
4. Try [video conference example](../examples/video-conference/)

### For Intermediate Users

1. Understand the [Architecture](architecture.md)
2. Learn about [WebRTC SFU](architecture.md#3-webrtc-sfu-pkgstreamingwebrtc)
3. Implement [authentication](../examples/auth/)
4. Setup [recording](tutorials/02-recording-streams.md)

### For Advanced Users

1. Study [clustering](architecture.md#10-clustering-pkgcluster)
2. Implement [custom storage](../examples/storage/)
3. Build [custom analytics](../examples/analytics/)
4. Deploy to [production](deployment.md)

## 🔍 API Quick Reference

### REST API

```bash
# Create room
POST /api/rooms

# List rooms
GET /api/rooms

# Get room details
GET /api/rooms/:roomId

# Delete room
DELETE /api/rooms/:roomId

# Generate access token
POST /api/rooms/:roomId/tokens
```

### WebSocket API

```javascript
// Connect
ws://server/ws?token=JWT_TOKEN

// Join room
{type: "join_room", room_id: "room_123"}

// Publish track
{type: "publish_track", data: {track_id: "...", kind: "video"}}

// Leave room
{type: "leave_room", room_id: "room_123"}
```

### Go SDK

```go
// Create room
room, err := roomMgr.CreateRoom(&room.CreateRoomRequest{
    Name: "My Room",
    MaxParticipants: 10,
})

// Add participant
participant, err := room.AddParticipant(&room.Participant{
    UserID: "user_123",
    Role: room.RoleHost,
}, token)

// List participants
participants := room.ListParticipants()
```

## 💡 Use Cases

### 1. Video Conferencing Platform

Build a Zoom/Google Meet alternative:

- Create rooms for meetings
- Invite participants with access tokens
- Share screen, audio, video
- Record meetings for later playback

**Example:** [examples/video-conference/](../examples/video-conference/)

### 2. Livestreaming Platform

Build a Twitch/YouTube Live alternative:

- Accept RTMP streams from OBS
- Deliver via HLS to viewers
- Record streams to S3
- Real-time chat integration

**Example:** [examples/rtmp/](../examples/rtmp/), [examples/hls/](../examples/hls/)

### 3. Webinar Platform

Host webinars with one speaker and many viewers:

- Host publishes video/audio
- Attendees can only watch (no publishing)
- Q&A via data channels
- Recording for on-demand viewing

**Example:** Use room with role-based permissions

### 4. Virtual Events

Multi-stage virtual events:

- Multiple rooms (stages)
- Users can switch between rooms
- Different content in each room
- Analytics on room participation

**Example:** Multi-room session management

## 🛠️ Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/room/...
```

**[Full testing guide →](testing.md)**

### Building

```bash
# Build all packages
go build ./...

# Build examples
go build ./examples/...
```

### Contributing

We welcome contributions! Please:

1. Fork the repository
2. Create a feature branch
3. Write tests for new features
4. Submit a pull request

## 📦 Package Overview

| Package | Purpose | Key Types |
|---------|---------|-----------|
| `pkg/room/` | Room management | Room, Participant, RoomManager |
| `pkg/api/` | REST & WebSocket API | Server, RoomHandler, SignalingServer |
| `pkg/streaming/webrtc/` | WebRTC SFU | SFU, Publisher, Subscriber |
| `pkg/streaming/rtmp/` | RTMP server | RTMPServer, Stream |
| `pkg/streaming/hls/` | HLS server | HLSServer, Playlist |
| `pkg/auth/` | Authentication | JWTAuthenticator, Session |
| `pkg/storage/` | Recording | Recorder, S3Storage |
| `pkg/analytics/` | Metrics | Metrics, PerformanceMonitor |
| `pkg/security/` | Security | Encryption, RateLimiter |
| `pkg/cluster/` | Clustering | LoadBalancer, Discovery |

## 🌟 Comparison with Other Platforms

| Feature | ZenLive | LiveKit | Agora | Twilio |
|---------|---------|---------|-------|--------|
| Open Source | ✅ MIT | ✅ Apache | ❌ | ❌ |
| Self-Hosted | ✅ | ✅ | ❌ | ❌ |
| WebRTC | ✅ | ✅ | ✅ | ✅ |
| RTMP | ✅ | ❌ | ❌ | ❌ |
| HLS | ✅ | Egress | ✅ | ✅ |
| Recording | ✅ Local/S3 | ✅ Egress | ✅ Cloud | ✅ Cloud |
| Pricing | Free | Free (self) | Per min | Per min |
| Language | Go | Go | SDK | SDK |

## 🤝 Community & Support

### Getting Help

- **Documentation**: You're reading it! Start with [Quickstart](quickstart.md)
- **Examples**: Browse [`examples/`](../examples/) directory
- **Issues**: Report bugs on GitHub Issues
- **Discussions**: Join GitHub Discussions

### Reporting Issues

Found a bug? Please include:

1. ZenLive version
2. Go version
3. Operating system
4. Steps to reproduce
5. Expected vs actual behavior

### Feature Requests

Have an idea? Open a GitHub Discussion with:

1. Use case description
2. Proposed solution
3. Alternative approaches considered

## 📄 License

ZenLive is licensed under the MIT License. See [LICENSE](../LICENSE) for details.

## 🎯 Roadmap

### Completed ✅

- ✅ Room system foundation
- ✅ Multi-participant video conferencing
- ✅ REST & WebSocket API
- ✅ WebRTC SFU
- ✅ RTMP/HLS streaming
- ✅ Recording to local/S3
- ✅ Authentication & authorization
- ✅ Analytics & monitoring

### In Progress 🚧

- 🚧 Client SDKs (JavaScript, iOS, Android)
- 🚧 Advanced recording features
- 🚧 Enhanced clustering

### Planned 📋

- 📋 End-to-end encryption
- 📋 AI features (background blur, noise suppression)
- 📋 Mobile SDKs
- 📋 Admin dashboard
- 📋 Webhooks system

## 🚀 Quick Start Reminder

Get started in 3 commands:

```bash
# 1. Install
go get github.com/aminofox/zenlive

# 2. Create server
cat > main.go << 'EOF'
package main
import (
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/config"
)
func main() {
    sdk, _ := zenlive.New(config.DefaultConfig())
    sdk.Start()
    defer sdk.Stop()
    select {}
}
EOF

# 3. Run
go run main.go
```

**[Continue to Quickstart Guide →](quickstart.md)**

---

**Happy Building!** 🎉

If you find ZenLive useful, please ⭐ star the repository on GitHub!
