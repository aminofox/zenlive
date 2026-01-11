// Package hls_test tests HLS implementation
package hls

import (
	"testing"
	"time"
)

// TestCreateSegment tests segment creation
func TestCreateSegment(t *testing.T) {
	videoData := []byte{0x00, 0x00, 0x01, 0x67} // Fake H.264 data
	audioData := []byte{0xFF, 0xF1}             // Fake AAC data

	segment, err := CreateSegment(0, 6.0, videoData, audioData)
	if err != nil {
		t.Fatalf("Failed to create segment: %v", err)
	}

	if segment.Index != 0 {
		t.Errorf("Expected index 0, got %d", segment.Index)
	}

	if segment.Duration != 6.0 {
		t.Errorf("Expected duration 6.0, got %.2f", segment.Duration)
	}

	if len(segment.Data) == 0 {
		t.Error("Segment data is empty")
	}

	if segment.Filename == "" {
		t.Error("Segment filename is empty")
	}
}

// TestMediaPlaylist tests media playlist creation and manipulation
func TestMediaPlaylist(t *testing.T) {
	playlist := NewMediaPlaylist(6, PlaylistTypeLive)

	if playlist.Version != 3 {
		t.Errorf("Expected version 3, got %d", playlist.Version)
	}

	// Add segments
	for i := 0; i < 5; i++ {
		segment := &Segment{
			Index:     uint64(i),
			Duration:  6.0,
			Filename:  "segment_" + string(rune(i)) + ".ts",
			CreatedAt: time.Now(),
		}
		playlist.AddSegment(segment)
	}

	if playlist.GetSegmentCount() != 5 {
		t.Errorf("Expected 5 segments, got %d", playlist.GetSegmentCount())
	}

	totalDuration := playlist.GetTotalDuration()
	if totalDuration != 30.0 {
		t.Errorf("Expected total duration 30.0, got %.2f", totalDuration)
	}

	// Test removing old segments
	playlist.RemoveOldSegments(3)
	if playlist.GetSegmentCount() != 3 {
		t.Errorf("Expected 3 segments after removal, got %d", playlist.GetSegmentCount())
	}

	if playlist.MediaSequence != 2 {
		t.Errorf("Expected media sequence 2, got %d", playlist.MediaSequence)
	}
}

// TestMasterPlaylist tests master playlist creation
func TestMasterPlaylist(t *testing.T) {
	master := NewMasterPlaylist()

	// Add variants
	variants := CreateDefaultVariants()
	for _, v := range variants {
		master.AddVariant(v)
	}

	if len(master.Variants) != 4 {
		t.Errorf("Expected 4 variants, got %d", len(master.Variants))
	}

	// Sort by bandwidth
	master.SortVariantsByBandwidth()

	// Check if sorted (descending)
	for i := 0; i < len(master.Variants)-1; i++ {
		if master.Variants[i].Bandwidth < master.Variants[i+1].Bandwidth {
			t.Error("Variants not sorted correctly")
		}
	}

	// Test rendering
	content := master.Render()
	if content == "" {
		t.Error("Master playlist render returned empty string")
	}

	if len(content) < 100 {
		t.Errorf("Master playlist content too short: %d bytes", len(content))
	}
}

// TestPlaylistRender tests playlist rendering
func TestPlaylistRender(t *testing.T) {
	playlist := NewMediaPlaylist(6, PlaylistTypeLive)

	// Add a few segments
	for i := 0; i < 3; i++ {
		segment := &Segment{
			Index:           uint64(i),
			Duration:        6.0,
			Filename:        "segment_" + string(rune('0'+i)) + ".ts",
			ProgramDateTime: time.Now(),
			CreatedAt:       time.Now(),
		}
		playlist.AddSegment(segment)
	}

	content := playlist.Render()

	// Check for required tags
	if len(content) == 0 {
		t.Error("Playlist render returned empty string")
	}

	// Should contain #EXTM3U
	if len(content) < 7 || content[:7] != "#EXTM3U" {
		t.Error("Playlist doesn't start with #EXTM3U")
	}
}

