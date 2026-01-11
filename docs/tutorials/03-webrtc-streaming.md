# Tutorial: Low-Latency WebRTC Streaming

## Introduction

Build ultra-low latency streaming with WebRTC, achieving sub-second latency for interactive applications.

**What you'll learn**:
- Setting up WebRTC signaling server
- Implementing browser-based publishing
- Building a WebRTC player
- Optimizing for low latency
- Bandwidth adaptation

**Prerequisites**:
- Completed [First Streaming Server Tutorial](01-first-streaming-server.md)
- Basic JavaScript knowledge

**Estimated time**: 30 minutes

## Step 1: Add WebRTC Server

Update your `main.go`:

```go
import (
    "github.com/aminofox/zenlive/pkg/streaming/webrtc"
)

func main() {
    // ... previous RTMP/HLS setup ...
    
    // Create WebRTC server
    webrtcServer := webrtc.NewServer(&webrtc.ServerConfig{
        ListenAddr: ":8443",
        IceServers: []string{
            "stun:stun.l.google.com:19302",
        },
    })
    
    // Start WebRTC server
    go func() {
        if err := webrtcServer.Start(); err != nil {
            log.Fatal("WebRTC server error:", err)
        }
    }()
    
    log.Println("WebRTC: https://localhost:8443")
    
    // ... rest of code ...
}
```

## Step 2: Create Signaling Endpoint

Add WebSocket signaling:

```go
import (
    "github.com/gorilla/websocket"
    "net/http"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true // Allow all origins for development
    },
}

func handleSignaling(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("Upgrade error:", err)
        return
    }
    defer conn.Close()
    
    for {
        var msg map[string]interface{}
        if err := conn.ReadJSON(&msg); err != nil {
            log.Println("Read error:", err)
            break
        }
        
        msgType := msg["type"].(string)
        
        switch msgType {
        case "offer":
            // Handle WebRTC offer
            offer := msg["offer"]
            streamID := msg["streamId"].(string)
            
            answer, err := webrtcServer.HandleOffer(streamID, offer)
            if err != nil {
                log.Println("Offer error:", err)
                continue
            }
            
            conn.WriteJSON(map[string]interface{}{
                "type":   "answer",
                "answer": answer,
            })
            
        case "ice-candidate":
            // Handle ICE candidate
            candidate := msg["candidate"]
            streamID := msg["streamId"].(string)
            
            webrtcServer.AddICECandidate(streamID, candidate)
        }
    }
}

func main() {
    // ... previous code ...
    
    http.HandleFunc("/ws", handleSignaling)
    
    // ... rest of code ...
}
```

## Step 3: Create Publisher Page

Create `public/publish.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>WebRTC Publisher</title>
    <style>
        body { font-family: Arial, sans-serif; padding: 20px; }
        video { width: 640px; height: 480px; background: #000; }
        button { padding: 10px 20px; margin: 5px; font-size: 16px; }
        #status { margin: 10px 0; padding: 10px; background: #f0f0f0; }
    </style>
</head>
<body>
    <h1>WebRTC Publisher</h1>
    
    <div>
        <video id="localVideo" autoplay muted></video>
    </div>
    
    <div>
        <input type="text" id="streamId" placeholder="Stream ID" value="mystream">
        <button id="startBtn">Start Publishing</button>
        <button id="stopBtn" disabled>Stop Publishing</button>
    </div>
    
    <div id="status">Not connected</div>
    
    <script>
        const localVideo = document.getElementById('localVideo');
        const startBtn = document.getElementById('startBtn');
        const stopBtn = document.getElementById('stopBtn');
        const statusDiv = document.getElementById('status');
        const streamIdInput = document.getElementById('streamId');
        
        let pc = null;
        let ws = null;
        let localStream = null;
        
        function updateStatus(msg) {
            statusDiv.textContent = msg;
            console.log(msg);
        }
        
        async function startPublishing() {
            const streamId = streamIdInput.value;
            
            try {
                // Get user media
                updateStatus('Requesting camera/microphone access...');
                localStream = await navigator.mediaDevices.getUserMedia({
                    video: true,
                    audio: true
                });
                
                localVideo.srcObject = localStream;
                updateStatus('Camera access granted');
                
                // Create peer connection
                pc = new RTCPeerConnection({
                    iceServers: [
                        { urls: 'stun:stun.l.google.com:19302' }
                    ]
                });
                
                // Add tracks
                localStream.getTracks().forEach(track => {
                    pc.addTrack(track, localStream);
                });
                
                // Handle ICE candidates
                pc.onicecandidate = (event) => {
                    if (event.candidate) {
                        ws.send(JSON.stringify({
                            type: 'ice-candidate',
                            streamId: streamId,
                            candidate: event.candidate
                        }));
                    }
                };
                
                // Connect to signaling server
                updateStatus('Connecting to signaling server...');
                ws = new WebSocket('ws://localhost:8081/ws');
                
                ws.onopen = async () => {
                    updateStatus('Connected to signaling server');
                    
                    // Create offer
                    const offer = await pc.createOffer();
                    await pc.setLocalDescription(offer);
                    
                    // Send offer
                    ws.send(JSON.stringify({
                        type: 'offer',
                        streamId: streamId,
                        offer: offer
                    }));
                };
                
                ws.onmessage = async (event) => {
                    const msg = JSON.parse(event.data);
                    
                    if (msg.type === 'answer') {
                        await pc.setRemoteDescription(msg.answer);
                        updateStatus('Publishing to: ' + streamId);
                        
                        startBtn.disabled = true;
                        stopBtn.disabled = false;
                    }
                };
                
                ws.onerror = (error) => {
                    updateStatus('WebSocket error: ' + error);
                };
                
                ws.onclose = () => {
                    updateStatus('Disconnected');
                    stopPublishing();
                };
                
            } catch (error) {
                updateStatus('Error: ' + error.message);
            }
        }
        
        function stopPublishing() {
            if (localStream) {
                localStream.getTracks().forEach(track => track.stop());
                localStream = null;
            }
            
            if (pc) {
                pc.close();
                pc = null;
            }
            
            if (ws) {
                ws.close();
                ws = null;
            }
            
            localVideo.srcObject = null;
            updateStatus('Stopped');
            
            startBtn.disabled = false;
            stopBtn.disabled = true;
        }
        
        startBtn.addEventListener('click', startPublishing);
        stopBtn.addEventListener('click', stopPublishing);
    </script>
</body>
</html>
```

