// Package hls implements M3U8 playlist generation for HLS
package hls

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// NewMediaPlaylist creates a new media playlist
func NewMediaPlaylist(targetDuration int, playlistType string) *MediaPlaylist {
	return &MediaPlaylist{
		Version:        3,
		TargetDuration: targetDuration,
		Segments:       make([]*Segment, 0),
		PlaylistType:   playlistType,
		EndList:        false,
		DVREnabled:     false,
		DVRWindowSize:  DefaultDVRWindowSize,
	}
}

// AddSegment adds a segment to the media playlist
func (p *MediaPlaylist) AddSegment(segment *Segment) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Segments = append(p.Segments, segment)

	// Update target duration if needed
	if int(segment.Duration+0.5) > p.TargetDuration {
		p.TargetDuration = int(segment.Duration + 0.5)
	}
}

// RemoveOldSegments removes segments beyond the playlist size
func (p *MediaPlaylist) RemoveOldSegments(maxSegments int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.Segments) > maxSegments {
		removed := len(p.Segments) - maxSegments
		p.Segments = p.Segments[removed:]
		p.MediaSequence += uint64(removed)
	}
}

// RemoveSegmentsBefore removes segments before the given time (for DVR window)
func (p *MediaPlaylist) RemoveSegmentsBefore(before time.Time) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	removeCount := 0
	for i, seg := range p.Segments {
		if seg.CreatedAt.Before(before) {
			removeCount = i + 1
		} else {
			break
		}
	}

	if removeCount > 0 {
		p.Segments = p.Segments[removeCount:]
		p.MediaSequence += uint64(removeCount)
	}

	return removeCount
}

// GetSegmentCount returns the number of segments in the playlist
func (p *MediaPlaylist) GetSegmentCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.Segments)
}

// GetTotalDuration returns the total duration of all segments in seconds
func (p *MediaPlaylist) GetTotalDuration() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := 0.0
	for _, seg := range p.Segments {
		total += seg.Duration
	}
	return total
}

// Render generates the M3U8 playlist content
func (p *MediaPlaylist) Render() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	buf := &bytes.Buffer{}

	// #EXTM3U header
	buf.WriteString("#EXTM3U\n")

	// #EXT-X-VERSION
	fmt.Fprintf(buf, "#EXT-X-VERSION:%d\n", p.Version)

	// #EXT-X-TARGETDURATION
	fmt.Fprintf(buf, "#EXT-X-TARGETDURATION:%d\n", p.TargetDuration)

	// #EXT-X-MEDIA-SEQUENCE
	fmt.Fprintf(buf, "#EXT-X-MEDIA-SEQUENCE:%d\n", p.MediaSequence)

	// #EXT-X-PLAYLIST-TYPE (optional, only for VOD/EVENT)
	if p.PlaylistType != PlaylistTypeLive {
		fmt.Fprintf(buf, "#EXT-X-PLAYLIST-TYPE:%s\n", p.PlaylistType)
	}

	// Segments
	for _, seg := range p.Segments {
		// #EXT-X-DISCONTINUITY (if needed)
		if seg.Discontinuity {
			buf.WriteString("#EXT-X-DISCONTINUITY\n")
		}

		// #EXT-X-PROGRAM-DATE-TIME (for synchronization)
		if !seg.ProgramDateTime.IsZero() {
			fmt.Fprintf(buf, "#EXT-X-PROGRAM-DATE-TIME:%s\n",
				seg.ProgramDateTime.Format("2006-01-02T15:04:05.000Z07:00"))
		}

		// #EXTINF - duration and title
		fmt.Fprintf(buf, "#EXTINF:%.3f,\n", seg.Duration)

		// Segment URI
		fmt.Fprintf(buf, "%s\n", seg.Filename)
	}

	// #EXT-X-ENDLIST (for VOD)
	if p.EndList {
		buf.WriteString("#EXT-X-ENDLIST\n")
	}

	return buf.String()
}

// SetEndList marks the playlist as ended (VOD)
func (p *MediaPlaylist) SetEndList() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.EndList = true
}

// NewMasterPlaylist creates a new master playlist
func NewMasterPlaylist() *MasterPlaylist {
	return &MasterPlaylist{
		Version:   3,
		Variants:  make([]*Variant, 0),
		CreatedAt: time.Now(),
	}
}

// AddVariant adds a quality variant to the master playlist
func (m *MasterPlaylist) AddVariant(variant *Variant) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Variants = append(m.Variants, variant)
}

// RemoveVariant removes a variant by name
func (m *MasterPlaylist) RemoveVariant(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, v := range m.Variants {
		if v.Name == name {
			m.Variants = append(m.Variants[:i], m.Variants[i+1:]...)
			return
		}
	}
}

// GetVariant retrieves a variant by name
func (m *MasterPlaylist) GetVariant(name string) *Variant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, v := range m.Variants {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// SortVariantsByBandwidth sorts variants by bandwidth (descending)
func (m *MasterPlaylist) SortVariantsByBandwidth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simple bubble sort (sufficient for small number of variants)
	n := len(m.Variants)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if m.Variants[j].Bandwidth < m.Variants[j+1].Bandwidth {
				m.Variants[j], m.Variants[j+1] = m.Variants[j+1], m.Variants[j]
			}
		}
	}
}

