// Package main provides a WebRTC streaming example using ZenLive SDK.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/streaming/webrtc"
	pionwebrtc "github.com/pion/webrtc/v3"
)

func main() {
	fmt.Println("========================")
	fmt.Println("ZenLive WebRTC Streaming Example")
	fmt.Println("========================")

	// Create logger
	log := logger.NewDefaultLogger(logger.InfoLevel, "json")

	// Create SFU configuration
	sfuConfig := webrtc.DefaultSFUConfig()
	sfuConfig.MaxSubscribersPerStream = 100

	// Create SFU
	sfu := webrtc.NewSFU(sfuConfig, log)
	defer sfu.Close()

	// Create signaling server
	signalingConfig := webrtc.DefaultSignalingServerConfig()
	signalingConfig.ListenAddr = ":8081"
	signalingConfig.Path = "/ws"

	signalingServer := webrtc.NewSignalingServer(signalingConfig, log)

	// Set up signaling handlers
	setupSignalingHandlers(signalingServer, sfu, log)

	// Start signaling server in background
	go func() {
		fmt.Printf("Starting WebRTC signaling server on %s%s\n", signalingConfig.ListenAddr, signalingConfig.Path)
		if err := signalingServer.Start(); err != nil {
			log.Error("Failed to start signaling server",
				logger.Field{Key: "error", Value: err.Error()},
			)
		}
	}()

	// Wait a bit for server to start
	time.Sleep(500 * time.Millisecond)

	// Create example stream
	streamID := "test-stream"
	if err := sfu.CreateStream(streamID, "Test Stream"); err != nil {
		log.Error("Failed to create stream",
			logger.Field{Key: "error", Value: err.Error()},
		)
		os.Exit(1)
	}

	fmt.Printf("\nWebRTC streaming server is running!\n\n")
	fmt.Println("Usage:")
	fmt.Println("------")
	fmt.Println("1. Connect to WebSocket signaling server:")
	fmt.Printf("   ws://localhost%s%s\n\n", signalingConfig.ListenAddr, signalingConfig.Path)

	fmt.Println("2. Send offer as publisher:")
	fmt.Println("   {")
	fmt.Println("     \"type\": \"offer\",")
	fmt.Printf("     \"stream_id\": \"%s\",\n", streamID)
	fmt.Println("     \"peer_id\": \"publisher-1\",")
	fmt.Println("     \"sdp\": { ... }")
	fmt.Println("   }")

	fmt.Println("\n3. Subscribe to stream:")
	fmt.Println("   {")
	fmt.Println("     \"type\": \"subscribe\",")
	fmt.Printf("     \"stream_id\": \"%s\",\n", streamID)
	fmt.Println("     \"peer_id\": \"subscriber-1\"")
	fmt.Println("   }")

	fmt.Println("\nFeatures:")
	fmt.Println("- WebRTC publishing and subscribing")
	fmt.Println("- SFU architecture for efficient forwarding")
	fmt.Println("- Sub-second latency")
	fmt.Println("- Bandwidth estimation and adaptation")
	fmt.Println("- ICE/STUN/TURN support")

	fmt.Println("\nPress Ctrl+C to stop...")

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down WebRTC server...")

	// Graceful shutdown
	if err := signalingServer.Stop(); err != nil {
		log.Error("Error stopping signaling server",
			logger.Field{Key: "error", Value: err.Error()},
		)
	}

	fmt.Println("WebRTC server stopped")
}

// setupSignalingHandlers sets up WebRTC signaling message handlers
func setupSignalingHandlers(server *webrtc.SignalingServer, sfu *webrtc.SFU, log logger.Logger) {
	ctx := context.Background()

	// Handle offer from publisher
	server.OnOffer(func(peerID, streamID string, offer pionwebrtc.SessionDescription) error {
		log.Info("Received offer from publisher",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "stream_id", Value: streamID},
		)

		// Add publisher to stream
		publisher, err := sfu.AddPublisher(ctx, streamID, peerID)
		if err != nil {
			log.Error("Failed to add publisher",
				logger.Field{Key: "peer_id", Value: peerID},
				logger.Field{Key: "error", Value: err.Error()},
			)
			return err
		}

		// Handle offer and create answer
		answer, err := publisher.HandleOffer(offer)
		if err != nil {
			log.Error("Failed to handle offer",
				logger.Field{Key: "peer_id", Value: peerID},
				logger.Field{Key: "error", Value: err.Error()},
			)
			return err
		}

		// Send answer back to publisher
		msg := webrtc.SignalMessage{
			Type:     webrtc.SignalTypeAnswer,
			PeerID:   peerID,
			StreamID: streamID,
			SDP:      answer,
		}

		return server.SendMessage(peerID, msg)
	})

	// Handle subscribe request
	server.OnSubscribe(func(peerID, streamID string) error {
		log.Info("Received subscribe request",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "stream_id", Value: streamID},
		)

		// Add subscriber to stream
		subscriber, err := sfu.AddSubscriber(ctx, streamID, peerID)
		if err != nil {
			log.Error("Failed to add subscriber",
				logger.Field{Key: "peer_id", Value: peerID},
				logger.Field{Key: "error", Value: err.Error()},
			)
			return err
		}

		// Create offer for subscriber
		offer, err := subscriber.HandleOffer()
		if err != nil {
			log.Error("Failed to create offer",
				logger.Field{Key: "peer_id", Value: peerID},
				logger.Field{Key: "error", Value: err.Error()},
			)
			return err
		}

		// Send offer to subscriber
		msg := webrtc.SignalMessage{
			Type:     webrtc.SignalTypeOffer,
			PeerID:   peerID,
			StreamID: streamID,
			SDP:      offer,
		}

		return server.SendMessage(peerID, msg)
	})

	// Handle answer from subscriber
	server.OnAnswer(func(peerID, streamID string, answer pionwebrtc.SessionDescription) error {
		log.Info("Received answer from subscriber",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "stream_id", Value: streamID},
		)

		// Get stream
		stream, err := sfu.GetStream(streamID)
		if err != nil {
			return err
		}

		// Find subscriber
		stream.Subscribers[peerID].HandleAnswer(answer)

		return nil
	})

	// Handle ICE candidate
	server.OnCandidate(func(peerID string, candidate pionwebrtc.ICECandidateInit) error {
		log.Debug("Received ICE candidate",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "candidate", Value: candidate.Candidate},
		)

		// Handle ICE candidate (implementation depends on publisher/subscriber)
		// This is a simplified version
		return nil
	})

	// Handle unsubscribe
	server.OnUnsubscribe(func(peerID, streamID string) error {
		log.Info("Received unsubscribe request",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "stream_id", Value: streamID},
		)

		return sfu.RemoveSubscriber(streamID, peerID)
	})
}
