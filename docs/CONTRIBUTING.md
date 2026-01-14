# Contributing to ZenLive

## Development Setup

### Prerequisites

- Go 1.24+
- Docker & Docker Compose
- Redis (optional, for testing scaling)
- Git

### Clone and Build

```bash
# Clone repository
git clone https://github.com/aminofox/zenlive.git
cd zenlive

# Install dependencies
go mod download

# Build packages
make build

# Build server binary
make server

# Run tests
make test
```

### Project Structure

```
zenlive/
├── cmd/
│   └── zenlive-server/       # Server binary entry point
│       └── main.go
├── pkg/
│   ├── api/                  # HTTP REST API server
│   ├── auth/                 # Authentication & JWT tokens
│   │   ├── apikey.go         # API key management
│   │   ├── token_builder.go # Room token builder
│   │   └── jwt.go            # JWT signing/validation
│   ├── cache/                # Caching (memory/Redis)
│   ├── config/               # Configuration loading
│   ├── room/                 # Room management
│   │   ├── manager.go        # Room lifecycle
│   │   ├── participant.go    # Participant handling
│   │   ├── auth.go           # Room authentication
│   │   └── sfu.go            # WebRTC SFU integration
│   ├── streaming/            # Streaming protocols
│   │   ├── webrtc/           # WebRTC implementation
│   │   ├── hls/              # HLS streaming
│   │   └── rtmp/             # RTMP ingest
│   ├── storage/              # Recording & storage
│   └── logger/               # Logging utilities
├── examples/                 # SDK usage examples
├── tests/                    # Integration tests
├── docs/                     # Documentation
├── config.yaml               # Default configuration
├── Dockerfile                # Multi-stage Docker build
├── docker-compose.yml        # Development stack
└── Makefile                  # Build commands
```

---

## Building

### Local Binary

```bash
# Build server
make server

# Run with dev mode
./bin/zenlive-server --config config.yaml --dev

# Run with custom config
./bin/zenlive-server --config myconfig.yaml
```

### Docker Image

```bash
# Build image
make docker

# Run container
make docker-run

# Stop container
make docker-stop

# View logs
docker logs -f zenlive-server
```

### Build Variables

```bash
# Custom Docker image name
DOCKER_IMAGE=myuser/zenlive make docker

# Specific version tag
VERSION=v1.0.0 make docker
```

---

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Test specific package
go test ./pkg/auth/...

# With coverage
make coverage

# Verbose output
go test -v ./...
```

### Integration Tests

```bash
# Start test environment
docker-compose up -d redis

# Run integration tests
go test ./tests/integration/...

# Clean up
docker-compose down
```

### Manual Testing

```bash
# Run server
./bin/zenlive-server --dev

# Test health endpoint
curl http://localhost:7880/api/health

# Run example client
go run examples/room-auth/main.go
```

---

## Code Style

### Format Code

```bash
# Format all files
make fmt

# Check formatting
gofmt -l .

# Run linter
make vet
```

### Guidelines

- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Add comments for exported functions
- Write tests for new features
- Keep functions small and focused

### Example

```go
// GenerateAPIKey creates a new API key pair with optional expiration.
// Returns the generated key or an error if generation fails.
func (m *APIKeyManager) GenerateAPIKey(
    ctx context.Context,
    name string,
    expiresIn *time.Duration,
    metadata map[string]string,
) (*APIKey, error) {
    // Implementation...
}
```

---

## Docker Development

### Local Build

```dockerfile
# Dockerfile uses multi-stage build
FROM golang:1.24-alpine AS builder
# ... build stage ...

FROM alpine:latest
# ... runtime stage ...
```

Build process:
1. Install Go dependencies
2. Copy source code
3. Build static binary
4. Create minimal runtime image

### Publishing

```bash
# Login to Docker Hub
docker login

# Build with version tag
VERSION=v1.0.0 make docker

# Push to registry
make docker-push

# Verify
docker pull aminofox/zenlive:latest
docker run aminofox/zenlive:latest --version
```

### Multi-Architecture

```bash
# Setup buildx
docker buildx create --use --name multiarch

# Build for multiple platforms
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t aminofox/zenlive:latest \
  --push \
  .
```

---

## Architecture

### Authentication Flow

```
1. Generate API Key Pair
   ↓
2. Store API Key (with expiration)
   ↓
3. Build Room Token (JWT)
   ↓
4. Client joins room with token
   ↓
5. Server validates token
   ↓
6. Grant permissions (publish/subscribe)
```

### Room Management

```
RoomManager
├── CreateRoom(name)
├── GetRoom(name)
├── CloseRoom(name)
└── ListRooms()

