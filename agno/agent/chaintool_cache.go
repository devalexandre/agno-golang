package agent

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type CacheEntry struct {
	ToolName  string
	Input     interface{}
	Result    interface{}
	Error     error
	Timestamp int64
	HitCount  int
}

type ChainToolCache interface {
	Get(ctx context.Context, toolName string, input interface{}) (interface{}, bool)
	Set(ctx context.Context, toolName string, input interface{}, result interface{})
	SetError(ctx context.Context, toolName string, input interface{}, err error)
	Clear(toolName string)
	ClearAll()
	Stats() CacheStats
	Invalidate(duration time.Duration)
}

type CacheStats struct {
	TotalHits   int64
	TotalMisses int64
	HitRate     float64
	ItemCount   int
	MemoryUsage int64
}

type MemoryCache struct {
	mu       sync.RWMutex
	cache    map[string]*CacheEntry
	ttl      time.Duration
	maxItems int
	hits     int64
	misses   int64
}

func NewMemoryCache(ttl time.Duration, maxItems int) *MemoryCache {
	return &MemoryCache{
		cache:    make(map[string]*CacheEntry),
		ttl:      ttl,
		maxItems: maxItems,
	}
}

func (mc *MemoryCache) keyHash(toolName string, input interface{}) string {
	hash := md5.Sum([]byte(fmt.Sprintf("%s:%v", toolName, input)))
	return hex.EncodeToString(hash[:])
}

func (mc *MemoryCache) Get(ctx context.Context, toolName string, input interface{}) (interface{}, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	key := mc.keyHash(toolName, input)
	entry, exists := mc.cache[key]

	if !exists {
		mc.misses++
		return nil, false
	}

	if mc.ttl > 0 && time.Now().Unix()-entry.Timestamp > int64(mc.ttl.Seconds()) {
		mc.misses++
		go func() {
			mc.mu.Lock()
			defer mc.mu.Unlock()
			delete(mc.cache, key)
		}()
		return nil, false
	}

	entry.HitCount++
	mc.hits++
	return entry.Result, true
}

func (mc *MemoryCache) Set(ctx context.Context, toolName string, input interface{}, result interface{}) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if len(mc.cache) >= mc.maxItems && mc.maxItems > 0 {
		mc.evictOldest()
	}

	key := mc.keyHash(toolName, input)
	mc.cache[key] = &CacheEntry{
		ToolName:  toolName,
		Input:     input,
		Result:    result,
		Error:     nil,
		Timestamp: time.Now().Unix(),
		HitCount:  0,
	}
}

func (mc *MemoryCache) SetError(ctx context.Context, toolName string, input interface{}, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := mc.keyHash(toolName, input)
	mc.cache[key] = &CacheEntry{
		ToolName:  toolName,
		Input:     input,
		Result:    nil,
		Error:     err,
		Timestamp: time.Now().Unix(),
		HitCount:  0,
	}
}

func (mc *MemoryCache) Clear(toolName string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	for key, entry := range mc.cache {
		if entry.ToolName == toolName {
			delete(mc.cache, key)
		}
	}
}

func (mc *MemoryCache) ClearAll() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.cache = make(map[string]*CacheEntry)
	mc.hits = 0
	mc.misses = 0
}

func (mc *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime int64 = 9223372036854775807

	for key, entry := range mc.cache {
		if entry.Timestamp < oldestTime {
			oldestTime = entry.Timestamp
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(mc.cache, oldestKey)
	}
}

func (mc *MemoryCache) Stats() CacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	total := mc.hits + mc.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(mc.hits) / float64(total)
	}

	return CacheStats{
		TotalHits:   mc.hits,
		TotalMisses: mc.misses,
		HitRate:     hitRate,
		ItemCount:   len(mc.cache),
	}
}

func (mc *MemoryCache) Invalidate(duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	cutoff := time.Now().Unix() - int64(duration.Seconds())
	for key, entry := range mc.cache {
		if entry.Timestamp < cutoff {
			delete(mc.cache, key)
		}
	}
}

type NoCache struct{}

func (nc *NoCache) Get(ctx context.Context, toolName string, input interface{}) (interface{}, bool) {
	return nil, false
}

func (nc *NoCache) Set(ctx context.Context, toolName string, input interface{}, result interface{}) {}

func (nc *NoCache) SetError(ctx context.Context, toolName string, input interface{}, err error) {}

func (nc *NoCache) Clear(toolName string) {}

func (nc *NoCache) ClearAll() {}

func (nc *NoCache) Stats() CacheStats {
	return CacheStats{}
}

func (nc *NoCache) Invalidate(duration time.Duration) {}