// TestABRManager tests ABR manager
func TestABRManager(t *testing.T) {
	manager := NewABRManager(6)

	// Add variants
	variants := CreateDefaultVariants()
	for _, v := range variants {
		manager.AddVariant(v)
	}

	if len(manager.GetVariants()) != 4 {
		t.Errorf("Expected 4 variants, got %d", len(manager.GetVariants()))
	}

	// Test variant selection by bandwidth
	variant, err := manager.SelectVariant(3000000) // 3 Mbps
	if err != nil {
		t.Fatalf("Failed to select variant: %v", err)
	}

	// Should select a variant that fits within 90% of 3 Mbps (2.7 Mbps)
	// Expected: 480p (1.4 Mbps) since 720p (2.8 Mbps) exceeds the threshold
	if variant.Bandwidth > 2700000 {
		t.Errorf("Selected variant bandwidth %d exceeds threshold 2700000", variant.Bandwidth)
	}

	// Test variant selection by resolution
	variant, err = manager.SelectVariantByResolution(1920, 1080)
	if err != nil {
		t.Fatalf("Failed to select variant by resolution: %v", err)
	}

	if variant.Name != "1080p" {
		t.Errorf("Expected 1080p variant, got %s", variant.Name)
	}

	// Test variant selection by name
	variant, err = manager.SelectVariantByName("480p")
	if err != nil {
		t.Fatalf("Failed to select variant by name: %v", err)
	}

	if variant.Name != "480p" {
		t.Errorf("Expected 480p variant, got %s", variant.Name)
	}
}

// TestBandwidthEstimator tests bandwidth estimation
func TestBandwidthEstimator(t *testing.T) {
	estimator := NewBandwidthEstimator(5)

	// Add measurements
	estimator.AddMeasurement(1000000, 1*time.Second) // 8 Mbps
	estimator.AddMeasurement(500000, 1*time.Second)  // 4 Mbps
	estimator.AddMeasurement(750000, 1*time.Second)  // 6 Mbps

	bandwidth := estimator.GetBandwidth()
	if bandwidth == 0 {
		t.Error("Bandwidth estimate is zero")
	}

	// Should be weighted average (more recent measurements have higher weight)
	if bandwidth < 4000000 || bandwidth > 8000000 {
		t.Errorf("Bandwidth estimate out of expected range: %d bps", bandwidth)
	}

	measurements := estimator.GetMeasurements()
	if len(measurements) != 3 {
		t.Errorf("Expected 3 measurements, got %d", len(measurements))
	}

	// Test reset
	estimator.Reset()
	if estimator.GetBandwidth() != 0 {
		t.Error("Bandwidth not reset to zero")
	}
}

// TestDVRWindow tests DVR window functionality
func TestDVRWindow(t *testing.T) {
	window := NewDVRWindow(30) // 30 second window

	// Add segments
	for i := 0; i < 10; i++ {
		segment := &Segment{
			Index:     uint64(i),
			Duration:  6.0,
			Filename:  "segment.ts",
			CreatedAt: time.Now().Add(-time.Duration(60-i*6) * time.Second),
		}
		window.AddSegment(segment)
	}

	// Older segments should be removed
	count := window.GetSegmentCount()
	if count > 6 {
		t.Errorf("Expected <= 6 segments in 30s window, got %d", count)
	}

	// Test segment retrieval by index
	latestIndex := window.GetEndSequence()
	segment, err := window.GetSegmentByIndex(latestIndex)
	if err != nil {
		t.Errorf("Failed to get segment by index: %v", err)
	}

	if segment == nil {
		t.Error("Segment is nil")
	}

	// Test time range
	segments, err := window.GetSegmentRange(0, 20*time.Second)
	if err != nil {
		t.Errorf("Failed to get segment range: %v", err)
	}

	if len(segments) == 0 {
		t.Error("No segments in range")
	}
}

