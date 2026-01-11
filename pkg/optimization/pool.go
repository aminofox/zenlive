package optimization

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Connection represents a pooled connection
type Connection interface {
	// Close closes the connection
	Close() error

	// IsAlive checks if the connection is still alive
	IsAlive() bool

	// Reset resets the connection state
	Reset() error
}

// ConnectionFactory creates new connections
type ConnectionFactory interface {
	// Create creates a new connection
	Create(ctx context.Context) (Connection, error)

	// Validate validates a connection
	Validate(conn Connection) bool
}

// PoolConfig represents connection pool configuration
type PoolConfig struct {
	MaxIdle       int           // Maximum idle connections
	MaxActive     int           // Maximum active connections (0 = unlimited)
	MaxLifetime   time.Duration // Maximum lifetime of a connection
	IdleTimeout   time.Duration // Idle timeout before closing
	WaitTimeout   time.Duration // Wait timeout when pool is exhausted
	TestOnBorrow  bool          // Test connection before borrowing
	TestOnReturn  bool          // Test connection before returning
	TestWhileIdle bool          // Test idle connections periodically
	TestInterval  time.Duration // Interval for testing idle connections
}

// DefaultPoolConfig returns the default pool configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxIdle:       10,
		MaxActive:     100,
		MaxLifetime:   30 * time.Minute,
		IdleTimeout:   5 * time.Minute,
		WaitTimeout:   10 * time.Second,
		TestOnBorrow:  true,
		TestOnReturn:  false,
		TestWhileIdle: true,
		TestInterval:  30 * time.Second,
	}
}

// ConnectionPool manages a pool of connections
type ConnectionPool struct {
	factory ConnectionFactory
	config  PoolConfig

	idle       []*pooledConnection
	active     int
	waiters    int
	waiterChan chan struct{}

	mu       sync.Mutex
	stopChan chan struct{}
	running  bool
}

// pooledConnection wraps a connection with metadata
type pooledConnection struct {
	conn       Connection
	createdAt  time.Time
	lastUsed   time.Time
	usageCount int64
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(factory ConnectionFactory, config PoolConfig) *ConnectionPool {
	pool := &ConnectionPool{
		factory:    factory,
		config:     config,
		idle:       make([]*pooledConnection, 0, config.MaxIdle),
		waiterChan: make(chan struct{}, 1),
	}

	return pool
}

// Start starts the connection pool
func (cp *ConnectionPool) Start() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.running {
		return
	}

	cp.running = true
	cp.stopChan = make(chan struct{})

	if cp.config.TestWhileIdle {
		go cp.cleanupLoop()
	}
}

// Stop stops the connection pool
func (cp *ConnectionPool) Stop() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if !cp.running {
		return
	}

	cp.running = false
	close(cp.stopChan)

	// Close all idle connections
	for _, pc := range cp.idle {
		pc.conn.Close()
	}
	cp.idle = nil
}

// Get retrieves a connection from the pool
func (cp *ConnectionPool) Get(ctx context.Context) (Connection, error) {
	cp.mu.Lock()

	// Try to get an idle connection
	for len(cp.idle) > 0 {
		pc := cp.idle[len(cp.idle)-1]
		cp.idle = cp.idle[:len(cp.idle)-1]

		cp.mu.Unlock()

		// Validate if configured
		if cp.config.TestOnBorrow {
			if !cp.factory.Validate(pc.conn) {
				pc.conn.Close()
				cp.mu.Lock()
				continue
			}
		}

		// Check lifetime
		if cp.config.MaxLifetime > 0 && time.Since(pc.createdAt) > cp.config.MaxLifetime {
			pc.conn.Close()
			cp.mu.Lock()
			continue
		}

		pc.lastUsed = time.Now()
		pc.usageCount++
		cp.active++

		return &poolConnection{pool: cp, conn: pc}, nil
	}

	// Check if we can create a new connection
	if cp.config.MaxActive == 0 || cp.active < cp.config.MaxActive {
		cp.active++
		cp.mu.Unlock()

		conn, err := cp.factory.Create(ctx)
		if err != nil {
			cp.mu.Lock()
			cp.active--
			cp.mu.Unlock()
			return nil, err
		}

		pc := &pooledConnection{
			conn:       conn,
			createdAt:  time.Now(),
			lastUsed:   time.Now(),
			usageCount: 1,
		}

		return &poolConnection{pool: cp, conn: pc}, nil
	}

	// Wait for a connection to become available
	if cp.config.WaitTimeout == 0 {
		cp.mu.Unlock()
		return nil, errors.New("connection pool exhausted")
	}

	cp.waiters++
	cp.mu.Unlock()

	timer := time.NewTimer(cp.config.WaitTimeout)
	defer timer.Stop()

	select {
	case <-cp.waiterChan:
		cp.mu.Lock()
		cp.waiters--
		cp.mu.Unlock()
		return cp.Get(ctx)
	case <-timer.C:
		cp.mu.Lock()
		cp.waiters--
		cp.mu.Unlock()
		return nil, errors.New("connection pool wait timeout")
	case <-ctx.Done():
		cp.mu.Lock()
		cp.waiters--
		cp.mu.Unlock()
		return nil, ctx.Err()
	}
}

