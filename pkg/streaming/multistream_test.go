package streaming

import (
	"testing"
)

func TestMultiStreamManager_CreateSession(t *testing.T) {
	msm := NewMultiStreamManager()

	session, err := msm.CreateSession("stream1", "host1", 4)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session.StreamID != "stream1" {
		t.Errorf("Expected stream ID stream1, got %s", session.StreamID)
	}

	if session.HostUserID != "host1" {
		t.Errorf("Expected host user ID host1, got %s", session.HostUserID)
	}

	if session.MaxSources != 4 {
		t.Errorf("Expected max sources 4, got %d", session.MaxSources)
	}

	if session.Status != SessionStatusPending {
		t.Errorf("Expected status %s, got %s", SessionStatusPending, session.Status)
	}
}

func TestMultiStreamManager_StartSession(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)

	err := msm.StartSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	if session.Status != SessionStatusActive {
		t.Errorf("Expected status %s, got %s", SessionStatusActive, session.Status)
	}

	if session.StartedAt == nil {
		t.Error("StartedAt should be set")
	}
}

func TestMultiStreamManager_AddVideoSource(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)
	msm.StartSession(session.ID)

	resolution := Resolution{Width: 1920, Height: 1080}
	source, err := msm.AddVideoSource(session.ID, "user1", SourceTypeCamera, "rtmp://example.com/stream", resolution)
	if err != nil {
		t.Fatalf("Failed to add video source: %v", err)
	}

	if source.UserID != "user1" {
		t.Errorf("Expected user ID user1, got %s", source.UserID)
	}

	if source.Type != SourceTypeCamera {
		t.Errorf("Expected type %s, got %s", SourceTypeCamera, source.Type)
	}

	if !source.Active {
		t.Error("Source should be active")
	}

	if len(session.VideoSources) != 1 {
		t.Errorf("Expected 1 video source, got %d", len(session.VideoSources))
	}
}

func TestMultiStreamManager_MaxSourcesLimit(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 2) // Max 2 sources
	msm.StartSession(session.ID)

	resolution := Resolution{Width: 1920, Height: 1080}

	// Add 2 sources
	_, err1 := msm.AddVideoSource(session.ID, "user1", SourceTypeCamera, "rtmp://1", resolution)
	if err1 != nil {
		t.Fatalf("Failed to add first source: %v", err1)
	}

	_, err2 := msm.AddVideoSource(session.ID, "user2", SourceTypeCamera, "rtmp://2", resolution)
	if err2 != nil {
		t.Fatalf("Failed to add second source: %v", err2)
	}

	// Verify 2 sources added
	session, _ = msm.GetSession(session.ID)
	if len(session.VideoSources) != 2 {
		t.Fatalf("Expected 2 sources, got %d", len(session.VideoSources))
	}

	// Try to add 3rd source (should fail)
	_, err := msm.AddVideoSource(session.ID, "user3", SourceTypeCamera, "rtmp://3", resolution)
	if err == nil {
		t.Error("Expected error when exceeding max sources")
	}
}

func TestMultiStreamManager_RemoveVideoSource(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)
	msm.StartSession(session.ID)

	resolution := Resolution{Width: 1920, Height: 1080}
	source, _ := msm.AddVideoSource(session.ID, "user1", SourceTypeCamera, "rtmp://example.com/stream", resolution)

	err := msm.RemoveVideoSource(session.ID, source.ID)
	if err != nil {
		t.Fatalf("Failed to remove video source: %v", err)
	}

	if len(session.VideoSources) != 0 {
		t.Errorf("Expected 0 video sources, got %d", len(session.VideoSources))
	}
}

