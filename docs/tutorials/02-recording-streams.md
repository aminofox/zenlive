# Tutorial: Recording and Storing Streams

## Introduction

Learn how to automatically record live streams and store them locally or in S3-compatible storage.

**What you'll learn**:
- Configuring automatic recording
- Using local filesystem storage
- Using S3/MinIO for cloud storage
- Generating thumbnails
- Managing recording metadata

**Prerequisites**:
- Completed [First Streaming Server Tutorial](01-first-streaming-server.md)
- Basic understanding of cloud storage (for S3 section)

**Estimated time**: 20 minutes

## Step 1: Enable Local Recording

Update your `main.go`:

```go
import (
    "github.com/aminofox/zenlive/pkg/storage"
)

func main() {
    // ... previous RTMP/HLS setup ...
    
    // Create local storage
    localStorage := storage.NewLocalStorage(&storage.LocalConfig{
        BasePath: "./recordings",
    })
    
    // Create recorder
    recorder := storage.NewRecorder(&storage.RecorderConfig{
        Format:  storage.FormatMP4,
        Storage: localStorage,
    })
    
    // Auto-record on publish
    rtmpServer.OnPublishStart = func(streamID string) {
        log.Printf("Stream started: %s - Starting recording", streamID)
        if err := recorder.Start(streamID); err != nil {
            log.Printf("Failed to start recording: %v", err)
        }
    }
    
    rtmpServer.OnPublishEnd = func(streamID string) {
        log.Printf("Stream ended: %s - Stopping recording", streamID)
        if err := recorder.Stop(streamID); err != nil {
            log.Printf("Failed to stop recording: %v", err)
        }
    }
    
    // ... rest of code ...
}
```

## Step 2: Test Recording

Start your server and publish a stream:

```bash
ffmpeg -re -i test.mp4 -c copy -f flv rtmp://localhost:1935/live/mystream
```

Stop after a few seconds. Check the recordings directory:

```bash
ls -lh recordings/
# Should see: mystream_2026-01-11_10-30-00.mp4
```

Play the recorded file:

```bash
ffplay recordings/mystream_2026-01-11_10-30-00.mp4
```

## Step 3: Add Thumbnail Generation

Enable thumbnails:

```go
recorder := storage.NewRecorder(&storage.RecorderConfig{
    Format:            storage.FormatMP4,
    Storage:           localStorage,
    EnableThumbnails:  true,
    ThumbnailInterval: 30, // seconds
})
```

After recording, check for thumbnails:

```bash
ls -lh recordings/mystream/thumbnails/
# thumb_00000.jpg, thumb_00030.jpg, thumb_00060.jpg, ...
```

## Step 4: Store Metadata

Add metadata tracking:

```go
import (
    "time"
)

type RecordingMetadata struct {
    StreamID  string
    StartTime time.Time
    EndTime   time.Time
    Duration  time.Duration
    FileSize  int64
    FilePath  string
}

var recordings = make(map[string]*RecordingMetadata)

rtmpServer.OnPublishStart = func(streamID string) {
    recordings[streamID] = &RecordingMetadata{
        StreamID:  streamID,
        StartTime: time.Now(),
    }
    
    recorder.Start(streamID)
}

rtmpServer.OnPublishEnd = func(streamID string) {
    if meta, exists := recordings[streamID]; exists {
        meta.EndTime = time.Now()
        meta.Duration = meta.EndTime.Sub(meta.StartTime)
        
        // Get file info
        if info, err := recorder.GetRecordingInfo(streamID); err == nil {
            meta.FileSize = info.Size
            meta.FilePath = info.Path
        }
        
        log.Printf("Recording completed: %s (Duration: %v, Size: %d MB)",
            streamID, meta.Duration, meta.FileSize/1024/1024)
    }
    
    recorder.Stop(streamID)
}
```

## Step 5: Configure S3 Storage

Install AWS SDK:

```bash
go get github.com/aws/aws-sdk-go-v2
```

Configure S3 storage in code:

```go
import (
    "os"
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    cfg := config.DefaultConfig()
    
    // Configure S3 storage
    cfg.Storage.Type = "s3"
    cfg.Storage.S3 = config.S3Config{
        Bucket:          "my-streaming-recordings",
        Region:          "us-east-1",
        Endpoint:        "https://s3.amazonaws.com",
        AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
        SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
        UseSSL:          true,
    }
    
    // Or use local storage
    // cfg.Storage.Type = "local"
    // cfg.Storage.BasePath = "./recordings"
    
    // Create SDK with config
    sdk, _ := zenlive.New(cfg)
    
    // Create storage based on config
    storageBackend := createStorage()
    
    recorder := storage.NewRecorder(&storage.RecorderConfig{
        Format:  storage.FormatMP4,
        Storage: storageBackend,
    })
    
    // ... rest of code ...
}
```