// put returns a connection to the pool
func (cp *ConnectionPool) put(pc *pooledConnection) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.active--

	// Validate if configured
	if cp.config.TestOnReturn {
		if !cp.factory.Validate(pc.conn) {
			pc.conn.Close()
			return nil
		}
	}

	// Check lifetime
	if cp.config.MaxLifetime > 0 && time.Since(pc.createdAt) > cp.config.MaxLifetime {
		pc.conn.Close()
		return nil
	}

	// Add to idle pool if there's space
	if len(cp.idle) < cp.config.MaxIdle {
		pc.conn.Reset()
		cp.idle = append(cp.idle, pc)

		// Notify waiters
		if cp.waiters > 0 {
			select {
			case cp.waiterChan <- struct{}{}:
			default:
			}
		}
	} else {
		pc.conn.Close()
	}

	return nil
}

// cleanupLoop periodically cleans up idle connections
func (cp *ConnectionPool) cleanupLoop() {
	ticker := time.NewTicker(cp.config.TestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cp.cleanupIdle()
		case <-cp.stopChan:
			return
		}
	}
}

// cleanupIdle removes expired idle connections
func (cp *ConnectionPool) cleanupIdle() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	now := time.Now()
	newIdle := make([]*pooledConnection, 0, len(cp.idle))

	for _, pc := range cp.idle {
		// Check idle timeout
		if cp.config.IdleTimeout > 0 && now.Sub(pc.lastUsed) > cp.config.IdleTimeout {
			pc.conn.Close()
			continue
		}

		// Check max lifetime
		if cp.config.MaxLifetime > 0 && now.Sub(pc.createdAt) > cp.config.MaxLifetime {
			pc.conn.Close()
			continue
		}

		// Test connection
		if cp.config.TestWhileIdle {
			if !cp.factory.Validate(pc.conn) {
				pc.conn.Close()
				continue
			}
		}

		newIdle = append(newIdle, pc)
	}

	cp.idle = newIdle
}

// Stats returns pool statistics
func (cp *ConnectionPool) Stats() PoolStats {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	return PoolStats{
		IdleCount:   len(cp.idle),
		ActiveCount: cp.active,
		WaiterCount: cp.waiters,
	}
}

// PoolStats represents pool statistics
type PoolStats struct {
	IdleCount   int // Number of idle connections
	ActiveCount int // Number of active connections
	WaiterCount int // Number of waiting requests
}

// poolConnection wraps a pooled connection
type poolConnection struct {
	pool *ConnectionPool
	conn *pooledConnection
	mu   sync.Mutex
}

// Close returns the connection to the pool
func (pc *poolConnection) Close() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.conn == nil {
		return nil
	}

	err := pc.pool.put(pc.conn)
	pc.conn = nil

	return err
}

// IsAlive checks if the connection is alive
func (pc *poolConnection) IsAlive() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.conn == nil {
		return false
	}

	return pc.conn.conn.IsAlive()
}

// Reset resets the connection
func (pc *poolConnection) Reset() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.conn == nil {
		return errors.New("connection closed")
	}

	return pc.conn.conn.Reset()
}

// GetRawConnection returns the underlying connection
func (pc *poolConnection) GetRawConnection() Connection {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.conn == nil {
		return nil
	}

	return pc.conn.conn
}
