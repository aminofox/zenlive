package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/streaming/rtmp"
)

func main() {
	fmt.Println("=== ZenLive SDK - RTMP Server Example ===")
	fmt.Println()

	// Create logger
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")

	// Create RTMP server
	server := rtmp.NewServer(":1935", log)

	// Set callbacks
	server.SetOnPublish(func(streamKey string, metadata map[string]interface{}) error {
		fmt.Printf("✓ Stream publish started: %s\n", streamKey)
		if len(metadata) > 0 {
			fmt.Println("  Metadata:")
			for k, v := range metadata {
				fmt.Printf("    %s: %v\n", k, v)
			}
		}
		return nil
	})

	server.SetOnPlay(func(streamKey string) error {
		fmt.Printf("✓ Stream playback started: %s\n", streamKey)
		return nil
	})

	// Start server
	if err := server.Start(); err != nil {
		fmt.Printf("Failed to start RTMP server: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("RTMP server started on port 1935")
	fmt.Println()
	fmt.Println("To publish a stream using OBS Studio:")
	fmt.Println("  Server: rtmp://localhost:1935/live")
	fmt.Println("  Stream Key: test")
	fmt.Println()
	fmt.Println("To publish using FFmpeg:")
	fmt.Println("  ffmpeg -re -i input.mp4 -c copy -f flv rtmp://localhost:1935/live/test")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop...")
	fmt.Println()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nStopping RTMP server...")
	if err := server.Stop(); err != nil {
		fmt.Printf("Error stopping server: %v\n", err)
	}

	// Show final stats
	streams := server.GetStreams()
	if len(streams) > 0 {
		fmt.Println("\nActive streams:")
		for key, info := range streams {
			fmt.Printf("  %s: %v\n", key, info.IsPublishing)
		}
	}

	fmt.Println("Server stopped gracefully")
}
