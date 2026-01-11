// Package hls implements DVR (Digital Video Recorder) functionality
package hls

import (
	"fmt"
	"sync"
	"time"
)

// DVRWindow manages a sliding window of segments for DVR functionality
type DVRWindow struct {
	// WindowSize is the DVR window size in seconds
	WindowSize int

	// Segments contains all segments in the window
	Segments []*Segment

	// StartSequence is the media sequence of the first segment
	StartSequence uint64

	// TotalDuration is the total duration of segments in window
	TotalDuration float64

	mu sync.RWMutex
}

// NewDVRWindow creates a new DVR window
func NewDVRWindow(windowSize int) *DVRWindow {
	return &DVRWindow{
		WindowSize:    windowSize,
		Segments:      make([]*Segment, 0),
		StartSequence: 0,
		TotalDuration: 0,
	}
}

// AddSegment adds a segment to the DVR window
func (d *DVRWindow) AddSegment(segment *Segment) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.Segments = append(d.Segments, segment)
	d.TotalDuration += segment.Duration

	// Remove old segments if window is exceeded
	d.trimWindow()
}

// trimWindow removes segments that fall outside the DVR window
func (d *DVRWindow) trimWindow() {
	if d.WindowSize <= 0 {
		return
	}

	// Calculate cutoff time
	cutoffTime := time.Now().Add(-time.Duration(d.WindowSize) * time.Second)

	removeCount := 0
	for _, seg := range d.Segments {
		if seg.CreatedAt.Before(cutoffTime) {
			removeCount++
			d.TotalDuration -= seg.Duration
		} else {
			break
		}
	}

	if removeCount > 0 {
		d.Segments = d.Segments[removeCount:]
		d.StartSequence += uint64(removeCount)
	}
}

// GetSegments returns all segments in the DVR window
func (d *DVRWindow) GetSegments() []*Segment {
	d.mu.RLock()
	defer d.mu.RUnlock()

	segments := make([]*Segment, len(d.Segments))
	copy(segments, d.Segments)
	return segments
}

// GetSegmentCount returns the number of segments in the window
func (d *DVRWindow) GetSegmentCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.Segments)
}

// GetDuration returns the total duration of the DVR window
func (d *DVRWindow) GetDuration() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.TotalDuration
}

// GetSegmentByIndex retrieves a segment by its index
func (d *DVRWindow) GetSegmentByIndex(index uint64) (*Segment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Calculate position in window
	if index < d.StartSequence {
		return nil, fmt.Errorf("segment %d is outside DVR window (start: %d)", index, d.StartSequence)
	}

	pos := int(index - d.StartSequence)
	if pos >= len(d.Segments) {
		return nil, fmt.Errorf("segment %d not found", index)
	}

	return d.Segments[pos], nil
}

// GetSegmentByTime retrieves the segment at a specific time offset
func (d *DVRWindow) GetSegmentByTime(offset time.Duration) (*Segment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.Segments) == 0 {
		return nil, fmt.Errorf("no segments in DVR window")
	}

	offsetSeconds := offset.Seconds()
	currentTime := 0.0

	for _, seg := range d.Segments {
		if currentTime+seg.Duration > offsetSeconds {
			return seg, nil
		}
		currentTime += seg.Duration
	}

	// Return last segment if offset is beyond window
	return d.Segments[len(d.Segments)-1], nil
}

// GetSegmentRange returns segments within a time range
func (d *DVRWindow) GetSegmentRange(start, end time.Duration) ([]*Segment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.Segments) == 0 {
		return nil, fmt.Errorf("no segments in DVR window")
	}

	startSeconds := start.Seconds()
	endSeconds := end.Seconds()
	currentTime := 0.0
	result := make([]*Segment, 0)

	for _, seg := range d.Segments {
		segEnd := currentTime + seg.Duration

		// Check if segment overlaps with requested range
		if segEnd > startSeconds && currentTime < endSeconds {
			result = append(result, seg)
		}

		currentTime = segEnd

		// Stop if we've passed the end time
		if currentTime > endSeconds {
			break
		}
	}

	return result, nil
}

// Clear removes all segments from the DVR window
func (d *DVRWindow) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.Segments = make([]*Segment, 0)
	d.TotalDuration = 0
}

// SetWindowSize updates the DVR window size
func (d *DVRWindow) SetWindowSize(size int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.WindowSize = size
	d.trimWindow()
}

// GetWindowSize returns the current window size
func (d *DVRWindow) GetWindowSize() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.WindowSize
}

// GetStartSequence returns the media sequence of the first segment
func (d *DVRWindow) GetStartSequence() uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.StartSequence
}

// GetEndSequence returns the media sequence of the last segment
func (d *DVRWindow) GetEndSequence() uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.Segments) == 0 {
		return d.StartSequence
	}

	return d.StartSequence + uint64(len(d.Segments)) - 1
}

// IsInWindow checks if a segment index is within the DVR window
func (d *DVRWindow) IsInWindow(index uint64) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.Segments) == 0 {
		return false
	}

	endSequence := d.StartSequence + uint64(len(d.Segments)) - 1
	return index >= d.StartSequence && index <= endSequence
}

// GetOldestSegmentTime returns the timestamp of the oldest segment
func (d *DVRWindow) GetOldestSegmentTime() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.Segments) == 0 {
		return time.Time{}
	}

	return d.Segments[0].CreatedAt
}

// GetNewestSegmentTime returns the timestamp of the newest segment
func (d *DVRWindow) GetNewestSegmentTime() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.Segments) == 0 {
		return time.Time{}
	}

	return d.Segments[len(d.Segments)-1].CreatedAt
}

// GetAvailableTimeRange returns the time range covered by the DVR window
func (d *DVRWindow) GetAvailableTimeRange() (start, end time.Time, duration time.Duration) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.Segments) == 0 {
		return time.Time{}, time.Time{}, 0
	}

	start = d.Segments[0].CreatedAt
	end = d.Segments[len(d.Segments)-1].CreatedAt
	duration = end.Sub(start)

	return start, end, duration
}

// CreatePlaylistFromWindow creates a media playlist from the DVR window
func (d *DVRWindow) CreatePlaylistFromWindow(targetDuration int) *MediaPlaylist {
	d.mu.RLock()
	defer d.mu.RUnlock()

	playlist := NewMediaPlaylist(targetDuration, PlaylistTypeEvent)
	playlist.MediaSequence = d.StartSequence
	playlist.DVREnabled = true
	playlist.DVRWindowSize = d.WindowSize

	for _, seg := range d.Segments {
		playlist.AddSegment(seg)
	}

	return playlist
}

// Validate checks if the DVR window is valid
func (d *DVRWindow) Validate() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.WindowSize < 0 {
		return fmt.Errorf("invalid window size: %d", d.WindowSize)
	}

	if d.TotalDuration < 0 {
		return fmt.Errorf("invalid total duration: %.3f", d.TotalDuration)
	}

	// Check segment indices
	for i, seg := range d.Segments {
		expectedIndex := d.StartSequence + uint64(i)
		if seg.Index != expectedIndex {
			return fmt.Errorf("segment %d has wrong index: expected %d, got %d",
				i, expectedIndex, seg.Index)
		}
	}

	return nil
}