Room
├── AddParticipant(token)
├── RemoveParticipant(id)
├── PublishTrack(participant, track)
└── SubscribeToTrack(subscriber, trackId)
```

### WebRTC SFU

```
Publisher → Track → SFU → Subscribers
                     │
                     ├─→ Subscriber 1
                     ├─→ Subscriber 2
                     └─→ Subscriber N
```

---

## Configuration

### Development Config

```yaml
server:
  host: 0.0.0.0
  port: 7880
  signaling_port: 7881
  dev_mode: true

auth:
  jwt_secret: "dev-secret-change-in-prod"
  default_api_key: "devkey"
  default_secret_key: "devsecret"

redis:
  enabled: false  # Use in-memory for dev

logging:
  level: "debug"
  format: "text"
```

### Production Config

```yaml
server:
  dev_mode: false

auth:
  jwt_secret: "${JWT_SECRET}"  # From environment

redis:
  enabled: true
  address: "${REDIS_URL}"
  password: "${REDIS_PASSWORD}"

logging:
  level: "info"
  format: "json"
```

### Environment Override

```bash
# .env file
JWT_SECRET=production-secret
REDIS_URL=redis:6379
REDIS_PASSWORD=secure-password

# Load and run
source .env
./bin/zenlive-server --config config.yaml
```

---

## Future: Separate Server & SDK

### Current (Monorepo)

```
github.com/aminofox/zenlive
├── cmd/zenlive-server/    # Server binary
└── pkg/                   # SDK packages
```

**Pros:**
- Faster development
- Single version
- Easy to maintain

**Cons:**
- Large dependency tree
- Cannot version separately
- No multi-language SDK

### Future (Modular Architecture)

```
zenlive-server          # Server only
├── Server binary
└── Docker image

zenlive-go             # Go SDK
├── Client library
└── go get install

zenlive-protocol       # Protocol definitions
└── Protobuf schemas
```

**Benefits:**
- Smaller dependencies
- Independent versioning
- Multi-language support (JS, Swift, Kotlin)
- Cleaner separation

### Migration Steps

1. **Phase 1:** Extract protocol definitions
2. **Phase 2:** Split server into separate repo
3. **Phase 3:** Create clean Go SDK
4. **Phase 4:** Build JS/mobile SDKs

See implementation details in project planning docs.

---

## Common Tasks

### Add New API Endpoint

```go
// pkg/api/server.go
func (s *Server) handleNewEndpoint(w http.ResponseWriter, r *http.Request) {
    // Implementation
}

// Register in NewServer
mux.HandleFunc("/api/new-endpoint", s.handleNewEndpoint)
```

### Add Configuration Option

```go
// pkg/config/config.go
type ServerConfig struct {
    // ... existing fields ...
    NewOption string `yaml:"new_option"`
}

// Update DefaultConfig()
func DefaultConfig() *Config {
    return &Config{
        Server: ServerConfig{
            NewOption: "default-value",
        },
    }
}
```

### Add Room Permission

```go
// pkg/auth/token_builder.go
func (b *AccessTokenBuilder) SetCanDoSomething(can bool) *AccessTokenBuilder {
    b.grant.CanDoSomething = can
    return b
}

// pkg/room/participant.go
type Participant struct {
    CanDoSomething bool
}

// Enforce in room logic
if !participant.CanDoSomething {
    return errors.New("permission denied")
}
```

---

## Troubleshooting

### Build Errors

```bash
# Clear cache
go clean -cache

# Update dependencies
go mod tidy

# Rebuild
make clean && make build
```

### Docker Issues

```bash
# Remove old containers
docker-compose down -v

# Rebuild images
docker-compose build --no-cache

# Check logs
docker-compose logs -f
```

### Redis Connection

```bash
# Test Redis
redis-cli ping

# Check connection
docker exec -it zenlive-redis redis-cli ping

# View keys
docker exec -it zenlive-redis redis-cli keys "zenlive:*"
```

---

## Release Process

### Version Bump

```bash
# Tag release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# Build release binary
VERSION=v1.0.0 make server

# Build Docker image
VERSION=v1.0.0 make docker
```

### Publish

```bash
# Push Docker image
make docker-push

# Create GitHub release
# Upload bin/zenlive-server binary
```

### Changelog

Update CHANGELOG.md:
```markdown
## [1.0.0] - 2026-01-13
### Added
- Feature X
- Feature Y

### Fixed
- Bug fix Z
```

---

## Getting Help

- Read documentation in `docs/`
- Check examples in `examples/`
- Search existing issues
- Ask in discussions

---

## Pull Request Guidelines

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing`)
5. Open Pull Request

**PR Checklist:**
- [ ] Tests pass (`make test`)
- [ ] Code formatted (`make fmt`)
- [ ] Documentation updated
- [ ] Example added (if new feature)
- [ ] Changelog updated

---

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
