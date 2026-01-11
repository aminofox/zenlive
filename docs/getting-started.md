# Getting Started with ZenLive

## Introduction

ZenLive is a comprehensive Go SDK for building live streaming applications. It supports multiple protocols (RTMP, HLS, WebRTC), real-time chat, analytics, and advanced interactive features.

## Installation

### Prerequisites

- Go 1.23 or later
- Redis (for caching and session management)
- FFmpeg (optional, for video processing)

### Install the SDK

```bash
go get github.com/aminofox/zenlive
```

## Quick Start

### 1. Basic Streaming Server

Here's a simple example to start an RTMP streaming server:

```go
package main

import (
    "log"
    "github.com/aminofox/zenlive/pkg/streaming/rtmp"
)

func main() {
    // Create RTMP server
    server := rtmp.NewServer(&rtmp.ServerConfig{
        ListenAddr: ":1935",
    })
    
    // Start server
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
    
    log.Println("RTMP server running on :1935")
    select {} // Keep running
}
```

### 2. Publishing a Stream

To publish a stream using OBS or FFmpeg:

```bash
# Using FFmpeg
ffmpeg -re -i input.mp4 -c copy -f flv rtmp://localhost:1935/live/mystream

# OBS Settings
# Server: rtmp://localhost:1935/live
# Stream Key: mystream
```

### 3. Playing a Stream

#### RTMP Playback
```bash
ffplay rtmp://localhost:1935/live/mystream
```

#### HLS Playback
```html
<video controls>
    <source src="http://localhost:8080/live/mystream/index.m3u8" type="application/x-mpegURL">
</video>
```

#### WebRTC Playback
```javascript
const pc = new RTCPeerConnection();
const response = await fetch('/api/webrtc/play/mystream', {
    method: 'POST',
    body: JSON.stringify({ offer: await pc.createOffer() })
});
const answer = await response.json();
await pc.setRemoteDescription(answer);
```

## Core Concepts

### Stream Lifecycle

1. **Create Stream**: Register a new stream with metadata
2. **Publish**: Start publishing video/audio data
3. **Playback**: Viewers connect and watch the stream
4. **Stop**: End the stream and save recording
5. **Archive**: Store stream metadata and recordings

### Authentication

ZenLive provides built-in authentication:

```go
import "github.com/aminofox/zenlive/pkg/auth"

// Create authenticator
authenticator := auth.NewJWTAuthenticator(&auth.JWTConfig{
    SecretKey: "your-secret-key",
    Issuer:    "zenlive",
})

// Generate token
token, err := authenticator.GenerateToken(&auth.User{
    ID:       "user123",
    Username: "streamer",
    Roles:    []string{"publisher"},
})
```

### Stream Protocols

#### RTMP
Best for:
- OBS/Streamlabs streaming
- Low-complexity publishing
- Traditional broadcasting

```go
rtmpServer := rtmp.NewServer(&rtmp.ServerConfig{
    ListenAddr: ":1935",
    EnableAuth: true,
})
```

#### HLS
Best for:
- Wide device compatibility
- Adaptive bitrate streaming
- CDN distribution

```go
hlsServer := hls.NewServer(&hls.ServerConfig{
    ListenAddr:     ":8080",
    SegmentDuration: 4,
    PlaylistSize:   5,
})
```

#### WebRTC
Best for:
- Ultra-low latency (< 1 second)
- Interactive streaming
- Browser-based publishing

```go
webrtcServer := webrtc.NewServer(&webrtc.ServerConfig{
    ListenAddr: ":8443",
    IceServers: []string{"stun:stun.l.google.com:19302"},
})
```

## Project Structure

```
myapp/
├── main.go              # Entry point
├── config.json          # Configuration (optional)
├── handlers/
│   ├── stream.go       # Stream handlers
│   └── chat.go         # Chat handlers
└── storage/
    └── recordings/     # Saved streams
```

## Configuration

### Basic Configuration

```go
type Config struct {
    RTMP struct {
        Port int
        Auth bool
    }
    HLS struct {
        Port            int
        SegmentDuration int
        PlaylistSize    int
    }
    Storage struct {
        Type   string // "local", "s3"
        Path   string
        Bucket string
    }
}
```

## Common Tasks

### Recording Streams

```go
import "github.com/aminofox/zenlive/pkg/storage"

recorder := storage.NewRecorder(&storage.RecorderConfig{
    Format: storage.FormatMP4,
    Storage: localStorage,
})

// Start recording
recorder.Start(streamID)

// Stop recording
recorder.Stop(streamID)
```

### Real-time Chat

```go
import "github.com/aminofox/zenlive/pkg/chat"

chatServer := chat.NewServer(&chat.ServerConfig{
    ListenAddr: ":9000",
})

// Create chat room for stream
room := chatServer.CreateRoom(streamID)

// Send message
room.Broadcast(&chat.Message{
    UserID:  "user123",
    Content: "Hello everyone!",
})
```

### Analytics

```go
import "github.com/aminofox/zenlive/pkg/analytics"

metrics := analytics.NewMetricsCollector()

// Get viewer count
viewers := metrics.GetViewerCount(streamID)

// Get stream statistics
stats := metrics.GetStreamMetrics(streamID)
fmt.Printf("Bitrate: %d kbps, FPS: %d\n", stats.Bitrate, stats.FPS)
```

## Next Steps

- [Architecture Overview](architecture.md) - Understand system design
- [Configuration Guide](configuration.md) - Detailed configuration options
- [Tutorials](tutorials/) - Step-by-step guides
- [API Reference](https://pkg.go.dev/github.com/aminofox/zenlive) - Complete API documentation
- [Examples](../examples/) - Working code examples

## Community & Support

- GitHub Issues: [github.com/aminofox/zenlive/issues](https://github.com/aminofox/zenlive/issues)
- Documentation: [zenlive.dev/docs](https://zenlive.dev/docs)
- Discord: [discord.gg/zenlive](https://discord.gg/zenlive)

## License

ZenLive is open source under the MIT License. See [LICENSE](../LICENSE) for details.
