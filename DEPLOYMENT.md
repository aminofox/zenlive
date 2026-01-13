# ZenLive Server Deployment Guide

## Overview

ZenLive is a production-ready live streaming and video conference server, similar to LiveKit. It supports:

- **WebRTC SFU** for video/audio streaming
- **API Key Authentication** with token-based room access
- **Docker deployment** with Redis integration
- **RESTful API** for room and participant management
- **Horizontal scaling** with Redis backend

## Quick Start

### 1. Run with Docker Compose (Recommended)

```bash
# Start ZenLive server + Redis
docker-compose up -d

# View logs
docker-compose logs -f zenlive

# Stop services
docker-compose down
```

The server will be available at:
- HTTP API: `http://localhost:7880`
- WebRTC Signaling: `ws://localhost:7881`
- Metrics: `http://localhost:9090/metrics`

### 2. Run Locally

```bash
# Build the server
make server

# Run with config file
./bin/zenlive-server --config config.yaml

# Run in development mode
./bin/zenlive-server --config config.yaml --dev
```

## Configuration

### Environment Variables

Create `.env` file (see `.env.example`):

```bash
# Server
ZENLIVE_HOST=0.0.0.0
ZENLIVE_PORT=7880

# Authentication
JWT_SECRET=your-secret-key-change-in-production
API_KEY=your-api-key
SECRET_KEY=your-secret-key

# Redis (optional - for scaling)
REDIS_URL=redis:6379
REDIS_PASSWORD=

# WebRTC
STUN_SERVERS=stun:stun.l.google.com:19302
```

### Configuration File

Edit `config.yaml`:

```yaml
server:
  host: 0.0.0.0
  port: 7880
  signaling_port: 7881
  dev_mode: false

auth:
  jwt_secret: "change-me-in-production"
  default_api_key: ""    # Set in dev mode only
  default_secret_key: "" # Set in dev mode only

redis:
  enabled: true
  address: "redis:6379"
  password: ""
  db: 0

webrtc:
  stun_servers:
    - "stun:stun.l.google.com:19302"
```

## API Usage

### Health Check

```bash
curl http://localhost:7880/api/health
```

### Generate API Key Pair

```bash
# Use the SDK to generate keys
go run examples/apikey/main.go
```

### Create Room Access Token

```bash
# Use the SDK TokenBuilder
go run examples/room-auth/main.go
```

## Development

### Build from Source

```bash
# Install dependencies
go mod download

# Build server
make server

# Run tests
make test

# Run example
go run examples/basic/main.go
```

### Docker Build

```bash
# Build Docker image
docker build -t zenlive:latest .

# Run container
docker run -p 7880:7880 -p 7881:7881 zenlive:latest
```

## Architecture

```
┌─────────────┐         ┌──────────────┐         ┌─────────────┐
│   Client    │────────▶│  ZenLive     │────────▶│   Redis     │
│   (SDK)     │  WebRTC │   Server     │  Cache  │  (Optional) │
└─────────────┘         └──────────────┘         └─────────────┘
                              │
                              ▼
                        ┌──────────────┐
                        │   WebRTC     │
                        │     SFU      │
                        └──────────────┘
```

### Components

- **API Server** (port 7880): HTTP REST API for room management
- **WebRTC Signaling** (port 7881): WebSocket for WebRTC signaling
- **Room Manager**: Manages video call rooms and participants
- **API Key Manager**: Handles authentication with API key/secret pairs
- **Redis (optional)**: For session management and horizontal scaling

## Security

### Production Checklist

- [ ] Change `jwt_secret` in config
- [ ] Generate unique API keys (don't use default dev keys)
- [ ] Enable HTTPS/TLS for API endpoint
- [ ] Enable WSS (WebSocket Secure) for signaling
- [ ] Configure TURN servers for NAT traversal
- [ ] Set up firewall rules (allow ports 7880, 7881, 3478 UDP)
- [ ] Enable Redis authentication
- [ ] Use environment variables for secrets (never commit to git)

### API Key Management

```go
// Generate API key pair
apiKey, secretKey, err := apiKeyManager.GenerateAPIKey(ctx, "My App", nil, nil)

// Build room token
token := auth.NewAccessTokenBuilder(apiKey, secretKey).
    SetIdentity("user123").
    SetRoomJoin("my-room").
    SetCanPublish(true).
    SetCanSubscribe(true).
    Build()

// Join room with token
room.JoinRoomWithToken(ctx, "my-room", token)
```

## Scaling

### Horizontal Scaling

1. Enable Redis in `config.yaml`:
   ```yaml
   redis:
     enabled: true
     address: "redis:6379"
   ```

2. Run multiple ZenLive instances behind a load balancer

3. Use sticky sessions for WebSocket connections

### Monitoring

Prometheus metrics available at `http://localhost:9090/metrics`:

- Active rooms count
- Active participants count
- WebRTC tracks count
- API request rate
- Error rate

## Troubleshooting

### Server won't start

```bash
# Check config file
./bin/zenlive-server --config config.yaml --dev

# Check ports are available
lsof -i :7880
lsof -i :7881
```

### WebRTC connection fails

- Check STUN/TURN server configuration
- Verify firewall allows UDP ports
- Check NAT traversal settings
- Review browser console for ICE errors

### Redis connection fails

```bash
# Test Redis connection
redis-cli -h localhost -p 6379 ping

# Check Redis logs
docker-compose logs redis
```

## Examples

See `examples/` directory for:

- `basic/` - Simple room creation
- `apikey/` - API key generation
- `room-auth/` - Token-based authentication
- `video-call-media/` - Publishing and subscribing to media

## License

See [LICENSE](LICENSE) file.

## Support

- Documentation: `docs/`
- GitHub Issues: Create an issue for bugs or feature requests
- Examples: Check `examples/` directory
