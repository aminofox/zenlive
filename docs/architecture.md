# ZenLive Architecture

## Overview

ZenLive is designed as a modular, scalable live streaming platform built on modern software architecture principles. The system supports multiple streaming protocols, real-time interactions, and horizontal scaling.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Layer                             │
│  OBS/FFmpeg | Web Browser | Mobile Apps | Third-party Services  │
└──────────────────────┬──────────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────────┐
│                      Protocol Layer                              │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐           │
│  │  RTMP   │  │   HLS   │  │ WebRTC  │  │  Chat   │           │
│  │ Server  │  │ Server  │  │ Server  │  │ Server  │           │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘           │
└───────┼───────────┼────────────┼─────────────┼─────────────────┘
        │           │            │             │
┌───────▼───────────▼────────────▼─────────────▼─────────────────┐
│                     Core Services Layer                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │   Auth   │ │  Stream  │ │Analytics │ │Interactive│          │
│  │ Service  │ │ Manager  │ │ Service  │ │ Features │          │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘          │
└───────┼────────────┼────────────┼────────────┼─────────────────┘
        │            │            │            │
┌───────▼────────────▼────────────▼────────────▼─────────────────┐
│                   Data & Storage Layer                           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │  Redis   │ │   S3/    │ │Prometheus│ │  Audit   │          │
│  │  Cache   │ │  Local   │ │ Metrics  │ │   Logs   │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Protocol Layer

#### RTMP Server (`pkg/streaming/rtmp/`)

**Purpose**: Ingest streams from OBS, Streamlabs, and other RTMP encoders

**Components**:
- `server.go`: TCP listener and connection handler
- `handshake.go`: RTMP handshake protocol (C0, C1, C2, S0, S1, S2)
- `chunk.go`: RTMP chunk protocol handling
- `amf.go`: Action Message Format encoding/decoding
- `types.go`: RTMP message types and constants

**Data Flow**:
```
Publisher → RTMP Handshake → Connect → Publish → Media Chunks
                                                        ↓
                                                  Stream Manager
```

**Key Features**:
- Multiple concurrent streams
- Chunk aggregation and splitting
- Metadata extraction (resolution, bitrate, codec)
- Stream key authentication

#### HLS Server (`pkg/streaming/hls/`)

**Purpose**: Deliver streams to web and mobile clients via HTTP

**Components**:
- `transmuxer.go`: Convert RTMP/WebRTC to HLS (TS segments)
- `segment.go`: Create MPEG-TS segments
- `playlist.go`: Generate M3U8 playlists
- `abr.go`: Adaptive bitrate streaming
- `dvr.go`: DVR (sliding window) support
- `server.go`: HTTP server for segment delivery

**Data Flow**:
```
Stream Manager → Transmuxer → TS Segments → Playlist Generator
                                    ↓              ↓
                                  Cache      HTTP Server → Clients
```

**Key Features**:
- Multiple quality variants (ABR)
- Configurable segment duration (2-10 seconds)
- Sliding window DVR
- CDN-friendly caching headers

#### WebRTC Server (`pkg/streaming/webrtc/`)

**Purpose**: Ultra-low latency streaming for interactive applications

**Components**:
- `signaling.go`: WebSocket signaling server
- `peer.go`: PeerConnection management
- `sfu.go`: Selective Forwarding Unit
- `bwe.go`: Bandwidth estimation (Google Congestion Control)
- `ice.go`: ICE candidate handling

**Data Flow**:
```
Browser → WebSocket Signaling → SDP Exchange → ICE → Media Flow
                                                        ↓
                                                  SFU Router
                                                        ↓
                                              Multiple Subscribers
```

**Key Features**:
- Sub-second latency (< 500ms)
- SFU architecture for scalability
- Automatic bandwidth adaptation
- STUN/TURN support

### 2. Authentication & Authorization (`pkg/auth/`)

**Components**:
- `auth.go`: Core authentication interface
- `jwt.go`: JWT token generation and validation
- `rbac.go`: Role-Based Access Control
- `session.go`: Session management
- `ratelimit.go`: Request rate limiting

**Authentication Flow**:
```
Client → Login Request → Validate Credentials → Generate JWT
                                                      ↓
                                                Store Session
                                                      ↓
                                            Return Token to Client
```

**Authorization Flow**:
```
Request + Token → Validate Token → Extract User → Check Permissions
                                                           ↓
                                                    Allow/Deny
```

**Roles**:
- `admin`: Full system access
- `publisher`: Can create and manage own streams
- `viewer`: Can watch streams
- `moderator`: Can moderate chat and content

### 3. Stream Management (`pkg/sdk/`)

**Purpose**: Central stream lifecycle management

**Components**:
- `stream.go`: Stream CRUD operations
- `state.go`: Stream state machine
- `control.go`: Stream control (start, stop, pause)
- `events.go`: Event system
- `webhook.go`: Webhook delivery

**Stream States**:
```
IDLE → STARTING → LIVE → STOPPING → ENDED
         ↓                    ↓
      FAILED              PAUSED
```

**State Transitions**:
- `IDLE → STARTING`: Stream creation initiated
- `STARTING → LIVE`: First media packet received
- `LIVE → PAUSED`: Stream temporarily paused
- `PAUSED → LIVE`: Stream resumed
- `LIVE → STOPPING`: Stop command received
- `STOPPING → ENDED`: Stream gracefully stopped
- `* → FAILED`: Error occurred

**Events**:
- `OnStreamCreated`: New stream registered
- `OnStreamStarted`: Stream goes live
- `OnStreamEnded`: Stream stopped
- `OnStreamFailed`: Stream error
- `OnViewerJoined`: New viewer connected
- `OnViewerLeft`: Viewer disconnected

### 4. Storage Layer (`pkg/storage/`)

**Purpose**: Recording and media storage

