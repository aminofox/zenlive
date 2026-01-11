package database

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

// DBConfig represents database configuration
type DBConfig struct {
	Driver          string        // Database driver (postgres, mysql, etc.)
	DSN             string        // Data source name
	MaxOpenConns    int           // Maximum open connections
	MaxIdleConns    int           // Maximum idle connections
	ConnMaxLifetime time.Duration // Maximum connection lifetime
	ConnMaxIdleTime time.Duration // Maximum connection idle time
	ReadTimeout     time.Duration // Read query timeout
	WriteTimeout    time.Duration // Write query timeout
}

// DefaultDBConfig returns the default database configuration
func DefaultDBConfig() DBConfig {
	return DBConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
	}
}

// DBPool manages database connections
type DBPool struct {
	master   *sql.DB
	replicas []*sql.DB
	config   DBConfig

	// Round-robin index for read replicas
	replicaIndex uint32

	mu sync.RWMutex
}

// NewDBPool creates a new database pool
func NewDBPool(config DBConfig) (*DBPool, error) {
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 25
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 10
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = 5 * time.Minute
	}
	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = 2 * time.Minute
	}

	// Open master connection
	master, err := sql.Open(config.Driver, config.DSN)
	if err != nil {
		return nil, err
	}

	// Configure master connection pool
	master.SetMaxOpenConns(config.MaxOpenConns)
	master.SetMaxIdleConns(config.MaxIdleConns)
	master.SetConnMaxLifetime(config.ConnMaxLifetime)
	master.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	if err := master.Ping(); err != nil {
		master.Close()
		return nil, err
	}

	return &DBPool{
		master:   master,
		replicas: make([]*sql.DB, 0),
		config:   config,
	}, nil
}

// AddReplica adds a read replica
func (pool *DBPool) AddReplica(dsn string) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Open replica connection
	replica, err := sql.Open(pool.config.Driver, dsn)
	if err != nil {
		return err
	}

	// Configure replica connection pool
	replica.SetMaxOpenConns(pool.config.MaxOpenConns)
	replica.SetMaxIdleConns(pool.config.MaxIdleConns)
	replica.SetConnMaxLifetime(pool.config.ConnMaxLifetime)
	replica.SetConnMaxIdleTime(pool.config.ConnMaxIdleTime)

	// Test connection
	if err := replica.Ping(); err != nil {
		replica.Close()
		return err
	}

	pool.replicas = append(pool.replicas, replica)

	return nil
}

// Master returns the master database connection
func (pool *DBPool) Master() *sql.DB {
	return pool.master
}

// Replica returns a read replica connection (round-robin)
func (pool *DBPool) Replica() *sql.DB {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	// If no replicas, use master
	if len(pool.replicas) == 0 {
		return pool.master
	}

	// Round-robin selection
	index := pool.replicaIndex % uint32(len(pool.replicas))
	pool.replicaIndex++

	return pool.replicas[index]
}

// Query executes a read query on a replica
func (pool *DBPool) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	// Add timeout to context
	if pool.config.ReadTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, pool.config.ReadTimeout)
		defer cancel()
	}

	return pool.Replica().QueryContext(ctx, query, args...)
}

// QueryRow executes a read query returning a single row on a replica
func (pool *DBPool) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	// Add timeout to context
	if pool.config.ReadTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, pool.config.ReadTimeout)
		defer cancel()
	}

	return pool.Replica().QueryRowContext(ctx, query, args...)
}

// Exec executes a write query on the master
func (pool *DBPool) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	// Add timeout to context
	if pool.config.WriteTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, pool.config.WriteTimeout)
		defer cancel()
	}

	return pool.Master().ExecContext(ctx, query, args...)
}

// Begin starts a transaction on the master
func (pool *DBPool) Begin(ctx context.Context) (*sql.Tx, error) {
	return pool.Master().BeginTx(ctx, nil)
}

// BeginTx starts a transaction with options
func (pool *DBPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return pool.Master().BeginTx(ctx, opts)
}

// Close closes all database connections
func (pool *DBPool) Close() error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Close master
	if err := pool.master.Close(); err != nil {
		return err
	}

	// Close replicas
	for _, replica := range pool.replicas {
		if err := replica.Close(); err != nil {
			return err
		}
	}

	return nil
}

