# Migration Guide

## Overview

This guide helps you migrate from other live streaming solutions to ZenLive, or upgrade between ZenLive versions.

## Migrating from Other Solutions

### From Wowza Streaming Engine

#### Key Differences

| Feature | Wowza | ZenLive |
|---------|-------|---------|
| Configuration | XML | YAML / Code |
| Protocols | RTMP, HLS, WebRTC | RTMP, HLS, WebRTC |
| Language | Java | Go |
| License | Commercial | Open Source (MIT) |
| Clustering | Built-in | Redis-based |

#### Migration Steps

1. **Install ZenLive**:
   ```bash
   go get github.com/aminofox/zenlive
   ```

2. **Convert Application Configuration**:
   
   Wowza `Application.xml`:
   ```xml
   <Application>
       <Name>live</Name>
       <StreamType>live</StreamType>
       <Streams>
           <StreamType>live</StreamType>
       </Streams>
   </Application>
   ```
   
   ZenLive `config.json`:
   ```json
   {
     "streaming": {
       "rtmp": {
         "port": 1935
       },
       "hls": {
         "segment_duration": "4s"
       }
     },
     "auth": {
       "enable_rbac": true
     }
   }
   ```

3. **Update Publishing URLs**:
   
   Before (Wowza):
   ```
   rtmp://wowza-server:1935/live/stream123
   ```
   
   After (ZenLive):
   ```
   rtmp://zenlive-server:1935/live/stream123
   ```

4. **Update Playback URLs**:
   
   HLS Before:
   ```
   http://wowza-server:1935/live/stream123/playlist.m3u8
   ```
   
   HLS After:
   ```
   http://zenlive-server:8080/live/stream123/index.m3u8
   ```

5. **Migrate Authentication**:
   
   Wowza module:
   ```java
   public void onConnect(IClient client) {
       String username = client.getQueryStr().get("username");
       // Validate...
   }
   ```
   
   ZenLive:
   ```go
   rtmpServer := rtmp.NewServer(&rtmp.ServerConfig{
       OnAuth: func(streamKey string) bool {
           return validateStreamKey(streamKey)
       },
   })
   ```

6. **Migrate Recordings**:
   ```bash
   # Copy existing recordings
   rsync -av /usr/local/WowzaStreamingEngine/content/ \
             /var/zenlive/recordings/
   ```

### From NGINX-RTMP

#### Key Differences

| Feature | NGINX-RTMP | ZenLive |
|---------|------------|---------|
| Setup | nginx.conf | YAML / Code |
| Extensibility | C module | Go SDK |
| HLS | nginx-rtmp module | Built-in |
| WebRTC | Not supported | Built-in |
| Chat | Not included | Built-in |
| Analytics | Basic | Advanced |

#### Migration Steps

1. **Convert nginx.conf to config.json**:
   
   NGINX `nginx.conf`:
   ```nginx
   rtmp {
       server {
           listen 1935;
           application live {
               live on;
               record off;
           }
       }
   }
   ```
   
   ZenLive `config.json`:
   ```json
   {
     "streaming": {
       "rtmp": {
         "port": 1935
       }
     },
     "storage": {
       "type": "local",
       "base_path": "./recordings"
     }
   }
   ```
   
   ZenLive:
   ```yaml
   rtmp:
     port: 1935
   
   hls:
     port: 8080
   ```

2. **Replace NGINX exec commands**:
   
   NGINX:
   ```nginx
   exec ffmpeg -i rtmp://localhost/live/$name
     -c:v libx264 -c:a aac -f flv rtmp://localhost/hls/$name;
   ```
   
   ZenLive (automatic HLS transmuxing):
   ```yaml
   hls:
     enable: true
     segment_duration: 4
   ```

3. **Migrate authentication**:
   
   NGINX:
   ```nginx
   on_publish http://auth-server/publish;
   ```
   
   ZenLive:
   ```go
   rtmpServer.OnAuth = func(streamKey string) bool {
       resp, _ := http.Get("http://auth-server/validate?key=" + streamKey)
       return resp.StatusCode == 200
   }
   ```

4. **Update systemd service**:
   
   Before (`/etc/systemd/system/nginx-rtmp.service`):
   ```ini
   [Service]
   ExecStart=/usr/sbin/nginx
   ```
   
   After (`/etc/systemd/system/zenlive.service`):
   ```ini
   [Service]
   ExecStart=/usr/local/bin/zenlive
   ```