## Step 4: Create Player Page

Create `public/play.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>WebRTC Player</title>
    <style>
        body { font-family: Arial, sans-serif; padding: 20px; }
        video { width: 640px; height: 480px; background: #000; }
        button { padding: 10px 20px; margin: 5px; font-size: 16px; }
        #status { margin: 10px 0; padding: 10px; background: #f0f0f0; }
        #stats { margin: 10px 0; font-family: monospace; }
    </style>
</head>
<body>
    <h1>WebRTC Player</h1>
    
    <div>
        <video id="remoteVideo" autoplay controls></video>
    </div>
    
    <div>
        <input type="text" id="streamId" placeholder="Stream ID" value="mystream">
        <button id="playBtn">Play</button>
        <button id="stopBtn" disabled>Stop</button>
    </div>
    
    <div id="status">Not connected</div>
    <div id="stats"></div>
    
    <script>
        const remoteVideo = document.getElementById('remoteVideo');
        const playBtn = document.getElementById('playBtn');
        const stopBtn = document.getElementById('stopBtn');
        const statusDiv = document.getElementById('status');
        const statsDiv = document.getElementById('stats');
        const streamIdInput = document.getElementById('streamId');
        
        let pc = null;
        let ws = null;
        let statsInterval = null;
        
        function updateStatus(msg) {
            statusDiv.textContent = msg;
            console.log(msg);
        }
        
        async function startPlayback() {
            const streamId = streamIdInput.value;
            
            try {
                // Create peer connection
                pc = new RTCPeerConnection({
                    iceServers: [
                        { urls: 'stun:stun.l.google.com:19302' }
                    ]
                });
                
                // Handle incoming tracks
                pc.ontrack = (event) => {
                    if (event.streams && event.streams[0]) {
                        remoteVideo.srcObject = event.streams[0];
                        updateStatus('Playing: ' + streamId);
                    }
                };
                
                // Handle ICE candidates
                pc.onicecandidate = (event) => {
                    if (event.candidate) {
                        ws.send(JSON.stringify({
                            type: 'ice-candidate',
                            streamId: streamId,
                            candidate: event.candidate
                        }));
                    }
                };
                
                // Connect to signaling server
                updateStatus('Connecting...');
                ws = new WebSocket('ws://localhost:8081/ws');
                
                ws.onopen = async () => {
                    updateStatus('Connected');
                    
                    // Create offer
                    const offer = await pc.createOffer({
                        offerToReceiveVideo: true,
                        offerToReceiveAudio: true
                    });
                    await pc.setLocalDescription(offer);
                    
                    // Send offer
                    ws.send(JSON.stringify({
                        type: 'offer',
                        streamId: streamId,
                        offer: offer
                    }));
                };
                
                ws.onmessage = async (event) => {
                    const msg = JSON.parse(event.data);
                    
                    if (msg.type === 'answer') {
                        await pc.setRemoteDescription(msg.answer);
                    }
                };
                
                // Start stats monitoring
                statsInterval = setInterval(updateStats, 1000);
                
                playBtn.disabled = true;
                stopBtn.disabled = false;
                
            } catch (error) {
                updateStatus('Error: ' + error.message);
            }
        }
        
        async function updateStats() {
            if (!pc) return;
            
            const stats = await pc.getStats();
            let statsText = '';
            
            stats.forEach(report => {
                if (report.type === 'inbound-rtp' && report.kind === 'video') {
                    statsText += `Video:\n`;
                    statsText += `  Bitrate: ${Math.round(report.bytesReceived * 8 / 1000)} kbps\n`;
                    statsText += `  Frames: ${report.framesReceived}\n`;
                    statsText += `  Dropped: ${report.framesDropped}\n`;
                }
            });
            
            statsDiv.textContent = statsText;
        }
        
        function stopPlayback() {
            if (pc) {
                pc.close();
                pc = null;
            }
            
            if (ws) {
                ws.close();
                ws = null;
            }
            
            if (statsInterval) {
                clearInterval(statsInterval);
                statsInterval = null;
            }
            
            remoteVideo.srcObject = null;
            statsDiv.textContent = '';
            updateStatus('Stopped');
            
            playBtn.disabled = false;
            stopBtn.disabled = true;
        }
        
        playBtn.addEventListener('click', startPlayback);
        stopBtn.addEventListener('click', stopPlayback);
    </script>
</body>
</html>
```

