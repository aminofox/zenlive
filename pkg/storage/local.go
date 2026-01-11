package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// LocalStorage implements local filesystem storage
type LocalStorage struct {
	config StorageConfig
	logger logger.Logger
}

// NewLocalStorage creates a new local storage backend
func NewLocalStorage(config StorageConfig, log logger.Logger) (*LocalStorage, error) {
	if config.Type != StorageTypeLocal {
		return nil, fmt.Errorf("invalid storage type: %s", config.Type)
	}

	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	return &LocalStorage{
		config: config,
		logger: log,
	}, nil
}

// Upload uploads data to local filesystem
func (s *LocalStorage) Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	filePath := s.getFilePath(key)

	// Create directory if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Warn("Retrying upload",
				logger.Field{Key: "attempt", Value: attempt},
				logger.Field{Key: "key", Value: key},
			)
			time.Sleep(s.config.RetryDelay)
		}

		// Create file
		file, err := os.Create(filePath)
		if err != nil {
			lastErr = err
			continue
		}

		// Copy data
		written, err := io.Copy(file, data)
		file.Close()

		if err != nil {
			lastErr = err
			os.Remove(filePath)
			continue
		}

		if size > 0 && written != size {
			lastErr = fmt.Errorf("size mismatch: expected %d, wrote %d", size, written)
			os.Remove(filePath)
			continue
		}

		// Save metadata
		metadata := map[string]string{
			"content-type": contentType,
			"size":         fmt.Sprintf("%d", written),
			"uploaded-at":  time.Now().Format(time.RFC3339),
		}

		if err := s.saveMetadataFile(filePath, metadata); err != nil {
			s.logger.Warn("Failed to save metadata",
				logger.Field{Key: "error", Value: err},
			)
		}

		s.logger.Info("File uploaded",
			logger.Field{Key: "key", Value: key},
			logger.Field{Key: "size", Value: written},
		)

		return nil
	}

	return fmt.Errorf("upload failed after %d attempts: %w", s.config.MaxRetries+1, lastErr)
}

// Download downloads data from local filesystem
func (s *LocalStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	filePath := s.getFilePath(key)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete removes a file from local filesystem
func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	filePath := s.getFilePath(key)

	// Delete main file
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return ErrObjectNotFound
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Delete metadata file
	metaPath := filePath + ".meta"
	os.Remove(metaPath) // Ignore error

	s.logger.Info("File deleted",
		logger.Field{Key: "key", Value: key},
	)

	return nil
}

// Exists checks if a file exists
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	filePath := s.getFilePath(key)

	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// List lists files with the given prefix
func (s *LocalStorage) List(ctx context.Context, prefix string, maxKeys int) ([]StorageObject, error) {
	searchPath := s.getFilePath(prefix)
	baseDir := s.config.BasePath

	objects := make([]StorageObject, 0)
	count := 0

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and metadata files
		if info.IsDir() || strings.HasSuffix(path, ".meta") {
			return nil
		}

		// Check if path matches prefix
		if prefix != "" && !strings.HasPrefix(path, searchPath) {
			return nil
		}

		// Check max keys limit
		if maxKeys > 0 && count >= maxKeys {
			return filepath.SkipDir
		}

		// Get relative key
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		// Load metadata
		metadata, _ := s.loadMetadataFile(path)

		objects = append(objects, StorageObject{
			Key:          relPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			ContentType:  metadata["content-type"],
			Metadata:     metadata,
		})

		count++
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return objects, nil
}

// GetMetadata retrieves metadata for a file
func (s *LocalStorage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	filePath := s.getFilePath(key)

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}

	metadata, err := s.loadMetadataFile(filePath)
	if err != nil {
		return make(map[string]string), nil
	}

	return metadata, nil
}

// SetMetadata sets metadata for a file
func (s *LocalStorage) SetMetadata(ctx context.Context, key string, metadata map[string]string) error {
	filePath := s.getFilePath(key)

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return ErrObjectNotFound
		}
		return err
	}

	return s.saveMetadataFile(filePath, metadata)
}

// Copy copies a file to a new location
func (s *LocalStorage) Copy(ctx context.Context, srcKey, dstKey string) error {
	srcPath := s.getFilePath(srcKey)
	dstPath := s.getFilePath(dstKey)

	// Open source file
	src, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrObjectNotFound
		}
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer src.Close()

	// Create destination directory
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dst.Close()

	// Copy data
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Copy metadata
	if metadata, err := s.loadMetadataFile(srcPath); err == nil {
		s.saveMetadataFile(dstPath, metadata)
	}

	s.logger.Info("File copied",
		logger.Field{Key: "source", Value: srcKey},
		logger.Field{Key: "destination", Value: dstKey},
	)

	return nil
}

// GetURL returns a file:// URL for the file
func (s *LocalStorage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	filePath := s.getFilePath(key)

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return "", ErrObjectNotFound
		}
		return "", err
	}

	// Return file:// URL
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}

	return "file://" + absPath, nil
}

// Close closes the storage backend
func (s *LocalStorage) Close() error {
	s.logger.Info("Local storage closed")
	return nil
}

// getFilePath returns the full file path for a key
func (s *LocalStorage) getFilePath(key string) string {
	// Sanitize key to prevent directory traversal
	key = filepath.Clean(key)
	key = strings.TrimPrefix(key, "/")

	return filepath.Join(s.config.BasePath, key)
}

// saveMetadataFile saves metadata to a .meta file
func (s *LocalStorage) saveMetadataFile(filePath string, metadata map[string]string) error {
	metaPath := filePath + ".meta"

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, data, 0644)
}

// loadMetadataFile loads metadata from a .meta file
func (s *LocalStorage) loadMetadataFile(filePath string) (map[string]string, error) {
	metaPath := filePath + ".meta"

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var metadata map[string]string
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}
