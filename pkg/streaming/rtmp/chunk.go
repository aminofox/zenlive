package rtmp

import (
	"bufio"
	"encoding/binary"
	"io"
)

// ChunkReader reads RTMP chunks from a connection
type ChunkReader struct {
	r         *bufio.Reader
	chunkSize uint32
	streams   map[uint32]*ChunkStream
}

// ChunkStream tracks state for a chunk stream
type ChunkStream struct {
	header       *ChunkHeader
	receivedSize uint32
	message      *Message
}

// NewChunkReader creates a new chunk reader
func NewChunkReader(r io.Reader) *ChunkReader {
	return &ChunkReader{
		r:         bufio.NewReader(r),
		chunkSize: DefaultChunkSize,
		streams:   make(map[uint32]*ChunkStream),
	}
}

// SetChunkSize sets the chunk size
func (cr *ChunkReader) SetChunkSize(size uint32) {
	cr.chunkSize = size
}

// ReadMessage reads a complete RTMP message
func (cr *ChunkReader) ReadMessage() (*Message, error) {
	for {
		// Read chunk basic header
		csID, format, err := cr.readBasicHeader()
		if err != nil {
			return nil, err
		}

		// Get or create chunk stream
		stream, exists := cr.streams[csID]
		if !exists {
			stream = &ChunkStream{
				header: &ChunkHeader{
					ChunkStreamID: csID,
				},
			}
			cr.streams[csID] = stream
		}

		// Read message header based on format
		if err := cr.readMessageHeader(stream, format); err != nil {
			return nil, err
		}

		// Read chunk data
		toRead := stream.header.MessageLength - stream.receivedSize
		if toRead > cr.chunkSize {
			toRead = cr.chunkSize
		}

		chunkData := make([]byte, toRead)
		if _, err := io.ReadFull(cr.r, chunkData); err != nil {
			return nil, err
		}

		// Append to message
		if stream.message == nil {
			stream.message = &Message{
				ChunkStreamID:   csID,
				Timestamp:       stream.header.Timestamp,
				MessageTypeID:   stream.header.MessageTypeID,
				MessageStreamID: stream.header.MessageStreamID,
				Payload:         make([]byte, 0, stream.header.MessageLength),
			}
		}
		stream.message.Payload = append(stream.message.Payload, chunkData...)
		stream.receivedSize += toRead

		// Check if message is complete
		if stream.receivedSize >= stream.header.MessageLength {
			msg := stream.message
			stream.message = nil
			stream.receivedSize = 0
			return msg, nil
		}
	}
}

func (cr *ChunkReader) readBasicHeader() (uint32, byte, error) {
	firstByte, err := cr.r.ReadByte()
	if err != nil {
		return 0, 0, err
	}

	format := (firstByte >> 6) & 0x03
	csID := uint32(firstByte & 0x3F)

	if csID == 0 {
		// 2-byte form
		secondByte, err := cr.r.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		csID = uint32(secondByte) + 64
	} else if csID == 1 {
		// 3-byte form
		buf := make([]byte, 2)
		if _, err := io.ReadFull(cr.r, buf); err != nil {
			return 0, 0, err
		}
		csID = uint32(buf[1])*256 + uint32(buf[0]) + 64
	}

	return csID, format, nil
}

func (cr *ChunkReader) readMessageHeader(stream *ChunkStream, format byte) error {
	header := stream.header

	switch format {
	case ChunkFormat0:
		// Type 0: 11 bytes
		buf := make([]byte, 11)
		if _, err := io.ReadFull(cr.r, buf); err != nil {
			return err
		}
		header.Timestamp = uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])
		header.MessageLength = uint32(buf[3])<<16 | uint32(buf[4])<<8 | uint32(buf[5])
		header.MessageTypeID = buf[6]
		header.MessageStreamID = binary.LittleEndian.Uint32(buf[7:11])

		if header.Timestamp == 0xFFFFFF {
			// Extended timestamp
			var extTimestamp uint32
			if err := binary.Read(cr.r, binary.BigEndian, &extTimestamp); err != nil {
				return err
			}
			header.Timestamp = extTimestamp
			header.ExtendedTimestamp = true
		}

	case ChunkFormat1:
		// Type 1: 7 bytes (no message stream ID)
		buf := make([]byte, 7)
		if _, err := io.ReadFull(cr.r, buf); err != nil {
			return err
		}
		timestampDelta := uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])
		header.Timestamp += timestampDelta
		header.MessageLength = uint32(buf[3])<<16 | uint32(buf[4])<<8 | uint32(buf[5])
		header.MessageTypeID = buf[6]

	case ChunkFormat2:
		// Type 2: 3 bytes (timestamp delta only)
		buf := make([]byte, 3)
		if _, err := io.ReadFull(cr.r, buf); err != nil {
			return err
		}
		timestampDelta := uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])
		header.Timestamp += timestampDelta

	case ChunkFormat3:
		// Type 3: no header, use previous values
		// Nothing to read
	}

	return nil
}

