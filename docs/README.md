# ZenLive Documentation

**ZenLive** is a Go SDK for building live streaming platforms with RTMP, HLS, WebRTC, real-time chat, and analytics.

## 📖 Documentation

### 🚀 [QUICKSTART.md](QUICKSTART.md) - Get Started in 5 Minutes ⭐

**Read this first!** Learn how to integrate the SDK into your project.

**Contents:**
- Installation & basic setup
- 3 steps to run a streaming server
- Common use cases (livestream, video call, conference)
- Configuration templates (dev, production, cluster)
- Adding features (auth, chat, recording, analytics)
- Troubleshooting

### 🏗️ [ARCHITECTURE.md](ARCHITECTURE.md) - Understand the SDK

**Read this to understand how ZenLive works.**

**Contents:**
- System overview & data flow
- Core components (RTMP, HLS, WebRTC, Chat, Storage)
- SDK philosophy (real-time vs persistence)
- Performance & scalability
- Deployment architectures

## 🎯 Where to Start?

### "I want to integrate the SDK now"
👉 Read [QUICKSTART.md](QUICKSTART.md) → Follow code examples → Done!

### "I want to understand the SDK first"
👉 Read [ARCHITECTURE.md](ARCHITECTURE.md) → [QUICKSTART.md](QUICKSTART.md) → Code

### "I need detailed configuration"
👉 [QUICKSTART.md](QUICKSTART.md) has all the config templates you need

### "I have a problem"
👉 [QUICKSTART.md](QUICKSTART.md) Troubleshooting section

## 📦 Quick Install

```bash
go get github.com/aminofox/zenlive
```

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
    
    select {}
}
```

## 🎨 Use Cases

| Use Case | Config |
|----------|--------|
| **Livestream Platform** | RTMP + HLS + Chat |
| **Video Call (1-1)** | WebRTC only |
| **Video Conference** | WebRTC + Chat |
| **Recording Server** | RTMP + HLS + Storage |

See details in [QUICKSTART.md](QUICKSTART.md).

## 💡 Important Points

### What SDK Does
✅ Real-time streaming (RTMP, HLS, WebRTC)  
✅ Real-time chat delivery  
✅ Session management  
✅ Recording to local/S3  

### What SDK Does NOT Do
❌ Database persistence (you handle this)  
❌ Chat history storage (you save to your DB)  
❌ User management (your responsibility)  

### When Do You Need Redis?
- ✅ Multi-server deployment (cluster mode)
- ❌ Single server (not needed)

See details in [ARCHITECTURE.md](ARCHITECTURE.md).

## 📚 Code Examples

See the [`/examples`](../examples/) directory with 11+ examples:

- `basic/` - Simplest streaming server
- `chat/` - Add real-time chat
- `auth/` - JWT authentication
- `storage/` - Recording streams
- `webrtc/` - Low latency streaming
- `scalability/` - Multi-server cluster

Each example has complete code and instructions.

## 🔗 Links

- **GitHub**: [github.com/aminofox/zenlive](https://github.com/aminofox/zenlive)
- **API Docs**: [pkg.go.dev/github.com/aminofox/zenlive](https://pkg.go.dev/github.com/aminofox/zenlive)
- **Examples**: [/examples](../examples/)
- **Issues**: [GitHub Issues](https://github.com/aminofox/zenlive/issues)

## 📄 Documentation Files

```
docs/
├── README.md         ← You are here (overview)
├── QUICKSTART.md     ← Integrate SDK (START HERE!)
└── ARCHITECTURE.md   ← Understand how SDK works
```

**Only 3 files - Simple & Clear!**

## ⚡ Quick Reference

### Install
```bash
go get github.com/aminofox/zenlive
```

### Basic Usage
```go
cfg := config.DefaultConfig()
sdk, _ := zenlive.New(cfg)
sdk.Start()
```

### Publish Stream
```bash
ffmpeg -re -i video.mp4 -c copy -f flv rtmp://localhost:1935/live/mystream
```

### Watch Stream
```html
<video src="http://localhost:8080/live/mystream/index.m3u8" controls>
```

## 🆘 Need Help?

1. Read [QUICKSTART.md](QUICKSTART.md) - 90% of questions answered here
2. Check [Examples](../examples/) - Complete working code
3. Visit [GitHub Issues](https://github.com/aminofox/zenlive/issues)

---

**Happy Streaming! 🎥**
