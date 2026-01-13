package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminofox/zenlive/pkg/api"
	"github.com/aminofox/zenlive/pkg/auth"
	"github.com/aminofox/zenlive/pkg/config"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Parse flags
	configFile := flag.String("config", "config.yaml", "Path to config file")
	devMode := flag.Bool("dev", false, "Enable development mode")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ZenLive Server %s (commit: %s, built: %s)\n", version, commit, date)
		return
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override dev mode from flag
	if *devMode {
		cfg.Server.DevMode = true
	}

	// Initialize logger
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	if cfg.Server.DevMode {
		log = logger.NewDefaultLogger(logger.DebugLevel, "text")
		log.Info("Running in development mode")
	}

	ctx := context.Background()

	// Initialize API key manager (used by token builder)
	apiKeyStore := auth.NewMemoryAPIKeyStore()
	_ = auth.NewAPIKeyManager(apiKeyStore) // Will be used for API endpoints later

	// Generate default API key in dev mode
	if cfg.Server.DevMode && cfg.Auth.DefaultAPIKey != "" && cfg.Auth.DefaultSecretKey != "" {
		// Store the default dev API key
		expiresAt := time.Now().Add(365 * 24 * time.Hour) // 1 year
		defaultKey := &auth.APIKey{
			AccessKey: cfg.Auth.DefaultAPIKey,
			SecretKey: cfg.Auth.DefaultSecretKey,
			Name:      "Default Dev Key",
			ExpiresAt: &expiresAt,
			CreatedAt: time.Now(),
			IsActive:  true,
		}
		err := apiKeyStore.StoreAPIKey(ctx, defaultKey)
		if err != nil {
			log.Error("Failed to store default API key", logger.Err(err))
		} else {
			log.Info("Default dev API key configured")
		}
	}

	// Initialize JWT authenticator (with nil stores for simplicity)
	jwtAuth := auth.NewJWTAuthenticator(cfg.Auth.JWTSecret, nil, nil)

	// Initialize room manager
	roomMgr := room.NewRoomManager(log)
	log.Info("Room manager initialized")

	// Initialize API server
	apiCfg := &api.Config{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		JWTSecret:    cfg.Auth.JWTSecret,
		RateLimitRPM: 60,
		CORSOrigins:  []string{"*"},
		CORSMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		CORSHeaders:  []string{"Content-Type", "Authorization"},
	}
	apiServer := api.NewServer(roomMgr, jwtAuth, apiCfg, log)

	// Start API server in background
	go func() {
		log.Info("Starting API server", logger.String("addr", apiCfg.Addr))
		if err := apiServer.Start(); err != nil {
			log.Error("API server error", logger.Err(err))
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Info("ZenLive server started successfully")
	log.Info("Press Ctrl+C to shutdown")

	// Block until signal received
	<-sigChan
	log.Info("Shutdown signal received, starting graceful shutdown...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Cleanup (room manager doesn't have Close method yet)
	_ = shutdownCtx // Use the context if needed

	log.Info("ZenLive server stopped")
}
