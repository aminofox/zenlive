package rtmp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/errors"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/types"
)

// Server represents an RTMP server
type Server struct {
	addr      string
	listener  net.Listener
	logger    logger.Logger
	mu        sync.RWMutex
	streams   map[string]*StreamInfo
	conns     map[net.Conn]*Connection
	onPublish func(streamKey string, metadata map[string]interface{}) error
	onPlay    func(streamKey string) error
	running   bool
}

// Connection represents an RTMP client connection
type Connection struct {
	conn        net.Conn
	reader      *ChunkReader
	writer      *ChunkWriter
	state       ConnectionState
	streamKey   string
	streamID    uint32
	publishMode PublishMode
	metadata    map[string]interface{}
}

// NewServer creates a new RTMP server
func NewServer(addr string, log logger.Logger) *Server {
	return &Server{
		addr:    addr,
		logger:  log,
		streams: make(map[string]*StreamInfo),
		conns:   make(map[net.Conn]*Connection),
	}
}

// SetOnPublish sets the callback for when a stream starts publishing
func (s *Server) SetOnPublish(fn func(streamKey string, metadata map[string]interface{}) error) {
	s.onPublish = fn
}

// SetOnPlay sets the callback for when a client starts playing
func (s *Server) SetOnPlay(fn func(streamKey string) error) {
	s.onPlay = fn
}

// Start starts the RTMP server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return errors.Wrap(errors.ErrCodeNetworkError, "failed to start RTMP server", err)
	}

	s.listener = listener
	s.running = true
	s.logger.Info("RTMP server started", logger.Field{Key: "addr", Value: s.addr})

	go s.acceptLoop()
	return nil
}

// Stop stops the RTMP server
func (s *Server) Stop() error {
	s.running = false

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return err
		}
	}

	// Close all connections
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.conns {
		conn.Close()
	}

	s.logger.Info("RTMP server stopped")
	return nil
}

func (s *Server) acceptLoop() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.running {
				return
			}
			s.logger.Error("Failed to accept connection", logger.Field{Key: "error", Value: err})
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(netConn net.Conn) {
	defer netConn.Close()

	s.logger.Info("New RTMP connection", logger.Field{Key: "remote", Value: netConn.RemoteAddr()})

	conn := &Connection{
		conn:     netConn,
		reader:   NewChunkReader(netConn),
		writer:   NewChunkWriter(netConn),
		state:    StateInit,
		metadata: make(map[string]interface{}),
	}

	s.mu.Lock()
	s.conns[netConn] = conn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.conns, netConn)
		s.mu.Unlock()
	}()

	// Perform handshake
	if err := s.performHandshake(conn); err != nil {
		s.logger.Error("Handshake failed", logger.Field{Key: "error", Value: err})
		return
	}

	conn.state = StateConnected

	// Send initial control messages
	if err := s.sendInitialMessages(conn); err != nil {
		s.logger.Error("Failed to send initial messages", logger.Field{Key: "error", Value: err})
		return
	}

	// Handle messages
	if err := s.handleMessages(conn); err != nil {
		s.logger.Error("Message handling error", logger.Field{Key: "error", Value: err})
	}
}

func (s *Server) performHandshake(conn *Connection) error {
	handshake := NewHandshake()
	return handshake.PerformServerHandshake(conn.conn)
}

func (s *Server) sendInitialMessages(conn *Connection) error {
	// Set chunk size
	if err := conn.writer.WriteSetChunkSize(4096); err != nil {
		return err
	}

	// Set window ack size
	if err := conn.writer.WriteWindowAckSize(DefaultWindowAckSize); err != nil {
		return err
	}

	// Set peer bandwidth
	if err := conn.writer.WriteSetPeerBandwidth(DefaultPeerBandwidth, 2); err != nil {
		return err
	}

	return nil
}

func (s *Server) handleMessages(conn *Connection) error {
	for {
		msg, err := conn.reader.ReadMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if err := s.handleMessage(conn, msg); err != nil {
			return err
		}
	}
}

func (s *Server) handleMessage(conn *Connection, msg *Message) error {
	switch msg.MessageTypeID {
	case MessageTypeCommandAMF0:
		return s.handleCommandMessage(conn, msg)
	case MessageTypeAudio:
		return s.handleAudioMessage(conn, msg)
	case MessageTypeVideo:
		return s.handleVideoMessage(conn, msg)
	case MessageTypeDataAMF0:
		return s.handleDataMessage(conn, msg)
	case MessageTypeSetChunkSize:
		return s.handleSetChunkSize(conn, msg)
	default:
		s.logger.Debug("Unhandled message type", logger.Field{Key: "type", Value: msg.MessageTypeID})
	}
	return nil
}

