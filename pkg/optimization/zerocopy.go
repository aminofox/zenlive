package optimization

import (
	"errors"
	"sync"
	"unsafe"
)

// Buffer represents a zero-copy buffer
type Buffer struct {
	data []byte
	refs int32
	pool *BufferPool
	mu   sync.Mutex
}

// NewBuffer creates a new buffer
func NewBuffer(size int) *Buffer {
	return &Buffer{
		data: make([]byte, size),
		refs: 1,
	}
}

// Data returns the buffer data
func (b *Buffer) Data() []byte {
	return b.data
}

// Len returns the buffer length
func (b *Buffer) Len() int {
	return len(b.data)
}

// Cap returns the buffer capacity
func (b *Buffer) Cap() int {
	return cap(b.data)
}

// Slice returns a slice of the buffer
func (b *Buffer) Slice(start, end int) []byte {
	return b.data[start:end]
}

// Retain increments the reference count
func (b *Buffer) Retain() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refs++
}

// Release decrements the reference count and returns to pool if zero
func (b *Buffer) Release() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refs--
	if b.refs <= 0 && b.pool != nil {
		b.pool.Put(b)
	}
}

// BufferPool manages a pool of reusable buffers
type BufferPool struct {
	pools map[int]*sync.Pool
	sizes []int
	mu    sync.RWMutex
}

// NewBufferPool creates a new buffer pool
func NewBufferPool(sizes []int) *BufferPool {
	bp := &BufferPool{
		pools: make(map[int]*sync.Pool),
		sizes: sizes,
	}

	for _, size := range sizes {
		s := size // Capture for closure
		bp.pools[size] = &sync.Pool{
			New: func() interface{} {
				return &Buffer{
					data: make([]byte, s),
					refs: 0,
					pool: bp,
				}
			},
		}
	}

	return bp
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get(size int) *Buffer {
	// Find the smallest pool that can fit the size
	poolSize := bp.findPoolSize(size)

	bp.mu.RLock()
	pool, exists := bp.pools[poolSize]
	bp.mu.RUnlock()

	if !exists {
		// No suitable pool, create a new buffer
		return NewBuffer(size)
	}

	buf := pool.Get().(*Buffer)
	buf.refs = 1
	buf.data = buf.data[:size]

	return buf
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buf *Buffer) {
	if buf.pool != bp {
		return
	}

	poolSize := cap(buf.data)

	bp.mu.RLock()
	pool, exists := bp.pools[poolSize]
	bp.mu.RUnlock()

	if !exists {
		return
	}

	// Reset buffer state
	buf.refs = 0
	buf.data = buf.data[:cap(buf.data)]

	pool.Put(buf)
}

// findPoolSize finds the smallest pool size that can fit the requested size
func (bp *BufferPool) findPoolSize(size int) int {
	for _, poolSize := range bp.sizes {
		if poolSize >= size {
			return poolSize
		}
	}

	// Return the largest pool size or the requested size
	if len(bp.sizes) > 0 {
		largest := bp.sizes[len(bp.sizes)-1]
		if largest >= size {
			return largest
		}
	}

	return size
}

// ZeroCopyWriter writes data without copying
type ZeroCopyWriter struct {
	buffers []*Buffer
	offset  int
	mu      sync.Mutex
}

// NewZeroCopyWriter creates a new zero-copy writer
func NewZeroCopyWriter() *ZeroCopyWriter {
	return &ZeroCopyWriter{
		buffers: make([]*Buffer, 0),
	}
}

// Write writes data to the writer
func (zcw *ZeroCopyWriter) Write(data []byte) (int, error) {
	zcw.mu.Lock()
	defer zcw.mu.Unlock()

	// Create a new buffer for the data
	buf := NewBuffer(len(data))
	copy(buf.data, data)

	zcw.buffers = append(zcw.buffers, buf)

	return len(data), nil
}

// WriteBuffer writes a buffer to the writer
func (zcw *ZeroCopyWriter) WriteBuffer(buf *Buffer) {
	zcw.mu.Lock()
	defer zcw.mu.Unlock()

	buf.Retain()
	zcw.buffers = append(zcw.buffers, buf)
}

// Bytes returns all data as a single byte slice
func (zcw *ZeroCopyWriter) Bytes() []byte {
	zcw.mu.Lock()
	defer zcw.mu.Unlock()

	totalLen := 0
	for _, buf := range zcw.buffers {
		totalLen += buf.Len()
	}

	result := make([]byte, totalLen)
	offset := 0

	for _, buf := range zcw.buffers {
		copy(result[offset:], buf.Data())
		offset += buf.Len()
	}

	return result
}

// Buffers returns all buffers
func (zcw *ZeroCopyWriter) Buffers() []*Buffer {
	zcw.mu.Lock()
	defer zcw.mu.Unlock()

	buffers := make([]*Buffer, len(zcw.buffers))
	copy(buffers, zcw.buffers)

	return buffers
}

// Reset resets the writer
func (zcw *ZeroCopyWriter) Reset() {
	zcw.mu.Lock()
	defer zcw.mu.Unlock()

	for _, buf := range zcw.buffers {
		buf.Release()
	}

	zcw.buffers = zcw.buffers[:0]
	zcw.offset = 0
}

// Len returns the total length of data
func (zcw *ZeroCopyWriter) Len() int {
	zcw.mu.Lock()
	defer zcw.mu.Unlock()

	totalLen := 0
	for _, buf := range zcw.buffers {
		totalLen += buf.Len()
	}

	return totalLen
}

// ByteSliceToString converts a byte slice to string without copying
func ByteSliceToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringToByteSlice converts a string to byte slice without copying
func StringToByteSlice(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}

// CopyAvoidance represents strategies to avoid memory copies
type CopyAvoidance struct {
	// EnableZeroCopy enables zero-copy optimizations
	EnableZeroCopy bool

	// EnableBufferPooling enables buffer pooling
	EnableBufferPooling bool

	// BufferPool is the buffer pool to use
	BufferPool *BufferPool
}

// DefaultCopyAvoidance returns default copy avoidance settings
func DefaultCopyAvoidance() *CopyAvoidance {
	return &CopyAvoidance{
		EnableZeroCopy:      true,
		EnableBufferPooling: true,
		BufferPool: NewBufferPool([]int{
			1024,    // 1KB
			4096,    // 4KB
			16384,   // 16KB
			65536,   // 64KB
			262144,  // 256KB
			1048576, // 1MB
		}),
	}
}

// SharedMemory represents a shared memory region
type SharedMemory struct {
	data   []byte
	size   int
	offset int
	mu     sync.RWMutex
}

// NewSharedMemory creates a new shared memory region
func NewSharedMemory(size int) *SharedMemory {
	return &SharedMemory{
		data: make([]byte, size),
		size: size,
	}
}

// Allocate allocates a slice from shared memory
func (sm *SharedMemory) Allocate(size int) ([]byte, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.offset+size > sm.size {
		return nil, errors.New("insufficient shared memory")
	}

	slice := sm.data[sm.offset : sm.offset+size]
	sm.offset += size

	return slice, nil
}

// Reset resets the shared memory allocator
func (sm *SharedMemory) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.offset = 0
}

// Used returns the amount of used memory
func (sm *SharedMemory) Used() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.offset
}

// Available returns the amount of available memory
func (sm *SharedMemory) Available() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.size - sm.offset
}

// Size returns the total size
func (sm *SharedMemory) Size() int {
	return sm.size
}
