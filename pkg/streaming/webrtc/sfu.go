// Package webrtc provides Selective Forwarding Unit (SFU) implementation for WebRTC.
package webrtc

import (
	"context"
	"fmt"
	"sync"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/pion/rtp"
)

// SFU implements a Selective Forwarding Unit for WebRTC streaming
type SFU struct {
	// config is the SFU configuration
	config SFUConfig

	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// streams stores active streams by stream ID
	streams map[string]*SFUStream

	// peerManager manages peer connections
	peerManager *PeerManager

	// trackManager manages media tracks
	trackManager *TrackManager

	// bwe for bandwidth estimation
	bwe *BandwidthEstimator

	// ctx for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// SFUStream represents a stream in the SFU
type SFUStream struct {
	// ID is the stream identifier
	ID string

	// Name is the stream name
	Name string

	// Publisher is the publisher for this stream
	Publisher *Publisher

	// Subscribers is the list of subscribers
	Subscribers map[string]*Subscriber

	// mu protects concurrent access
	mu sync.RWMutex

	// createdAt is the creation timestamp
	createdAt int64
}

// NewSFU creates a new SFU instance
func NewSFU(config SFUConfig, log logger.Logger) *SFU {
	ctx, cancel := context.WithCancel(context.Background())

	peerManager := NewPeerManager(config.WebRTCConfig, log)
	trackManager := NewTrackManager(log)

	return &SFU{
		config:       config,
		logger:       log,
		streams:      make(map[string]*SFUStream),
		peerManager:  peerManager,
		trackManager: trackManager,
		bwe:          NewBandwidthEstimator(DefaultBWEConfig(), log),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// CreateStream creates a new stream in the SFU
func (sfu *SFU) CreateStream(streamID, name string) error {
	sfu.mu.Lock()
	defer sfu.mu.Unlock()

	if _, exists := sfu.streams[streamID]; exists {
		return &WebRTCError{Code: "STREAM_EXISTS", Message: "stream already exists"}
	}

	stream := &SFUStream{
		ID:          streamID,
		Name:        name,
		Subscribers: make(map[string]*Subscriber),
	}

	sfu.streams[streamID] = stream

	sfu.logger.Info("Created stream",
		logger.Field{Key: "stream_id", Value: streamID},
		logger.Field{Key: "name", Value: name},
	)

	return nil
}

// DeleteStream deletes a stream from the SFU
func (sfu *SFU) DeleteStream(streamID string) error {
	sfu.mu.Lock()
	stream, exists := sfu.streams[streamID]
	if !exists {
		sfu.mu.Unlock()
		return ErrStreamNotFound
	}

	delete(sfu.streams, streamID)
	sfu.mu.Unlock()

	// Stop publisher
	if stream.Publisher != nil {
		stream.Publisher.Stop()
	}

	// Stop all subscribers
	stream.mu.Lock()
	for _, subscriber := range stream.Subscribers {
		subscriber.Stop()
	}
	stream.mu.Unlock()

	sfu.logger.Info("Deleted stream",
		logger.Field{Key: "stream_id", Value: streamID},
	)

	return nil
}

// AddPublisher adds a publisher to a stream
func (sfu *SFU) AddPublisher(ctx context.Context, streamID, publisherID string) (*Publisher, error) {
	sfu.mu.RLock()
	stream, exists := sfu.streams[streamID]
	sfu.mu.RUnlock()

	if !exists {
		return nil, ErrStreamNotFound
	}

	stream.mu.Lock()
	if stream.Publisher != nil {
		stream.mu.Unlock()
		return nil, ErrPublisherExists
	}
	stream.mu.Unlock()

	// Create publisher
	publisher := NewPublisher(publisherID, streamID, sfu.peerManager, sfu.trackManager, sfu.logger)

	// Set up packet forwarding
	publisher.OnVideoPacket(func(packet *rtp.Packet) {
		sfu.forwardVideoPacket(streamID, packet)
	})

	publisher.OnAudioPacket(func(packet *rtp.Packet) {
		sfu.forwardAudioPacket(streamID, packet)
	})

	// Start publisher
	if err := publisher.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start publisher: %w", err)
	}

	stream.mu.Lock()
	stream.Publisher = publisher
	stream.mu.Unlock()

	sfu.logger.Info("Added publisher",
		logger.Field{Key: "stream_id", Value: streamID},
		logger.Field{Key: "publisher_id", Value: publisherID},
	)

	return publisher, nil
}

// RemovePublisher removes a publisher from a stream
func (sfu *SFU) RemovePublisher(streamID string) error {
	sfu.mu.RLock()
	stream, exists := sfu.streams[streamID]
	sfu.mu.RUnlock()

	if !exists {
		return ErrStreamNotFound
	}

	stream.mu.Lock()
	publisher := stream.Publisher
	stream.Publisher = nil
	stream.mu.Unlock()

	if publisher != nil {
		publisher.Stop()
		sfu.logger.Info("Removed publisher",
			logger.Field{Key: "stream_id", Value: streamID},
			logger.Field{Key: "publisher_id", Value: publisher.GetID()},
		)
	}

	return nil
}

// AddSubscriber adds a subscriber to a stream
func (sfu *SFU) AddSubscriber(ctx context.Context, streamID, subscriberID string) (*Subscriber, error) {
	sfu.mu.RLock()
	stream, exists := sfu.streams[streamID]
	sfu.mu.RUnlock()

	if !exists {
		return nil, ErrStreamNotFound
	}

	// Check subscriber limit
	stream.mu.RLock()
	count := len(stream.Subscribers)
	stream.mu.RUnlock()

	if count >= sfu.config.MaxSubscribersPerStream {
		return nil, ErrMaxSubscribersReached
	}

	// Create subscriber
	subscriber := NewSubscriber(subscriberID, streamID, sfu.peerManager, sfu.trackManager, sfu.logger)

	// Start subscriber
	if err := subscriber.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start subscriber: %w", err)
	}

	stream.mu.Lock()
	stream.Subscribers[subscriberID] = subscriber
	stream.mu.Unlock()

	sfu.logger.Info("Added subscriber",
		logger.Field{Key: "stream_id", Value: streamID},
		logger.Field{Key: "subscriber_id", Value: subscriberID},
	)

	return subscriber, nil
}

// RemoveSubscriber removes a subscriber from a stream
func (sfu *SFU) RemoveSubscriber(streamID, subscriberID string) error {
	sfu.mu.RLock()
	stream, exists := sfu.streams[streamID]
	sfu.mu.RUnlock()

	if !exists {
		return ErrStreamNotFound
	}

	stream.mu.Lock()
	subscriber, exists := stream.Subscribers[subscriberID]
	delete(stream.Subscribers, subscriberID)
	stream.mu.Unlock()

	if exists {
		subscriber.Stop()
		sfu.logger.Info("Removed subscriber",
			logger.Field{Key: "stream_id", Value: streamID},
			logger.Field{Key: "subscriber_id", Value: subscriberID},
		)
	}

	return nil
}

// forwardVideoPacket forwards a video RTP packet to all subscribers
func (sfu *SFU) forwardVideoPacket(streamID string, packet *rtp.Packet) {
	sfu.mu.RLock()
	stream, exists := sfu.streams[streamID]
	sfu.mu.RUnlock()

	if !exists {
		return
	}

	stream.mu.RLock()
	defer stream.mu.RUnlock()

	for _, subscriber := range stream.Subscribers {
		// Forward packet to subscriber (ignore errors)
		if err := subscriber.WriteVideoPacket(packet); err != nil {
			sfu.logger.Debug("Failed to write video packet to subscriber",
				logger.Field{Key: "subscriber_id", Value: subscriber.GetID()},
				logger.Field{Key: "error", Value: err.Error()},
			)
		}
	}
}

// forwardAudioPacket forwards an audio RTP packet to all subscribers
func (sfu *SFU) forwardAudioPacket(streamID string, packet *rtp.Packet) {
	sfu.mu.RLock()
	stream, exists := sfu.streams[streamID]
	sfu.mu.RUnlock()

	if !exists {
		return
	}

	stream.mu.RLock()
	defer stream.mu.RUnlock()

	for _, subscriber := range stream.Subscribers {
		// Forward packet to subscriber (ignore errors)
		if err := subscriber.WriteAudioPacket(packet); err != nil {
			sfu.logger.Debug("Failed to write audio packet to subscriber",
				logger.Field{Key: "subscriber_id", Value: subscriber.GetID()},
				logger.Field{Key: "error", Value: err.Error()},
			)
		}
	}
}

// GetStream returns a stream by ID
func (sfu *SFU) GetStream(streamID string) (*SFUStream, error) {
	sfu.mu.RLock()
	defer sfu.mu.RUnlock()

	stream, exists := sfu.streams[streamID]
	if !exists {
		return nil, ErrStreamNotFound
	}

	return stream, nil
}

// GetStreams returns all active streams
func (sfu *SFU) GetStreams() []*SFUStream {
	sfu.mu.RLock()
	defer sfu.mu.RUnlock()

	streams := make([]*SFUStream, 0, len(sfu.streams))
	for _, stream := range sfu.streams {
		streams = append(streams, stream)
	}

	return streams
}

// GetStreamCount returns the number of active streams
func (sfu *SFU) GetStreamCount() int {
	sfu.mu.RLock()
	defer sfu.mu.RUnlock()

	return len(sfu.streams)
}

// GetSubscriberCount returns the number of subscribers for a stream
func (sfu *SFU) GetSubscriberCount(streamID string) int {
	sfu.mu.RLock()
	stream, exists := sfu.streams[streamID]
	sfu.mu.RUnlock()

	if !exists {
		return 0
	}

	stream.mu.RLock()
	defer stream.mu.RUnlock()

	return len(stream.Subscribers)
}

// Close closes the SFU and all streams
func (sfu *SFU) Close() error {
	sfu.cancel()

	sfu.mu.Lock()
	streamIDs := make([]string, 0, len(sfu.streams))
	for id := range sfu.streams {
		streamIDs = append(streamIDs, id)
	}
	sfu.mu.Unlock()

	// Delete all streams
	for _, streamID := range streamIDs {
		sfu.DeleteStream(streamID)
	}

	// Close peer manager
	sfu.peerManager.CloseAll()

	// Close track manager
	sfu.trackManager.CloseAll()

	sfu.logger.Info("SFU closed")

	return nil
}

// GetPublisher returns the publisher for a stream
func (s *SFUStream) GetPublisher() *Publisher {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Publisher
}

// GetSubscribers returns all subscribers for a stream
func (s *SFUStream) GetSubscribers() []*Subscriber {
	s.mu.RLock()
	defer s.mu.RUnlock()

	subscribers := make([]*Subscriber, 0, len(s.Subscribers))
	for _, sub := range s.Subscribers {
		subscribers = append(subscribers, sub)
	}

	return subscribers
}

// GetSubscriberCount returns the number of subscribers
func (s *SFUStream) GetSubscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.Subscribers)
}

// HasPublisher returns whether the stream has a publisher
func (s *SFUStream) HasPublisher() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Publisher != nil
}
