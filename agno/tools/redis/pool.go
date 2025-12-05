package redis

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// RedisConn represents a Redis connection
type RedisConn struct {
	conn      net.Conn
	createdAt time.Time
	lastUsed  time.Time
	healthy   bool
}

// RedisPool manages a pool of Redis connections
type RedisPool struct {
	host                 string
	port                 int
	maxConns             int
	idleTimeout          time.Duration
	dialTimeout          time.Duration
	connChan             chan *RedisConn
	availableConnections int
	mu                   sync.RWMutex
	closed               bool
}

// NewRedisPool creates a new Redis connection pool
func NewRedisPool(host string, port int, maxConns int) *RedisPool {
	return &RedisPool{
		host:                 host,
		port:                 port,
		maxConns:             maxConns,
		idleTimeout:          5 * time.Minute,
		dialTimeout:          5 * time.Second,
		connChan:             make(chan *RedisConn, maxConns),
		availableConnections: 0,
		closed:               false,
	}
}

// Get retrieves a connection from the pool
func (rp *RedisPool) Get() (*RedisConn, error) {
	rp.mu.RLock()
	if rp.closed {
		rp.mu.RUnlock()
		return nil, fmt.Errorf("pool is closed")
	}
	rp.mu.RUnlock()

	select {
	case conn := <-rp.connChan:
		// Check if connection is still valid
		if time.Since(conn.lastUsed) > rp.idleTimeout {
			conn.conn.Close()
			return rp.newConnection()
		}
		conn.lastUsed = time.Now()
		return conn, nil
	default:
		return rp.newConnection()
	}
}

// Put returns a connection to the pool
func (rp *RedisPool) Put(conn *RedisConn) {
	if conn == nil {
		return
	}

	rp.mu.RLock()
	defer rp.mu.RUnlock()

	if rp.closed {
		conn.conn.Close()
		return
	}

	conn.lastUsed = time.Now()
	select {
	case rp.connChan <- conn:
		rp.availableConnections++
	default:
		conn.conn.Close()
	}
}

// newConnection creates a new Redis connection
func (rp *RedisPool) newConnection() (*RedisConn, error) {
	conn, err := net.DialTimeout("tcp",
		fmt.Sprintf("%s:%d", rp.host, rp.port),
		rp.dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	return &RedisConn{
		conn:      conn,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		healthy:   true,
	}, nil
}

// Stats returns pool statistics
func (rp *RedisPool) Stats() map[string]interface{} {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	available := len(rp.connChan)
	utilization := float64(rp.maxConns-available) / float64(rp.maxConns)

	return map[string]interface{}{
		"host":        rp.host,
		"port":        rp.port,
		"available":   available,
		"max_conns":   rp.maxConns,
		"utilization": utilization,
		"closed":      rp.closed,
	}
}

// Close closes all connections in the pool
func (rp *RedisPool) Close() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.closed {
		return fmt.Errorf("pool already closed")
	}

	rp.closed = true
	close(rp.connChan)

	for {
		select {
		case conn := <-rp.connChan:
			if conn != nil && conn.conn != nil {
				conn.conn.Close()
			}
		default:
			return nil
		}
	}
}

// PoolManager manages multiple Redis connection pools
type PoolManager struct {
	pools map[string]*RedisPool
	mu    sync.RWMutex
}

// NewPoolManager creates a new pool manager
func NewPoolManager() *PoolManager {
	return &PoolManager{
		pools: make(map[string]*RedisPool),
	}
}

// GetPool retrieves or creates a pool for the specified host:port
func (pm *PoolManager) GetPool(host string, port int, maxConns int) *RedisPool {
	key := fmt.Sprintf("%s:%d", host, port)

	pm.mu.RLock()
	if pool, exists := pm.pools[key]; exists {
		pm.mu.RUnlock()
		return pool
	}
	pm.mu.RUnlock()

	// Create new pool
	pool := NewRedisPool(host, port, maxConns)

	pm.mu.Lock()
	pm.pools[key] = pool
	pm.mu.Unlock()

	return pool
}

// ClosePool closes a pool
func (pm *PoolManager) ClosePool(host string, port int) error {
	key := fmt.Sprintf("%s:%d", host, port)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	pool, exists := pm.pools[key]
	if !exists {
		return fmt.Errorf("pool %s not found", key)
	}

	err := pool.Close()
	delete(pm.pools, key)
	return err
}

// CloseAll closes all pools
func (pm *PoolManager) CloseAll() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var errs []error
	for key, pool := range pm.pools {
		if err := pool.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close pool %s: %w", key, err))
		}
	}

	pm.pools = make(map[string]*RedisPool)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing pools: %v", errs)
	}
	return nil
}

// GetStats returns statistics for all pools
func (pm *PoolManager) GetStats() map[string]map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for key, pool := range pm.pools {
		stats[key] = pool.Stats()
	}
	return stats
}

// Global pool manager
var globalPoolManager = NewPoolManager()

// GetGlobalPoolManager returns the global pool manager
func GetGlobalPoolManager() *PoolManager {
	return globalPoolManager
}
