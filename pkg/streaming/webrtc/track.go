// Package webrtc provides track management and RTP processing for WebRTC streaming.
package webrtc

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

// TrackManager manages media tracks for WebRTC streaming
type TrackManager struct {
	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// localTracks stores local tracks by track ID
	localTracks map[string]*webrtc.TrackLocalStaticRTP

	// remoteTracks stores remote tracks by track ID
	remoteTracks map[string]*webrtc.TrackRemote

	// trackReaders stores active track readers
	trackReaders map[string]*TrackReader
}

// TrackReader reads RTP packets from a track
type TrackReader struct {
	// Track is the remote track
	Track *webrtc.TrackRemote

	// OnPacket is called for each received RTP packet
	OnPacket func(*rtp.Packet)

	// ctx for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// stats for track statistics
	stats *TrackStats

	// mu protects concurrent access
	mu sync.RWMutex
}

// TrackStats represents track statistics
type TrackStats struct {
	// PacketsReceived is the number of packets received
	PacketsReceived uint64

	// BytesReceived is the number of bytes received
	BytesReceived uint64

	// PacketsLost is the number of packets lost
	PacketsLost uint64

	// Jitter is the packet jitter in milliseconds
	Jitter float64

	// Bitrate is the current bitrate in bps
	Bitrate int

	// LastPacketTime is the timestamp of the last received packet
	LastPacketTime time.Time

	// LastSequence is the last received sequence number
	LastSequence uint16
}

// NewTrackManager creates a new track manager
func NewTrackManager(log logger.Logger) *TrackManager {
	return &TrackManager{
		logger:       log,
		localTracks:  make(map[string]*webrtc.TrackLocalStaticRTP),
		remoteTracks: make(map[string]*webrtc.TrackRemote),
		trackReaders: make(map[string]*TrackReader),
	}
}

// CreateLocalTrack creates a new local track
func (tm *TrackManager) CreateLocalTrack(codec webrtc.RTPCodecCapability, id, streamID string) (*webrtc.TrackLocalStaticRTP, error) {
	track, err := webrtc.NewTrackLocalStaticRTP(codec, id, streamID)
	if err != nil {
		tm.logger.Error("Failed to create local track",
			logger.Field{Key: "error", Value: err.Error()},
			logger.Field{Key: "codec", Value: codec.MimeType},
		)
		return nil, err
	}

	tm.mu.Lock()
	tm.localTracks[id] = track
	tm.mu.Unlock()

	tm.logger.Info("Created local track",
		logger.Field{Key: "track_id", Value: id},
		logger.Field{Key: "stream_id", Value: streamID},
		logger.Field{Key: "codec", Value: codec.MimeType},
	)

	return track, nil
}

// GetLocalTrack retrieves a local track by ID
func (tm *TrackManager) GetLocalTrack(trackID string) *webrtc.TrackLocalStaticRTP {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.localTracks[trackID]
}

// AddRemoteTrack adds a remote track to the manager
func (tm *TrackManager) AddRemoteTrack(track *webrtc.TrackRemote) {
	tm.mu.Lock()
	tm.remoteTracks[track.ID()] = track
	tm.mu.Unlock()

	tm.logger.Info("Added remote track",
		logger.Field{Key: "track_id", Value: track.ID()},
		logger.Field{Key: "kind", Value: track.Kind().String()},
		logger.Field{Key: "codec", Value: track.Codec().MimeType},
	)
}

// GetRemoteTrack retrieves a remote track by ID
func (tm *TrackManager) GetRemoteTrack(trackID string) *webrtc.TrackRemote {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.remoteTracks[trackID]
}

// StartTrackReader starts reading RTP packets from a remote track
func (tm *TrackManager) StartTrackReader(ctx context.Context, track *webrtc.TrackRemote, onPacket func(*rtp.Packet)) error {
	readerCtx, cancel := context.WithCancel(ctx)

	reader := &TrackReader{
		Track:    track,
		OnPacket: onPacket,
		ctx:      readerCtx,
		cancel:   cancel,
		stats: &TrackStats{
			LastPacketTime: time.Now(),
		},
	}

	tm.mu.Lock()
	tm.trackReaders[track.ID()] = reader
	tm.mu.Unlock()

	go reader.readLoop()

	tm.logger.Info("Started track reader",
		logger.Field{Key: "track_id", Value: track.ID()},
	)

	return nil
}

// StopTrackReader stops reading from a track
func (tm *TrackManager) StopTrackReader(trackID string) {
	tm.mu.Lock()
	reader, exists := tm.trackReaders[trackID]
	if exists {
		delete(tm.trackReaders, trackID)
	}
	tm.mu.Unlock()

	if exists {
		reader.cancel()
		tm.logger.Info("Stopped track reader",
			logger.Field{Key: "track_id", Value: trackID},
		)
	}
}

// GetTrackStats returns statistics for a track
func (tm *TrackManager) GetTrackStats(trackID string) *TrackStats {
	tm.mu.RLock()
	reader := tm.trackReaders[trackID]
	tm.mu.RUnlock()

	if reader == nil {
		return nil
	}

	reader.mu.RLock()
	defer reader.mu.RUnlock()

	// Make a copy of stats
	statsCopy := *reader.stats
	return &statsCopy
}