## Step 6: Use MinIO (Local S3)

For local development with S3-compatible storage:

Start MinIO with Docker:

```bash
docker run -d \
  -p 9000:9000 \
  -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data --console-address ":9001"
```

Create bucket:

```bash
# Access MinIO console at http://localhost:9001
# Login: minioadmin / minioadmin
# Create bucket: "recordings"
```

Configure MinIO in code:

```go
import (
    "github.com/aminofox/zenlive/pkg/config"
)

cfg := config.DefaultConfig()
cfg.Storage.Type = "s3"
cfg.Storage.S3 = config.S3Config{
    Bucket:          "recordings",
    Region:          "us-east-1",
    Endpoint:        "http://localhost:9000",
    AccessKeyID:     "minioadmin",
    SecretAccessKey: "minioadmin",
    UseSSL:          false,
}
```

## Step 7: Add Recording API

Create REST API to list and download recordings:

```go
func setupRecordingAPI(recorder *storage.Recorder) {
    // List all recordings
    http.HandleFunc("/api/recordings", func(w http.ResponseWriter, r *http.Request) {
        recordings := recorder.ListRecordings()
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(recordings)
    })
    
    // Get recording details
    http.HandleFunc("/api/recordings/", func(w http.ResponseWriter, r *http.Request) {
        streamID := r.URL.Path[len("/api/recordings/"):]
        
        info, err := recorder.GetRecordingInfo(streamID)
        if err != nil {
            http.Error(w, "Recording not found", 404)
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(info)
    })
    
    // Download recording
    http.HandleFunc("/api/recordings/download/", func(w http.ResponseWriter, r *http.Request) {
        streamID := r.URL.Path[len("/api/recordings/download/"):]
        
        reader, err := recorder.Download(streamID)
        if err != nil {
            http.Error(w, "Recording not found", 404)
            return
        }
        defer reader.Close()
        
        w.Header().Set("Content-Type", "video/mp4")
        w.Header().Set("Content-Disposition", "attachment; filename="+streamID+".mp4")
        io.Copy(w, reader)
    })
}

func main() {
    // ... previous code ...
    
    setupRecordingAPI(recorder)
    
    // ... rest of code ...
}
```

Test the API:

```bash
# List recordings
curl http://localhost:8081/api/recordings

# Get recording info
curl http://localhost:8081/api/recordings/mystream

# Download recording
curl -O http://localhost:8081/api/recordings/download/mystream
```

## Step 8: Automatic Cleanup

Add automatic cleanup of old recordings:

```go
import (
    "time"
)

func startCleanupJob(recorder *storage.Recorder) {
    ticker := time.NewTicker(24 * time.Hour) // Run daily
    
    go func() {
        for range ticker.C {
            log.Println("Running cleanup job...")
            
            // Delete recordings older than 30 days
            cutoff := time.Now().AddDate(0, 0, -30)
            
            recordings := recorder.ListRecordings()
            for _, rec := range recordings {
                if rec.CreatedAt.Before(cutoff) {
                    log.Printf("Deleting old recording: %s", rec.StreamID)
                    if err := recorder.Delete(rec.StreamID); err != nil {
                        log.Printf("Failed to delete %s: %v", rec.StreamID, err)
                    }
                }
            }
        }
    }()
}

func main() {
    // ... previous code ...
    
    startCleanupJob(recorder)
    
    // ... rest of code ...
}
```

## Step 9: Recording Statistics

Track recording statistics:

```go
type RecordingStats struct {
    TotalRecordings   int
    TotalDuration     time.Duration
    TotalSize         int64
    AverageDuration   time.Duration
    LargestRecording  int64
}

func getRecordingStats(recorder *storage.Recorder) RecordingStats {
    recordings := recorder.ListRecordings()
    
    stats := RecordingStats{
        TotalRecordings: len(recordings),
    }
    
    for _, rec := range recordings {
        stats.TotalDuration += rec.Duration
        stats.TotalSize += rec.FileSize
        
        if rec.FileSize > stats.LargestRecording {
            stats.LargestRecording = rec.FileSize
        }
    }
    
    if len(recordings) > 0 {
        stats.AverageDuration = stats.TotalDuration / time.Duration(len(recordings))
    }
    
    return stats
}

// Add to API
http.HandleFunc("/api/recordings/stats", func(w http.ResponseWriter, r *http.Request) {
    stats := getRecordingStats(recorder)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
})
```

## Complete Example

Here's the complete `main.go` with recording:

```go
package main

import (
    "encoding/json"
    "io"
    "log"
    "net/http"
    "time"
    
    "github.com/aminofox/zenlive/pkg/streaming/rtmp"
    "github.com/aminofox/zenlive/pkg/streaming/hls"
    "github.com/aminofox/zenlive/pkg/storage"
    "github.com/aminofox/zenlive/pkg/config"
)

type RecordingMetadata struct {
    StreamID  string
    StartTime time.Time
    EndTime   time.Time
    Duration  time.Duration
    FileSize  int64
    FilePath  string
}

var recordings = make(map[string]*RecordingMetadata)

func createStorage(cfg *config.Config) storage.Storage {
    switch cfg.Storage.Type {
    case "s3":
        return storage.NewS3Storage(&storage.S3Config{
            Bucket:          cfg.Storage.S3.Bucket,
            Region:          cfg.Storage.S3.Region,
            Endpoint:        cfg.Storage.S3.Endpoint,
            AccessKeyID:     cfg.Storage.S3.AccessKeyID,
            SecretAccessKey: cfg.Storage.S3.SecretAccessKey,
            UseSSL:          cfg.Storage.S3.UseSSL,
        })
    default:
        return storage.NewLocalStorage(&storage.LocalConfig{
            BasePath: cfg.Storage.BasePath,
        })
    }
}

func setupRecordingAPI(recorder *storage.Recorder) {
    http.HandleFunc("/api/recordings", func(w http.ResponseWriter, r *http.Request) {
        list := recorder.ListRecordings()
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(list)
    })
    
    http.HandleFunc("/api/recordings/download/", func(w http.ResponseWriter, r *http.Request) {
        streamID := r.URL.Path[len("/api/recordings/download/"):]
        
        reader, err := recorder.Download(streamID)
        if err != nil {
            http.Error(w, "Not found", 404)
            return
        }
        defer reader.Close()
        
        w.Header().Set("Content-Type", "video/mp4")
        w.Header().Set("Content-Disposition", "attachment; filename="+streamID+".mp4")
        io.Copy(w, reader)
    })
}

func main() {
    loadConfig()
    
    // Create storage and recorder
    storageBackend := createStorage()
    recorder := storage.NewRecorder(&storage.RecorderConfig{
        Format:            storage.FormatMP4,
        Storage:           storageBackend,
        EnableThumbnails:  true,
        ThumbnailInterval: 30,
    })
    
    // Create servers
    rtmpServer := rtmp.NewServer(&rtmp.ServerConfig{
        ListenAddr: ":1935",
        OnPublishStart: func(streamID string) {
            log.Printf("▶ Stream started: %s", streamID)
            
            recordings[streamID] = &RecordingMetadata{
                StreamID:  streamID,
                StartTime: time.Now(),
            }
            
            if err := recorder.Start(streamID); err != nil {
                log.Printf("Failed to start recording: %v", err)
            }
        },
        OnPublishEnd: func(streamID string) {
            log.Printf("■ Stream ended: %s", streamID)
            
            if meta, exists := recordings[streamID]; exists {
                meta.EndTime = time.Now()
                meta.Duration = meta.EndTime.Sub(meta.StartTime)
                
                if info, err := recorder.GetRecordingInfo(streamID); err == nil {
                    meta.FileSize = info.Size
                    meta.FilePath = info.Path
                }
                
                log.Printf("Recording: %s (Duration: %v, Size: %d MB)",
                    streamID, meta.Duration, meta.FileSize/1024/1024)
            }
            
            if err := recorder.Stop(streamID); err != nil {
                log.Printf("Failed to stop recording: %v", err)
            }
        },
    })
    
    hlsServer := hls.NewServer(&hls.ServerConfig{
        ListenAddr:      ":8080",
        SegmentDuration: 4,
        PlaylistSize:    5,
    })
    
    // Start servers
    go rtmpServer.Start()
    go hlsServer.Start()
    
    // Setup APIs
    setupRecordingAPI(recorder)
    go http.ListenAndServe(":8081", nil)
    
    log.Println("=== Streaming Server with Recording ===")
    log.Println("RTMP: rtmp://localhost:1935/live/{streamkey}")
    log.Println("HLS: http://localhost:8080/live/{streamkey}/index.m3u8")
    log.Println("API: http://localhost:8081/api/recordings")
    
    select {}
}
```

## Next Steps

- [WebRTC Streaming Tutorial](03-webrtc-streaming.md)
- [Chat Integration Tutorial](04-chat-integration.md)
- [Deployment Guide](../getting-started.md#deployment)

## Troubleshooting

**Recording file is empty**:
- Check disk space
- Verify write permissions on recordings directory
- Check logs for encoding errors

**S3 upload fails**:
- Verify credentials
- Check bucket permissions
- Test with AWS CLI

**Thumbnails not generating**:
- Ensure FFmpeg is installed
- Check `enable_thumbnails` is true
- Verify stream has video track