### From Red5

#### Migration Steps

1. **Convert application configuration**:
   
   Red5 `red5-web.xml`:
   ```xml
   <bean id="web.handler" class="org.red5.server.Application">
       <property name="name" value="live"/>
   </bean>
   ```
   
   ZenLive SDK:
   ```go
   app := zenlive.New(&zenlive.Config{
       AppName: "live",
   })
   ```

2. **Migrate Java applications**:
   
   Red5 Java:
   ```java
   public void streamPublishStart(IBroadcastStream stream) {
       String name = stream.getPublishedName();
       // Handle publish...
   }
   ```
   
   ZenLive Go:
   ```go
   rtmpServer.OnPublishStart = func(streamID string) {
       // Handle publish...
   }
   ```

3. **Update client applications**:
   - Publishing URLs remain compatible (RTMP)
   - Update playback URLs for HLS

### From Custom FFmpeg Solution

#### Migration Benefits

- **Simplified Management**: No manual FFmpeg process management
- **Built-in HLS**: No need to manually invoke FFmpeg for HLS
- **WebRTC Support**: Ultra-low latency streaming
- **Authentication**: Built-in auth instead of shell scripts
- **Analytics**: Real-time metrics without external tools

#### Migration Steps

1. **Replace FFmpeg command**:
   
   Before:
   ```bash
   ffmpeg -i rtmp://source/stream \
     -c:v libx264 -c:a aac \
     -hls_time 4 -hls_list_size 5 \
     -hls_flags delete_segments \
     /var/www/hls/stream/index.m3u8
   ```
   
   After (automatic):
   ```yaml
   hls:
     segment_duration: 4
     playlist_size: 5
   ```

2. **Replace bash scripts with Go**:
   
   Before (auth.sh):
   ```bash
   #!/bin/bash
   STREAM_KEY=$1
   curl http://api/validate?key=$STREAM_KEY
   ```
   
   After:
   ```go
   rtmpServer.OnAuth = func(streamKey string) bool {
       return auth.ValidateKey(streamKey)
   }
   ```

3. **Consolidate multiple processes**:
   - RTMP server
   - FFmpeg transcoder
   - HLS server
   - All in one ZenLive process

## Version Upgrades

### Upgrading from v1.0 to v2.0

#### Breaking Changes

1. **Configuration Structure Changed**:
   
   v1.0:
   ```yaml
   server:
     rtmp_port: 1935
   ```
   
   v2.0:
   ```yaml
   rtmp:
     port: 1935
   ```

2. **API Changes**:
   
   v1.0:
   ```go
   stream := zenlive.CreateStream(name)
   ```
   
   v2.0:
   ```go
   stream, err := sdk.CreateStream(&sdk.CreateStreamRequest{
       Name: name,
   })
   ```

3. **Database Schema Updates**:
   ```sql
   -- Run migration
   ALTER TABLE streams ADD COLUMN created_at TIMESTAMP;
   ```

#### Migration Steps

1. **Backup Configuration and Data**:
   ```bash
   cp config.json config.json.backup
   pg_dump zenlive > backup.sql
   ```

2. **Update Dependencies**:
   ```bash
   go get -u github.com/aminofox/zenlive@v2.0.0
   ```

3. **Update Configuration**:
   ```bash
   # Manually update config.json with new structure
   # See configuration.md for new format
   ```

4. **Update Code**:
   ```go
   // Replace old API calls
   // See CHANGELOG.md for complete list
   ```

5. **Test in Staging**:
   ```bash
   # Run tests
   go test ./...
   
   # Test with sample stream
   ./test-stream.sh
   ```

6. **Deploy to Production**:
   ```bash
   # Zero-downtime rolling update
   kubectl rolling-update zenlive --image=zenlive:v2.0.0
   ```

### Upgrading from v2.0 to v3.0

#### New Features

- WebRTC SFU improvements
- Enhanced analytics
- Multi-region support
- Advanced ABR

#### Breaking Changes

None - backward compatible

#### Migration Steps

1. **Update**:
   ```bash
   go get -u github.com/aminofox/zenlive@v3.0.0
   ```

