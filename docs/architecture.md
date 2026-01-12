# ZenLive Architecture

## Overview

ZenLive is a **video conferencing and livestreaming platform** similar to LiveKit, built entirely in Go. It provides real-time video/audio communication using WebRTC SFU (Selective Forwarding Unit) architecture, combined with traditional streaming protocols (RTMP/HLS).

## Design Philosophy

1. **Platform-First Approach** - ZenLive is a complete platform, not just a library
2. **Real-Time Focus** - Optimized for low-latency, real-time media delivery
3. **Scalability** - Designed for horizontal scaling across multiple servers
4. **Extensibility** - Modular architecture allows adding custom features
5. **Production-Ready** - Built with security, monitoring, and reliability in mind

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                          │
├──────────────┬──────────────┬──────────────┬────────────────┤
│  Web SDK     │  Mobile SDK  │  RTMP Client │  WebRTC Client │
│ (JavaScript) │ (iOS/Android)│    (OBS)     │   (Browser)    │
└──────────────┴──────────────┴──────────────┴────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                         API Layer                            │
├──────────────┬──────────────┬──────────────┬────────────────┤
│  REST API    │  WebSocket   │  RTMP Server │  WebRTC SFU    │
│  (HTTP/HTTPS)│  (Signaling) │  (Port 1935) │  (UDP/TCP)     │
└──────────────┴──────────────┴──────────────┴────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                        Core Services                         │
├──────────────┬──────────────┬──────────────┬────────────────┤
│ Room Manager │ Session Mgr  │ Auth Service │  Event Bus     │
│              │              │              │                │
└──────────────┴──────────────┴──────────────┴────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Supporting Services                       │
├──────────────┬──────────────┬──────────────┬────────────────┤
│  Recording   │  Analytics   │   Storage    │  Clustering    │
│              │              │   (S3/Local) │   (Redis)      │
└──────────────┴──────────────┴──────────────┴────────────────┘
```

## Core Components

### 1. Room System (`pkg/room/`)

The **Room** is the central concept in ZenLive. A room is a virtual space where participants can communicate via video/audio.

#### Key Components:

- **Room** - Container for participants, tracks, and metadata
- **Participant** - User in a room with specific role and permissions
- **RoomManager** - CRUD operations for rooms
- **MediaTrack** - Audio/video track published by a participant
- **EventBus** - Pub/sub system for room events

#### Room Lifecycle:

```
Create Room → Participant Joins → Publish Tracks → Subscribe to Tracks → Leave → Delete Room
```

#### Participant Roles:

| Role | Publish | Subscribe | Manage Room | Use Case |
|------|---------|-----------|-------------|----------|
| **Host** | ✅ | ✅ | ✅ | Room creator, full control |
| **Speaker** | ✅ | ✅ | ❌ | Active participant (panelist) |
| **Attendee** | ❌ | ✅ | ❌ | Viewer only (audience) |

### 2. API Layer (`pkg/api/`)

ZenLive provides a **REST API** and **WebSocket API** for room management and real-time signaling.

#### REST API Endpoints:

```
POST   /api/rooms                              # Create room
GET    /api/rooms                              # List rooms
GET    /api/rooms/:roomId                      # Get room details
DELETE /api/rooms/:roomId                      # Delete room
GET    /api/rooms/:roomId/participants         # List participants
POST   /api/rooms/:roomId/participants         # Add participant
DELETE /api/rooms/:roomId/participants/:id     # Remove participant
POST   /api/rooms/:roomId/tokens               # Generate access token
```

#### WebSocket API (Signaling):

Clients connect to `ws://server/ws` with JWT token for real-time communication.

**Message Types:**

- `join_room` - Join a room
- `leave_room` - Leave a room
- `publish_track` - Publish audio/video track
- `unpublish_track` - Stop publishing track
- `subscribe_track` - Subscribe to another participant's track
- `unsubscribe_track` - Unsubscribe from track
- `update_metadata` - Update participant metadata
- `send_data` - Send data channel message

### 3. WebRTC SFU (`pkg/streaming/webrtc/`)

ZenLive uses **SFU (Selective Forwarding Unit)** architecture for WebRTC.

#### Why SFU?

- **Scalable** - Server forwards media without transcoding
- **Low Latency** - Direct peer connections with server mediation
- **Quality Control** - Adaptive bitrate per subscriber
- **Efficient** - Lower CPU usage than MCU (Multi-point Control Unit)

#### SFU Flow:

```
Publisher                    SFU Server                  Subscribers
    │                            │                            │
    ├── Publish Track ────────►  │                            │
    │   (Camera/Mic)             │                            │
    │                            │  ◄──── Subscribe ──────────┤
    │                            │                            │
    │                            ├─── Forward Track ─────────►│
    │                            │   (Adaptive Bitrate)       │
```

