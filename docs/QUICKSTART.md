# ZenLive Quickstart Guide

Get started with ZenLive in less than 10 minutes! This guide will help you set up your first video conferencing room.

## Table of Contents

- [Installation](#installation)
- [Basic Setup](#basic-setup)
- [Create Your First Room](#create-your-first-room)
- [Video Conference Example](#video-conference-example)
- [WebSocket Client Example](#websocket-client-example)
- [Next Steps](#next-steps)

## Installation

### Prerequisites

- Go 1.23 or higher
- Git

### Install ZenLive

```bash
go get github.com/aminofox/zenlive
```

## Basic Setup

### 1. Minimal Server

Create a file `main.go`:

```go
package main

import (
    "log"
    
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    // Create default configuration
    cfg := config.DefaultConfig()
    
    // Create SDK instance
    sdk, err := zenlive.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    
    // Start the SDK
    if err := sdk.Start(); err != nil {
        log.Fatal(err)
    }
    defer sdk.Stop()
    
    log.Println("ZenLive server started")
    
    // Keep running
    select {}
}
```

Run it:

```bash
go run main.go
```

That's it! Your ZenLive server is now running.

### 2. Server with REST API

To enable the REST API for room management:

```go
package main

import (
    "log"
    
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/api"
    "github.com/aminofox/zenlive/pkg/auth"
    "github.com/aminofox/zenlive/pkg/config"
)

func main() {
    // Create configuration
    cfg := config.DefaultConfig()
    
    // Create SDK
    sdk, err := zenlive.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    
    // Create JWT authenticator
    jwtSecret := "your-secret-key-change-in-production"
    jwtAuth := auth.NewJWTAuthenticator(jwtSecret)
    
    // Create API server
    apiConfig := api.DefaultConfig()
    apiConfig.JWTSecret = jwtSecret
    
    apiServer := api.NewServer(
        sdk.GetRoomManager(),
        jwtAuth,
        apiConfig,
    )
    
    // Start SDK
    if err := sdk.Start(); err != nil {
        log.Fatal(err)
    }
    defer sdk.Stop()
    
    // Start API server
    log.Println("Starting API server on :8080")
    if err := apiServer.Start(); err != nil {
        log.Fatal(err)
    }
}
```

Run it:

```bash
go run main.go
```

Your server now has:
- REST API on `http://localhost:8080`
- WebSocket signaling on `ws://localhost:8080/ws`
- Health check on `http://localhost:8080/api/health`

## Create Your First Room

### Using REST API

#### 1. Create a Room

```bash
curl -X POST http://localhost:8080/api/rooms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My First Room",
    "max_participants": 10
  }'
```

Response:

```json
{
  "id": "room_abc123",
  "name": "My First Room",
  "created_at": "2026-01-12T10:00:00Z",
  "max_participants": 10,
  "num_participants": 0
}
```

#### 2. List Rooms

```bash
curl http://localhost:8080/api/rooms
```

#### 3. Get Room Details

```bash
curl http://localhost:8080/api/rooms/room_abc123
```

#### 4. Generate Access Token

```bash
curl -X POST http://localhost:8080/api/rooms/room_abc123/tokens \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user_123",
    "username": "John Doe",
    "role": "host"
  }'
```

Response:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "participant_id": "participant_xyz789"
}
```

### Using SDK Directly

```go
package main

import (
    "log"
    "time"
    
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/config"
    "github.com/aminofox/zenlive/pkg/room"
)

func main() {
    cfg := config.DefaultConfig()
    sdk, _ := zenlive.New(cfg)
    sdk.Start()
    defer sdk.Stop()
    
    // Get room manager
    roomMgr := sdk.GetRoomManager()
    
    // Create a room
    newRoom, err := roomMgr.CreateRoom(&room.CreateRoomRequest{
        Name:            "My Video Room",
        MaxParticipants: 10,
        EmptyTimeout:    5 * time.Minute,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Room created: %s (ID: %s)", newRoom.Name, newRoom.ID)
    
    // Add participant
    participant, err := newRoom.AddParticipant(&room.Participant{
        UserID:   "user_123",
        Username: "John Doe",
        Role:     room.RoleHost,
    }, "")
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Participant joined: %s", participant.Username)
    
    // List participants
    participants := newRoom.ListParticipants()
    log.Printf("Total participants: %d", len(participants))
    
    select {}
}
```

## Video Conference Example

Complete example of a video conference server:

```go
package main

import (
    "log"
    "time"
    
    "github.com/aminofox/zenlive"
    "github.com/aminofox/zenlive/pkg/api"
    "github.com/aminofox/zenlive/pkg/auth"
    "github.com/aminofox/zenlive/pkg/config"
    "github.com/aminofox/zenlive/pkg/logger"
    "github.com/aminofox/zenlive/pkg/room"
)

func main() {
    // Setup configuration
    cfg := config.DefaultConfig()
    cfg.Logging.Level = "debug"
    
    // Create SDK
    sdk, err := zenlive.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    
    // Setup authentication
    jwtSecret := "my-super-secret-key"
    jwtAuth := auth.NewJWTAuthenticator(jwtSecret)
    
    // Setup API server
    apiConfig := api.DefaultConfig()
    apiConfig.Addr = ":8080"
    apiConfig.JWTSecret = jwtSecret
    
    apiServer := api.NewServer(
        sdk.GetRoomManager(),
        jwtAuth,
        apiConfig,
    )
    
    // Setup event callbacks
    roomMgr := sdk.GetRoomManager()
    
    roomMgr.OnRoomCreated(func(r *room.Room) {
        log.Printf("[EVENT] Room created: %s (ID: %s)", r.Name, r.ID)
    })
    
    roomMgr.OnRoomDeleted(func(roomID string) {
        log.Printf("[EVENT] Room deleted: %s", roomID)
    })
    
    roomMgr.OnParticipantJoined(func(roomID string, p *room.Participant) {
        log.Printf("[EVENT] Participant joined room %s: %s", roomID, p.Username)
    })
    
    roomMgr.OnParticipantLeft(func(roomID, participantID string) {
        log.Printf("[EVENT] Participant left room %s: %s", roomID, participantID)
    })
    
    roomMgr.OnTrackPublished(func(roomID, participantID, trackID string) {
        log.Printf("[EVENT] Track published in room %s by %s: %s", 
            roomID, participantID, trackID)
    })
    
    // Start SDK
    if err := sdk.Start(); err != nil {
        log.Fatal(err)
    }
    defer sdk.Stop()
    
    // Create demo rooms
    createDemoRooms(roomMgr)
    
    // Start API server
    log.Printf("🚀 ZenLive Video Conference Server started")
    log.Printf("📡 REST API: http://localhost:8080")
    log.Printf("🔌 WebSocket: ws://localhost:8080/ws")
    log.Printf("❤️  Health: http://localhost:8080/api/health")
    
    if err := apiServer.Start(); err != nil {
        log.Fatal(err)
    }
}

func createDemoRooms(roomMgr *room.RoomManager) {
    rooms := []struct {
        name string
        max  int
    }{
        {"Team Standup", 10},
        {"Product Demo", 50},
        {"All Hands Meeting", 100},
    }
    
    for _, r := range rooms {
        room, err := roomMgr.CreateRoom(&room.CreateRoomRequest{
            Name:            r.name,
            MaxParticipants: r.max,
            EmptyTimeout:    10 * time.Minute,
        })
        if err != nil {
            log.Printf("Failed to create room %s: %v", r.name, err)
            continue
        }
        log.Printf("✓ Created demo room: %s (ID: %s)", room.Name, room.ID)
    }
}
```

Save as `server.go` and run:

```bash
go run server.go
```

## WebSocket Client Example

Connect to a room via WebSocket:

```go
package main

import (
    "encoding/json"
    "log"
    "net/url"
    "os"
    "os/signal"
    "time"
    
    "github.com/gorilla/websocket"
)

type WSMessage struct {
    Type   string          `json:"type"`
    RoomID string          `json:"room_id,omitempty"`
    Data   json.RawMessage `json:"data,omitempty"`
}

func main() {
    // Get token (from REST API)
    token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
    
    // Connect to WebSocket
    u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
    headers := map[string][]string{
        "Authorization": {token},
    }
    
    conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    
    log.Println("Connected to WebSocket")
    
    // Handle incoming messages
    done := make(chan struct{})
    
    go func() {
        defer close(done)
        for {
            var msg WSMessage
            err := conn.ReadJSON(&msg)
            if err != nil {
                log.Println("Read error:", err)
                return
            }
            log.Printf("Received: %s - %s", msg.Type, string(msg.Data))
        }
    }()
    
    // Join room
    joinMsg := WSMessage{
        Type:   "join_room",
        RoomID: "room_abc123",
    }
    if err := conn.WriteJSON(joinMsg); err != nil {
        log.Fatal(err)
    }
    log.Println("Sent: join_room")
    
    // Publish track
    time.Sleep(1 * time.Second)
    publishMsg := WSMessage{
        Type: "publish_track",
        Data: json.RawMessage(`{
            "track_id": "track_123",
            "kind": "video",
            "source": "camera"
        }`),
    }
    if err := conn.WriteJSON(publishMsg); err != nil {
        log.Fatal(err)
    }
    log.Println("Sent: publish_track")
    
    // Wait for interrupt signal
    interrupt := make(chan os.Signal, 1)
    signal.Notify(interrupt, os.Interrupt)
    
    <-interrupt
    log.Println("Closing connection...")
}
```

## Testing Your Setup

### 1. Health Check

```bash
curl http://localhost:8080/api/health
```

Expected response:

```json
{
  "status": "ok",
  "timestamp": "2026-01-12T10:00:00Z"
}
```

### 2. Create Room and Join

```bash
# Create room
ROOM_RESPONSE=$(curl -s -X POST http://localhost:8080/api/rooms \
  -H "Content-Type: application/json" \
  -d '{"name": "Test Room"}')

ROOM_ID=$(echo $ROOM_RESPONSE | jq -r '.id')
echo "Room ID: $ROOM_ID"

# Generate token
TOKEN_RESPONSE=$(curl -s -X POST http://localhost:8080/api/rooms/$ROOM_ID/tokens \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_123", "username": "Test User", "role": "host"}')

TOKEN=$(echo $TOKEN_RESPONSE | jq -r '.token')
echo "Access Token: $TOKEN"

# Get room details
curl http://localhost:8080/api/rooms/$ROOM_ID
```

### 3. WebSocket Test (using wscat)

Install wscat:

```bash
npm install -g wscat
```

Connect to WebSocket:

```bash
# Replace TOKEN with your JWT token
wscat -c "ws://localhost:8080/ws" -H "Authorization: Bearer TOKEN"
```

Send messages:

```json
# Join room
{"type": "join_room", "room_id": "room_abc123"}

# Publish track
{"type": "publish_track", "data": {"track_id": "track_123", "kind": "video", "source": "camera"}}

# Leave room
{"type": "leave_room", "room_id": "room_abc123"}
```

## Configuration Options

Customize your server configuration:

```go
cfg := &config.Config{
    Server: config.ServerConfig{
        Host: "0.0.0.0",
        Port: 8080,
    },
    Logging: config.LoggingConfig{
        Level:  "info",   // debug, info, warn, error
        Format: "json",   // json, text
    },
}
```

## Common Use Cases

### 1. Video Call (1-on-1)

```go
room, _ := roomMgr.CreateRoom(&room.CreateRoomRequest{
    Name:            "Private Call",
    MaxParticipants: 2,
})
```

### 2. Video Conference (Team Meeting)

```go
room, _ := roomMgr.CreateRoom(&room.CreateRoomRequest{
    Name:            "Team Meeting",
    MaxParticipants: 10,
})
```

### 3. Webinar (One Speaker, Many Viewers)

```go
room, _ := roomMgr.CreateRoom(&room.CreateRoomRequest{
    Name:            "Company Webinar",
    MaxParticipants: 1000,
})

// Host can publish
hostParticipant := &room.Participant{
    Role: room.RoleHost,
}

// Attendees can only watch
attendeeParticipant := &room.Participant{
    Role: room.RoleAttendee, // Cannot publish
}
```

## Troubleshooting

### Server won't start

**Problem:** Port already in use

```
panic: listen tcp :8080: bind: address already in use
```

**Solution:** Change the port

```go
apiConfig.Addr = ":8081"
```

### Cannot create room

**Problem:** Missing room name

**Solution:** Always provide a room name

```go
roomMgr.CreateRoom(&room.CreateRoomRequest{
    Name: "My Room", // Required!
})
```

### WebSocket connection refused

**Problem:** Not using correct authentication

**Solution:** Include JWT token in Authorization header

```go
headers := map[string][]string{
    "Authorization": {"Bearer " + token},
}
```

## Next Steps

Now that you have a working ZenLive server, explore more features:

1. **[Architecture Guide](architecture.md)** - Understand how ZenLive works
2. **[Examples Directory](../examples/)** - Browse working code examples
3. **[API Reference](api-reference.md)** - Complete API documentation
4. **[Deployment Guide](deployment.md)** - Deploy to production

### Recommended Examples

- [examples/room/](../examples/room/) - Basic room management
- [examples/video-conference/](../examples/video-conference/) - Full video conference server
- [examples/websocket/](../examples/websocket/) - WebSocket client example
- [examples/api/](../examples/api/) - REST API examples

## Getting Help

- **Documentation**: Check the [docs/](.) folder
- **Examples**: Browse [examples/](../examples/)
- **Issues**: Report bugs on GitHub
- **Community**: Join our discussions

## What's Next?

- Add recording functionality
- Enable RTMP streaming
- Setup clustering for scale
- Build a web client (JavaScript SDK)
- Mobile apps (iOS/Android SDKs)

Happy coding! 🚀
