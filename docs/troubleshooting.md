# Troubleshooting Guide

## Common Issues and Solutions

### Installation Issues

#### Go Version Mismatch

**Problem**: Build fails with "go version required: 1.23 or later"

**Solution**:
```bash
# Check current Go version
go version

# Update Go (macOS)
brew upgrade go

# Update Go (Linux)
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
```

#### Dependency Download Fails

**Problem**: `go get` fails with network errors

**Solution**:
```bash
# Set Go proxy
export GOPROXY=https://proxy.golang.org,direct

# Or use China mirror
export GOPROXY=https://goproxy.cn,direct

# Retry
go mod download
```

### RTMP Issues

#### Cannot Connect to RTMP Server

**Problem**: OBS shows "Failed to connect to server"

**Checklist**:
1. Check server is running:
   ```bash
   netstat -an | grep 1935
   ```

2. Check firewall:
   ```bash
   # Allow RTMP port
   sudo ufw allow 1935/tcp
   ```

3. Verify server logs:
   ```bash
   tail -f /var/log/zenlive/rtmp.log
   ```

4. Test with FFmpeg:
   ```bash
   ffmpeg -re -i test.mp4 -c copy -f flv rtmp://localhost:1935/live/test
   ```

#### Stream Key Authentication Fails

**Problem**: "Authentication failed" error

**Solution**:
```go
// Verify stream key format
// Expected: rtmp://server/app/streamkey

// Check authentication configuration
rtmpServer := rtmp.NewServer(&rtmp.ServerConfig{
    EnableAuth: true,
    OnAuth: func(streamKey string) bool {
        // Implement authentication logic
        return validateStreamKey(streamKey)
    },
})
```

#### High Latency in RTMP

**Problem**: 10-20 second delay

**Solutions**:
1. Reduce OBS buffer:
   - Settings → Advanced → Network → Enable Dynamic Bitrate
   - Set "Keyframe Interval" to 2 seconds

2. Optimize server settings:
   ```yaml
   rtmp:
     chunk_size: 4096  # Larger chunks = lower latency
     buffer_size: 512   # Smaller buffer = lower latency
   ```

3. Use lower latency mode in OBS:
   - Settings → Output → Advanced → check "Low Latency Mode"

### HLS Issues

#### HLS Playlist Not Found

**Problem**: 404 error when accessing `.m3u8` file

**Checklist**:
1. Verify stream is live:
   ```bash
   curl http://localhost:8080/streams
   ```

2. Check HLS server is running:
   ```bash
   curl http://localhost:8080/health
   ```

3. Verify stream path:
   ```
   # Correct format:
   http://localhost:8080/live/{streamID}/index.m3u8
   ```

4. Check logs:
   ```bash
   grep "HLS" /var/log/zenlive/server.log
   ```

#### HLS Segments Not Playing

**Problem**: Playlist loads but video doesn't play

**Solutions**:
1. Check segment generation:
   ```bash
   ls -la /var/zenlive/hls/stream123/
   # Should see .ts files
   ```

2. Verify codec compatibility:
   ```bash
   ffprobe segment0.ts
   # Look for H.264 video, AAC audio
   ```

3. Check CORS headers:
   ```go
   hlsServer := hls.NewServer(&hls.ServerConfig{
       EnableCORS: true,
       CORSOrigins: "*",  // or specific domain
   })
   ```

4. Test in different browsers:
   - Safari: Native HLS support
   - Chrome/Firefox: Requires hls.js library

#### ABR Not Switching Quality

**Problem**: Video stays at one quality level

**Solutions**:
1. Verify multiple variants exist:
   ```bash
   curl http://localhost:8080/live/stream123/master.m3u8
   # Should list multiple STREAM-INF entries
   ```

2. Check bandwidth detection:
   ```javascript
   // In browser console
   hls.on(Hls.Events.LEVEL_SWITCHED, (event, data) => {
       console.log('Switched to level', data.level);
   });
   ```

