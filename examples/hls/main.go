package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/streaming/hls"
)

func main() {
	// Create logger
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")

	// Create transmuxer config
	config := hls.DefaultTransmuxerConfig()
	config.OutputDir = "./hls_output"
	config.SegmentDuration = 6
	config.PlaylistSize = 5
	config.EnableDVR = true
	config.DVRWindowSize = 60
	config.EnableABR = true
	config.Variants = hls.CreateDefaultVariants()

	// Create transmuxer
	transmuxer, err := hls.NewTransmuxer(config, log)
	if err != nil {
		log.Error("Failed to create transmuxer", logger.Field{Key: "error", Value: err})
		os.Exit(1)
	}
	defer transmuxer.Close()

	// Set callbacks
	transmuxer.SetOnStreamStart(func(streamKey string) {
		log.Info("Stream started", logger.Field{Key: "streamKey", Value: streamKey})
	})

	transmuxer.SetOnSegmentComplete(func(streamKey string, segment *hls.Segment) {
		log.Info("Segment created",
			logger.Field{Key: "streamKey", Value: streamKey},
			logger.Field{Key: "index", Value: segment.Index},
			logger.Field{Key: "duration", Value: segment.Duration})
	})

	transmuxer.SetOnStreamEnd(func(streamKey string) {
		log.Info("Stream ended", logger.Field{Key: "streamKey", Value: streamKey})
	})

	// Create HLS HTTP server config
	serverConfig := hls.DefaultServerConfig()
	serverConfig.Address = ":8080"
	serverConfig.EnableCORS = true

	// Create HLS server
	server, err := hls.NewServer(serverConfig, transmuxer, log)
	if err != nil {
		log.Error("Failed to create HLS server", logger.Field{Key: "error", Value: err})
		os.Exit(1)
	}

	// Start a test stream
	streamKey := "test"
	if err := transmuxer.StartStream(streamKey); err != nil {
		log.Error("Failed to start stream", logger.Field{Key: "error", Value: err})
		os.Exit(1)
	}

	// Start HTTP server in background
	go func() {
		log.Info("Starting HLS HTTP server", logger.Field{Key: "address", Value: serverConfig.Address})
		if err := server.Start(); err != nil {
			log.Error("HLS server error", logger.Field{Key: "error", Value: err})
		}
	}()

	// Print usage info
	fmt.Println("\n=== HLS Server Started ===")
	fmt.Println("Stream Key:", streamKey)
	fmt.Println("Master Playlist: http://localhost:8080/" + streamKey + "/master.m3u8")
	fmt.Println("Media Playlist: http://localhost:8080/" + streamKey + "/playlist.m3u8")
	fmt.Println("")
	fmt.Println("Configuration:")
	fmt.Printf("  - Segment Duration: %d seconds\n", config.SegmentDuration)
	fmt.Printf("  - Playlist Size: %d segments\n", config.PlaylistSize)
	fmt.Printf("  - DVR Window: %d seconds\n", config.DVRWindowSize)
	fmt.Printf("  - ABR Enabled: %v\n", config.EnableABR)
	if config.EnableABR {
		fmt.Println("  - Available Variants:")
		for _, v := range config.Variants {
			fmt.Printf("    * %s (%dx%d, %d kbps)\n",
				v.Name, v.Width, v.Height, v.Bandwidth/1000)
		}
	}
	fmt.Println("")
	fmt.Println("Playback with FFmpeg:")
	fmt.Println("  ffmpeg -i http://localhost:8080/" + streamKey + "/playlist.m3u8 -c copy output.mp4")
	fmt.Println("")
	fmt.Println("Playback with VLC:")
	fmt.Println("  vlc http://localhost:8080/" + streamKey + "/playlist.m3u8")
	fmt.Println("")
	fmt.Println("Press Ctrl+C to stop...")
	fmt.Println("========================")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down...")

	// Stop stream
	if err := transmuxer.StopStream(streamKey); err != nil {
		log.Error("Failed to stop stream", logger.Field{Key: "error", Value: err})
	}

	// Stop server
	if err := server.Stop(); err != nil {
		log.Error("Failed to stop server", logger.Field{Key: "error", Value: err})
	}

	log.Info("Shutdown complete")
}