2. **Enable New Features** (optional):
   ```yaml
   webrtc:
     sfu_version: 2  # New SFU engine
   
   analytics:
     enable_advanced: true
   ```

3. **Deploy**:
   ```bash
   # Standard deployment
   ```

## Data Migration

### Migrating Stream Metadata

Export from old system:
```bash
# Example: Export from Wowza
curl http://wowza:8087/v2/servers/_defaultServer_/vhosts/_defaultVHost_/applications/live/instances/_definst_/incomingstreams \
  | jq '.[] | {name, uptime}' > streams.json
```

Import to ZenLive:
```go
func importStreams(filename string) error {
    data, _ := os.ReadFile(filename)
    var streams []Stream
    json.Unmarshal(data, &streams)
    
    for _, s := range streams {
        sdk.CreateStream(&sdk.CreateStreamRequest{
            Name: s.Name,
            // ... map fields
        })
    }
    return nil
}
```

### Migrating User Data

Export users:
```sql
-- From old system
SELECT id, username, email, created_at
FROM users
INTO OUTFILE '/tmp/users.csv';
```

Import to ZenLive:
```go
func importUsers(csvPath string) error {
    file, _ := os.Open(csvPath)
    reader := csv.NewReader(file)
    
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        
        auth.CreateUser(&auth.User{
            ID:       record[0],
            Username: record[1],
            Email:    record[2],
        })
    }
    return nil
}
```

### Migrating Recordings

Copy files:
```bash
# Local to local
rsync -avz --progress /old/recordings/ /var/zenlive/recordings/

# Local to S3
aws s3 sync /old/recordings/ s3://zenlive-recordings/
```

Update metadata:
```sql
UPDATE recordings 
SET storage_path = REPLACE(storage_path, '/old/recordings/', 's3://zenlive-recordings/')
WHERE storage_type = 'local';

UPDATE recordings
SET storage_type = 's3';
```

## Cloud Migration

### AWS to GCP

1. **Copy S3 to Google Cloud Storage**:
   ```bash
   gsutil -m rsync -r s3://zenlive-recordings/ gs://zenlive-recordings/
   ```

2. **Update configuration**:
   ```yaml
   storage:
     type: gcs
     gcs_bucket: zenlive-recordings
   ```

### On-Premise to Cloud

1. **Setup cloud infrastructure**:
   ```bash
   # Terraform example
   terraform init
   terraform plan -var="env=production"
   terraform apply
   ```

2. **Migrate data**:
   ```bash
   # Upload recordings to S3
   aws s3 sync /var/zenlive/recordings/ s3://zenlive-recordings/
   
   # Export database
   pg_dump zenlive | gzip > backup.sql.gz
   
   # Import to cloud database
   gunzip < backup.sql.gz | psql -h db.cloud.com zenlive
   ```

3. **Update DNS**:
   ```bash
   # Point domain to cloud load balancer
   # Test first with hosts file
   echo "52.1.2.3 stream.example.com" >> /etc/hosts
   ```

4. **Gradual migration**:
   - Run both systems in parallel
   - Route new streams to cloud
   - Migrate existing streams gradually
   - Monitor metrics

## Rollback Procedures

### Quick Rollback

1. **Docker**:
   ```bash
   docker pull zenlive:v1.0.0
   docker-compose down
   docker-compose up -d
   ```

2. **Kubernetes**:
   ```bash
   kubectl rollout undo deployment/zenlive
   ```

3. **Binary**:
   ```bash
   systemctl stop zenlive
   cp /usr/local/bin/zenlive-v1.0.0 /usr/local/bin/zenlive
   systemctl start zenlive
   ```

### Data Rollback

1. **Restore configuration**:
   ```bash
   cp config.json.backup config.json
   ```

2. **Restore database**:
   ```bash
   psql zenlive < backup.sql
   ```

3. **Restore recordings**:
   ```bash
   rsync -av backup/recordings/ /var/zenlive/recordings/
   ```

## Testing Migration

### Pre-Migration Checklist

- [ ] Backup all data and configurations
- [ ] Test migration in staging environment
- [ ] Document all custom configurations
- [ ] Notify users of planned downtime
- [ ] Prepare rollback plan
- [ ] Set up monitoring for new system

### Post-Migration Verification

1. **Test streaming**:
   ```bash
   # RTMP
   ffmpeg -re -i test.mp4 -c copy -f flv rtmp://new-server/live/test
   
   # Playback
   ffplay http://new-server:8080/live/test/index.m3u8
   ```