**Components**:
- `recorder.go`: Stream recording engine
- `local.go`: Local filesystem storage
- `s3.go`: S3-compatible storage (AWS, MinIO)
- `thumbnail.go`: Thumbnail extraction
- `metadata.go`: Recording metadata
- `formats/mp4.go`: MP4 muxer
- `formats/flv.go`: FLV muxer

**Recording Flow**:
```
Live Stream → Recorder → Segment Buffer → Muxer → Storage
                              ↓
                      Thumbnail Extractor
```

**Storage Interface**:
```go
type Storage interface {
    Upload(path string, data io.Reader) error
    Download(path string) (io.ReadCloser, error)
    Delete(path string) error
    List(prefix string) ([]string, error)
}
```

### 5. Real-time Chat (`pkg/chat/`)

**Purpose**: Live chat during streams

**Components**:
- `server.go`: WebSocket server
- `room.go`: Chat room management (one per stream)
- `message.go`: Message handling
- `moderation.go`: Moderation features
- `storage.go`: Message persistence

**Message Flow**:
```
Client → WebSocket → Room → Broadcast → All Clients
                       ↓
                  Moderation Filter
                       ↓
                  Message Storage
```

**Moderation Features**:
- User mute/ban
- Message deletion
- Profanity filter
- Rate limiting
- Moderator commands

### 6. Analytics (`pkg/analytics/`)

**Purpose**: Stream and viewer analytics

**Components**:
- `metrics.go`: Metrics collection
- `stream_metrics.go`: Stream-level metrics
- `viewers.go`: Viewer tracking
- `performance.go`: Performance monitoring
- `prometheus.go`: Prometheus exporter
- `report.go`: Report generation

**Metrics Collected**:

**Stream Metrics**:
- Current bitrate (kbps)
- Frame rate (fps)
- Resolution (width x height)
- Dropped frames
- Audio/video codec
- Stream uptime

**Viewer Metrics**:
- Concurrent viewers
- Peak viewers
- Total view count
- Average watch time
- Geographic distribution
- Device/platform breakdown

**System Metrics**:
- CPU usage
- Memory usage
- Network bandwidth
- Disk I/O
- Goroutine count

### 7. Interactive Features (`pkg/interactive/`)

**Purpose**: Engagement and monetization

**Components**:
- `poll.go`: Live polling system
- `gift.go`: Virtual gifts
- `currency.go`: Virtual currency
- `reaction.go`: Real-time reactions

**Poll System**:
```
Create Poll → Broadcast to Viewers → Collect Votes → Real-time Results
```

**Gift System**:
```
Purchase Gift → Validate Currency → Send Animation → Credit Streamer
```

### 8. Security Layer (`pkg/security/`)

**Purpose**: Comprehensive security and compliance

**Components**:
- `tls.go`: Certificate management
- `encryption.go`: Data encryption (AES-256-GCM)
- `ratelimit.go`: Multi-level rate limiting
- `firewall.go`: IP filtering
- `keyrotation.go`: Stream key rotation
- `watermark.go`: Video watermarking
- `audit.go`: Audit logging

**Security Layers**:

1. **Transport Security**: TLS 1.2+ for all connections
2. **Authentication**: JWT with token rotation
3. **Authorization**: RBAC with fine-grained permissions
4. **Rate Limiting**: Global, IP, user, endpoint, stream levels
5. **Encryption**: AES-256-GCM for sensitive data
6. **Audit**: Complete audit trail for compliance

### 9. Cluster & Scalability (`pkg/cluster/`)

**Purpose**: Horizontal scaling and high availability

**Components**:
- `loadbalancer.go`: Load balancing
- `router.go`: Stream routing across nodes
- `session.go`: Distributed session management
- `discovery.go`: Service discovery

**Scaling Architecture**:
```
                    Load Balancer
                          ↓
        ┌─────────────────┼─────────────────┐
        ↓                 ↓                 ↓
   Node 1            Node 2            Node 3
   (RTMP)         (HLS/WebRTC)      (Recording)
        ↓                 ↓                 ↓
              Shared Redis Cluster
                          ↓
              Shared Storage (S3)
```

**Key Features**:
- Consistent hashing for stream routing
- Session affinity for viewers
- Automatic failover
- Health checks
- Service discovery (Consul/etcd)

## Data Flow Examples

### Publishing Flow (RTMP → HLS)

```
1. OBS connects to RTMP server (port 1935)
2. RTMP handshake completed
3. Authentication validates stream key
4. Stream Manager creates stream session
5. RTMP server receives media chunks
6. HLS Transmuxer converts to TS segments
7. Segments stored in cache
8. M3U8 playlist updated
9. CDN pulls segments
10. Viewers fetch playlist and segments
```

### Playback Flow (WebRTC)

```
1. Browser requests stream via WebSocket
2. Signaling server creates SDP offer
3. ICE candidates exchanged
4. DTLS/SRTP established
5. SFU routes media to subscriber
6. Bandwidth adaptation adjusts quality
7. Real-time metrics collected
```

### Recording Flow

```
1. Stream goes LIVE
2. Recorder starts for stream
3. Media packets buffered
4. MP4 muxer creates fragments
5. Fragments uploaded to S3
6. Thumbnails extracted every N seconds
7. Stream ENDS
8. Final MP4 finalized
9. Metadata saved
10. Cleanup temporary files
```

## Performance Characteristics

### Latency

- **RTMP**: 5-15 seconds
- **HLS**: 10-30 seconds (standard), 2-6 seconds (LL-HLS)
- **WebRTC**: 0.5-2 seconds

### Scalability

- **Streams per Node**: 500-1000 (depending on bitrate)
- **Viewers per Stream**: 10,000+ (with CDN)
- **WebRTC Viewers**: 50-100 per SFU node

### Resource Usage

- **RTMP Stream**: ~100 MB RAM per stream
- **HLS Transmuxing**: ~50 MB RAM per stream
- **WebRTC Peer**: ~10 MB RAM per connection
- **Recording**: ~200 MB RAM + disk I/O

## Design Patterns

### 1. Interface-Based Design

