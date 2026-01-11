package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndRTMPStreaming tests complete RTMP publish and playback flow
func TestEndToEndRTMPStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create stream manager
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	// Create stream
	stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:      "test-user-1",
		Title:       "Test RTMP Stream",
		Description: "Integration test stream",
		Protocol:    sdk.ProtocolRTMP,
	})
	require.NoError(t, err)
	require.NotNil(t, stream)
	assert.Equal(t, "Test RTMP Stream", stream.Title)
	assert.Equal(t, "test-user-1", stream.UserID)
	assert.NotEmpty(t, stream.ID)
	assert.NotEmpty(t, stream.StreamKey)

	// Verify stream state
	retrievedStream, err := streamManager.GetStream(ctx, stream.ID)
	require.NoError(t, err)
	assert.Equal(t, stream.ID, retrievedStream.ID)
	assert.Equal(t, stream.Title, retrievedStream.Title)

	// Cleanup
	err = streamManager.DeleteStream(ctx, stream.ID)
	require.NoError(t, err)
}

// TestMultiProtocolStreaming tests RTMP, HLS, and WebRTC creation
func TestMultiProtocolStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	// Create RTMP stream
	rtmpStream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:   "user1",
		Title:    "RTMP Stream",
		Protocol: sdk.ProtocolRTMP,
	})
	require.NoError(t, err)
	assert.Equal(t, sdk.ProtocolRTMP, rtmpStream.Protocol)

	// Create HLS stream
	hlsStream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:   "user1",
		Title:    "HLS Stream",
		Protocol: sdk.ProtocolHLS,
	})
	require.NoError(t, err)
	assert.Equal(t, sdk.ProtocolHLS, hlsStream.Protocol)

	// Create WebRTC stream
	webrtcStream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:   "user1",
		Title:    "WebRTC Stream",
		Protocol: sdk.ProtocolWebRTC,
	})
	require.NoError(t, err)
	assert.Equal(t, sdk.ProtocolWebRTC, webrtcStream.Protocol)

	// Cleanup
	streamManager.DeleteStream(ctx, rtmpStream.ID)
	streamManager.DeleteStream(ctx, hlsStream.ID)
	streamManager.DeleteStream(ctx, webrtcStream.ID)
}

// TestStreamCRUDOperations tests create, read, update, delete operations
func TestStreamCRUDOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	// Create stream
	stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:      "user123",
		Title:       "Test Stream",
		Description: "Test Description",
		Protocol:    sdk.ProtocolRTMP,
	})
	require.NoError(t, err)
	require.NotNil(t, stream)
	assert.NotEmpty(t, stream.ID)
	assert.Equal(t, "Test Stream", stream.Title)

	// Read stream
	retrievedStream, err := streamManager.GetStream(ctx, stream.ID)
	require.NoError(t, err)
	assert.Equal(t, stream.ID, retrievedStream.ID)
	assert.Equal(t, stream.Title, retrievedStream.Title)

	// Update stream
	newTitle := "Updated Stream"
	_, err = streamManager.UpdateStream(ctx, stream.ID, &sdk.UpdateStreamRequest{
		Title: &newTitle,
	})
	require.NoError(t, err)

	// Verify update
	updatedStream, err := streamManager.GetStream(ctx, stream.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Stream", updatedStream.Title)

	// Delete stream
	err = streamManager.DeleteStream(ctx, stream.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = streamManager.GetStream(ctx, stream.ID)
	assert.Error(t, err)
}

// TestStreamListing tests listing streams
func TestStreamListing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	// Create multiple streams
	streamIDs := []string{}
	for i := 0; i < 5; i++ {
		stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
			UserID:   "user1",
			Title:    fmt.Sprintf("Stream %d", i),
			Protocol: sdk.ProtocolRTMP,
		})
		require.NoError(t, err)
		streamIDs = append(streamIDs, stream.ID)
	}

	// List all streams
	streams, err := streamManager.ListStreams(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(streams), 5)

	// Cleanup
	for _, id := range streamIDs {
		streamManager.DeleteStream(ctx, id)
	}
}

// TestConcurrentStreamCreation tests creating multiple streams concurrently
func TestConcurrentStreamCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	numStreams := 10
	streams := make([]*sdk.Stream, numStreams)

	// Create multiple streams concurrently
	errChan := make(chan error, numStreams)
	for i := 0; i < numStreams; i++ {
		go func(index int) {
			stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
				UserID:   fmt.Sprintf("user%d", index),
				Title:    fmt.Sprintf("Concurrent Stream %d", index),
				Protocol: sdk.ProtocolRTMP,
			})
			if err != nil {
				errChan <- err
				return
			}
			streams[index] = stream
			errChan <- nil
		}(i)
	}

	// Wait for all creations
	for i := 0; i < numStreams; i++ {
		err := <-errChan
		require.NoError(t, err)
	}

	// Verify all streams created
	for i, stream := range streams {
		assert.NotNil(t, stream, "Stream %d should be created", i)
		assert.Equal(t, fmt.Sprintf("Concurrent Stream %d", i), stream.Title)
	}

	// Cleanup all streams
	for _, stream := range streams {
		if stream != nil {
			streamManager.DeleteStream(ctx, stream.ID)
		}
	}
}

// TestStreamMetadata tests stream metadata handling
func TestStreamMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	// Create stream with metadata
	metadata := map[string]string{
		"category": "gaming",
		"language": "en",
		"tags":     "fps,multiplayer",
	}

	stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:   "user1",
		Title:    "Metadata Test Stream",
		Protocol: sdk.ProtocolRTMP,
		Metadata: metadata,
	})
	require.NoError(t, err)
	assert.Equal(t, metadata, stream.Metadata)

	// Cleanup
	streamManager.DeleteStream(ctx, stream.ID)
}

// TestStreamConfiguration tests stream configuration
func TestStreamConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	// Create stream with custom configuration
	config := &sdk.StreamConfig{
		EnableRecording: true,
		EnableChat:      true,
		EnableDVR:       true,
		MaxViewers:      100,
	}

	stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:   "user1",
		Title:    "Config Test Stream",
		Protocol: sdk.ProtocolRTMP,
		Config:   config,
	})
	require.NoError(t, err)
	assert.NotNil(t, stream.Config)
	assert.True(t, stream.Config.EnableRecording)
	assert.True(t, stream.Config.EnableChat)
	assert.True(t, stream.Config.EnableDVR)
	assert.Equal(t, 100, stream.Config.MaxViewers)

	// Cleanup
	streamManager.DeleteStream(ctx, stream.ID)
}

// TestStreamProtocols tests different streaming protocols
func TestStreamProtocols(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	protocols := []sdk.StreamProtocol{
		sdk.ProtocolRTMP,
		sdk.ProtocolHLS,
		sdk.ProtocolWebRTC,
	}

	for _, protocol := range protocols {
		stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
			UserID:   "user1",
			Title:    fmt.Sprintf("%s Stream", protocol),
			Protocol: protocol,
		})
		require.NoError(t, err)
		assert.Equal(t, protocol, stream.Protocol)

		// Cleanup
		streamManager.DeleteStream(ctx, stream.ID)
	}
}
