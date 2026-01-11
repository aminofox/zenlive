// Package hls implements MPEG-TS segment generation for HLS
package hls

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
)

// MPEG-TS constants
const (
	// SyncByte is the MPEG-TS sync byte (0x47)
	SyncByte = 0x47

	// PID values for MPEG-TS
	PIDPAT   = 0x0000 // Program Association Table
	PIDPMT   = 0x1000 // Program Map Table
	PIDVideo = 0x0100 // Video PID
	PIDAudio = 0x0101 // Audio PID
	PIDPCR   = 0x1000 // PCR PID

	// Stream types for PMT
	StreamTypeH264 = 0x1B // H.264 video
	StreamTypeAAC  = 0x0F // AAC audio

	// Table IDs
	TableIDPAT = 0x00
	TableIDPMT = 0x02
)

// NewTSWriter creates a new MPEG-TS writer
func NewTSWriter() *TSWriter {
	return &TSWriter{
		continuityCounter: make(map[uint16]byte),
		pcrBase:           0,
		pts:               0,
		dts:               0,
	}
}

// WritePacket writes a single TS packet with the given payload
func (w *TSWriter) WritePacket(pid uint16, payload []byte, hasPCR, hasPayload, payloadStart bool) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	packet := make([]byte, TSPacketSize)
	pos := 0

	// Sync byte
	packet[pos] = SyncByte
	pos++

	// Transport Error Indicator (1 bit) = 0
	// Payload Unit Start Indicator (1 bit)
	// Transport Priority (1 bit) = 0
	// PID (13 bits)
	pidField := uint16(0)
	if payloadStart {
		pidField |= 0x4000 // Set PUSI bit
	}
	pidField |= (pid & 0x1FFF)
	binary.BigEndian.PutUint16(packet[pos:], pidField)
	pos += 2

	// Transport Scrambling Control (2 bits) = 0
	// Adaptation Field Control (2 bits)
	// Continuity Counter (4 bits)
	adaptationControl := byte(0)
	if hasPCR {
		adaptationControl = 0x30 // Adaptation field + payload
	} else if hasPayload {
		adaptationControl = 0x10 // Payload only
	} else {
		adaptationControl = 0x20 // Adaptation field only
	}

	cc := w.continuityCounter[pid]
	packet[pos] = (adaptationControl << 4) | (cc & 0x0F)
	pos++

	// Increment continuity counter
	w.continuityCounter[pid] = (cc + 1) & 0x0F

	// Adaptation field
	if hasPCR {
		adaptationLength := 7 // PCR is 6 bytes + flags byte
		packet[pos] = byte(adaptationLength)
		pos++

		// Flags: PCR present
		packet[pos] = 0x10
		pos++

		// PCR (Program Clock Reference) - 6 bytes
		// PCR = PCR_base * 300 + PCR_extension
		pcrBase := w.pcrBase
		pcrExt := uint16(0)

		// PCR base (33 bits) + reserved (6 bits) + PCR extension (9 bits)
		packet[pos] = byte(pcrBase >> 25)
		packet[pos+1] = byte(pcrBase >> 17)
		packet[pos+2] = byte(pcrBase >> 9)
		packet[pos+3] = byte(pcrBase >> 1)
		packet[pos+4] = byte(((pcrBase & 0x01) << 7) | 0x7E | uint64((pcrExt>>8)&0x01))
		packet[pos+5] = byte(pcrExt)
		pos += 6

		w.pcrBase += 90000 / 25 // Increment PCR for 25fps
	}

	// Payload
	if hasPayload && len(payload) > 0 {
		payloadSize := TSPacketSize - pos
		if len(payload) < payloadSize {
			copy(packet[pos:], payload)
			// Fill rest with 0xFF
			for i := pos + len(payload); i < TSPacketSize; i++ {
				packet[i] = 0xFF
			}
		} else {
			copy(packet[pos:], payload[:payloadSize])
		}
	} else {
		// Fill with stuffing bytes
		for i := pos; i < TSPacketSize; i++ {
			packet[i] = 0xFF
		}
	}

	w.packetCount++
	return packet, nil
}

// WritePAT writes a Program Association Table
func (w *TSWriter) WritePAT() ([]byte, error) {
	// PAT payload
	payload := &bytes.Buffer{}

	// Pointer field (for sections starting in this packet)
	payload.WriteByte(0x00)

	// Table ID
	payload.WriteByte(TableIDPAT)

	// Section syntax indicator (1), reserved (1), reserved (2), section length (12)
	sectionLength := uint16(13) // Rest of section after this field
	binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0xB000|sectionLength)
	payload.Write(make([]byte, 2))

	// Transport stream ID
	binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0x0001)
	payload.Write(make([]byte, 2))

	// Reserved (2), version (5), current/next indicator (1)
	payload.WriteByte(0xC1)

	// Section number
	payload.WriteByte(0x00)

	// Last section number
	payload.WriteByte(0x00)

	// Program number (1)
	binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0x0001)
	payload.Write(make([]byte, 2))

	// Reserved (3), Program map PID (13)
	binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0xE000|PIDPMT)
	payload.Write(make([]byte, 2))

	// CRC32 (calculate later)
	crc := calculateCRC32(payload.Bytes()[1:])
	binary.BigEndian.PutUint32(payload.Bytes()[payload.Len():payload.Len()+4], crc)
	payload.Write(make([]byte, 4))

	return w.WritePacket(PIDPAT, payload.Bytes(), false, true, true)
}

