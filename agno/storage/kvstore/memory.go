package kvstore

import (
	"context"
	"sort"
	"sync"
)

type memEntry struct {
	doc   []byte
	index map[string]string
}

// MemoryBackend is an in-process Backend, the reference implementation used
// by tests and by backends' fakes.
type MemoryBackend struct {
	mu   sync.RWMutex
	data map[string]map[string]memEntry
}

// NewMemoryBackend creates an empty MemoryBackend.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{data: map[string]map[string]memEntry{}}
}

// Init is a no-op.
func (m *MemoryBackend) Init(context.Context) error { return nil }

func cloneBytes(b []byte) []byte { return append([]byte(nil), b...) }

// Put stores a document.
func (m *MemoryBackend) Put(_ context.Context, coll, id string, doc []byte, index map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[coll] == nil {
		m.data[coll] = map[string]memEntry{}
	}
	m.data[coll][id] = memEntry{cloneBytes(doc), index}
	return nil
}

// Insert stores a document only if absent.
func (m *MemoryBackend) Insert(_ context.Context, coll, id string, doc []byte, index map[string]string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[coll] == nil {
		m.data[coll] = map[string]memEntry{}
	}
	if _, ok := m.data[coll][id]; ok {
		return false, nil
	}
	m.data[coll][id] = memEntry{cloneBytes(doc), index}
	return true, nil
}

// Get returns a document or nil.
func (m *MemoryBackend) Get(_ context.Context, coll, id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.data[coll][id]
	if !ok {
		return nil, nil
	}
	return cloneBytes(e.doc), nil
}

// Delete removes a document.
func (m *MemoryBackend) Delete(_ context.Context, coll, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data[coll], id)
	return nil
}

// List returns the documents matching where, ordered by id.
func (m *MemoryBackend) List(_ context.Context, coll string, where map[string]string) ([][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.data[coll]))
	for id, e := range m.data[coll] {
		if MatchIndex(e.index, where) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := make([][]byte, 0, len(ids))
	for _, id := range ids {
		out = append(out, cloneBytes(m.data[coll][id].doc))
	}
	return out, nil
}

// Drop removes a collection.
func (m *MemoryBackend) Drop(_ context.Context, coll string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, coll)
	return nil
}

// MatchIndex reports whether index satisfies every equality in where.
func MatchIndex(index, where map[string]string) bool {
	for k, v := range where {
		if index[k] != v {
			return false
		}
	}
	return true
}

var (
	_ Backend  = (*MemoryBackend)(nil)
	_ Inserter = (*MemoryBackend)(nil)
)