func TestMultiStreamManager_AddAudioSource(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)
	msm.StartSession(session.ID)

	source, err := msm.AddAudioSource(session.ID, "user1", "rtmp://example.com/audio", 48000, 2)
	if err != nil {
		t.Fatalf("Failed to add audio source: %v", err)
	}

	if source.SampleRate != 48000 {
		t.Errorf("Expected sample rate 48000, got %d", source.SampleRate)
	}

	if source.Channels != 2 {
		t.Errorf("Expected 2 channels, got %d", source.Channels)
	}

	if source.Volume != 1.0 {
		t.Errorf("Expected volume 1.0, got %f", source.Volume)
	}

	if source.Muted {
		t.Error("Source should not be muted by default")
	}
}

func TestMultiStreamManager_SetAudioVolume(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)
	msm.StartSession(session.ID)

	source, _ := msm.AddAudioSource(session.ID, "user1", "rtmp://example.com/audio", 48000, 2)

	err := msm.SetAudioVolume(session.ID, source.ID, 0.5)
	if err != nil {
		t.Fatalf("Failed to set audio volume: %v", err)
	}

	if source.Volume != 0.5 {
		t.Errorf("Expected volume 0.5, got %f", source.Volume)
	}

	// Test invalid volume
	err = msm.SetAudioVolume(session.ID, source.ID, 1.5)
	if err == nil {
		t.Error("Expected error for invalid volume")
	}
}

func TestMultiStreamManager_MuteAudio(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)
	msm.StartSession(session.ID)

	source, _ := msm.AddAudioSource(session.ID, "user1", "rtmp://example.com/audio", 48000, 2)

	err := msm.MuteAudioSource(session.ID, source.ID, true)
	if err != nil {
		t.Fatalf("Failed to mute audio: %v", err)
	}

	if !source.Muted {
		t.Error("Source should be muted")
	}

	// Unmute
	msm.MuteAudioSource(session.ID, source.ID, false)
	if source.Muted {
		t.Error("Source should be unmuted")
	}
}

func TestMultiStreamManager_SetLayout(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)
	msm.StartSession(session.ID)

	resolution := Resolution{Width: 1920, Height: 1080}
	source, _ := msm.AddVideoSource(session.ID, "user1", SourceTypeCamera, "rtmp://example.com/stream", resolution)

	// Set single layout
	layout := &Layout{
		Type:         LayoutTypeSingle,
		MainSourceID: source.ID,
	}

	err := msm.SetLayout(session.ID, layout)
	if err != nil {
		t.Fatalf("Failed to set layout: %v", err)
	}

	if session.Layout.Type != LayoutTypeSingle {
		t.Errorf("Expected layout type %s, got %s", LayoutTypeSingle, session.Layout.Type)
	}
}

func TestMultiStreamManager_AutoAdjustLayout(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)
	msm.StartSession(session.ID)

	resolution := Resolution{Width: 1920, Height: 1080}

	// Add 1 source - should be single layout
	source1, _ := msm.AddVideoSource(session.ID, "user1", SourceTypeCamera, "rtmp://1", resolution)

	// Retrieve session to check layout
	session, _ = msm.GetSession(session.ID)
	session.mu.RLock()
	layoutType1 := session.Layout.Type
	mainSource1 := session.Layout.MainSourceID
	sourceCount1 := len(session.VideoSources)
	session.mu.RUnlock()

	if layoutType1 != LayoutTypeSingle {
		t.Errorf("Expected single layout for 1 source, got %s", layoutType1)
	}
	if mainSource1 != source1.ID {
		t.Error("Main source should be set")
	}
	if sourceCount1 != 1 {
		t.Errorf("Expected 1 source, got %d", sourceCount1)
	}

	// Add 2nd source - should be side by side
	_, err2 := msm.AddVideoSource(session.ID, "user2", SourceTypeCamera, "rtmp://2", resolution)
	if err2 != nil {
		t.Fatalf("Failed to add 2nd source: %v", err2)
	}
	session, _ = msm.GetSession(session.ID)
	session.mu.RLock()
	layoutType2 := session.Layout.Type
	sourceCount2 := len(session.VideoSources)
	session.mu.RUnlock()

	if sourceCount2 != 2 {
		t.Errorf("Expected 2 sources, got %d", sourceCount2)
	}
	if layoutType2 != LayoutTypeSideBySide {
		t.Errorf("Expected side-by-side layout for 2 sources, got %s", layoutType2)
	}

	// Add 3rd source - should be grid
	_, err3 := msm.AddVideoSource(session.ID, "user3", SourceTypeCamera, "rtmp://3", resolution)
	if err3 != nil {
		t.Fatalf("Failed to add 3rd source: %v", err3)
	}
	session, _ = msm.GetSession(session.ID)
	session.mu.RLock()
	layoutType3 := session.Layout.Type
	gridRows := session.Layout.GridRows
	gridCols := session.Layout.GridCols
	sourceCount3 := len(session.VideoSources)
	session.mu.RUnlock()

	if sourceCount3 != 3 {
		t.Errorf("Expected 3 sources, got %d", sourceCount3)
	}
	if layoutType3 != LayoutTypeGrid {
		t.Errorf("Expected grid layout for 3 sources, got %s", layoutType3)
	}
	if gridRows != 2 || gridCols != 2 {
		t.Errorf("Expected 2x2 grid, got %dx%d", gridRows, gridCols)
	}
}