3. Enable ABR in config:
   ```yaml
   hls:
     enable_abr: true
     abr_variants:
       - {name: "720p", bitrate: 2500}
       - {name: "480p", bitrate: 1000}
   ```

### WebRTC Issues

#### ICE Connection Failed

**Problem**: WebRTC connection stuck at "connecting"

**Solutions**:
1. Check ICE servers:
   ```yaml
   webrtc:
     ice_servers:
       - stun:stun.l.google.com:19302
   ```

2. Test STUN server:
   ```bash
   # Install stuntman
   stunclient stun.l.google.com 19302
   ```

3. Configure TURN server for firewalled networks:
   ```yaml
   ice_servers:
     - turn:turn.example.com:3478
       username: user
       credential: pass
   ```

4. Check NAT/firewall:
   - Open UDP ports 10000-20000 for media
   - Allow WebSocket port (8443)

#### No Video/Audio Received

**Problem**: Connection established but no media

**Checklist**:
1. Check peer connection state:
   ```javascript
   pc.oniceconnectionstatechange = () => {
       console.log('ICE state:', pc.iceConnectionState);
   };
   ```

2. Verify track existence:
   ```javascript
   pc.ontrack = (event) => {
       console.log('Received track:', event.track.kind);
   };
   ```

3. Check codec compatibility:
   ```javascript
   const capabilities = RTCRtpReceiver.getCapabilities('video');
   console.log('Supported codecs:', capabilities.codecs);
   ```

4. Server-side: Verify track forwarding:
   ```bash
   grep "track" /var/log/zenlive/webrtc.log
   ```

#### Poor Video Quality

**Problem**: Pixelated or blurry video

**Solutions**:
1. Increase bitrate:
   ```yaml
   webrtc:
     max_bitrate: 5000  # kbps
   ```

2. Enable simulcast:
   ```yaml
   webrtc:
     enable_simulcast: true
   ```

3. Adjust bandwidth estimation:
   ```go
   // Server-side
   bweConfig := &webrtc.BWEConfig{
       MinBitrate: 500,
       MaxBitrate: 5000,
       StartBitrate: 2000,
   }
   ```

### Chat Issues

#### Messages Not Sending

**Problem**: Chat messages fail to send

**Solutions**:
1. Check WebSocket connection:
   ```javascript
   ws.onopen = () => console.log('Connected');
   ws.onerror = (err) => console.error('Error:', err);
   ```

2. Verify authentication:
   ```javascript
   ws.send(JSON.stringify({
       type: 'auth',
       token: 'your-jwt-token'
   }));
   ```

3. Check rate limiting:
   ```yaml
   chat:
     rate_limit: 10  # messages per second
   ```

4. Server logs:
   ```bash
   tail -f /var/log/zenlive/chat.log
   ```

#### Chat Room Not Found

**Problem**: "Room does not exist" error

**Solution**:
```go
// Ensure room is created when stream starts
chatServer.CreateRoom(streamID)

// Or auto-create on join
room := chatServer.GetOrCreateRoom(streamID)
```

### Storage Issues

#### Recording Fails to Start

**Problem**: "Failed to start recording" error

**Checklist**:
1. Check disk space:
   ```bash
   df -h /var/zenlive/recordings
   ```

2. Verify permissions:
   ```bash
   ls -ld /var/zenlive/recordings
   # Should be writable by zenlive user
   ```

3. Check storage configuration:
   ```yaml
   storage:
     type: local
     local_path: /var/zenlive/recordings
   ```

4. Test write access:
   ```bash
   touch /var/zenlive/recordings/test.txt
   ```

#### S3 Upload Fails

**Problem**: "Access Denied" error when uploading to S3

**Solutions**:
1. Verify credentials:
   ```bash
   aws s3 ls s3://your-bucket --profile zenlive
   ```

