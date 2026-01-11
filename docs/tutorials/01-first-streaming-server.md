# Tutorial: Building Your First Streaming Server

## Introduction

In this tutorial, you'll build a complete live streaming server from scratch using ZenLive. By the end, you'll have a working server that supports RTMP publishing and HLS playback.

**What you'll learn**:
- Setting up a ZenLive project
- Configuring RTMP and HLS servers
- Implementing basic authentication
- Testing with OBS and FFmpeg
- Monitoring stream health

**Prerequisites**:
- Go 1.23+ installed
- Basic Go programming knowledge
- OBS Studio or FFmpeg for testing

**Estimated time**: 30 minutes

## Step 1: Create Project Structure

Create a new project directory:

```bash
mkdir mystreaming-server
cd mystreaming-server
go mod init mystreaming-server
```

Install ZenLive:

```bash
go get github.com/aminofox/zenlive
```

## Step 2: Create Basic Server

Create `main.go`:

```go
package main

import (
    "log"
    "github.com/aminofox/zenlive/pkg/streaming/rtmp"
    "github.com/aminofox/zenlive/pkg/streaming/hls"
)

func main() {
    // Create RTMP server for publishing
    rtmpServer := rtmp.NewServer(&rtmp.ServerConfig{
        ListenAddr: ":1935",
    })
    
    // Create HLS server for playback
    hlsServer := hls.NewServer(&hls.ServerConfig{
        ListenAddr:      ":8080",
        SegmentDuration: 4,
        PlaylistSize:    5,
    })
    
    // Start RTMP server
    go func() {
        if err := rtmpServer.Start(); err != nil {
            log.Fatal("RTMP server error:", err)
        }
    }()
    
    // Start HLS server
    go func() {
        if err := hlsServer.Start(); err != nil {
            log.Fatal("HLS server error:", err)
        }
    }()
    
    log.Println("Streaming server started")
    log.Println("RTMP: rtmp://localhost:1935/live/{streamkey}")
    log.Println("HLS: http://localhost:8080/live/{streamkey}/index.m3u8")
    
    // Keep running
    select {}
}
```

Run the server:

```bash
go run main.go
```

**Expected output**:
```
Streaming server started
RTMP: rtmp://localhost:1935/live/{streamkey}
HLS: http://localhost:8080/live/{streamkey}/index.m3u8
```

## Step 3: Test with FFmpeg

Publish a test stream:

```bash
ffmpeg -re -i test.mp4 -c copy -f flv rtmp://localhost:1935/live/mystream
```

In another terminal, play the stream:

```bash
ffplay http://localhost:8080/live/mystream/index.m3u8
```

**Success!** You should see the video playing.

## Step 4: Add Authentication

Update `main.go` to add stream key validation:

```go
package main

import (
    "log"
    "github.com/aminofox/zenlive/pkg/streaming/rtmp"
    "github.com/aminofox/zenlive/pkg/streaming/hls"
    "github.com/aminofox/zenlive/pkg/auth"
)

// Valid stream keys
var validKeys = map[string]bool{
    "secret-key-123": true,
    "another-key-456": true,
}

func main() {
    // Create authenticator
    authenticator := auth.NewJWTAuthenticator(&auth.JWTConfig{
        SecretKey: "my-secret-key",
        Issuer:    "mystreaming-server",
    })
    
    // Create RTMP server with authentication
    rtmpServer := rtmp.NewServer(&rtmp.ServerConfig{
        ListenAddr: ":1935",
        EnableAuth: true,
        OnAuth: func(streamKey string) bool {
            // Validate stream key
            valid := validKeys[streamKey]
            if valid {
                log.Printf("Stream key validated: %s", streamKey)
            } else {
                log.Printf("Invalid stream key: %s", streamKey)
            }
            return valid
        },
    })
    
    // Create HLS server
    hlsServer := hls.NewServer(&hls.ServerConfig{
        ListenAddr:      ":8080",
        SegmentDuration: 4,
        PlaylistSize:    5,
    })
    
    // Start servers
    go func() {
        if err := rtmpServer.Start(); err != nil {
            log.Fatal("RTMP server error:", err)
        }
    }()
    
    go func() {
        if err := hlsServer.Start(); err != nil {
            log.Fatal("HLS server error:", err)
        }
    }()
    
    log.Println("Streaming server started with authentication")
    log.Println("Valid stream keys:", []string{"secret-key-123", "another-key-456"})
    
    select {}
}
```

Test with valid key:

```bash
ffmpeg -re -i test.mp4 -c copy -f flv rtmp://localhost:1935/live/secret-key-123
```

Test with invalid key (should fail):

```bash
ffmpeg -re -i test.mp4 -c copy -f flv rtmp://localhost:1935/live/wrong-key
```

## Step 5: Add Stream Events

Add event callbacks to track stream lifecycle:

```go
rtmpServer := rtmp.NewServer(&rtmp.ServerConfig{
    ListenAddr: ":1935",
    EnableAuth: true,
    OnAuth: func(streamKey string) bool {
        return validKeys[streamKey]
    },
    OnPublishStart: func(streamID string) {
        log.Printf("Stream started: %s", streamID)
        // Notify your system, update database, etc.
    },
    OnPublishEnd: func(streamID string) {
        log.Printf("Stream ended: %s", streamID)
        // Clean up resources, update statistics, etc.
    },
})
```

## Step 6: Add Basic Monitoring

Create a simple health check endpoint:

