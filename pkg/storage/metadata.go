package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// InMemoryMetadataStore implements an in-memory metadata store
type InMemoryMetadataStore struct {
	data   map[string]*RecordingMetadata
	mu     sync.RWMutex
	logger logger.Logger
}

// NewInMemoryMetadataStore creates a new in-memory metadata store
func NewInMemoryMetadataStore(log logger.Logger) *InMemoryMetadataStore {
	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	return &InMemoryMetadataStore{
		data:   make(map[string]*RecordingMetadata),
		logger: log,
	}
}

// Save saves recording metadata
func (s *InMemoryMetadataStore) Save(ctx context.Context, metadata *RecordingMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata.CreatedAt = time.Now()
	metadata.UpdatedAt = time.Now()

	s.data[metadata.RecordingID] = metadata

	s.logger.Info("Metadata saved",
		logger.Field{Key: "recording_id", Value: metadata.RecordingID},
	)

	return nil
}

// Get retrieves recording metadata by ID
func (s *InMemoryMetadataStore) Get(ctx context.Context, recordingID string) (*RecordingMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metadata, ok := s.data[recordingID]
	if !ok {
		return nil, ErrObjectNotFound
	}

	result := *metadata
	return &result, nil
}

// Update updates existing recording metadata
func (s *InMemoryMetadataStore) Update(ctx context.Context, metadata *RecordingMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[metadata.RecordingID]; !ok {
		return ErrObjectNotFound
	}

	metadata.UpdatedAt = time.Now()
	s.data[metadata.RecordingID] = metadata

	s.logger.Info("Metadata updated",
		logger.Field{Key: "recording_id", Value: metadata.RecordingID},
	)

	return nil
}

// Delete deletes recording metadata
func (s *InMemoryMetadataStore) Delete(ctx context.Context, recordingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[recordingID]; !ok {
		return ErrObjectNotFound
	}

	delete(s.data, recordingID)

	s.logger.Info("Metadata deleted",
		logger.Field{Key: "recording_id", Value: recordingID},
	)

	return nil
}

// Query queries recording metadata with filters
func (s *InMemoryMetadataStore) Query(ctx context.Context, query MetadataQuery) ([]*RecordingMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*RecordingMetadata, 0)

	for _, metadata := range s.data {
		if s.matchesQuery(metadata, query) {
			result := *metadata
			results = append(results, &result)
		}
	}

	s.sortResults(results, query.SortBy, query.SortOrder)

	start := query.Offset
	end := query.Offset + query.Limit

	if start >= len(results) {
		return []*RecordingMetadata{}, nil
	}

	if end > len(results) || query.Limit == 0 {
		end = len(results)
	}

	return results[start:end], nil
}

// IncrementViews increments the view count for a recording
func (s *InMemoryMetadataStore) IncrementViews(ctx context.Context, recordingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, ok := s.data[recordingID]
	if !ok {
		return ErrObjectNotFound
	}

	metadata.ViewCount++
	metadata.UpdatedAt = time.Now()

	return nil
}

// Close closes the metadata store
func (s *InMemoryMetadataStore) Close() error {
	s.logger.Info("In-memory metadata store closed")
	return nil
}