2. Check IAM permissions:
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "s3:PutObject",
           "s3:GetObject",
           "s3:DeleteObject",
           "s3:ListBucket"
         ],
         "Resource": [
           "arn:aws:s3:::your-bucket/*",
           "arn:aws:s3:::your-bucket"
         ]
       }
     ]
   }
   ```

3. Test with AWS CLI:
   ```bash
   aws s3 cp test.txt s3://your-bucket/
   ```

4. Check endpoint for MinIO:
   ```yaml
   storage:
     s3_endpoint: http://minio:9000
   ```

### Authentication Issues

#### JWT Token Invalid

**Problem**: "Invalid token" error

**Solutions**:
1. Check token expiry:
   ```bash
   # Decode JWT (use jwt.io or jwt-cli)
   echo "your-token" | jwt decode -
   ```

2. Verify secret key matches:
   ```yaml
   auth:
     jwt_secret: "same-secret-on-all-nodes"
   ```

3. Check clock skew:
   ```bash
   # Sync time with NTP
   sudo ntpdate pool.ntp.org
   ```

4. Regenerate token:
   ```bash
   curl -X POST http://localhost:8080/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username": "user", "password": "pass"}'
   ```

#### Permission Denied

**Problem**: "Insufficient permissions" error

**Solution**:
```go
// Check user roles
user := getCurrentUser()
if !user.HasRole("publisher") {
    return errors.New("insufficient permissions")
}

// Verify RBAC configuration
rbac := auth.NewRBAC()
rbac.AddPolicy("publisher", "stream", "create")
rbac.AddPolicy("publisher", "stream", "update")
```

### Performance Issues

#### High CPU Usage

**Problem**: CPU usage constantly above 80%

**Solutions**:
1. Check number of active streams:
   ```bash
   curl http://localhost:8080/api/streams/count
   ```

2. Profile the application:
   ```bash
   go tool pprof http://localhost:6060/debug/pprof/profile
   ```

3. Optimize transcoding:
   ```yaml
   hls:
     enable_abr: false  # Disable if not needed
   ```

4. Scale horizontally:
   - Add more nodes
   - Use load balancer

#### High Memory Usage

**Problem**: Memory grows continuously

**Solutions**:
1. Check for memory leaks:
   ```bash
   go tool pprof http://localhost:6060/debug/pprof/heap
   ```

2. Reduce buffer sizes:
   ```yaml
   rtmp:
     buffer_size: 512
   ```

3. Enable garbage collection tuning:
   ```go
   debug.SetGCPercent(50)  // More aggressive GC
   ```

4. Limit concurrent connections:
   ```yaml
   rtmp:
     max_connections: 500
   ```

### Network Issues

#### Bandwidth Saturation

**Problem**: Network bandwidth maxed out

**Solutions**:
1. Monitor bandwidth:
   ```bash
   iftop -i eth0
   ```

2. Enable CDN:
   ```yaml
   cdn:
     enabled: true
     provider: cloudflare
   ```

3. Limit bitrate:
   ```yaml
   rtmp:
     max_bitrate: 5000  # kbps per stream
   ```

4. Use adaptive bitrate to reduce load

#### Packet Loss

**Problem**: Video stuttering and quality drops

**Solutions**:
1. Check packet loss:
   ```bash
   ping -c 100 your-server.com
   ```

2. Increase buffer:
   ```yaml
   webrtc:
     jitter_buffer: 200  # ms
   ```

3. Enable FEC (Forward Error Correction):
   ```yaml
   webrtc:
     enable_fec: true
   ```

4. Use lower latency protocol (WebRTC instead of RTMP)

### Database/Redis Issues

#### Redis Connection Failed

**Problem**: "Could not connect to Redis" error

**Solutions**:
1. Check Redis is running:
   ```bash
   redis-cli ping
   # Should return PONG
   ```

2. Verify connection string:
   ```yaml
   redis:
     url: redis://localhost:6379
   ```

3. Check authentication:
   ```bash
   redis-cli -a your-password ping
   ```

4. Test connectivity:
   ```bash
   telnet localhost 6379
   ```

#### Redis Memory Full

**Problem**: "OOM command not allowed" error

**Solutions**:
1. Check memory usage:
   ```bash
   redis-cli info memory
   ```

2. Set maxmemory policy:
   ```bash
   redis-cli CONFIG SET maxmemory-policy allkeys-lru
   ```

3. Increase maxmemory:
   ```bash
   redis-cli CONFIG SET maxmemory 2gb
   ```

4. Clear old data:
   ```bash
   redis-cli FLUSHDB
   ```

## Debugging Tools

### Enable Debug Logging

```yaml
logging:
  level: debug
  format: json