2. **Verify authentication**:
   ```bash
   curl -X POST http://new-server/api/auth/login \
     -d '{"username":"test","password":"test"}'
   ```

3. **Check recordings**:
   ```bash
   curl http://new-server/api/recordings
   ```

4. **Monitor metrics**:
   ```bash
   curl http://new-server:9090/metrics | grep stream
   ```

### Performance Comparison

Before:
```
Active Streams: 100
CPU Usage: 60%
Memory: 8GB
Latency: 15s
```

After:
```
Active Streams: 100
CPU Usage: 40%
Memory: 6GB
Latency: 2s (WebRTC), 8s (HLS)
```

## Common Migration Issues

### Issue: Stream Keys Don't Work

**Cause**: Different authentication format

**Solution**:
```go
// Add compatibility layer
func migrateStreamKey(oldKey string) string {
    // Convert old format to new format
    return newFormat(oldKey)
}
```

### Issue: URLs Changed

**Cause**: Different path structure

**Solution**: Set up redirects
```nginx
location /old-path/ {
    rewrite ^/old-path/(.*)$ /new-path/$1 permanent;
}
```

### Issue: Performance Degradation

**Cause**: Misconfiguration or insufficient resources

**Solution**:
1. Review configuration
2. Scale resources
3. Enable caching
4. Optimize database queries

## Getting Help

If you encounter issues during migration:

