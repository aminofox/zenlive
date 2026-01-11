package optimization

import (
	"context"
	"testing"
	"time"
)

// mockConnection is a mock connection for testing
type mockConnection struct {
	closed bool
	alive  bool
}

func (mc *mockConnection) Close() error {
	mc.closed = true
	return nil
}

func (mc *mockConnection) IsAlive() bool {
	return mc.alive
}

func (mc *mockConnection) Reset() error {
	return nil
}

// mockFactory creates mock connections
type mockFactory struct {
	created int
}

func (mf *mockFactory) Create(ctx context.Context) (Connection, error) {
	mf.created++
	return &mockConnection{alive: true}, nil
}

func (mf *mockFactory) Validate(conn Connection) bool {
	if mc, ok := conn.(*mockConnection); ok {
		return mc.alive
	}
	return false
}

func TestConnectionPool(t *testing.T) {
	factory := &mockFactory{}
	config := DefaultPoolConfig()
	config.MaxIdle = 2
	config.MaxActive = 5

	pool := NewConnectionPool(factory, config)
	pool.Start()
	defer pool.Stop()

	ctx := context.Background()

	// Get a connection
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if factory.created != 1 {
		t.Errorf("Expected 1 connection created, got %d", factory.created)
	}

	// Return connection
	conn1.Close()

	// Get another connection - should reuse
	conn2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Should still be 1 created (reused)
	if factory.created != 1 {
		t.Errorf("Expected 1 connection created (reused), got %d", factory.created)
	}

	conn2.Close()

	// Check stats
	stats := pool.Stats()
	if stats.IdleCount != 1 {
		t.Errorf("Expected 1 idle connection, got %d", stats.IdleCount)
	}
}

func TestConnectionPoolExhaustion(t *testing.T) {
	factory := &mockFactory{}
	config := DefaultPoolConfig()
	config.MaxActive = 2
	config.WaitTimeout = 100 * time.Millisecond

	pool := NewConnectionPool(factory, config)
	pool.Start()
	defer pool.Stop()

	ctx := context.Background()

	// Get max connections
	conn1, _ := pool.Get(ctx)
	conn2, _ := pool.Get(ctx)

	// Try to get another - should timeout
	_, err := pool.Get(ctx)
	if err == nil {
		t.Error("Expected error when pool exhausted")
	}

	// Return one connection
	conn1.Close()

	// Now should be able to get a connection
	conn3, err := pool.Get(ctx)
	if err != nil {
		t.Errorf("Expected to get connection after return, got error: %v", err)
	}

	conn2.Close()
	conn3.Close()
}

func TestBufferPool(t *testing.T) {
	sizes := []int{1024, 4096, 16384}
	pool := NewBufferPool(sizes)

	// Get a 1KB buffer
	buf1 := pool.Get(1024)
	if buf1.Cap() < 1024 {
		t.Errorf("Expected buffer capacity >= 1024, got %d", buf1.Cap())
	}

	// Release buffer
	buf1.Release()

	// Get another 1KB buffer - should reuse
	buf2 := pool.Get(1024)
	if buf2.Cap() < 1024 {
		t.Errorf("Expected buffer capacity >= 1024, got %d", buf2.Cap())
	}

	buf2.Release()

	// Get a larger buffer
	buf3 := pool.Get(8192)
	if buf3.Cap() < 8192 {
		t.Errorf("Expected buffer capacity >= 8192, got %d", buf3.Cap())
	}

	buf3.Release()
}

func TestZeroCopyWriter(t *testing.T) {
	writer := NewZeroCopyWriter()

	// Write some data
	data1 := []byte("Hello, ")
	data2 := []byte("World!")

	writer.Write(data1)
	writer.Write(data2)

	// Get all data
	result := writer.Bytes()
	expected := "Hello, World!"

	if string(result) != expected {
		t.Errorf("Expected %s, got %s", expected, string(result))
	}

	// Check length
	if writer.Len() != len(expected) {
		t.Errorf("Expected length %d, got %d", len(expected), writer.Len())
	}

	// Reset
	writer.Reset()

	if writer.Len() != 0 {
		t.Errorf("Expected length 0 after reset, got %d", writer.Len())
	}
}

func TestSharedMemory(t *testing.T) {
	sm := NewSharedMemory(1024)

	// Allocate memory
	slice1, err := sm.Allocate(100)
	if err != nil {
		t.Fatalf("Allocate failed: %v", err)
	}

	if len(slice1) != 100 {
		t.Errorf("Expected slice length 100, got %d", len(slice1))
	}

	// Check used memory
	used := sm.Used()
	if used != 100 {
		t.Errorf("Expected 100 bytes used, got %d", used)
	}

	// Allocate more
	slice2, err := sm.Allocate(200)
	if err != nil {
		t.Fatalf("Allocate failed: %v", err)
	}

	if len(slice2) != 200 {
		t.Errorf("Expected slice length 200, got %d", len(slice2))
	}

	// Check total used
	used = sm.Used()
	if used != 300 {
		t.Errorf("Expected 300 bytes used, got %d", used)
	}

	// Reset
	sm.Reset()

	used = sm.Used()
	if used != 0 {
		t.Errorf("Expected 0 bytes used after reset, got %d", used)
	}
}

func TestByteSliceToString(t *testing.T) {
	data := []byte("Hello, World!")
	str := ByteSliceToString(data)

	if str != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", str)
	}
}

func TestStringToByteSlice(t *testing.T) {
	str := "Hello, World!"
	data := StringToByteSlice(str)

	if string(data) != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", string(data))
	}
}