```

```go
logger.SetLevel(logger.DEBUG)
```

### Health Check Endpoints

```bash
# Overall health
curl http://localhost:8080/health

# Component-specific
curl http://localhost:8080/health/rtmp
curl http://localhost:8080/health/hls
curl http://localhost:8080/health/webrtc
curl http://localhost:8080/health/redis
```

### Metrics and Monitoring

```bash
# Prometheus metrics
curl http://localhost:9090/metrics

# Stream statistics
curl http://localhost:8080/api/stats/streams

# Viewer statistics
curl http://localhost:8080/api/stats/viewers
```

### Log Analysis

```bash
# Find errors
grep ERROR /var/log/zenlive/*.log

# Count errors by type
grep ERROR /var/log/zenlive/*.log | cut -d' ' -f5 | sort | uniq -c

# Monitor in real-time
tail -f /var/log/zenlive/*.log | grep ERROR
```

### Network Debugging

```bash
# Check listening ports
netstat -tlnp | grep zenlive

# Monitor connections
ss -s

# Capture RTMP traffic
tcpdump -i any -w rtmp.pcap port 1935

# Analyze with Wireshark
wireshark rtmp.pcap
```

## Getting Help

### Before Asking for Help

1. **Check logs**: Look at server logs for error messages
2. **Reproduce**: Can you consistently reproduce the issue?
3. **Isolate**: Does it happen with minimal configuration?
4. **Search**: Check GitHub issues and Stack Overflow
5. **Document**: Write down steps to reproduce

### Provide This Information

```
**Environment**:
- ZenLive version: vX.Y.Z
- Go version: 1.23.0
- OS: Ubuntu 22.04
- Deployment: Docker / Kubernetes / Bare metal

**Configuration**:
```yaml
# Relevant config section
```

**Steps to Reproduce**:
1. Start server with config X
2. Connect with OBS
3. Error occurs

**Expected Behavior**:
Stream should connect and publish

**Actual Behavior**:
Connection fails with "authentication error"

**Logs**:
```
ERROR: authentication failed for stream key: xyz
```
```

### Support Channels

- GitHub Issues: [github.com/aminofox/zenlive/issues](https://github.com/aminofox/zenlive/issues)
- Discord: [discord.gg/zenlive](https://discord.gg/zenlive)
- Stack Overflow: Tag `zenlive`
- Email: support@zenlive.dev

## FAQ

**Q: Can I use ZenLive in production?**  
A: Yes, ZenLive is production-ready. Ensure you follow security best practices.

**Q: What's the maximum number of concurrent streams?**  
A: Depends on server resources. Typical: 500-1000 streams per node.

**Q: Do I need a CDN?**  
A: Recommended for >100 concurrent viewers per stream.

**Q: Can I run ZenLive behind a reverse proxy?**  
A: Yes, configure WebSocket and media ports properly.

**Q: How do I update ZenLive?**  
A: Follow migration guide for version-specific changes.

**Q: Is clustering supported?**  
A: Yes, see [Architecture Guide](architecture.md) for details.

## Next Steps

- [Getting Started](getting-started.md)
- [Configuration Guide](configuration.md)
- [Architecture](architecture.md)
- [Migration Guide](migration.md)