1. **Check Documentation**: [docs/](../)
2. **Search Issues**: [GitHub Issues](https://github.com/aminofox/zenlive/issues)
3. **Ask Community**: [Discord](https://discord.gg/zenlive)
4. **Contact Support**: support@zenlive.dev

## Next Steps

- [Getting Started](getting-started.md)
- [Configuration Guide](configuration.md)
- [Troubleshooting](troubleshooting.md)
- [Architecture](architecture.md)
# SDK Simplification - Changelog

## Overview

This document tracks the simplification changes made to ZenLive SDK to make it lighter and more focused on real-time delivery only. Users are now responsible for their own data persistence.

## Philosophy

**Before**: SDK included in-memory storage implementations that bloated the codebase
**After**: SDK only handles real-time message delivery during video/audio/livestream calls

## Removed Components

### 1. Chat Storage System

#### Deleted Files
- `pkg/chat/storage.go` - Complete file removed
  - `Storage` interface
  - `InMemoryStorage` implementation
  - `MessageQuery` and `MessageQueryResult` types
  - `MessageQueryBuilder` helper
  - Helper functions: `GetRecentMessages()`, `GetUserMessages()`

#### Why Removed
- Users have their own databases (PostgreSQL, MySQL, MongoDB)
- In-memory storage was unnecessary duplication
- Chat is just real-time data transmission during calls
- Users persist what they want, when they want

### 2. Message History in Rooms

#### Removed from `pkg/chat/room.go`
- `messageHistory []Message` field
- `maxHistorySize int` field  
- `GetHistory(limit int)` method
- `addToHistory(msg *Message)` method

#### Impact
- Rooms only maintain active connections
- No message caching in memory
- Real-time broadcast only

### 3. Moderation Event Logging

#### Removed from `pkg/chat/moderation.go`
- `ModerationEvent` struct (stored events)
- `moderationLog []*ModerationEvent` field
- `deletedMessages map[string]map[string]bool` field
- `timeouts map[string]map[string]*time.Time` field
- `ActionTimeout` constant
- `ActionDeleteMessage` constant
- `TimeoutUser()` method
- `DeleteMessage()` method
- `IsMessageDeleted()` method
- `GetModerationLog()` method

#### What Remains
- `bannedUsers` - Current session bans
- `mutedUsers` - Current session mutes with expiration
- `BanUser()`, `UnbanUser()`, `IsUserBanned()`
- `MuteUser()`, `UnmuteUser()`, `IsUserMuted()`
- `GetBannedUsers()`, `GetMutedUsers()`
- `CleanupExpired()` - Cleanup expired mutes

#### Why Changed
- Moderation actions should be logged to user's database
- SDK only needs to know current session state
- No need to store history of who did what

### 4. Server Storage Dependency

#### Removed from `pkg/chat/server.go`
- `storage Storage` field from Server struct
- `storage` parameter from `NewServer()` function
- Storage save logic after broadcasting messages

#### What Changed
```go
// Before
func NewServer(config ServerConfig, log logger.Logger, storage Storage) *Server

// After
func NewServer(config ServerConfig, log logger.Logger) *Server
```

### 5. Configuration Misleading Fields

#### Updated in `pkg/config/config.go`
```go
// Before
EnablePersistence bool `json:"enable_persistence"` // No comment

// After  
EnablePersistence bool `json:"enable_persistence"` // In-memory only, users handle DB
```

#### Removed
- `DatabaseConfig` struct - Completely removed
- `Database DatabaseConfig` field from `Config`

#### Why
- SDK never used database
- Users handle their own PostgreSQL/MySQL/MongoDB connections
- Configuration was misleading

## Updated Examples

### `examples/chat/main.go`

**Removed:**
- `runStorageExample()` function - Entire function deleted
- `NewInMemoryStorage()` call
- `storage` parameter to `NewServer()`
- `GetHistory()` calls
- Storage-related logging

**Updated:**
- Added comments about user responsibility for persistence
- Removed `MessageCount` from stats (no longer tracked)
- Simplified moderation example to show current state only

### `README.md`

**Updated:**
- Chat example no longer shows `NewInMemoryStorage()`
- Added comment: "SDK only handles real-time delivery"
- Added comment: "Users handle their own message persistence"

## Documentation Updates

### Created
- `docs/SDK_PHILOSOPHY.md` - Comprehensive design principles
- `docs/CONFIGURATION_SUMMARY.md` - Quick reference guide

### Updated
- `docs/architecture-analysis.md` - Reflects no-database philosophy
- All example config JSON files - Removed database sections

## Test Updates

### `pkg/chat/chat_test.go`

**Removed:**
- `TestStorage()` function - Complete test deleted
- `context` import (no longer needed)
- `ctx` parameter from moderation test methods

**Updated:**
- `TestModerator` - Removed `context.Background()` parameter from method calls
- Updated method signatures: `BanUser()`, `MuteUser()`, `UnbanUser()`, `UnmuteUser()`

## Size Reduction

**Lines of Code Removed:**
- `storage.go`: ~350 lines
- Message history: ~50 lines
- Moderation logging: ~200 lines
- Storage tests: ~60 lines
- Storage example: ~80 lines
- **Total: ~740 lines removed**

**Files Deleted:**
- 1 complete file (`pkg/chat/storage.go`)

## Migration Guide for Users

### If you were using InMemoryStorage:

**Before:**
```go
storage := chat.NewInMemoryStorage()
server := chat.NewServer(config, log, storage)
```

**After:**
```go
server := chat.NewServer(config, log)

// In your WebSocket handler or message receiver:
// Save to YOUR database
err := db.Exec("INSERT INTO messages (room_id, user_id, content) VALUES (?, ?, ?)", 
    msg.RoomID, msg.UserID, msg.Content)
```

### If you were using GetHistory():

**Before:**
```go
history := room.GetHistory(10)
```

**After:**
```go
// Query YOUR database
rows, err := db.Query("SELECT * FROM messages WHERE room_id = ? ORDER BY created_at DESC LIMIT 10", 
    roomID)
```

### If you were logging moderation events:

**Before:**
```go
log := moderator.GetModerationLog(roomID, 10)
```

**After:**
```go
// Save moderation events to YOUR database when they happen:
func onBanUser(roomID, userID, reason string) {
    db.Exec("INSERT INTO moderation_log (room_id, user_id, action, reason) VALUES (?, ?, ?, ?)",
        roomID, userID, "ban", reason)
}
```

## Benefits

1. **Lighter SDK**: 740+ fewer lines of unnecessary code
2. **Clearer Purpose**: SDK does real-time only, users do persistence
3. **More Flexible**: Users choose their own database technology
4. **Better Performance**: No in-memory caching overhead
5. **Scalability**: Users can scale their database independently

## Philosophy Summary

> "ZenLive SDK transmits real-time data during video/audio calls. Just like you wouldn't expect a WebSocket library to include a database, ZenLive doesn't store your application data. We deliver messages in real-time - you persist them however you want."

---

**Date**: 2026-01-11  
**Version**: Post-simplification  
**Status**: Complete ✅
