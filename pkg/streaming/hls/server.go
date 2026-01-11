// Package hls implements HTTP server for HLS delivery
package hls

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// Server serves HLS content over HTTP
type Server struct {
	config *ServerConfig
	logger logger.Logger

	// transmuxer is the HLS transmuxer
	transmuxer *Transmuxer

	// httpServer is the HTTP server
	httpServer *http.Server

	// segmentCache caches segments in memory
	segmentCache map[string]*CachedSegment

	// running indicates if server is running
	running bool

	mu sync.RWMutex
}

// CachedSegment represents a cached HLS segment
type CachedSegment struct {
	Data      []byte
	CachedAt  time.Time
	MimeType  string
	ExpiresAt time.Time
}

// NewServer creates a new HLS HTTP server
func NewServer(config *ServerConfig, transmuxer *Transmuxer, log logger.Logger) (*Server, error) {
	if config == nil {
		config = DefaultServerConfig()
	}

	server := &Server{
		config:       config,
		logger:       log,
		transmuxer:   transmuxer,
		segmentCache: make(map[string]*CachedSegment),
		running:      false,
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleRequest)

	server.httpServer = &http.Server{
		Addr:         config.Address,
		Handler:      mux,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	return server, nil
}

// Start starts the HLS HTTP server
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("Starting HLS HTTP server", logger.Field{Key: "address", Value: s.config.Address})

	// Start cache cleanup goroutine
	go s.cacheCleanupLoop()

	// Start HTTP server
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

// Stop stops the HLS HTTP server
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return fmt.Errorf("server not running")
	}
	s.mu.Unlock()

	s.logger.Info("Stopping HLS HTTP server")

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	return nil
}

// handleRequest handles HTTP requests for playlists and segments
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Log request
	s.logger.Debug("HTTP request",
		logger.Field{Key: "method", Value: r.Method},
		logger.Field{Key: "path", Value: r.URL.Path},
		logger.Field{Key: "remote", Value: r.RemoteAddr})

	// Set CORS headers if enabled
	if s.config.EnableCORS {
		s.setCORSHeaders(w, r)

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request path
	// Expected format: /{streamKey}/master.m3u8 or /{streamKey}/playlist.m3u8 or /{streamKey}/segment_N.ts
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		s.serveIndex(w, r)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	streamKey := parts[0]
	filename := parts[len(parts)-1]

	// Check if transmuxer has this stream
	if s.transmuxer != nil {
		if _, err := s.transmuxer.GetStreamInfo(streamKey); err != nil {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}
	}

	// Determine file type
	if strings.HasSuffix(filename, ".m3u8") {
		s.servePlaylist(w, r, streamKey, filename)
	} else if strings.HasSuffix(filename, ".ts") {
		s.serveSegment(w, r, streamKey, filename)
	} else {
		http.Error(w, "Invalid file type", http.StatusBadRequest)
	}
}

// servePlaylist serves M3U8 playlists
func (s *Server) servePlaylist(w http.ResponseWriter, r *http.Request, streamKey, filename string) {
	// Build file path
	filePath := filepath.Join(s.transmuxer.config.OutputDir, streamKey, filename)

	// Read playlist file
	data, err := os.ReadFile(filePath)
	if err != nil {
		s.logger.Error("Failed to read playlist",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "path", Value: filePath})
		http.Error(w, "Playlist not found", http.StatusNotFound)
		return
	}

	// Set headers
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", s.config.PlaylistCacheControl)

	// Write response
	w.WriteHeader(http.StatusOK)
	w.Write(data)

	s.logger.Debug("Served playlist",
		logger.Field{Key: "streamKey", Value: streamKey},
		logger.Field{Key: "filename", Value: filename},
		logger.Field{Key: "size", Value: len(data)})
}

// serveSegment serves TS segments
func (s *Server) serveSegment(w http.ResponseWriter, r *http.Request, streamKey, filename string) {
	cacheKey := fmt.Sprintf("%s/%s", streamKey, filename)

	// Check cache first
	if cached := s.getFromCache(cacheKey); cached != nil {
		s.logger.Debug("Serving segment from cache", logger.Field{Key: "cacheKey", Value: cacheKey})
		w.Header().Set("Content-Type", cached.MimeType)
		w.Header().Set("Cache-Control", s.config.SegmentCacheControl)
		w.WriteHeader(http.StatusOK)
		w.Write(cached.Data)
		return
	}

	// Build file path
	filePath := filepath.Join(s.transmuxer.config.OutputDir, streamKey, filename)

	// Read segment file
	data, err := os.ReadFile(filePath)
	if err != nil {
		s.logger.Error("Failed to read segment",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "path", Value: filePath})
		http.Error(w, "Segment not found", http.StatusNotFound)
		return
	}

	// Cache segment
	s.addToCache(cacheKey, data, "video/mp2t", 24*time.Hour)

	// Set headers
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", s.config.SegmentCacheControl)

	// Write response
	w.WriteHeader(http.StatusOK)
	w.Write(data)

	s.logger.Debug("Served segment",
		logger.Field{Key: "streamKey", Value: streamKey},
		logger.Field{Key: "filename", Value: filename},
		logger.Field{Key: "size", Value: len(data)})
}

// serveIndex serves the index page with available streams
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>HLS Server</title>
</head>
<body>
    <h1>HLS Server</h1>
    <p>Active streams:</p>
    <ul>`

	if s.transmuxer != nil {
		streams := s.transmuxer.GetActiveStreams()
		for _, streamKey := range streams {
			html += fmt.Sprintf(`<li><a href="/%s/master.m3u8">%s</a></li>`, streamKey, streamKey)
		}
	}

	html += `
    </ul>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// setCORSHeaders sets CORS headers
func (s *Server) setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	// Check if origin is allowed
	allowed := false
	for _, allowedOrigin := range s.config.AllowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			allowed = true
			break
		}
	}

	if allowed {
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range")
		w.Header().Set("Access-Control-Max-Age", "86400")
	}
}

// getFromCache retrieves a segment from cache
func (s *Server) getFromCache(key string) *CachedSegment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cached, ok := s.segmentCache[key]
	if !ok {
		return nil
	}

	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		return nil
	}

	return cached
}

// addToCache adds a segment to cache
func (s *Server) addToCache(key string, data []byte, mimeType string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.segmentCache[key] = &CachedSegment{
		Data:      data,
		CachedAt:  time.Now(),
		MimeType:  mimeType,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// cacheCleanupLoop periodically removes expired cache entries
func (s *Server) cacheCleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupCache()
		}

		// Check if server is still running
		s.mu.RLock()
		running := s.running
		s.mu.RUnlock()

		if !running {
			return
		}
	}
}

// cleanupCache removes expired entries from cache
func (s *Server) cleanupCache() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, cached := range s.segmentCache {
		if now.After(cached.ExpiresAt) {
			delete(s.segmentCache, key)
			removed++
		}
	}

	if removed > 0 {
		s.logger.Debug("Cleaned up cache", logger.Field{Key: "removed", Value: removed})
	}
}

// GetCacheSize returns the current cache size in bytes
func (s *Server) GetCacheSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	size := 0
	for _, cached := range s.segmentCache {
		size += len(cached.Data)
	}
	return size
}

// ClearCache clears all cached segments
func (s *Server) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.segmentCache = make(map[string]*CachedSegment)
	s.logger.Info("Cache cleared")
}