func (s *Server) handleCommandMessage(conn *Connection, msg *Message) error {
	decoder := NewAMF0Decoder(bytes.NewReader(msg.Payload))

	// Read command name
	cmdName, err := decoder.DecodeString()
	if err != nil {
		return err
	}

	s.logger.Debug("RTMP command", logger.Field{Key: "command", Value: cmdName})

	switch cmdName {
	case "connect":
		return s.handleConnect(conn, decoder)
	case "createStream":
		return s.handleCreateStream(conn, decoder)
	case "publish":
		return s.handlePublish(conn, decoder)
	case "play":
		return s.handlePlay(conn, decoder)
	case "deleteStream":
		return s.handleDeleteStream(conn, decoder)
	}

	return nil
}

func (s *Server) handleConnect(conn *Connection, decoder *AMF0Decoder) error {
	// Read transaction ID
	_, _ = decoder.DecodeNumber()

	// Read command object
	cmdObj, _ := decoder.Decode()
	if obj, ok := cmdObj.(map[string]interface{}); ok {
		conn.metadata = obj
	}

	// Send response
	return s.sendConnectResponse(conn)
}

func (s *Server) sendConnectResponse(conn *Connection) error {
	buf := &bytes.Buffer{}
	encoder := NewAMF0Encoder(buf)

	// _result
	encoder.EncodeString("_result")
	encoder.EncodeNumber(1)

	// Properties
	props := map[string]interface{}{
		"fmsVer":       "ZenLive/1,0,0,0",
		"capabilities": float64(31),
	}
	encoder.EncodeObject(props)

	// Information
	info := map[string]interface{}{
		"level":          "status",
		"code":           "NetConnection.Connect.Success",
		"description":    "Connection succeeded",
		"objectEncoding": float64(0),
	}
	encoder.EncodeObject(info)

	msg := &Message{
		ChunkStreamID:   ChunkStreamIDCommand,
		Timestamp:       0,
		MessageTypeID:   MessageTypeCommandAMF0,
		MessageStreamID: 0,
		Payload:         buf.Bytes(),
	}

	return conn.writer.WriteMessage(msg)
}

func (s *Server) handleCreateStream(conn *Connection, decoder *AMF0Decoder) error {
	// Read transaction ID
	transactionID, _ := decoder.DecodeNumber()

	// Assign stream ID
	conn.streamID = 1

	// Send response
	buf := &bytes.Buffer{}
	encoder := NewAMF0Encoder(buf)

	encoder.EncodeString("_result")
	encoder.EncodeNumber(transactionID)
	encoder.EncodeNull()
	encoder.EncodeNumber(float64(conn.streamID))

	msg := &Message{
		ChunkStreamID:   ChunkStreamIDCommand,
		Timestamp:       0,
		MessageTypeID:   MessageTypeCommandAMF0,
		MessageStreamID: 0,
		Payload:         buf.Bytes(),
	}

	return conn.writer.WriteMessage(msg)
}

func (s *Server) handlePublish(conn *Connection, decoder *AMF0Decoder) error {
	// Read transaction ID
	_, _ = decoder.DecodeNumber()
	// Read null
	_, _ = decoder.Decode()
	// Read stream key
	streamKey, _ := decoder.DecodeString()
	// Read publish type
	publishType, _ := decoder.DecodeString()

	conn.streamKey = streamKey
	conn.publishMode = PublishMode(publishType)
	conn.state = StatePublishing

	s.logger.Info("Stream publishing started",
		logger.Field{Key: "key", Value: streamKey},
		logger.Field{Key: "type", Value: publishType})

	// Register stream
	s.mu.Lock()
	s.streams[streamKey] = &StreamInfo{
		StreamKey:    streamKey,
		StreamID:     conn.streamID,
		PublishType:  publishType,
		StartTime:    time.Now(),
		Metadata:     conn.metadata,
		IsPublishing: true,
	}
	s.mu.Unlock()

	// Call callback
	if s.onPublish != nil {
		if err := s.onPublish(streamKey, conn.metadata); err != nil {
			return err
		}
	}

	// Send publish status
	return s.sendPublishStatus(conn, streamKey)
}