```go
import (
    "net/http"
    "encoding/json"
)

func main() {
    // ... previous code ...
    
    // Add HTTP handler for status
    http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
        status := map[string]interface{}{
            "rtmp_running": rtmpServer.IsRunning(),
            "hls_running":  hlsServer.IsRunning(),
            "active_streams": rtmpServer.GetActiveStreamCount(),
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(status)
    })
    
    go http.ListenAndServe(":8081", nil)
    log.Println("Status API: http://localhost:8081/status")
    
    // ... rest of code ...
}
```

Check server status:

```bash
curl http://localhost:8081/status
```

**Expected output**:
```json
{
  "rtmp_running": true,
  "hls_running": true,
  "active_streams": 1
}
```

## Step 7: Configure with OBS

Open OBS Studio:

1. **Settings → Stream**:
   - Service: Custom
   - Server: `rtmp://localhost:1935/live`
   - Stream Key: `secret-key-123`

2. **Click "Start Streaming"**

3. **Play in browser** using a player like:
   ```html
   <!DOCTYPE html>
   <html>
   <head>
       <script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
   </head>
   <body>
       <video id="video" controls width="640"></video>
       <script>
           var video = document.getElementById('video');
           var videoSrc = 'http://localhost:8080/live/secret-key-123/index.m3u8';
           
           if (Hls.isSupported()) {
               var hls = new Hls();
               hls.loadSource(videoSrc);
               hls.attachMedia(video);
           } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
               video.src = videoSrc;
           }
       </script>
   </body>
   </html>
   ```

## Step 8: Add Configuration

Use programmatic configuration:

```go
import (
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    // Create config
    cfg := config.DefaultConfig()
    
    // Customize settings
    cfg.Streaming.RTMP.Port = 1935
    cfg.Streaming.HLS.SegmentDuration = 4 * time.Second
    cfg.Streaming.HLS.PlaylistLength = 5
    cfg.Auth.JWTSecret = "my-secret-key"
    
    // Or load from JSON file
    // file, _ := os.Open("config.json")
    // json.NewDecoder(file).Decode(&cfg)
    
    // Use config in your server setup
    rtmpPort := cfg.Streaming.RTMP.Port
    hlsSegDuration := cfg.Streaming.HLS.SegmentDuration
}
```

## Complete Code

Here's the final `main.go`:

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "time"
    
    "github.com/aminofox/zenlive/pkg/streaming/rtmp"
    "github.com/aminofox/zenlive/pkg/streaming/hls"
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    // Create configuration
    cfg := config.DefaultConfig()
    cfg.Streaming.RTMP.Port = 1935
    cfg.Streaming.HLS.SegmentDuration = 4 * time.Second
    cfg.Streaming.HLS.PlaylistLength = 5
    
    // Define valid stream keys
    validKeys := map[string]bool{
        "secret-key-123":   true,
        "another-key-456": true,
    }
    
    // Create RTMP server
    rtmpServer := rtmp.NewServer(":1935", logger)
        OnAuth: func(streamKey string) bool {
            valid := validKeys[streamKey]
            if valid {
                log.Printf("✓ Stream key validated: %s", streamKey)
            } else {
                log.Printf("✗ Invalid stream key: %s", streamKey)
            }
            return valid
        },
        OnPublishStart: func(streamID string) {
            log.Printf("▶ Stream started: %s", streamID)
        },
        OnPublishEnd: func(streamID string) {
            log.Printf("■ Stream ended: %s", streamID)
        },
    })
    
    // Create HLS server
    hlsServer := hls.NewServer(":8080", logger)
    
    // Start RTMP server
    go func() {
        if err := rtmpServer.Start(); err != nil {
            log.Fatal("RTMP server error:", err)
        }
    }()
    
    // Start HLS server
    go func() {
        if err := hlsServer.Start(); err != nil {
            log.Fatal("HLS server error:", err)
        }
    }()
    
    // Status API
    http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
        status := map[string]interface{}{
            "rtmp_running":   true,
            "hls_running":    true,
            "active_streams": len(rtmpServer.GetStreams()),
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(status)
    })
    
    go http.ListenAndServe(":8081", nil)
    
    log.Println("=== Streaming Server Started ===")
    log.Printf("RTMP: rtmp://localhost:1935/live/{streamkey}")
    log.Printf("HLS: http://localhost:8080/live/{streamkey}/index.m3u8")
    log.Printf("Status: http://localhost:8081/status")
    
    select {}
}
```

## Next Steps

Congratulations! You've built a basic streaming server. Here's what to explore next:

1. **Add Recording**: [Storage Tutorial](02-recording-streams.md)
2. **Enable WebRTC**: [WebRTC Tutorial](03-webrtc-streaming.md)
3. **Add Chat**: [Chat Tutorial](04-chat-integration.md)
4. **Deploy to Production**: [Deployment Guide](../getting-started.md#deployment)

## Troubleshooting

**Server won't start**:
- Check if ports 1935 and 8080 are available
- Run with sudo if using ports < 1024
- Check firewall settings

**Stream won't play**:
- Verify stream is publishing: `curl http://localhost:8081/status`
- Check HLS playlist exists: `curl http://localhost:8080/live/secret-key-123/index.m3u8`
- Try different player (VLC, FFplay)

**Authentication fails**:
- Verify stream key is in validKeys map in code
- Check server logs for auth messages
- Ensure authentication callback returns nil error

## Resources

- [Getting Started Guide](../getting-started.md)
- [Configuration Reference](../configuration.md)
- [Example Code](../../examples/basic/)