// Render generates the master M3U8 playlist content
func (m *MasterPlaylist) Render() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	buf := &bytes.Buffer{}

	// #EXTM3U header
	buf.WriteString("#EXTM3U\n")

	// #EXT-X-VERSION
	fmt.Fprintf(buf, "#EXT-X-VERSION:%d\n", m.Version)

	// Variants
	for _, variant := range m.Variants {
		// #EXT-X-STREAM-INF
		attrs := []string{
			fmt.Sprintf("BANDWIDTH=%d", variant.Bandwidth),
		}

		if variant.AverageBandwidth > 0 {
			attrs = append(attrs, fmt.Sprintf("AVERAGE-BANDWIDTH=%d", variant.AverageBandwidth))
		}

		if variant.Codecs != "" {
			attrs = append(attrs, fmt.Sprintf("CODECS=\"%s\"", variant.Codecs))
		}

		if variant.Resolution != "" {
			attrs = append(attrs, fmt.Sprintf("RESOLUTION=%s", variant.Resolution))
		}

		if variant.FrameRate > 0 {
			attrs = append(attrs, fmt.Sprintf("FRAME-RATE=%.3f", variant.FrameRate))
		}

		fmt.Fprintf(buf, "#EXT-X-STREAM-INF:%s\n", strings.Join(attrs, ","))

		// Variant URI
		fmt.Fprintf(buf, "%s\n", variant.URI)
	}

	return buf.String()
}

// CreateDefaultVariants creates a set of default quality variants
func CreateDefaultVariants() []*Variant {
	return []*Variant{
		{
			Name:             "1080p",
			Bandwidth:        5000000,
			AverageBandwidth: 4500000,
			Codecs:           "avc1.640028,mp4a.40.2",
			Resolution:       "1920x1080",
			FrameRate:        30.0,
			Width:            1920,
			Height:           1080,
			VideoBitrate:     4500000,
			AudioBitrate:     192000,
			URI:              "playlist_1080p.m3u8",
		},
		{
			Name:             "720p",
			Bandwidth:        2800000,
			AverageBandwidth: 2500000,
			Codecs:           "avc1.64001f,mp4a.40.2",
			Resolution:       "1280x720",
			FrameRate:        30.0,
			Width:            1280,
			Height:           720,
			VideoBitrate:     2400000,
			AudioBitrate:     128000,
			URI:              "playlist_720p.m3u8",
		},
		{
			Name:             "480p",
			Bandwidth:        1400000,
			AverageBandwidth: 1200000,
			Codecs:           "avc1.64001e,mp4a.40.2",
			Resolution:       "854x480",
			FrameRate:        30.0,
			Width:            854,
			Height:           480,
			VideoBitrate:     1100000,
			AudioBitrate:     128000,
			URI:              "playlist_480p.m3u8",
		},
		{
			Name:             "360p",
			Bandwidth:        800000,
			AverageBandwidth: 700000,
			Codecs:           "avc1.64001e,mp4a.40.2",
			Resolution:       "640x360",
			FrameRate:        30.0,
			Width:            640,
			Height:           360,
			VideoBitrate:     600000,
			AudioBitrate:     96000,
			URI:              "playlist_360p.m3u8",
		},
	}
}

// ValidatePlaylist validates a media playlist
func ValidatePlaylist(p *MediaPlaylist) error {
	if p.TargetDuration <= 0 {
		return fmt.Errorf("invalid target duration: %d", p.TargetDuration)
	}

	if p.Version < 1 || p.Version > 7 {
		return fmt.Errorf("invalid HLS version: %d", p.Version)
	}

	for i, seg := range p.Segments {
		if seg.Duration <= 0 {
			return fmt.Errorf("segment %d has invalid duration: %.3f", i, seg.Duration)
		}

		if seg.Duration > float64(p.TargetDuration)+1.0 {
			return fmt.Errorf("segment %d duration %.3f exceeds target duration %d",
				i, seg.Duration, p.TargetDuration)
		}

		if seg.Filename == "" {
			return fmt.Errorf("segment %d has empty filename", i)
		}
	}

	return nil
}

// ValidateMasterPlaylist validates a master playlist
func ValidateMasterPlaylist(m *MasterPlaylist) error {
	if len(m.Variants) == 0 {
		return fmt.Errorf("master playlist has no variants")
	}

	for i, variant := range m.Variants {
		if variant.Bandwidth <= 0 {
			return fmt.Errorf("variant %d has invalid bandwidth: %d", i, variant.Bandwidth)
		}

		if variant.URI == "" {
			return fmt.Errorf("variant %d has empty URI", i)
		}

		if variant.Resolution != "" {
			// Validate resolution format (WIDTHxHEIGHT)
			parts := strings.Split(variant.Resolution, "x")
			if len(parts) != 2 {
				return fmt.Errorf("variant %d has invalid resolution format: %s",
					i, variant.Resolution)
			}
		}
	}

	return nil
}