func TestMultiStreamManager_EndSession(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)
	msm.StartSession(session.ID)

	resolution := Resolution{Width: 1920, Height: 1080}
	source, _ := msm.AddVideoSource(session.ID, "user1", SourceTypeCamera, "rtmp://example.com/stream", resolution)

	err := msm.EndSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to end session: %v", err)
	}

	if session.Status != SessionStatusEnded {
		t.Errorf("Expected status %s, got %s", SessionStatusEnded, session.Status)
	}

	if session.EndedAt == nil {
		t.Error("EndedAt should be set")
	}

	if source.Active {
		t.Error("Source should be inactive after session ends")
	}
}

func TestMultiStreamManager_GetSessionByStream(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)

	retrieved, err := msm.GetSessionByStream("stream1")
	if err != nil {
		t.Fatalf("Failed to get session by stream: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Error("Retrieved wrong session")
	}

	// Test non-existent stream
	_, err = msm.GetSessionByStream("stream999")
	if err == nil {
		t.Error("Expected error for non-existent stream")
	}
}

func TestMultiStreamManager_GetActiveSessions(t *testing.T) {
	msm := NewMultiStreamManager()

	session1, _ := msm.CreateSession("stream1", "host1", 4)
	_, _ = msm.CreateSession("stream2", "host2", 4)

	msm.StartSession(session1.ID)
	// session2 stays pending

	active := msm.GetActiveSessions()

	if len(active) != 1 {
		t.Errorf("Expected 1 active session, got %d", len(active))
	}

	if active[0].ID != session1.ID {
		t.Error("Wrong session in active list")
	}
}

func TestMultiStreamManager_MixAudio(t *testing.T) {
	msm := NewMultiStreamManager()

	session, _ := msm.CreateSession("stream1", "host1", 4)
	msm.StartSession(session.ID)

	source1, _ := msm.AddAudioSource(session.ID, "user1", "rtmp://1", 48000, 2)
	source2, _ := msm.AddAudioSource(session.ID, "user2", "rtmp://2", 48000, 2)

	// Set volume
	msm.SetAudioVolume(session.ID, source1.ID, 0.8)
	msm.SetAudioVolume(session.ID, source2.ID, 0.5)

	// Mock audio buffers
	audioBuffers := map[string][]byte{
		source1.ID: {100, 100, 100},
		source2.ID: {50, 50, 50},
	}

	mixed, err := msm.MixAudio(session.ID, audioBuffers)
	if err != nil {
		t.Fatalf("Failed to mix audio: %v", err)
	}

	if len(mixed) == 0 {
		t.Error("Mixed audio should not be empty")
	}
}