// TestValidatePlaylist tests playlist validation
func TestValidatePlaylist(t *testing.T) {
	playlist := NewMediaPlaylist(6, PlaylistTypeLive)

	// Valid playlist
	for i := 0; i < 3; i++ {
		segment := &Segment{
			Index:     uint64(i),
			Duration:  5.5,
			Filename:  "segment.ts",
			CreatedAt: time.Now(),
		}
		playlist.AddSegment(segment)
	}

	if err := ValidatePlaylist(playlist); err != nil {
		t.Errorf("Valid playlist failed validation: %v", err)
	}

	// Invalid playlist (segment exceeds target + 1 should fail)
	badPlaylist := NewMediaPlaylist(6, PlaylistTypeLive)
	badSegment := &Segment{
		Index:     0,
		Duration:  8.0, // Exceeds target (6) + 1 (7)
		Filename:  "segment.ts",
		CreatedAt: time.Now(),
	}
	badPlaylist.AddSegment(badSegment)
	// Manually set target back to 6 to test validation
	badPlaylist.TargetDuration = 6

	if err := ValidatePlaylist(badPlaylist); err == nil {
		t.Error("Invalid playlist passed validation")
	}
}

// TestValidateMasterPlaylist tests master playlist validation
func TestValidateMasterPlaylist(t *testing.T) {
	master := NewMasterPlaylist()

	// Add valid variants
	variants := CreateDefaultVariants()
	for _, v := range variants {
		master.AddVariant(v)
	}

	if err := ValidateMasterPlaylist(master); err != nil {
		t.Errorf("Valid master playlist failed validation: %v", err)
	}

	// Empty master playlist should fail
	emptyMaster := NewMasterPlaylist()
	if err := ValidateMasterPlaylist(emptyMaster); err == nil {
		t.Error("Empty master playlist passed validation")
	}

	// Invalid variant (no bandwidth)
	badMaster := NewMasterPlaylist()
	badVariant := &Variant{
		Name:      "bad",
		Bandwidth: 0,
		URI:       "test.m3u8",
	}
	badMaster.AddVariant(badVariant)

	if err := ValidateMasterPlaylist(badMaster); err == nil {
		t.Error("Master playlist with invalid variant passed validation")
	}
}

// TestVariantSelector tests different variant selection strategies
func TestVariantSelector(t *testing.T) {
	variants := CreateDefaultVariants()

	// Test default selector
	defaultSelector := &DefaultVariantSelector{}
	variant, err := defaultSelector.SelectVariant(variants, 6000000) // 6 Mbps for 1080p
	if err != nil {
		t.Fatalf("Default selector failed: %v", err)
	}

	// Should select highest variant that fits within 90% (5.4 Mbps) = 1080p (5 Mbps)
	if variant.Bandwidth > 5400000 {
		t.Errorf("Default selector chose variant with too high bandwidth: %d", variant.Bandwidth)
	}

	// Test conservative selector
	conservativeSelector := &ConservativeVariantSelector{}
	variant, err = conservativeSelector.SelectVariant(variants, 8000000) // 8 Mbps
	if err != nil {
		t.Fatalf("Conservative selector failed: %v", err)
	}

	// Should use 75% of bandwidth (6 Mbps) = 1080p (5 Mbps) fits
	if variant.Bandwidth > 6000000 {
		t.Errorf("Conservative selector chose variant with too high bandwidth: %d", variant.Bandwidth)
	}

	// Test aggressive selector
	aggressiveSelector := &AggressiveVariantSelector{}
	variant, err = aggressiveSelector.SelectVariant(variants, 6000000)
	if err != nil {
		t.Fatalf("Aggressive selector failed: %v", err)
	}

	// Should use 95% of bandwidth (5.7 Mbps) = 1080p (5 Mbps) fits
	if variant.Bandwidth > 5700000 {
		t.Errorf("Aggressive selector chose variant with too high bandwidth: %d", variant.Bandwidth)
	}
}
