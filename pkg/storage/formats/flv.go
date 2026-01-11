package formats

import (
	"context"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/storage"
)

// FLVRecorder implements recording to FLV format
type FLVRecorder struct {
	*storage.BaseRecorder
	logger logger.Logger
}

// NewFLVRecorder creates a new FLV recorder
func NewFLVRecorder(config storage.RecordingConfig, log logger.Logger) (*FLVRecorder, error) {
	if config.Format != storage.FormatFLV {
		return nil, storage.ErrInvalidFormat
	}

	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	base := storage.NewBaseRecorder(config, log)

	return &FLVRecorder{
		BaseRecorder: base,
		logger:       log,
	}, nil
}

// Start begins FLV recording
func (r *FLVRecorder) Start(ctx context.Context) error {
	r.logger.Info("Starting FLV recording",
		logger.Field{Key: "recording_id", Value: r.GetInfo().ID},
	)

	return r.BaseRecorder.Start(ctx)
}

// Stop ends FLV recording
func (r *FLVRecorder) Stop(ctx context.Context) error {
	r.logger.Info("Stopping FLV recording",
		logger.Field{Key: "recording_id", Value: r.GetInfo().ID},
	)

	return r.BaseRecorder.Stop(ctx)
}

// Close closes the FLV recorder
func (r *FLVRecorder) Close() error {
	r.logger.Info("Closing FLV recorder",
		logger.Field{Key: "recording_id", Value: r.GetInfo().ID},
	)

	return r.BaseRecorder.Close()
}
