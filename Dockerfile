FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o zenlive-server ./cmd/zenlive-server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/zenlive-server .
COPY --from=builder /app/config.yaml .

# Create recordings directory
RUN mkdir -p /app/recordings

# Expose ports
EXPOSE 7880 7881 9090

# Run the server
ENTRYPOINT ["./zenlive-server"]
CMD ["-config", "config.yaml"]