func (s *Server) sendPublishStatus(conn *Connection, streamKey string) error {
	buf := &bytes.Buffer{}
	encoder := NewAMF0Encoder(buf)

	encoder.EncodeString("onStatus")
	encoder.EncodeNumber(0)
	encoder.EncodeNull()

	info := map[string]interface{}{
		"level":       "status",
		"code":        "NetStream.Publish.Start",
		"description": fmt.Sprintf("Publishing %s", streamKey),
	}
	encoder.EncodeObject(info)

	msg := &Message{
		ChunkStreamID:   ChunkStreamIDCommand,
		Timestamp:       0,
		MessageTypeID:   MessageTypeCommandAMF0,
		MessageStreamID: conn.streamID,
		Payload:         buf.Bytes(),
	}

	return conn.writer.WriteMessage(msg)
}

func (s *Server) handlePlay(conn *Connection, decoder *AMF0Decoder) error {
	// Read transaction ID
	_, _ = decoder.DecodeNumber()
	// Read null
	_, _ = decoder.Decode()
	// Read stream key
	streamKey, _ := decoder.DecodeString()

	conn.streamKey = streamKey
	conn.state = StatePlaying

	s.logger.Info("Stream playback started", logger.Field{Key: "key", Value: streamKey})

	// Call callback
	if s.onPlay != nil {
		if err := s.onPlay(streamKey); err != nil {
			return err
		}
	}

	return s.sendPlayStatus(conn, streamKey)
}

func (s *Server) sendPlayStatus(conn *Connection, streamKey string) error {
	buf := &bytes.Buffer{}
	encoder := NewAMF0Encoder(buf)

	encoder.EncodeString("onStatus")
	encoder.EncodeNumber(0)
	encoder.EncodeNull()

	info := map[string]interface{}{
		"level":       "status",
		"code":        "NetStream.Play.Start",
		"description": fmt.Sprintf("Playing %s", streamKey),
	}
	encoder.EncodeObject(info)

	msg := &Message{
		ChunkStreamID:   ChunkStreamIDCommand,
		Timestamp:       0,
		MessageTypeID:   MessageTypeCommandAMF0,
		MessageStreamID: conn.streamID,
		Payload:         buf.Bytes(),
	}

	return conn.writer.WriteMessage(msg)
}

func (s *Server) handleDeleteStream(conn *Connection, decoder *AMF0Decoder) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if info, exists := s.streams[conn.streamKey]; exists {
		info.IsPublishing = false
		delete(s.streams, conn.streamKey)
	}

	s.logger.Info("Stream deleted", logger.Field{Key: "key", Value: conn.streamKey})
	return nil
}

func (s *Server) handleAudioMessage(conn *Connection, msg *Message) error {
	s.logger.Debug("Audio data received", logger.Field{Key: "size", Value: len(msg.Payload)})
	// Audio data handling would go here (forwarding to players, recording, etc.)
	return nil
}

func (s *Server) handleVideoMessage(conn *Connection, msg *Message) error {
	s.logger.Debug("Video data received", logger.Field{Key: "size", Value: len(msg.Payload)})
	// Video data handling would go here (forwarding to players, recording, etc.)
	return nil
}

func (s *Server) handleDataMessage(conn *Connection, msg *Message) error {
	decoder := NewAMF0Decoder(bytes.NewReader(msg.Payload))
	dataType, _ := decoder.DecodeString()

	if dataType == "@setDataFrame" {
		// Read metadata
		_, _ = decoder.DecodeString()
		metadata, _ := decoder.Decode()

		if meta, ok := metadata.(map[string]interface{}); ok {
			conn.metadata = meta
			s.logger.Info("Stream metadata received", logger.Field{Key: "metadata", Value: meta})
		}
	}

	return nil
}

func (s *Server) handleSetChunkSize(conn *Connection, msg *Message) error {
	if len(msg.Payload) < 4 {
		return fmt.Errorf("invalid chunk size message")
	}

	chunkSize := uint32(msg.Payload[0])<<24 | uint32(msg.Payload[1])<<16 |
		uint32(msg.Payload[2])<<8 | uint32(msg.Payload[3])

	conn.reader.SetChunkSize(chunkSize)
	s.logger.Debug("Chunk size updated", logger.Field{Key: "size", Value: chunkSize})

	return nil
}

// GetStreams returns all active streams
func (s *Server) GetStreams() map[string]*StreamInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	streams := make(map[string]*StreamInfo)
	for k, v := range s.streams {
		streams[k] = v
	}
	return streams
}

// GetStream returns information about a specific stream
func (s *Server) GetStream(streamKey string) (*StreamInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stream, exists := s.streams[streamKey]
	return stream, exists
}

// Addr returns the server address
func (s *Server) Addr() string {
	return s.addr
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	return s.running
}

// Protocol returns the protocol name
func (s *Server) Protocol() types.StreamProtocol {
	return types.ProtocolRTMP
}
