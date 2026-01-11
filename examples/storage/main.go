package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/storage"
	"github.com/aminofox/zenlive/pkg/storage/formats"
)

func main() {
	fmt.Println("ZenLive Storage & Recording Example")
	fmt.Println("====================================\n")

	// Initialize logger
	lgr := logger.NewDefaultLogger(logger.InfoLevel, "text")

	// Example 1: Recording with MP4 format
	fmt.Println("Example 1: MP4 Recording")
	fmt.Println("------------------------")
	runMP4Example(lgr)

	fmt.Println()

	// Example 2: Local storage usage
	fmt.Println("Example 2: Local Storage")
	fmt.Println("------------------------")
	runLocalStorageExample(lgr)

	fmt.Println()

	// Example 3: Metadata management
	fmt.Println("Example 3: Metadata Management")
	fmt.Println("------------------------------")
	runMetadataExample(lgr)

	fmt.Println("\nAll examples completed successfully!")
}

func runMP4Example(lgr logger.Logger) {
	ctx := context.Background()

	// Configure recording
	config := storage.DefaultRecordingConfig()
	config.StreamID = "example-stream-123"
	config.Format = storage.FormatMP4
	config.OutputPath = "./recordings"
	config.SegmentDuration = 30 * time.Second // Short duration for demo
	config.AutoUpload = false

	// Create MP4 recorder
	recorder, err := formats.NewMP4Recorder(config, lgr)
	if err != nil {
		log.Fatalf("Failed to create recorder: %v", err)
	}
	defer recorder.Close()

	// Start recording
	if err := recorder.Start(ctx); err != nil {
		log.Fatalf("Failed to start recording: %v", err)
	}

	fmt.Println("Recording started...")

	// Simulate recording for a few seconds
	time.Sleep(2 * time.Second)

	// Get recording info
	info := recorder.GetInfo()
	fmt.Printf("Recording ID: %s\n", info.ID)
	fmt.Printf("Stream ID: %s\n", info.StreamID)
	fmt.Printf("State: %s\n", info.State)
	fmt.Printf("Format: %s\n", info.Format)

	// Stop recording
	if err := recorder.Stop(ctx); err != nil {
		log.Fatalf("Failed to stop recording: %v", err)
	}

	// Get segments
	segments := recorder.GetSegments()
	fmt.Printf("Created %d segment(s)\n", len(segments))
	for _, seg := range segments {
		fmt.Printf("  - Segment %d: %s (%.2f MB)\n", seg.Index, seg.Path, float64(seg.Size)/(1024*1024))
	}
}

func runLocalStorageExample(lgr logger.Logger) {
	ctx := context.Background()

	// Configure local storage
	config := storage.DefaultStorageConfig()
	config.Type = storage.StorageTypeLocal
	config.BasePath = "./storage-demo"

	// Create local storage
	store, err := storage.NewLocalStorage(config, lgr)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	// Upload a test file
	testData := []byte("Hello, ZenLive Storage!")
	testKey := "test/demo.txt"

	fmt.Println("Uploading test file...")
	err = store.Upload(ctx, testKey, bytes.NewReader(testData), int64(len(testData)), "text/plain")
	if err != nil {
		log.Fatalf("Failed to upload: %v", err)
	}

	// Check if file exists
	exists, err := store.Exists(ctx, testKey)
	if err != nil {
		log.Fatalf("Failed to check existence: %v", err)
	}
	fmt.Printf("File exists: %v\n", exists)

	// Download file
	fmt.Println("Downloading file...")
	reader, err := store.Download(ctx, testKey)
	if err != nil {
		log.Fatalf("Failed to download: %v", err)
	}
	defer reader.Close()

	downloadedData, _ := io.ReadAll(reader)
	fmt.Printf("Downloaded content: %s\n", string(downloadedData))

	// List files
	objects, err := store.List(ctx, "test/", 10)
	if err != nil {
		log.Fatalf("Failed to list: %v", err)
	}
	fmt.Printf("Found %d object(s)\n", len(objects))
	for _, obj := range objects {
		fmt.Printf("  - %s (%.2f KB)\n", obj.Key, float64(obj.Size)/1024)
	}

	// Clean up
	fmt.Println("Cleaning up...")
	store.Delete(ctx, testKey)
}

func runMetadataExample(lgr logger.Logger) {
	ctx := context.Background()

	// Create in-memory metadata store
	store := storage.NewInMemoryMetadataStore(lgr)
	defer store.Close()

	// Create sample metadata
	metadata := &storage.RecordingMetadata{
		RecordingID:  "rec-001",
		StreamID:     "stream-123",
		UserID:       "user-456",
		Title:        "Sample Livestream Recording",
		Description:  "This is a demo recording",
		StartTime:    time.Now().Add(-1 * time.Hour),
		EndTime:      time.Now(),
		Duration:     1 * time.Hour,
		FileSize:     1024 * 1024 * 500, // 500 MB
		Format:       storage.FormatMP4,
		SegmentCount: 6,
		Tags:         []string{"demo", "example"},
	}

	// Save metadata
	fmt.Println("Saving metadata...")
	if err := store.Save(ctx, metadata); err != nil {
		log.Fatalf("Failed to save metadata: %v", err)
	}

	// Retrieve metadata
	retrieved, err := store.Get(ctx, "rec-001")
	if err != nil {
		log.Fatalf("Failed to get metadata: %v", err)
	}
	fmt.Printf("Retrieved: %s - %s\n", retrieved.RecordingID, retrieved.Title)
	fmt.Printf("Duration: %v\n", retrieved.Duration)
	fmt.Printf("File size: %.2f MB\n", float64(retrieved.FileSize)/(1024*1024))

	// Increment views
	store.IncrementViews(ctx, "rec-001")
	store.IncrementViews(ctx, "rec-001")

	// Query metadata
	query := storage.MetadataQuery{
		StreamID:  "stream-123",
		SortBy:    "start_time",
		SortOrder: "desc",
		Limit:     10,
	}

	results, err := store.Query(ctx, query)
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
	}
	fmt.Printf("\nQuery results: %d recording(s)\n", len(results))
	for _, rec := range results {
		fmt.Printf("  - %s: %s (Views: %d)\n", rec.RecordingID, rec.Title, rec.ViewCount)
	}
}
