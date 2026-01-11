package formats

import (
	"context"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/storage"
)

// MP4Recorder implements recording to MP4 format
type MP4Recorder struct {
	*storage.BaseRecorder
	logger logger.Logger
}

// NewMP4Recorder creates a new MP4 recorder
func NewMP4Recorder(config storage.RecordingConfig, log logger.Logger) (*MP4Recorder, error) {
	if config.Format != storage.FormatMP4 {
		return nil, storage.ErrInvalidFormat
	}

	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	base := storage.NewBaseRecorder(config, log)

	return &MP4Recorder{
		BaseRecorder: base,
		logger:       log,
	}, nil
}

// Start begins MP4 recording
func (r *MP4Recorder) Start(ctx context.Context) error {
	r.logger.Info("Starting MP4 recording",
		logger.Field{Key: "recording_id", Value: r.GetInfo().ID},
	)

	return r.BaseRecorder.Start(ctx)
}

// Stop ends MP4 recording
func (r *MP4Recorder) Stop(ctx context.Context) error {
	r.logger.Info("Stopping MP4 recording",
		logger.Field{Key: "recording_id", Value: r.GetInfo().ID},
	)

	return r.BaseRecorder.Stop(ctx)
}

// Close closes the MP4 recorder
func (r *MP4Recorder) Close() error {
	r.logger.Info("Closing MP4 recorder",
		logger.Field{Key: "recording_id", Value: r.GetInfo().ID},
	)

	return r.BaseRecorder.Close()
}