#### Features:

- **Simulcast** - Multiple quality layers (1080p/720p/360p)
- **Adaptive Bitrate** - Automatic quality adjustment
- **Network Quality Monitoring** - Packet loss, jitter, RTT tracking
- **Auto Reconnection** - Handle network disruptions

### 4. Multi-Room Sessions (`pkg/room/session.go`)

Users can participate in **multiple rooms simultaneously**.

#### Use Cases:

- Monitor multiple meetings
- Cross-room communication
- Breakout rooms
- Virtual events with multiple stages

#### Resource Management:

- **Bandwidth allocation** per room
- **Track limits** per user
- **Connection pooling**

### 5. Authentication & Authorization (`pkg/auth/`)

#### JWT-Based Authentication:

```go
token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
    "room_id": "room-123",
    "user_id": "user-456",
    "role": "host",
    "permissions": {
        "can_publish": true,
        "can_subscribe": true
    },
    "exp": time.Now().Add(24 * time.Hour).Unix()
})
```

#### Features:

- JWT token generation and validation
- Role-based access control (RBAC)
- Rate limiting
- Session management

### 6. Streaming Protocols

#### RTMP (`pkg/streaming/rtmp/`)

- **Ingress** - Accept streams from OBS, FFmpeg
- **Use Case** - Livestreaming, broadcasting
- **Port** - 1935

#### HLS (`pkg/streaming/hls/`)

- **Delivery** - HTTP-based adaptive streaming
- **Features** - ABR, DVR (rewind), multiple quality levels
- **Use Case** - Web/mobile playback

#### WebRTC (`pkg/streaming/webrtc/`)

- **Real-Time** - Ultra-low latency (<1s)
- **Use Case** - Video conferencing, live interactions

### 7. Recording & Storage (`pkg/storage/`)

Record room sessions for later playback.

#### Features:

- **Local Storage** - Save to disk
- **S3 Compatible** - AWS S3, MinIO, etc.
- **Formats** - MP4, WebM, FLV
- **Thumbnails** - Generate preview images
- **Metadata** - Track recording info

#### Recording Workflow:

```
Room Session → Compositor → Encoder → Storage (Local/S3)
```

### 8. Analytics & Monitoring (`pkg/analytics/`)

Real-time metrics and performance monitoring.

#### Metrics:

- **Stream metrics** - Bitrate, FPS, codec info
- **Viewer metrics** - Concurrent viewers, watch time
- **Network quality** - Packet loss, jitter, RTT
- **Resource usage** - CPU, memory, bandwidth

#### Integrations:

- **Prometheus** - Metrics export
- **Health checks** - `/api/health` endpoint
- **Performance reports** - Aggregated statistics

### 9. Security (`pkg/security/`)

Production-grade security features.

#### Features:

- **TLS/SSL** - Encrypted connections
- **Token encryption** - Secure token storage
- **Rate limiting** - DDoS protection
- **Audit logging** - Track security events
- **Watermarking** - Video watermarks
- **Firewall** - IP whitelisting/blacklisting

### 10. Clustering (`pkg/cluster/`)

Horizontal scaling for high availability.

#### Features:

- **Service discovery** - Automatic node registration
- **Load balancing** - Distribute rooms across servers
- **Session routing** - Route clients to correct server
- **State synchronization** - Redis-based state sharing

#### Architecture:

```
         Load Balancer
              │
    ┌─────────┼─────────┐
    ▼         ▼         ▼
  Node1     Node2     Node3
    │         │         │
    └─────────┴─────────┘
              │
          Redis Cache
```

## Data Flow Examples

### Example 1: Video Conference Call

```
1. Client A creates room via REST API
   POST /api/rooms
   → Returns room_id

2. Client A gets access token
   POST /api/rooms/{room_id}/tokens
   → Returns JWT token

3. Client A connects to WebSocket
   ws://server/ws?token={jwt}

4. Client A joins room
   → send: {type: "join_room", room_id: "..."}
   ← recv: {type: "participant_joined", participant: {...}}

5. Client A publishes camera track
   → send: {type: "publish_track", kind: "video"}
   ← recv: {type: "track_published", track_id: "..."}

6. Client B joins same room
   → Auto-subscribes to Client A's track
   ← recv: {type: "track_subscribed", publisher: "A", track: {...}}

7. Media flows through WebRTC SFU
   Client A ──► SFU ──► Client B
```

### Example 2: Livestream Recording

```
1. Streamer publishes RTMP stream
   rtmp://server:1935/live/stream-key

2. Server converts RTMP → HLS
   → Generates .m3u8 playlist
   → Creates .ts segments

3. Viewers watch HLS stream
   http://server/live/stream-key/index.m3u8

4. Recording service saves stream
   → Encodes to MP4
   → Uploads to S3
   → Generates thumbnail
```

