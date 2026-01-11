package rtmp

import (
	"bytes"
	"testing"
)

func TestAMF0Encoding(t *testing.T) {
	buf := &bytes.Buffer{}
	encoder := NewAMF0Encoder(buf)

	t.Run("EncodeNumber", func(t *testing.T) {
		buf.Reset()
		err := encoder.EncodeNumber(123.456)
		if err != nil {
			t.Fatalf("Failed to encode number: %v", err)
		}
		if buf.Len() == 0 {
			t.Error("No data written")
		}
	})

	t.Run("EncodeString", func(t *testing.T) {
		buf.Reset()
		err := encoder.EncodeString("test")
		if err != nil {
			t.Fatalf("Failed to encode string: %v", err)
		}
		if buf.Len() == 0 {
			t.Error("No data written")
		}
	})

	t.Run("EncodeBoolean", func(t *testing.T) {
		buf.Reset()
		err := encoder.EncodeBoolean(true)
		if err != nil {
			t.Fatalf("Failed to encode boolean: %v", err)
		}
		if buf.Len() == 0 {
			t.Error("No data written")
		}
	})

	t.Run("EncodeObject", func(t *testing.T) {
		buf.Reset()
		obj := map[string]interface{}{
			"name": "test",
			"age":  float64(25),
		}
		err := encoder.EncodeObject(obj)
		if err != nil {
			t.Fatalf("Failed to encode object: %v", err)
		}
		if buf.Len() == 0 {
			t.Error("No data written")
		}
	})
}

func TestAMF0Decoding(t *testing.T) {
	t.Run("DecodeNumber", func(t *testing.T) {
		buf := &bytes.Buffer{}
		encoder := NewAMF0Encoder(buf)
		encoder.EncodeNumber(123.456)

		decoder := NewAMF0Decoder(buf)
		value, err := decoder.Decode()
		if err != nil {
			t.Fatalf("Failed to decode number: %v", err)
		}
		if num, ok := value.(float64); ok {
			if num != 123.456 {
				t.Errorf("Expected 123.456, got %f", num)
			}
		} else {
			t.Errorf("Expected float64, got %T", value)
		}
	})

	t.Run("DecodeString", func(t *testing.T) {
		buf := &bytes.Buffer{}
		encoder := NewAMF0Encoder(buf)
		encoder.EncodeString("test")

		decoder := NewAMF0Decoder(buf)
		value, err := decoder.Decode()
		if err != nil {
			t.Fatalf("Failed to decode string: %v", err)
		}
		if str, ok := value.(string); ok {
			if str != "test" {
				t.Errorf("Expected 'test', got '%s'", str)
			}
		} else {
			t.Errorf("Expected string, got %T", value)
		}
	})

	t.Run("DecodeBoolean", func(t *testing.T) {
		buf := &bytes.Buffer{}
		encoder := NewAMF0Encoder(buf)
		encoder.EncodeBoolean(true)

		decoder := NewAMF0Decoder(buf)
		value, err := decoder.Decode()
		if err != nil {
			t.Fatalf("Failed to decode boolean: %v", err)
		}
		if b, ok := value.(bool); ok {
			if !b {
				t.Error("Expected true, got false")
			}
		} else {
			t.Errorf("Expected bool, got %T", value)
		}
	})
}

func TestChunkWriterAndReader(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewChunkWriter(buf)
	reader := NewChunkReader(buf)

	t.Run("WriteAndReadMessage", func(t *testing.T) {
		// Create message
		payload := []byte("test payload")
		msg := &Message{
			ChunkStreamID:   ChunkStreamIDCommand,
			Timestamp:       1000,
			MessageTypeID:   MessageTypeCommandAMF0,
			MessageStreamID: 1,
			Payload:         payload,
		}

		// Write message
		err := writer.WriteMessage(msg)
		if err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}

		// Read message
		readMsg, err := reader.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		// Verify
		if readMsg.ChunkStreamID != msg.ChunkStreamID {
			t.Errorf("ChunkStreamID mismatch: expected %d, got %d",
				msg.ChunkStreamID, readMsg.ChunkStreamID)
		}
		if readMsg.MessageTypeID != msg.MessageTypeID {
			t.Errorf("MessageTypeID mismatch: expected %d, got %d",
				msg.MessageTypeID, readMsg.MessageTypeID)
		}
		if !bytes.Equal(readMsg.Payload, msg.Payload) {
			t.Errorf("Payload mismatch: expected %v, got %v",
				msg.Payload, readMsg.Payload)
		}
	})
}

func TestConnectionState(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{StateInit, "Init"},
		{StateHandshake0, "Handshake0"},
		{StateConnected, "Connected"},
		{StatePublishing, "Publishing"},
		{StatePlaying, "Playing"},
		{StateClosed, "Closed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.state.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.state.String())
			}
		})
	}
}