// RemoveLocalTrack removes a local track
func (tm *TrackManager) RemoveLocalTrack(trackID string) {
	tm.mu.Lock()
	delete(tm.localTracks, trackID)
	tm.mu.Unlock()

	tm.logger.Info("Removed local track",
		logger.Field{Key: "track_id", Value: trackID},
	)
}

// RemoveRemoteTrack removes a remote track
func (tm *TrackManager) RemoveRemoteTrack(trackID string) {
	tm.StopTrackReader(trackID)

	tm.mu.Lock()
	delete(tm.remoteTracks, trackID)
	tm.mu.Unlock()

	tm.logger.Info("Removed remote track",
		logger.Field{Key: "track_id", Value: trackID},
	)
}

// CloseAll closes all tracks and readers
func (tm *TrackManager) CloseAll() {
	tm.mu.Lock()
	readerIDs := make([]string, 0, len(tm.trackReaders))
	for id := range tm.trackReaders {
		readerIDs = append(readerIDs, id)
	}
	tm.mu.Unlock()

	for _, id := range readerIDs {
		tm.StopTrackReader(id)
	}

	tm.mu.Lock()
	tm.localTracks = make(map[string]*webrtc.TrackLocalStaticRTP)
	tm.remoteTracks = make(map[string]*webrtc.TrackRemote)
	tm.mu.Unlock()

	tm.logger.Info("Closed all tracks")
}

// readLoop reads RTP packets from the track
func (tr *TrackReader) readLoop() {
	buffer := make([]byte, 1500) // MTU size

	for {
		select {
		case <-tr.ctx.Done():
			return
		default:
		}

		n, _, err := tr.Track.Read(buffer)
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		// Parse RTP packet
		packet := &rtp.Packet{}
		if err := packet.Unmarshal(buffer[:n]); err != nil {
			continue
		}

		// Update statistics
		tr.updateStats(packet, n)

		// Call callback
		if tr.OnPacket != nil {
			tr.OnPacket(packet)
		}
	}
}

// updateStats updates track statistics
func (tr *TrackReader) updateStats(packet *rtp.Packet, size int) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.stats.PacketsReceived++
	tr.stats.BytesReceived += uint64(size)

	now := time.Now()

	// Calculate packet loss
	if tr.stats.LastSequence != 0 {
		expected := tr.stats.LastSequence + 1
		if packet.SequenceNumber != expected {
			diff := packet.SequenceNumber - expected
			tr.stats.PacketsLost += uint64(diff)
		}
	}

	// Calculate bitrate
	if !tr.stats.LastPacketTime.IsZero() {
		duration := now.Sub(tr.stats.LastPacketTime).Seconds()
		if duration > 0 {
			tr.stats.Bitrate = int(float64(size*8) / duration)
		}
	}

	tr.stats.LastSequence = packet.SequenceNumber
	tr.stats.LastPacketTime = now
}

// WriteRTPToTrack writes an RTP packet to a local track
func WriteRTPToTrack(track *webrtc.TrackLocalStaticRTP, packet *rtp.Packet) error {
	data, err := packet.Marshal()
	if err != nil {
		return err
	}

	if _, err := track.Write(data); err != nil {
		return err
	}

	return nil
}

// CreateVideoTrack creates a local video track with H.264 codec
func CreateVideoTrack(id, streamID string) (*webrtc.TrackLocalStaticRTP, error) {
	codec := webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeH264,
		ClockRate: 90000,
	}

	return webrtc.NewTrackLocalStaticRTP(codec, id, streamID)
}

// CreateAudioTrack creates a local audio track with Opus codec
func CreateAudioTrack(id, streamID string) (*webrtc.TrackLocalStaticRTP, error) {
	codec := webrtc.RTPCodecCapability{
		MimeType:    webrtc.MimeTypeOpus,
		ClockRate:   48000,
		Channels:    2,
		SDPFmtpLine: "minptime=10;useinbandfec=1",
	}

	return webrtc.NewTrackLocalStaticRTP(codec, id, streamID)
}

// CreateVP8Track creates a local video track with VP8 codec
func CreateVP8Track(id, streamID string) (*webrtc.TrackLocalStaticRTP, error) {
	codec := webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeVP8,
		ClockRate: 90000,
	}

	return webrtc.NewTrackLocalStaticRTP(codec, id, streamID)
}

// TrackKind represents the kind of media track
type TrackKind string

const (
	// TrackKindVideo represents a video track
	TrackKindVideo TrackKind = "video"

	// TrackKindAudio represents an audio track
	TrackKindAudio TrackKind = "audio"
)

// GetTrackKind returns the kind of a track
func GetTrackKind(track *webrtc.TrackRemote) TrackKind {
	if track.Kind() == webrtc.RTPCodecTypeVideo {
		return TrackKindVideo
	}
	return TrackKindAudio
}

// IsVideoTrack checks if a track is a video track
func IsVideoTrack(track *webrtc.TrackRemote) bool {
	return track.Kind() == webrtc.RTPCodecTypeVideo
}

// IsAudioTrack checks if a track is an audio track
func IsAudioTrack(track *webrtc.TrackRemote) bool {
	return track.Kind() == webrtc.RTPCodecTypeAudio
}