## Package Structure

```
zenlive/
├── pkg/
│   ├── api/              # REST & WebSocket API
│   │   ├── server.go     # HTTP server
│   │   ├── room_handler.go
│   │   ├── token_handler.go
│   │   ├── websocket.go  # WebSocket signaling
│   │   └── middleware.go # Auth, CORS, rate limit
│   │
│   ├── room/             # Room system
│   │   ├── room.go       # Room entity
│   │   ├── manager.go    # CRUD operations
│   │   ├── participant.go
│   │   ├── sfu.go        # WebRTC SFU integration
│   │   ├── session.go    # Multi-room sessions
│   │   ├── subscription.go # Track subscriptions
│   │   ├── quality.go    # Network quality monitoring
│   │   ├── reconnection.go # Auto reconnection
│   │   └── events.go     # Event bus
│   │
│   ├── streaming/        # Streaming protocols
│   │   ├── webrtc/       # WebRTC SFU
│   │   ├── rtmp/         # RTMP server
│   │   └── hls/          # HLS server
│   │
│   ├── auth/             # Authentication
│   │   ├── jwt.go
│   │   ├── rbac.go
│   │   └── session.go
│   │
│   ├── storage/          # Recording & storage
│   │   ├── recorder.go
│   │   ├── s3.go
│   │   └── formats/      # MP4, FLV encoders
│   │
│   ├── analytics/        # Metrics & monitoring
│   ├── security/         # Security features
│   ├── cluster/          # Clustering support
│   ├── cache/            # Redis caching
│   ├── logger/           # Logging
│   ├── errors/           # Error handling
│   └── config/           # Configuration
│
├── examples/             # Code examples
└── docs/                 # Documentation
```

## Technology Stack

- **Language**: Go 1.23+
- **WebRTC**: Pion WebRTC
- **HTTP**: Standard library `net/http`
- **WebSocket**: gorilla/websocket
- **Authentication**: JWT (dgrijalva/jwt-go)
- **Storage**: AWS SDK (S3), local filesystem
- **Cache**: Redis (optional)
- **Monitoring**: Prometheus

## Performance Characteristics

### Scalability:

- **Single server**: 100+ concurrent rooms, 1,000+ participants
- **Clustered**: Unlimited (horizontal scaling)

### Latency:

- **WebRTC**: <500ms (typically <100ms)
- **HLS**: 3-10 seconds (adaptive streaming)
- **RTMP**: 1-3 seconds

### Resource Usage:

- **Memory**: ~2GB per 100 participants (WebRTC)
- **CPU**: Low (SFU forwards without transcoding)
- **Bandwidth**: Proportional to participant count

## Deployment Patterns

### 1. Standalone Server

```
Single ZenLive instance
- REST API
- WebSocket
- WebRTC SFU
- RTMP/HLS servers
```

### 2. Clustered Deployment

```
Load Balancer → Multiple ZenLive nodes → Redis (shared state)
```

### 3. Microservices

```
API Gateway → Room Service → SFU Service
                          → Recording Service
                          → Analytics Service
```

## Comparison with LiveKit

| Feature | ZenLive | LiveKit |
|---------|---------|---------|
| **Language** | Go | Go |
| **Architecture** | SFU | SFU |
| **API** | REST + WebSocket | gRPC |
| **Protocols** | WebRTC, RTMP, HLS | WebRTC |
| **Room System** | ✅ Custom | ✅ Built-in |
| **Simulcast** | ✅ | ✅ |
| **Recording** | ✅ Local/S3 | ✅ Egress |
| **Open Source** | ✅ MIT | ✅ Apache 2.0 |
| **Clustering** | ✅ Redis | ✅ Built-in |

**Key Differences:**

1. **API Design** - ZenLive uses REST/WebSocket, LiveKit uses gRPC
2. **Token System** - ZenLive uses simple JWT, LiveKit uses AccessToken grants
3. **Protocols** - ZenLive includes RTMP/HLS for livestreaming
4. **Package Structure** - Different internal architecture

## Security Considerations

1. **Use HTTPS/WSS** in production
2. **Rotate JWT secrets** regularly
3. **Enable rate limiting** to prevent abuse
4. **Implement IP whitelisting** for admin APIs
5. **Use strong passwords** for RTMP stream keys
6. **Enable audit logging** for compliance
7. **Monitor for anomalies** (unusual traffic patterns)

## Next Steps

- [Quickstart Guide](quickstart.md) - Get started in 5 minutes
- [API Reference](api-reference.md) - Complete API documentation
- [Examples](../examples/) - Working code samples
- [Deployment Guide](deployment.md) - Production deployment

## Contributing

We welcome contributions! See our [Contributing Guide](../CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](../LICENSE) file.