All major components use interfaces for flexibility:

```go
type StreamProvider interface {
    Start() error
    Stop() error
    Publish(stream Stream) error
    Subscribe(stream Stream) (MediaStream, error)
}
```

### 2. Event-Driven Architecture

Events propagate through the system:

```go
type EventBus interface {
    Publish(event Event)
    Subscribe(eventType EventType, handler Handler)
}
```

### 3. Middleware Pattern

Request processing uses middleware:

```go
type Middleware func(HandlerFunc) HandlerFunc

chain := Chain(
    AuthMiddleware,
    RateLimitMiddleware,
    LoggingMiddleware,
)
```

### 4. Repository Pattern

Data access abstracted:

```go
type StreamRepository interface {
    Create(stream *Stream) error
    Get(id string) (*Stream, error)
    Update(stream *Stream) error
    Delete(id string) error
}
```

## Technology Stack

- **Language**: Go 1.23+
- **Protocols**: RTMP, HLS, WebRTC
- **WebRTC**: Pion WebRTC
- **Cache**: Redis
- **Storage**: S3-compatible (AWS S3, MinIO)
- **Metrics**: Prometheus
- **Logging**: Zap (structured logging)
- **Testing**: Testify, Go testing package

## Configuration Management

Configuration follows the 12-factor app principles:

```go
type Config struct {
    Environment string // dev, staging, production
    
    RTMP    RTMPConfig
    HLS     HLSConfig
    WebRTC  WebRTCConfig
    Auth    AuthConfig
    Storage StorageConfig
    Redis   RedisConfig
}
```

## Security Architecture

### Defense in Depth

1. **Network Layer**: Firewall, DDoS protection
2. **Transport Layer**: TLS 1.2+, certificate pinning
3. **Application Layer**: Authentication, authorization
4. **Data Layer**: Encryption at rest and in transit

### Compliance

- **GDPR**: User data encryption, right to deletion, audit logs
- **COPPA**: Age verification, parental consent
- **DMCA**: Content takedown procedures
- **SOC 2**: Security controls, audit trails

## Monitoring & Observability

### Metrics (Prometheus)

- Request rate, latency, errors
- Stream count, viewer count
- Resource usage (CPU, memory, network)
- Custom business metrics

### Logging (Structured)

```json
{
  "timestamp": "2026-01-11T10:30:00Z",
  "level": "INFO",
  "component": "rtmp-server",
  "stream_id": "stream123",
  "event": "stream_started",
  "user_id": "user456"
}
```

### Tracing (OpenTelemetry)

Distributed tracing for request flow across services

### Health Checks

- Liveness: Is service running?
- Readiness: Can service handle requests?
- Dependency checks: Redis, S3, etc.

## Future Enhancements

- **AI/ML**: Content moderation, highlight detection
- **Edge Computing**: CDN-integrated processing
- **Multi-region**: Global stream distribution
- **Advanced Analytics**: ML-powered insights
- **Enhanced Interactivity**: AR filters, 3D effects

## References