// WritePMT writes a Program Map Table
func (w *TSWriter) WritePMT(hasVideo, hasAudio bool) ([]byte, error) {
	payload := &bytes.Buffer{}

	// Pointer field
	payload.WriteByte(0x00)

	// Table ID
	payload.WriteByte(TableIDPMT)

	// Calculate section length
	sectionLength := 13 // Base length
	if hasVideo {
		sectionLength += 5 // Video elementary stream info
	}
	if hasAudio {
		sectionLength += 5 // Audio elementary stream info
	}

	// Section syntax indicator (1), reserved (1), reserved (2), section length (12)
	binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0xB000|uint16(sectionLength))
	payload.Write(make([]byte, 2))

	// Program number
	binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0x0001)
	payload.Write(make([]byte, 2))

	// Reserved (2), version (5), current/next indicator (1)
	payload.WriteByte(0xC1)

	// Section number
	payload.WriteByte(0x00)

	// Last section number
	payload.WriteByte(0x00)

	// Reserved (3), PCR PID (13)
	binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0xE000|PIDPCR)
	payload.Write(make([]byte, 2))

	// Reserved (4), program info length (12)
	binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0xF000)
	payload.Write(make([]byte, 2))

	// Elementary stream info
	if hasVideo {
		// Stream type (H.264)
		payload.WriteByte(StreamTypeH264)

		// Reserved (3), elementary PID (13)
		binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0xE000|PIDVideo)
		payload.Write(make([]byte, 2))

		// Reserved (4), ES info length (12)
		binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0xF000)
		payload.Write(make([]byte, 2))
	}

	if hasAudio {
		// Stream type (AAC)
		payload.WriteByte(StreamTypeAAC)

		// Reserved (3), elementary PID (13)
		binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0xE000|PIDAudio)
		payload.Write(make([]byte, 2))

		// Reserved (4), ES info length (12)
		binary.BigEndian.PutUint16(payload.Bytes()[payload.Len():payload.Len()+2], 0xF000)
		payload.Write(make([]byte, 2))
	}

	// CRC32
	crc := calculateCRC32(payload.Bytes()[1:])
	binary.BigEndian.PutUint32(payload.Bytes()[payload.Len():payload.Len()+4], crc)
	payload.Write(make([]byte, 4))

	return w.WritePacket(PIDPMT, payload.Bytes(), false, true, true)
}

// WritePES writes a Packetized Elementary Stream packet
func (w *TSWriter) WritePES(pid uint16, data []byte, pts, dts uint64, isVideo bool) ([][]byte, error) {
	// Build PES header
	header := &bytes.Buffer{}

	// Packet start code prefix (0x000001)
	header.WriteByte(0x00)
	header.WriteByte(0x00)
	header.WriteByte(0x01)

	// Stream ID
	if isVideo {
		header.WriteByte(0xE0) // Video stream 0
	} else {
		header.WriteByte(0xC0) // Audio stream 0
	}

	// PES packet length (0 = unbounded for video)
	if isVideo {
		binary.BigEndian.PutUint16(header.Bytes()[header.Len():header.Len()+2], 0)
		header.Write(make([]byte, 2))
	} else {
		length := uint16(len(data) + 8) // Data + PES header extension
		binary.BigEndian.PutUint16(header.Bytes()[header.Len():header.Len()+2], length)
		header.Write(make([]byte, 2))
	}

	// Marker bits (10), scrambling (2), priority (1), alignment (1), copyright (1), original (1)
	header.WriteByte(0x80)

	// PTS/DTS flags (2), ESCR (1), ES rate (1), DSM trick mode (1), additional copy info (1), CRC (1), extension (1)
	ptsFlags := byte(0x80) // PTS only
	if isVideo && dts != pts {
		ptsFlags = 0xC0 // Both PTS and DTS
	}
	header.WriteByte(ptsFlags)

	// PES header data length
	headerDataLength := byte(5) // PTS is 5 bytes
	if ptsFlags == 0xC0 {
		headerDataLength = 10 // PTS + DTS
	}
	header.WriteByte(headerDataLength)

	// Write PTS (5 bytes)
	w.writePTS(header, pts, ptsFlags>>6)

	// Write DTS if needed (5 bytes)
	if ptsFlags == 0xC0 {
		w.writePTS(header, dts, 0x01)
	}

	// Combine header and data
	pesPacket := append(header.Bytes(), data...)

	// Fragment into TS packets
	packets := [][]byte{}
	offset := 0
	first := true

	for offset < len(pesPacket) {
		var packet []byte
		var err error

		if first {
			// First packet may have PCR
			payloadStart := offset
			packet, err = w.WritePacket(pid, pesPacket[payloadStart:], first && isVideo, true, first)
			if err != nil {
				return nil, err
			}
			packets = append(packets, packet)

			// Calculate how much data was actually written
			headerSize := 4 // Basic TS header
			if first && isVideo {
				headerSize += 8 // Adaptation field with PCR
			}
			payloadSize := TSPacketSize - headerSize
			if len(pesPacket)-offset < payloadSize {
				offset = len(pesPacket)
			} else {
				offset += payloadSize
			}
			first = false
		} else {
			// Subsequent packets
			payloadStart := offset
			packet, err = w.WritePacket(pid, pesPacket[payloadStart:], false, true, false)
			if err != nil {
				return nil, err
			}
			packets = append(packets, packet)

			// Calculate how much data was written
			payloadSize := TSPacketSize - 4 // Basic TS header only
			if len(pesPacket)-offset < payloadSize {
				offset = len(pesPacket)
			} else {
				offset += payloadSize
			}
		}
	}

	return packets, nil
}