// Stats returns database pool statistics
func (pool *DBPool) Stats() PoolStats {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	masterStats := pool.master.Stats()

	stats := PoolStats{
		Master: DBStats{
			OpenConnections:   masterStats.OpenConnections,
			InUse:             masterStats.InUse,
			Idle:              masterStats.Idle,
			WaitCount:         masterStats.WaitCount,
			WaitDuration:      masterStats.WaitDuration,
			MaxIdleClosed:     masterStats.MaxIdleClosed,
			MaxLifetimeClosed: masterStats.MaxLifetimeClosed,
		},
		Replicas: make([]DBStats, len(pool.replicas)),
	}

	for i, replica := range pool.replicas {
		replicaStats := replica.Stats()
		stats.Replicas[i] = DBStats{
			OpenConnections:   replicaStats.OpenConnections,
			InUse:             replicaStats.InUse,
			Idle:              replicaStats.Idle,
			WaitCount:         replicaStats.WaitCount,
			WaitDuration:      replicaStats.WaitDuration,
			MaxIdleClosed:     replicaStats.MaxIdleClosed,
			MaxLifetimeClosed: replicaStats.MaxLifetimeClosed,
		}
	}

	return stats
}

// Ping checks the health of all connections
func (pool *DBPool) Ping(ctx context.Context) error {
	// Ping master
	if err := pool.master.PingContext(ctx); err != nil {
		return err
	}

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	// Ping replicas
	for i, replica := range pool.replicas {
		if err := replica.PingContext(ctx); err != nil {
			return errors.New("replica " + string(rune(i)) + " unhealthy")
		}
	}

	return nil
}

// DBStats represents database statistics
type DBStats struct {
	OpenConnections   int           // Open connections
	InUse             int           // Connections in use
	Idle              int           // Idle connections
	WaitCount         int64         // Total wait count
	WaitDuration      time.Duration // Total wait duration
	MaxIdleClosed     int64         // Connections closed due to idle
	MaxLifetimeClosed int64         // Connections closed due to lifetime
}

// PoolStats represents pool statistics
type PoolStats struct {
	Master   DBStats   // Master database stats
	Replicas []DBStats // Replica database stats
}

// QueryBuilder helps build SQL queries safely
type QueryBuilder struct {
	query string
	args  []interface{}
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		args: make([]interface{}, 0),
	}
}

// Append adds a query fragment
func (qb *QueryBuilder) Append(fragment string, args ...interface{}) *QueryBuilder {
	if qb.query != "" {
		qb.query += " "
	}
	qb.query += fragment
	qb.args = append(qb.args, args...)
	return qb
}

// Where adds a WHERE clause
func (qb *QueryBuilder) Where(condition string, args ...interface{}) *QueryBuilder {
	if qb.query != "" {
		qb.query += " WHERE "
	}
	qb.query += condition
	qb.args = append(qb.args, args...)
	return qb
}

// And adds an AND condition
func (qb *QueryBuilder) And(condition string, args ...interface{}) *QueryBuilder {
	qb.query += " AND " + condition
	qb.args = append(qb.args, args...)
	return qb
}

// Or adds an OR condition
func (qb *QueryBuilder) Or(condition string, args ...interface{}) *QueryBuilder {
	qb.query += " OR " + condition
	qb.args = append(qb.args, args...)
	return qb
}

// OrderBy adds an ORDER BY clause
func (qb *QueryBuilder) OrderBy(column string, direction string) *QueryBuilder {
	qb.query += " ORDER BY " + column + " " + direction
	return qb
}

// Limit adds a LIMIT clause
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.query += " LIMIT ?"
	qb.args = append(qb.args, limit)
	return qb
}

// Offset adds an OFFSET clause
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.query += " OFFSET ?"
	qb.args = append(qb.args, offset)
	return qb
}

// Build returns the final query and arguments
func (qb *QueryBuilder) Build() (string, []interface{}) {
	return qb.query, qb.args
}

// String returns the query string
func (qb *QueryBuilder) String() string {
	return qb.query
}

// Args returns the query arguments
func (qb *QueryBuilder) Args() []interface{} {
	return qb.args
}