## Step 5: Serve Static Files

Add static file server to `main.go`:

```go
func main() {
    // ... previous code ...
    
    // Serve static files
    http.Handle("/", http.FileServer(http.Dir("./public")))
    
    // ... rest of code ...
}
```

## Step 6: Test WebRTC

1. **Start server**:
   ```bash
   go run main.go
   ```

2. **Open publisher** in browser:
   ```
   http://localhost:8081/publish.html
   ```
   - Allow camera/microphone access
   - Click "Start Publishing"

3. **Open player** in another browser tab:
   ```
   http://localhost:8081/play.html
   ```
   - Click "Play"
   - Should see video with < 1 second latency

## Step 7: Add Bandwidth Adaptation

Enable adaptive bitrate in server:

```go
webrtcServer := webrtc.NewServer(&webrtc.ServerConfig{
    ListenAddr: ":8443",
    IceServers: []string{
        "stun:stun.l.google.com:19302",
    },
    EnableBWE: true, // Bandwidth estimation
    MinBitrate: 500,  // kbps
    MaxBitrate: 5000, // kbps
})
```

## Step 8: Add TURN Server Support

For clients behind strict firewalls, add TURN server in code:

```go
import "github.com/aminofox/zenlive/pkg/config"

cfg := config.DefaultConfig()
cfg.Streaming.WebRTC.TURNServers = []config.TURNServer{
    {
        URLs:       []string{"turn:turn.example.com:3478"},
        Username:   "user",
        Credential: "password",
    },
}
cfg.Streaming.WebRTC.STUNServers = []string{
    "stun:stun.l.google.com:19302",
}
```

Setup coturn (TURN server):

```bash
# Install coturn
sudo apt install coturn

# Configure /etc/turnserver.conf
listening-port=3478
fingerprint
lt-cred-mech
user=user:password
realm=turn.example.com

# Start coturn
sudo systemctl start coturn
```

## Step 9: Optimize for Low Latency

Update publisher page for minimal latency:

```javascript
pc = new RTCPeerConnection({
    iceServers: [
        { urls: 'stun:stun.l.google.com:19302' }
    ],
    bundlePolicy: 'max-bundle',
    rtcpMuxPolicy: 'require'
});

// Disable jitter buffer
const sender = pc.getSenders()[0];
const parameters = sender.getParameters();
if (!parameters.encodings) {
    parameters.encodings = [{}];
}
parameters.encodings[0].maxBitrate = 2500000; // 2.5 Mbps
sender.setParameters(parameters);
```

## Complete Example

The complete WebRTC streaming server example is available in:
- [examples/webrtc/main.go](../../examples/webrtc/main.go)

## Testing Latency

Measure end-to-end latency:

```javascript
// In publisher
setInterval(() => {
    const timestamp = Date.now();
    dataChannel.send(JSON.stringify({ timestamp }));
}, 1000);

// In player
dataChannel.onmessage = (event) => {
    const { timestamp } = JSON.parse(event.data);
    const latency = Date.now() - timestamp;
    console.log('Latency:', latency, 'ms');
};
```

**Expected latency**: 200-800ms

## Next Steps

- [Chat Integration Tutorial](04-chat-integration.md)
- [Advanced Features Tutorial](05-advanced-features.md)
- [Production Deployment](../getting-started.md#deployment)

## Troubleshooting

**No video appears**:
- Check browser console for errors
- Verify camera permissions granted
- Test STUN server: `stunclient stun.l.google.com 19302`

**High latency**:
- Check network conditions
- Add TURN server for firewall traversal
- Reduce video resolution/bitrate

**Connection fails**:
- Verify WebSocket connection
- Check ICE candidates are exchanged
- Test with simpler STUN server

**Choppy video**:
- Check bandwidth
- Enable adaptive bitrate
- Reduce video quality
