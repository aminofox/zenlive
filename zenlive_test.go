package zenlive

import (
	"testing"

	"github.com/aminofox/zenlive/pkg/config"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name:    "with default config",
			cfg:     nil,
			wantErr: false,
		},
		{
			name:    "with custom config",
			cfg:     config.DefaultConfig(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdk, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && sdk == nil {
				t.Error("New() returned nil SDK")
			}
		})
	}
}

func TestSDK_StartStop(t *testing.T) {
	sdk, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create SDK: %v", err)
	}

	// Test initial state
	if sdk.IsRunning() {
		t.Error("SDK should not be running initially")
	}

	// Test start
	if err := sdk.Start(); err != nil {
		t.Errorf("Failed to start SDK: %v", err)
	}

	if !sdk.IsRunning() {
		t.Error("SDK should be running after Start()")
	}

	// Test double start
	if err := sdk.Start(); err == nil {
		t.Error("Starting SDK twice should return an error")
	}

	// Test stop
	if err := sdk.Stop(); err != nil {
		t.Errorf("Failed to stop SDK: %v", err)
	}

	if sdk.IsRunning() {
		t.Error("SDK should not be running after Stop()")
	}

	// Test double stop
	if err := sdk.Stop(); err == nil {
		t.Error("Stopping SDK twice should return an error")
	}
}

func TestSDK_Version(t *testing.T) {
	sdk, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create SDK: %v", err)
	}

	version := sdk.Version()
	if version == "" {
		t.Error("Version should not be empty")
	}
}

func TestSDK_Config(t *testing.T) {
	customConfig := config.DefaultConfig()
	customConfig.Server.Port = 9999

	sdk, err := New(customConfig)
	if err != nil {
		t.Fatalf("Failed to create SDK: %v", err)
	}

	cfg := sdk.Config()
	if cfg.Server.Port != 9999 {
		t.Errorf("Expected port 9999, got %d", cfg.Server.Port)
	}
}
