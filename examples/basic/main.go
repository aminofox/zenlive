package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aminofox/zenlive"
	"github.com/aminofox/zenlive/pkg/config"
	"github.com/aminofox/zenlive/pkg/logger"
)

func main() {
	// Create SDK configuration
	cfg := config.DefaultConfig()

	// Customize configuration if needed
	cfg.Server.Port = 8080
	cfg.Logging.Level = "debug"

	// Create SDK instance
	sdk, err := zenlive.New(cfg)
	if err != nil {
		fmt.Printf("Failed to create SDK: %v\n", err)
		os.Exit(1)
	}

	log := sdk.Logger()
	log.Info("ZenLive SDK Example",
		logger.String("version", sdk.Version()),
	)

	// Start the SDK
	if err := sdk.Start(); err != nil {
		log.Error("Failed to start SDK", logger.Err(err))
		os.Exit(1)
	}

	log.Info("SDK started successfully",
		logger.Int("port", cfg.Server.Port),
	)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Press Ctrl+C to stop...")
	<-sigChan

	log.Info("Shutting down...")

	// Stop the SDK
	if err := sdk.Stop(); err != nil {
		log.Error("Failed to stop SDK", logger.Err(err))
		os.Exit(1)
	}

	log.Info("SDK stopped successfully")
}