// ChunkWriter writes RTMP chunks to a connection
type ChunkWriter struct {
	w         io.Writer
	chunkSize uint32
}

// NewChunkWriter creates a new chunk writer
func NewChunkWriter(w io.Writer) *ChunkWriter {
	return &ChunkWriter{
		w:         w,
		chunkSize: DefaultChunkSize,
	}
}

// SetChunkSize sets the chunk size
func (cw *ChunkWriter) SetChunkSize(size uint32) {
	cw.chunkSize = size
}

// WriteMessage writes a complete RTMP message as chunks
func (cw *ChunkWriter) WriteMessage(msg *Message) error {
	payloadLen := uint32(len(msg.Payload))
	offset := uint32(0)
	isFirst := true

	for offset < payloadLen {
		// Write basic header
		if err := cw.writeBasicHeader(msg.ChunkStreamID, isFirst); err != nil {
			return err
		}

		// Write message header (only on first chunk)
		if isFirst {
			if err := cw.writeMessageHeader(msg); err != nil {
				return err
			}
			isFirst = false
		}

		// Write chunk data
		toWrite := payloadLen - offset
		if toWrite > cw.chunkSize {
			toWrite = cw.chunkSize
		}

		if _, err := cw.w.Write(msg.Payload[offset : offset+toWrite]); err != nil {
			return err
		}

		offset += toWrite
	}

	return nil
}

func (cw *ChunkWriter) writeBasicHeader(csID uint32, isFirst bool) error {
	var format byte
	if isFirst {
		format = ChunkFormat0
	} else {
		format = ChunkFormat3
	}

	if csID < 64 {
		// 1-byte form
		return binary.Write(cw.w, binary.BigEndian, byte((format<<6)|byte(csID)))
	} else if csID < 320 {
		// 2-byte form
		if err := binary.Write(cw.w, binary.BigEndian, byte(format<<6)); err != nil {
			return err
		}
		return binary.Write(cw.w, binary.BigEndian, byte(csID-64))
	} else {
		// 3-byte form
		if err := binary.Write(cw.w, binary.BigEndian, byte((format<<6)|1)); err != nil {
			return err
		}
		csID -= 64
		return binary.Write(cw.w, binary.BigEndian, uint16(csID))
	}
}

func (cw *ChunkWriter) writeMessageHeader(msg *Message) error {
	// Type 0 header (11 bytes)
	buf := make([]byte, 11)

	// Timestamp (3 bytes)
	timestamp := msg.Timestamp
	if timestamp >= 0xFFFFFF {
		timestamp = 0xFFFFFF
	}
	buf[0] = byte(timestamp >> 16)
	buf[1] = byte(timestamp >> 8)
	buf[2] = byte(timestamp)

	// Message length (3 bytes)
	msgLen := uint32(len(msg.Payload))
	buf[3] = byte(msgLen >> 16)
	buf[4] = byte(msgLen >> 8)
	buf[5] = byte(msgLen)

	// Message type ID (1 byte)
	buf[6] = msg.MessageTypeID

	// Message stream ID (4 bytes, little endian)
	binary.LittleEndian.PutUint32(buf[7:11], msg.MessageStreamID)

	if _, err := cw.w.Write(buf); err != nil {
		return err
	}

	// Extended timestamp if needed
	if msg.Timestamp >= 0xFFFFFF {
		if err := binary.Write(cw.w, binary.BigEndian, msg.Timestamp); err != nil {
			return err
		}
	}

	return nil
}

// WriteControlMessage writes a protocol control message
func (cw *ChunkWriter) WriteControlMessage(messageType uint8, payload []byte) error {
	msg := &Message{
		ChunkStreamID:   ChunkStreamIDProtocolControl,
		Timestamp:       0,
		MessageTypeID:   messageType,
		MessageStreamID: 0,
		Payload:         payload,
	}
	return cw.WriteMessage(msg)
}

// WriteSetChunkSize writes a set chunk size message
func (cw *ChunkWriter) WriteSetChunkSize(size uint32) error {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, size)
	return cw.WriteControlMessage(MessageTypeSetChunkSize, payload)
}

// WriteWindowAckSize writes a window acknowledgement size message
func (cw *ChunkWriter) WriteWindowAckSize(size uint32) error {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, size)
	return cw.WriteControlMessage(MessageTypeWindowAckSize, payload)
}

// WriteSetPeerBandwidth writes a set peer bandwidth message
func (cw *ChunkWriter) WriteSetPeerBandwidth(size uint32, limitType byte) error {
	payload := make([]byte, 5)
	binary.BigEndian.PutUint32(payload[0:4], size)
	payload[4] = limitType
	return cw.WriteControlMessage(MessageTypeSetPeerBandwidth, payload)
}
