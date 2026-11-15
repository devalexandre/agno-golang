package fs

import (
	"sort"
	"sync"
	"time"
)

type memKey struct{ ns, user, path string }

type memFile struct {
	content string
	version int
	updated int64
}

// MemoryBackend is an in-process, versioned backend (supports CAS). Useful
// for tests and ephemeral agents.
type MemoryBackend struct {
	mu    sync.RWMutex
	files map[memKey]*memFile
}

// NewMemoryBackend returns an empty in-memory backend.
func NewMemoryBackend() *MemoryBackend { return &MemoryBackend{files: map[memKey]*memFile{}} }

func (m *MemoryBackend) meta(k memKey, f *memFile) FileMeta {
	return FileMeta{Path: k.path, SizeBytes: len(f.content), Version: intPtr(f.version), UpdatedAt: int64Ptr(f.updated), UserID: k.user}
}

// Read implements Backend.
func (m *MemoryBackend) Read(ns, path, user string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.files[memKey{ns, user, path}]
	if !ok {
		return "", false, nil
	}
	return f.content, true, nil
}

// Write implements Backend with optional compare-and-swap.
func (m *MemoryBackend) Write(ns, path, content, user string, expected *int) (FileMeta, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := memKey{ns, user, path}
	f, ok := m.files[k]
	if expected != nil {
		if !ok {
			return FileMeta{}, &VersionConflictError{Expected: *expected}
		}
		if f.version != *expected {
			return FileMeta{}, &VersionConflictError{Expected: *expected, Actual: intPtr(f.version)}
		}
	}
	if !ok {
		f = &memFile{}
		m.files[k] = f
	}
	f.content = content
	f.version++
	f.updated = time.Now().Unix()
	return m.meta(k, f), nil
}

// List implements Backend.
func (m *MemoryBackend) List(ns, directory, user string) ([]FileMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []FileMeta
	for k, f := range m.files {
		if k.ns == ns && k.user == user && PathInDirectory(k.path, directory) {
			out = append(out, m.meta(k, f))
		}
	}
	return out, nil
}

// Delete implements Backend.
func (m *MemoryBackend) Delete(ns, path, user string, expected *int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := memKey{ns, user, path}
	f, ok := m.files[k]
	if !ok {
		return false, nil
	}
	if expected != nil && f.version != *expected {
		return false, &VersionConflictError{Expected: *expected, Actual: intPtr(f.version)}
	}
	delete(m.files, k)
	return true, nil
}

// Stat implements Stater.
func (m *MemoryBackend) Stat(ns, path, user string) (*FileMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k := memKey{ns, user, path}
	if f, ok := m.files[k]; ok {
		meta := m.meta(k, f)
		return &meta, nil
	}
	return nil, nil
}

// Partitions implements Partitioner.
func (m *MemoryBackend) Partitions(ns string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := map[string]bool{}
	for k := range m.files {
		if k.ns == ns && k.user != "" {
			seen[k.user] = true
		}
	}
	out := make([]string, 0, len(seen))
	for u := range seen {
		out = append(out, u)
	}
	sort.Strings(out)
	return out, nil
}