// writePTS writes a PTS or DTS timestamp (5 bytes)
func (w *TSWriter) writePTS(buf *bytes.Buffer, timestamp uint64, marker byte) {
	// Format: marker (4 bits) | timestamp[32..30] (3 bits) | marker bit (1)
	buf.WriteByte((marker << 4) | byte((timestamp>>29)&0x0E) | 0x01)

	// timestamp[29..15] (15 bits) | marker bit (1)
	binary.BigEndian.PutUint16(buf.Bytes()[buf.Len():buf.Len()+2], uint16((timestamp>>14)&0xFFFE)|0x01)
	buf.Write(make([]byte, 2))

	// timestamp[14..0] (15 bits) | marker bit (1)
	binary.BigEndian.PutUint16(buf.Bytes()[buf.Len():buf.Len()+2], uint16((timestamp<<1)&0xFFFE)|0x01)
	buf.Write(make([]byte, 2))
}

// calculateCRC32 calculates CRC32 for MPEG-TS tables
func calculateCRC32(data []byte) uint32 {
	crc := uint32(0xFFFFFFFF)

	for _, b := range data {
		for i := 0; i < 8; i++ {
			if ((crc >> 31) ^ uint32((b>>uint(7-i))&0x01)) != 0 {
				crc = (crc << 1) ^ 0x04C11DB7
			} else {
				crc = crc << 1
			}
		}
	}

	return crc
}

// CreateSegment creates a complete HLS TS segment
func CreateSegment(index uint64, duration float64, videoData, audioData []byte) (*Segment, error) {
	writer := NewTSWriter()
	buffer := &bytes.Buffer{}

	// Write PAT
	pat, err := writer.WritePAT()
	if err != nil {
		return nil, fmt.Errorf("failed to write PAT: %w", err)
	}
	buffer.Write(pat)

	// Write PMT
	pmt, err := writer.WritePMT(len(videoData) > 0, len(audioData) > 0)
	if err != nil {
		return nil, fmt.Errorf("failed to write PMT: %w", err)
	}
	buffer.Write(pmt)

	// Calculate PTS/DTS (90kHz clock)
	basePTS := uint64(index) * uint64(duration*90000)
	baseDTS := basePTS

	// Write video PES
	if len(videoData) > 0 {
		videoPackets, err := writer.WritePES(PIDVideo, videoData, basePTS, baseDTS, true)
		if err != nil {
			return nil, fmt.Errorf("failed to write video PES: %w", err)
		}
		for _, packet := range videoPackets {
			buffer.Write(packet)
		}
	}

	// Write audio PES
	if len(audioData) > 0 {
		audioPackets, err := writer.WritePES(PIDAudio, audioData, basePTS, basePTS, false)
		if err != nil {
			return nil, fmt.Errorf("failed to write audio PES: %w", err)
		}
		for _, packet := range audioPackets {
			buffer.Write(packet)
		}
	}

	segment := &Segment{
		Index:           index,
		Duration:        duration,
		Filename:        fmt.Sprintf("segment_%d.ts", index),
		Data:            buffer.Bytes(),
		ProgramDateTime: time.Now(),
		Type:            SegmentTypeMuxed,
		KeyFrame:        true,
		CreatedAt:       time.Now(),
	}

	return segment, nil
}