// matchesQuery checks if metadata matches the query filters
func (s *InMemoryMetadataStore) matchesQuery(metadata *RecordingMetadata, query MetadataQuery) bool {
	if query.StreamID != "" && metadata.StreamID != query.StreamID {
		return false
	}

	if query.UserID != "" && metadata.UserID != query.UserID {
		return false
	}

	if len(query.Tags) > 0 {
		hasTag := false
		for _, tag := range query.Tags {
			for _, metaTag := range metadata.Tags {
				if tag == metaTag {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	if !query.StartDate.IsZero() && metadata.StartTime.Before(query.StartDate) {
		return false
	}

	if !query.EndDate.IsZero() && metadata.EndTime.After(query.EndDate) {
		return false
	}

	if query.MinDuration > 0 && metadata.Duration < query.MinDuration {
		return false
	}

	if query.MaxDuration > 0 && metadata.Duration > query.MaxDuration {
		return false
	}

	return true
}

// sortResults sorts results based on sort field and order
func (s *InMemoryMetadataStore) sortResults(results []*RecordingMetadata, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "start_time"
	}

	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(results, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "start_time":
			less = results[i].StartTime.Before(results[j].StartTime)
		case "end_time":
			less = results[i].EndTime.Before(results[j].EndTime)
		case "duration":
			less = results[i].Duration < results[j].Duration
		case "views":
			less = results[i].ViewCount < results[j].ViewCount
		case "file_size":
			less = results[i].FileSize < results[j].FileSize
		default:
			less = results[i].StartTime.Before(results[j].StartTime)
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}

// FileMetadataStore implements a file-based metadata store
type FileMetadataStore struct {
	basePath string
	cache    map[string]*RecordingMetadata
	mu       sync.RWMutex
	logger   logger.Logger
}

// NewFileMetadataStore creates a new file-based metadata store
func NewFileMetadataStore(basePath string, log logger.Logger) (*FileMetadataStore, error) {
	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	store := &FileMetadataStore{
		basePath: basePath,
		cache:    make(map[string]*RecordingMetadata),
		logger:   log,
	}

	if err := store.loadCache(); err != nil {
		log.Warn("Failed to load metadata cache",
			logger.Field{Key: "error", Value: err},
		)
	}

	return store, nil
}

// Save saves recording metadata to file
func (s *FileMetadataStore) Save(ctx context.Context, metadata *RecordingMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata.CreatedAt = time.Now()
	metadata.UpdatedAt = time.Now()

	if err := s.saveToFile(metadata); err != nil {
		return err
	}

	s.cache[metadata.RecordingID] = metadata

	s.logger.Info("Metadata saved to file",
		logger.Field{Key: "recording_id", Value: metadata.RecordingID},
	)

	return nil
}

// Get retrieves recording metadata from file
func (s *FileMetadataStore) Get(ctx context.Context, recordingID string) (*RecordingMetadata, error) {
	s.mu.RLock()

	if metadata, ok := s.cache[recordingID]; ok {
		result := *metadata
		s.mu.RUnlock()
		return &result, nil
	}

	s.mu.RUnlock()

	metadata, err := s.loadFromFile(recordingID)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[recordingID] = metadata
	s.mu.Unlock()

	result := *metadata
	return &result, nil
}

// Update updates existing recording metadata in file
func (s *FileMetadataStore) Update(ctx context.Context, metadata *RecordingMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.loadFromFile(metadata.RecordingID); err != nil {
		return ErrObjectNotFound
	}

	metadata.UpdatedAt = time.Now()

	if err := s.saveToFile(metadata); err != nil {
		return err
	}

	s.cache[metadata.RecordingID] = metadata

	s.logger.Info("Metadata updated in file",
		logger.Field{Key: "recording_id", Value: metadata.RecordingID},
	)

	return nil
}

// Delete deletes recording metadata file
func (s *FileMetadataStore) Delete(ctx context.Context, recordingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.getMetadataFilePath(recordingID)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return ErrObjectNotFound
		}
		return err
	}

	delete(s.cache, recordingID)

	s.logger.Info("Metadata file deleted",
		logger.Field{Key: "recording_id", Value: recordingID},
	)

	return nil
}

// Query queries recording metadata from files
func (s *FileMetadataStore) Query(ctx context.Context, query MetadataQuery) ([]*RecordingMetadata, error) {
	inMemStore := &InMemoryMetadataStore{
		data:   s.cache,
		logger: s.logger,
	}

	return inMemStore.Query(ctx, query)
}

// IncrementViews increments the view count for a recording
func (s *FileMetadataStore) IncrementViews(ctx context.Context, recordingID string) error {
	metadata, err := s.Get(ctx, recordingID)
	if err != nil {
		return err
	}

	metadata.ViewCount++

	return s.Update(ctx, metadata)
}

// Close closes the metadata store
func (s *FileMetadataStore) Close() error {
	s.logger.Info("File metadata store closed")
	return nil
}

// getMetadataFilePath returns the file path for a metadata file
func (s *FileMetadataStore) getMetadataFilePath(recordingID string) string {
	shard := ""
	if len(recordingID) >= 2 {
		shard = recordingID[:2]
	}

	return filepath.Join(s.basePath, shard, recordingID+".json")
}

// saveToFile saves metadata to a JSON file
func (s *FileMetadataStore) saveToFile(metadata *RecordingMetadata) error {
	filePath := s.getMetadataFilePath(metadata.RecordingID)

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// loadFromFile loads metadata from a JSON file
func (s *FileMetadataStore) loadFromFile(recordingID string) (*RecordingMetadata, error) {
	filePath := s.getMetadataFilePath(recordingID)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata RecordingMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// loadCache loads all metadata files into cache
func (s *FileMetadataStore) loadCache() error {
	return filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			s.logger.Warn("Failed to read metadata file",
				logger.Field{Key: "path", Value: path},
				logger.Field{Key: "error", Value: err},
			)
			return nil
		}

		var metadata RecordingMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			s.logger.Warn("Failed to parse metadata file",
				logger.Field{Key: "path", Value: path},
				logger.Field{Key: "error", Value: err},
			)
			return nil
		}

		s.cache[metadata.RecordingID] = &metadata

		return nil
	})
}