- [Getting Started](getting-started.md)
- [Configuration Guide](configuration.md)
- [API Documentation](https://pkg.go.dev/github.com/aminofox/zenlive)
# ZenLive SDK Philosophy

## Design Principles

### 1. **Core Focus: Real-time Communication**

ZenLive SDK focuses on **real-time delivery** of streams and messages. We do NOT handle application-level data persistence.

**What we do:**
- ✅ Real-time RTMP/HLS/WebRTC streaming
- ✅ Real-time chat message delivery via WebSocket
- ✅ In-memory session management
- ✅ Real-time analytics metrics

**What we DON'T do:**
- ❌ Database persistence (users handle their own data)
- ❌ Long-term storage of chat history
- ❌ User account management
- ❌ Application business logic

### 2. **User Responsibility: Data Persistence**

Users are responsible for persisting data to their own database based on their needs.

**Examples:**

#### Chat History
The SDK delivers chat messages in real-time. If you need to save chat history:

```go
// Your application code
chatServer.SetOnMessage(func(msg *chat.Message) {
    // 1. SDK delivers message to all connected clients (real-time)
    
    // 2. YOU decide if/how to persist it
    db.SaveChatMessage(msg) // Your database logic
})
```

#### Stream Metadata
```go
// Your application code  
sdk.SetOnStreamStart(func(stream *types.Stream) {
    // 1. SDK handles stream ingestion/delivery
    
    // 2. YOU save metadata
    db.SaveStreamInfo(stream) // Your database logic
})
```

#### User Actions
```go
// Your application code
sdk.SetOnReaction(func(userID, streamID string, reaction string) {
    // 1. SDK delivers reaction in real-time
    
    // 2. YOU log it if needed
    db.LogUserAction(userID, "reaction", reaction) // Your choice
})
```

### 3. **Redis for Cluster Only**

Redis is **ONLY** used for distributed session management in cluster mode.

**When Redis is required:**
- ✅ Multi-server deployment (`Cluster.Enabled = true`)
- ✅ Horizontal scaling
- ✅ Session sharing across nodes

**When Redis is NOT needed:**
- ❌ Single server deployment
- ❌ Development/testing
- ❌ Small-scale applications

**What Redis stores:**
- Session state across cluster nodes
- Stream routing information
- Node health status

**What Redis does NOT store:**
- Chat messages (real-time only)
- User data (your responsibility)
- Application state (your responsibility)

### 4. **Chat is Just an Action**

Chat is an **optional feature** during video/audio calls, not a core requirement.

**For Livestreaming:**
- Chat is important for viewer interaction
- Enable with `Chat.Enabled = true`
- In-memory history for current session only

**For Video/Audio Calls:**
- Chat is optional (like a text message during a call)
- Can disable with `Chat.Enabled = false`
- No persistence needed (just real-time delivery)

**Configuration:**
```go
// Livestream - enable chat
cfg.Chat.Enabled = true
cfg.Chat.EnablePersistence = false  // In-memory only

// Video call - disable chat
cfg.Chat.Enabled = false  // Users can handle text separately

// Video call with optional chat
cfg.Chat.Enabled = true
cfg.Chat.EnablePersistence = false  // Just for the call duration
```

---

## Architecture Layers

### Layer 1: SDK (ZenLive)
**Responsibility:** Real-time communication infrastructure

- Stream ingestion (RTMP)
- Stream delivery (HLS, WebRTC)
- Real-time chat delivery
- Session management
- Analytics metrics

### Layer 2: Application (Your Code)
**Responsibility:** Business logic and data persistence

- User authentication/authorization
- Database schema design
- Chat history storage
- Stream metadata storage
- Analytics aggregation
- Business rules

### Layer 3: Storage (Your Choice)
**Responsibility:** Data persistence

- PostgreSQL, MySQL, MongoDB, etc.
- Your schema, your rules
- Your backup strategy
- Your scaling approach

---

## Configuration Philosophy

### Minimal Defaults

The default configuration is optimized for **quick start** and **single server** deployment:

```go
cfg := config.DefaultConfig()
// Cluster: disabled
// Redis: disabled  
// Analytics: disabled
// Chat persistence: disabled (in-memory only)
```

### Progressive Enhancement

Enable features as you scale:

**Day 1 - Development**
```go
cfg := config.DefaultConfig()
cfg.Streaming.EnableRTMP = true
cfg.Streaming.EnableHLS = true
```

**Week 1 - Add Chat**
```go
cfg.Chat.Enabled = true
cfg.Chat.EnablePersistence = false  // Real-time only
```

**Month 1 - Production**
```go
cfg.Analytics.Enabled = true
cfg.Analytics.EnablePrometheus = true
```

**Month 3 - Scale Horizontally**
```go
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true  // Now required
cfg.Redis.Host = "redis.example.com"
```

---

## Data Flow Examples

### Example 1: Livestream with Chat

```
┌─────────────┐
│   Streamer  │
│    (OBS)    │
└──────┬──────┘
       │ RTMP
       ▼
┌─────────────────────────────────────┐
│         ZenLive SDK                 │
│  ┌──────────────────────────────┐  │
│  │ Real-time Stream Delivery    │──┼──HLS──▶ Viewers
│  │ (no persistence)             │  │
│  └──────────────────────────────┘  │
│                                     │
│  ┌──────────────────────────────┐  │
│  │ Real-time Chat Delivery      │──┼──WS───▶ Viewers
│  │ (in-memory buffer only)      │  │
│  └──────────────────────────────┘  │
└──────────┬──────────────────────────┘
           │ Events (OnMessage, OnStream, etc.)
           ▼
┌─────────────────────────────────────┐
│      Your Application               │
│  ┌──────────────────────────────┐  │
│  │ Save chat to DB if needed    │  │
│  │ Save stream metadata         │  │
│  │ Business logic               │  │
│  └──────────────────────────────┘  │
└──────────┬──────────────────────────┘
           ▼
┌─────────────────────────────────────┐
│      Your Database                  │
│  (PostgreSQL, MySQL, etc.)          │
└─────────────────────────────────────┘
```

### Example 2: Video Call (No Chat)

```
┌─────────────┐                    ┌─────────────┐
│   User A    │                    │   User B    │
└──────┬──────┘                    └──────┬──────┘
       │                                  │
       │          WebRTC P2P              │
       └──────────────┬───────────────────┘
                      ▼
         ┌─────────────────────────┐
         │    ZenLive SDK          │
         │  ┌───────────────────┐  │
         │  │ WebRTC Signaling  │  │
         │  │ STUN/TURN         │  │
         │  │ (no persistence)  │  │
         │  └───────────────────┘  │
         └─────────────────────────┘

         No database needed
         No chat needed
         Just real-time connection
```

### Example 3: Multi-Server Cluster

```
┌─────────────────────────────────────────────────────┐
│                  Your Application                   │
│              (with your own database)               │
└─────────────────────────────────────────────────────┘
                      ▲
                      │ Events
                      │
┌─────────────────────┼─────────────────────────────┐
│                     │                             │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  │
│  │ ZenLive    │  │ ZenLive    │  │ ZenLive    │  │
│  │ Node 1     │  │ Node 2     │  │ Node 3     │  │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  │
│        │               │               │         │
│        └───────────────┼───────────────┘         │
│                        │                         │
│                        ▼                         │
│              ┌─────────────────┐                 │
│              │  Redis Cluster  │                 │
│              │ (session state) │                 │
│              └─────────────────┘                 │
│                                                  │
└──────────────────────────────────────────────────┘

Redis stores: session routing only
Your DB stores: all application data
```

---

## Best Practices

### ✅ DO

1. **Handle your own persistence**
   ```go
   chatServer.OnMessage(func(msg *chat.Message) {
       // Deliver in real-time
       chatServer.Broadcast(msg)
       
       // Save to YOUR database
       myDB.SaveChatMessage(msg)
   })
   ```

2. **Use Redis only for cluster**
   ```go
   if cfg.Cluster.Enabled {
       cfg.Redis.Enabled = true  // Required
   }
   ```

3. **Disable chat for simple calls**
   ```go
   // 1-1 video call
   cfg.Chat.Enabled = false
   ```

4. **Start simple, scale up**
   ```go
   // Start
   cfg := config.DefaultConfig()
   
   // Scale later
   cfg.Cluster.Enabled = true
   cfg.Redis.Enabled = true
   ```

### ❌ DON'T

1. **Don't expect SDK to persist your data**
   ```go
   // ❌ Wrong expectation
   chatServer.Send(msg)
   // Expecting SDK to save to database - IT WON'T
   
   // ✅ Correct approach
   chatServer.Send(msg)
   myDB.Save(msg)  // YOU do this
   ```

2. **Don't enable Redis without cluster**
   ```go
   // ❌ Waste of resources
   cfg.Cluster.Enabled = false
   cfg.Redis.Enabled = true  // Not needed!
   
   // ✅ Correct
   cfg.Cluster.Enabled = true
   cfg.Redis.Enabled = true  // Now required
   ```

3. **Don't use chat persistence for database storage**
   ```go
   // ❌ Misunderstanding
   cfg.Chat.EnablePersistence = true  // This is in-memory only!
   
   // ✅ Correct understanding
   cfg.Chat.EnablePersistence = false  // In-memory buffer only
   myDB.SaveMessages()  // Your responsibility
   ```

---

## FAQ

### Q: Where does the SDK store chat messages?

**A:** In-memory only. The SDK delivers messages in real-time. If you need history, save to your own database.

### Q: Do I need a database to use ZenLive?

**A:** No. The SDK doesn't require any database. If YOUR application needs data persistence, use your own database.

### Q: When do I need Redis?

**A:** Only when `Cluster.Enabled = true` for multi-server deployments. Single server doesn't need Redis.

### Q: Can I use MongoDB/MySQL/PostgreSQL?

**A:** Yes! Use any database you want for YOUR application data. The SDK doesn't care.

### Q: Is chat required for video calls?

**A:** No. Chat is optional. Disable it with `Chat.Enabled = false` for simple calls.

### Q: How do I persist stream recordings?

**A:** Set `Storage.Type = "s3"` or `"local"`. The SDK handles recording files, but stream metadata is your responsibility.

### Q: What happens to chat when EnablePersistence = true?

**A:** Messages are kept in memory for the current session only. NOT saved to database. You must handle database persistence yourself.

---

## Summary

| Component | SDK Responsibility | User Responsibility |
|-----------|-------------------|---------------------|
| **Streaming** | Real-time delivery | Metadata storage |
| **Chat** | Real-time delivery | Message history storage |
| **Sessions** | In-memory management | User account management |
| **Analytics** | Real-time metrics | Long-term aggregation |
| **Redis** | Cluster session state | Application caching (optional) |
| **Database** | ❌ None | ✅ All application data |

**Bottom line:** ZenLive handles real-time communication. You handle data persistence and business logic.
# ZenLive SDK - Configuration and Architecture Summary

## 📋 Quick Reference

### What ZenLive SDK Does
- ✅ Real-time RTMP/HLS/WebRTC streaming
- ✅ Real-time chat message delivery
- ✅ Session management (in-memory or distributed via Redis)
- ✅ Real-time analytics metrics
- ✅ Stream recording to local/S3 storage

### What ZenLive SDK Does NOT Do
- ❌ Database persistence for application data
- ❌ Long-term chat history storage
- ❌ User account management
- ❌ Application business logic
- ❌ Schema design

### Your Responsibilities
- 💾 Persist data to your own database (PostgreSQL, MySQL, MongoDB, etc.)
- 📝 Design your own database schema
- 👤 Manage user accounts and authentication
- 📊 Long-term analytics aggregation
- 💬 Save chat history if needed

---

## 🏗️ Core Components

### Required Components
1. **Streaming** (`pkg/streaming/`) - RTMP, HLS, WebRTC
2. **Auth** (`pkg/auth/`) - JWT authentication, RBAC
3. **Storage** (`pkg/storage/`) - Recording to local/S3
4. **Logger** (`pkg/logger/`) - Logging
5. **Types** (`pkg/types/`) - Common types
6. **Errors** (`pkg/errors/`) - Error handling

### Optional Components
1. **Analytics** (`pkg/analytics/`) - Real-time metrics, Prometheus
2. **Cluster** (`pkg/cluster/`) - Multi-server deployments
3. **Redis** - Required when `Cluster.Enabled = true`
4. **Chat** (`pkg/chat/`) - Real-time chat (disable for simple video calls)
5. **Interactive** (`pkg/interactive/`) - Polls, gifts, reactions
6. **Security** (`pkg/security/`) - Advanced security features
7. **CDN** (`pkg/cdn/`) - CDN integration
8. **SDK** (`pkg/sdk/`) - Client SDK

---

## 🔧 Configuration Structure

```go
type Config struct {
    Server    ServerConfig    // Required: Server settings
    Auth      AuthConfig      // Required: JWT authentication
    Storage   StorageConfig   // Required: Recording storage
    Streaming StreamingConfig // Required: RTMP/HLS/WebRTC
    Chat      ChatConfig      // Optional: Real-time chat
    Analytics AnalyticsConfig // Optional: Metrics
    Cluster   ClusterConfig   // Optional: Multi-server
    Redis     RedisConfig     // Required when Cluster.Enabled = true
    Logging   LoggingConfig   // Required: Logging
}
```

**Note:** No database config in SDK. You handle your own database.

---

## 📊 Use Case Matrix

| Use Case | RTMP | HLS | WebRTC | Chat | Analytics | Cluster | Redis | Database |
|----------|------|-----|--------|------|-----------|---------|-------|----------|
| **Livestream Platform** | ✅ | ✅ | ❌ | ✅ | ✅ | Optional | Optional | Your DB |
| **1-1 Video Call** | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | Your DB |
| **Audio Call** | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | Your DB |
| **Multi-Server Production** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Your DB |

---

## 💬 Chat Philosophy

**Chat is an optional action, not a core requirement.**

### Livestream
- Chat is important for viewer interaction
- Enable with `Chat.Enabled = true`
- SDK delivers messages in real-time
- YOU save to database if you want history

### Video/Audio Calls
- Chat is optional (like texting during a phone call)
- Can disable with `Chat.Enabled = false`
- If enabled, it's just another real-time action
- No persistence needed

### Configuration
```go
// Livestream - enable chat
cfg.Chat.Enabled = true
cfg.Chat.EnablePersistence = false  // In-memory only

// Your code - handle persistence
chatServer.OnMessage(func(msg *chat.Message) {
    // SDK delivers in real-time
    
    // YOU save to YOUR database
    myDB.SaveChatMessage(msg)
})
```

```go
// Video call - disable chat
cfg.Chat.Enabled = false
```

---

## 🔴 Redis Philosophy

**Redis is ONLY for cluster mode distributed sessions.**

### When Redis is Required
- Multi-server deployment
- `Cluster.Enabled = true`
- Distributed session management

### When Redis is NOT Needed
- Single server deployment
- Development/testing
- No horizontal scaling

### What Redis Stores
- Session state across cluster nodes
- Stream routing information
- Node health status

### What Redis Does NOT Store
- Chat messages (your database)
- User data (your database)
- Application data (your database)

### Configuration
```go
// Single server - no Redis
cfg.Cluster.Enabled = false
cfg.Redis.Enabled = false

// Multi-server - Redis required
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true  // Must be true
cfg.Redis.Host = "redis.example.com"
```

---

## 📁 Configuration Files

### 1. Development (Single Server)
**File:** `examples/config/livestream-simple.json`

```json
{
  "streaming": { "enable_rtmp": true, "enable_hls": true },
  "chat": { "enabled": true, "enable_persistence": false },
  "analytics": { "enabled": false },
  "cluster": { "enabled": false },
  "redis": { "enabled": false }
}
```

### 2. Video/Audio Call
**File:** `examples/config/video-call.json`

```json
{
  "streaming": { "enable_webrtc": true },
  "chat": { "enabled": false },
  "cluster": { "enabled": false },
  "redis": { "enabled": false }
}
```

### 3. Production (Multi-Server)
**File:** `examples/config/production-distributed.json`

```json
{
  "streaming": { "enable_rtmp": true, "enable_hls": true, "enable_webrtc": true },
  "chat": { "enabled": true, "enable_persistence": false },
  "analytics": { "enabled": true },
  "cluster": { "enabled": true },
  "redis": { "enabled": true, "host": "redis.example.com" }
}
```

---

## 🚀 Quick Start Guide

### Step 1: Install
```bash
go get github.com/aminofox/zenlive@v1.0.0
```

### Step 2: Basic Configuration
```go
package main

import (
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    // Start with defaults
    cfg := config.DefaultConfig()
    
    // Customize for your use case
    cfg.Streaming.EnableRTMP = true
    cfg.Streaming.EnableHLS = true
    cfg.Chat.Enabled = true
    cfg.Chat.EnablePersistence = false  // In-memory only
    
    // Create SDK
    sdk, err := zenlive.New(cfg)
    if err != nil {
        panic(err)
    }
    
    // Start
    if err := sdk.Start(); err != nil {
        panic(err)
    }
    defer sdk.Stop()
    
    // YOUR database for application data
    // db := sql.Open("postgres", "...")
    
    select {}
}
```

### Step 3: Handle Data Persistence (Your Responsibility)
```go
// Example: Save chat messages to YOUR database
chatServer := sdk.GetChatServer()

chatServer.OnMessage(func(msg *chat.Message) {
    // 1. SDK delivers in real-time
    chatServer.Broadcast(msg)
    
    // 2. YOU save to YOUR database
    _, err := db.Exec(
        "INSERT INTO chat_messages (user_id, room_id, message, timestamp) VALUES ($1, $2, $3, $4)",
        msg.UserID, msg.RoomID, msg.Content, msg.Timestamp,
    )
    if err != nil {
        log.Error("Failed to save message", err)
    }
})
```

---

## 📚 Documentation

1. **[SDK_PHILOSOPHY.md](docs/SDK_PHILOSOPHY.md)** - Design principles and philosophy
2. **[architecture-analysis.md](docs/architecture-analysis.md)** - Component analysis
3. **[examples/config/README.md](examples/config/README.md)** - Configuration examples
4. **[getting-started.md](docs/getting-started.md)** - Getting started guide

---

## ❓ FAQ

### Q: Do I need a database to use ZenLive?
**A:** No. ZenLive SDK doesn't require or include database configuration. If YOUR application needs data persistence, use your own database.

### Q: Where does chat history get stored?
**A:** Nowhere automatically. SDK delivers chat in real-time. YOU must save messages to YOUR database if you want history.

### Q: What does `Chat.EnablePersistence = true` do?
**A:** It enables in-memory message buffering for the current session only. NOT database persistence. You still need to save to your own database.

### Q: When do I need Redis?
**A:** Only when `Cluster.Enabled = true` for multi-server deployments. Single server doesn't need Redis.

### Q: Can I use MongoDB/PostgreSQL/MySQL?
**A:** Yes! Use any database you want for YOUR application data. The SDK doesn't care or manage your database.

### Q: Is chat required for video calls?
**A:** No. Chat is optional. Disable it with `Chat.Enabled = false` for simple video calls.

### Q: How do I persist stream metadata?
**A:** Listen to SDK events and save to YOUR database:
```go
sdk.OnStreamStart(func(stream *types.Stream) {
    // SDK handles streaming
    
    // YOU save metadata
    db.Exec("INSERT INTO streams ...")
})
```

---

## ✅ Summary

| Aspect | SDK Responsibility | Your Responsibility |
|--------|-------------------|---------------------|
| **Streaming** | Real-time delivery | Metadata storage |
| **Chat** | Real-time delivery | History storage |
| **Sessions** | In-memory / Redis | User accounts |
| **Analytics** | Real-time metrics | Long-term aggregation |
| **Redis** | Cluster sessions only | Application caching |
| **Database** | ❌ None | ✅ All application data |

**Bottom Line:** ZenLive handles real-time communication. You handle data persistence and business logic.
# ZenLive Architecture Analysis

## 📦 Package Components Classification

This document analyzes which components are **core/required** vs **optional** for different use cases.

## 🎯 SDK Philosophy

**ZenLive is a real-time communication SDK.** We focus on delivering streams and messages in real-time. We do NOT handle application-level data persistence.

- ✅ **SDK handles:** Real-time delivery (streaming, chat, sessions)
- ❌ **SDK does NOT handle:** Database persistence (your responsibility)
- 🔴 **Redis:** Only for cluster mode (distributed sessions)
- 💬 **Chat:** Optional action during calls, real-time delivery only

**See [SDK_PHILOSOPHY.md](SDK_PHILOSOPHY.md) for detailed design principles.**

---

## 🎯 Core Components (Always Required)

### 1. **Streaming** (`pkg/streaming/`)
- **Required for**: ALL use cases
- **Protocols**: RTMP, HLS, WebRTC
- **Purpose**: Core streaming functionality
- **Config**: `config.Streaming`

### 2. **Auth** (`pkg/auth/`)
- **Required for**: Production deployments
- **Purpose**: JWT authentication, RBAC, session management
- **Config**: `config.Auth`

### 3. **Storage** (`pkg/storage/`)
- **Required for**: Recording, playback
- **Purpose**: Local/S3 storage, recording, thumbnails
- **Config**: `config.Storage`

### 4. **Errors** (`pkg/errors/`)
- **Required for**: ALL use cases
- **Purpose**: Error handling

### 5. **Logger** (`pkg/logger/`)
- **Required for**: ALL use cases
- **Purpose**: Logging
- **Config**: `config.Logging`

### 6. **Types** (`pkg/types/`)
- **Required for**: ALL use cases
- **Purpose**: Common types and interfaces

---

## ⚙️ Optional Components

### 1. **Analytics** (`pkg/analytics/`) ⚠️

**When to enable:**
- Production monitoring
- Tracking viewer metrics
- Performance monitoring
- Prometheus integration

**When to disable:**
- Simple streaming apps
- Development/testing
- Minimal resource usage

**Configuration:**
```go
Analytics: AnalyticsConfig{
    Enabled:          false,  // Disable by default
    EnablePrometheus: false,
    PrometheusPort:   9090,
}
```

**Dependencies:** None

---

### 2. **Cluster** (`pkg/cluster/`) ⚠️

**When to enable:**
- Multi-server deployments
- Horizontal scaling
- Load balancing
- Distributed sessions

**When to disable:**
- Single server deployments
- Small-scale applications
- Development/testing

**Configuration:**
```go
Cluster: ClusterConfig{
    Enabled:       false,  // Disable by default
    NodeID:        "",
    DiscoveryType: "inmemory",
    VirtualNodes:  150,
}
```

**Dependencies:** 
- Requires `config.Redis.Enabled = true` for distributed sessions
- Optional: Service discovery (Consul, etcd)

---

### 3. **Chat** (`pkg/chat/`) 🎯

**When to enable:**
- Livestreaming platforms ✅
- Interactive streams ✅
- Optional for video/audio calls (just an action)

**When to disable:**
- Simple 1-1 video calls ❌
- Simple 1-1 audio calls ❌
- When not needed

**Important Notes:**
- SDK provides **real-time delivery only**
- **Users are responsible** for persisting chat history to their own database
- `EnablePersistence = false` means in-memory buffer only (not database)

**Configuration:**
```go
Chat: ChatConfig{
    Enabled:            true,   // Enable for livestream
    MaxMessageLength:   500,
    RateLimitPerSecond: 5,
    EnablePersistence:  false,  // In-memory only, YOU handle DB
}
```

**Usage Example:**
```go
ch✅ Multi-server deployment (`Cluster.Enabled = true`)
- ✅ Horizontal scaling
- ✅ Distributed session management

**When to disable:**
- ❌ Single server deployment
- ❌ Development/testing
- ❌ No clustering

**Important Notes:**
- **Required when `Cluster.Enabled = true`**
- Used ONLY for distributed session state
- NOT for chat persistence (that's your responsibility)
- NOT for application caching (unless you implement it)

**Configuration:**
```go
Redis: RedisConfig{
    Enabled:    true,   // Required for cluster
    Host:       "localhost",
    Port:       6379,
    Password:   "",
    DB:         0,
    PoolSize:   10,
    MaxRetries: 3,
    SessionTTL: 24 * time.Hour,
}
```

**Used by:**
- `pkg/cluster/session.go` - Distributed session management across nodes

**NOT used for:**
- Chat message storage (real-time only)
- User data (your database)
- Application caching (your choice)

---

### 5. **Database** ❌ REMOVED

**ZenLive SDK does NOT include database configuration.**

**Reason:** Data persistence is the user's responsibility. The SDK focuses on real-time communication only.

**What this means:**
- SDK delivers streams and messages in real-time
- YOU decide what to persist and how
- Use PostgreSQL, MySQL, MongoDB, etc. - your choice
- Design your own schema for your needs

**Example - You handle persistence:**
```go
// Your application code
sdk.OnStreamStart(func(stream *types.Stream) {
    // SDK handles real-time delivery
    
    // YOU save metadata
    myDB.Exec("INSERT INTO streams ...")
})

chatServer.OnMessage(func(msg *chat.Message) {
    // SDK delivers in real-time
    
    // YOU save history
    myDB.Exec("INSERT INTO messages ...")
}) Stateless services
- Simple use cases

**Configuration:**
```go
Database: DatabaseConfig{
    Enabled:            false,  // Disable by default
    Type:               "postgres",
    Host:               "localhost",
    Port:               5432,
    Database:           "zenlive",
    Username:           "postgres",
    Password:           "",
    MaxConnections:     25,
    MaxIdleConnections: 5,
    ConnectionLifetime: 5 * time.Minute,
}
```

---

### 6. **CDN** (`pkg/cdn/`) ⚠️

**When to enable:**
- Global content delivery
- Edge caching
- High traffic

**When to disable:**
- Local/regional deployment
- Development
- Low traffic

**Dependencies:** CDN provider configuration (Cloudflare, CloudFront, etc.)

---

### 7. **Interactive** (`pkg/interactive/`) ⚠️

**When to enable:**
- Virtual gifts
- Polls
- Reactions
- Gamification

**When to disable:**
- Simple streaming
- Basic video calls
- No monetization

---

### 8. **Security** (`pkg/security/`) 🔒

**When to enable:**
- Production deployments
- Enterprise use
- Compliance requirements

**Features:**
- Firewall
- Encryption
- Audit logging
- Key rotation
- Watermarking

---

### 9. **SDK** (`pkg/sdk/`) 📱

**Purpose:** Client SDK for remote control and querying
**When to enable:** External API access, remote management

---

## 📊 Use Case Recommendations

### 🎥 Livestreaming Platform (e.g., Twitch, YouTube Live)

**Enable:**
- ✅ Streaming (RTMP, HLS)
- ✅ Chat
- ✅ Auth
- ✅ Storage
- ✅ Analytics
- ✅ Interactive (gifts, polls)
- ✅ Security

**Optional:**
- Cluster (if scaling)
- Redis (if distributed)
- Database (for persistence)

**Sample Config:**
```go
cfg := config.DefaultConfig()
cfg.Streaming.EnableRTMP = true
cfg.Streaming.EnableHLS = true
cfg.Chat.Enabled = true
cfg.Analytics.Enabled = true
cfg.Redis.Enabled = false  // Single server
cfg.Cluster.Enabled = false
```

---

### 📹 1-1 Video Call (e.g., Zoom, Google Meet)

**Enable:**
- ✅ Streaming (WebRTC only)
- ✅ Auth
- ❌ Chat (not needed)
- ❌ Storage (optional)
- ❌ Analytics (optional)

**Sample Config:**
```go
cfg := config.DefaultConfig()
cfg.Streaming.EnableRTMP = false
cfg.Streaming.EnableHLS = false
cfg.Streaming.EnableWebRTC = true
cfg.Chat.Enabled = false  // Not needed for 1-1 calls
cfg.Analytics.Enabled = false
cfg.Storage.Type = "none"  // No recording
```

---

### 📞 Audio Call

**Enable:**
- ✅ Streaming (WebRTC - audio only)
- ✅ Auth
- ❌ Chat
- ❌ Storage
- ❌ Analytics

**Sample Config:**
```go
cfg := config.DefaultConfig()
cfg.Streaming.EnableRTMP = false
cfg.Streaming.EnableHLS = false
cfg.Streaming.EnableWebRTC = true
cfg.Chat.Enabled = false
cfg.Analytics.Enabled = false
```

---

### 🏢 Enterprise Deployment (Multi-region, High Scale)

**Enable:**
- ✅ ALL streaming protocols
- ✅ Chat
- ✅ Auth + RBAC
- ✅ Storage (S3)
- ✅ Analytics + Prometheus
- ✅ Cluster
- ✅ Redis
- ✅ Database
- ✅ Security (full)
- ✅ CDN

**Sample Config:**
```go
cfg := config.DefaultConfig()
cfg.Cluster.Enabled = true
cfg.Cluster.NodeID = "node-1"
cfg.Redis.Enabled = true
cfg.Redis.Host = "redis.example.com"
cfg.Database.Enabled = true
cfg.Analytics.Enabled = true
cfg.Storage.Type = "s3"
```

---

## 🚀 Migration Guide

### From Simple to Complex

**Step 1: Basic Streaming (Day 1)**
```go
cfg := &config.Config{
    Streaming: config.StreamingConfig{
        EnableRTMP: true,
        EnableHLS: true,
    },
}
```

**Step 2: Add Chat (Week 1)**
```go
cfg.Chat.Enabled = true
```

**Step 3: Add Analytics (Week 2)**
```go
cfg.Analytics.Enabled = true
cfg.Analytics.EnablePrometheus = true
```

**Step 4: Scale to Multiple Servers (Month 1)**
```go
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true
cfg.Redis.Host = "redis.example.com"
```

---

## ⚡ Performance Impact

| Component | Memory Usage | CPU Usage | Network | Required External Service |
|-----------|-------------|-----------|---------|--------------------------|
| Streaming | High | High | High | ❌ None |
| Chat | Medium | Low | Medium | ❌ None |
| Analytics | Medium | Medium | Low | ❌ None (Optional: Prometheus) |
| Cluster | Low | Low | Low | ✅ Redis (required when enabled) |
| Redis | - | - | Medium | ✅ Redis Server (cluster mode only) |

**Note:** Database is not included in SDK. Users handle their own data persistence.

---

## 📝 Configuration Best Practices

### Development
```go
cfg := config.DefaultConfig()
cfg.Analytics.Enabled = false
cfg.Cluster.Enabled = false
cfg.Redis.Enabled = false
cfg.Chat.EnablePersistence = false  // In-memory only
cfg.Logging.Level = "debug"
cfg.Logging.Format = "text"
```

### Production
```go
cfg := config.DefaultConfig()
cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")
cfg.Analytics.Enabled = true
cfg.Logging.Level = "info"
cfg.Logging.Format = "json"
cfg.Storage.Type = "s3"
cfg.Storage.S3.Bucket = "my-bucket"

// YOUR database for application data
myDB := sql.Open("postgres", "...")
```

### High Availability
```go
cfg := config.DefaultConfig()
cfg.Cluster.Enabled = true
cfg.Redis.Enabled = true  // Required for cluster
cfg.Redis.Host = "redis.example.com"

// YOUR database for application data  
myDB := sql.Open("postgres", "...")
```

---

## 🎯 Summary

### Must Have (Core)
- Streaming
- Auth
- Storage
- Logger
- Types
- Errors

### Should Have (Recommended)
- Chat (for livestreaming)
- Analytics (for production)
- Security (for production)

### Could Have (Optional)
- Cluster (for scaling)
- Redis (required when cluster enabled)
- CDN (for global delivery)
- Interactive (for engagement)
- SDK (for API access)

### User Responsibility (Not in SDK)
- Database (PostgreSQL, MySQL, MongoDB, etc.)
- Chat history persistence
- User account management
- Stream metadata storage
- Business logic
- Application caching
