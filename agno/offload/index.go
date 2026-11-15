package offload

import (
	"encoding/json"
	"sort"
	"sync"

	"github.com/devalexandre/agno-golang/agno/fs"
)

// Index stores the rows describing offloaded payloads (Python's
// agno_tool_results table).
type Index interface {
	Upsert(row Row) error
	Get(resultID string) (*Row, error)
	// ListForSession returns the session's rows newest first; limit <= 0 = all.
	ListForSession(sessionID string, limit int) ([]Row, error)
	// Expired returns rows whose ExpiresAt is set and <= now.
	Expired(now int64) ([]Row, error)
	Delete(resultIDs []string) error
}

func sortNewest(rows []Row, limit int) []Row {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].CreatedAt != rows[j].CreatedAt {
			return rows[i].CreatedAt > rows[j].CreatedAt
		}
		return rows[i].ResultID > rows[j].ResultID
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// MemoryIndex is an in-process Index.
type MemoryIndex struct {
	mu   sync.RWMutex
	rows map[string]Row
}

// NewMemoryIndex returns an empty in-memory index.
func NewMemoryIndex() *MemoryIndex { return &MemoryIndex{rows: map[string]Row{}} }

// Upsert implements Index.
func (m *MemoryIndex) Upsert(row Row) error {
	m.mu.Lock()
	m.rows[row.ResultID] = row
	m.mu.Unlock()
	return nil
}

// Get implements Index.
func (m *MemoryIndex) Get(id string) (*Row, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if r, ok := m.rows[id]; ok {
		return &r, nil
	}
	return nil, nil
}

// ListForSession implements Index.
func (m *MemoryIndex) ListForSession(sessionID string, limit int) ([]Row, error) {
	m.mu.RLock()
	var out []Row
	for _, r := range m.rows {
		if r.SessionID == sessionID {
			out = append(out, r)
		}
	}
	m.mu.RUnlock()
	return sortNewest(out, limit), nil
}

// Expired implements Index.
func (m *MemoryIndex) Expired(now int64) ([]Row, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Row
	for _, r := range m.rows {
		if r.ExpiresAt > 0 && r.ExpiresAt <= now {
			out = append(out, r)
		}
	}
	return out, nil
}

// Delete implements Index.
func (m *MemoryIndex) Delete(ids []string) error {
	m.mu.Lock()
	for _, id := range ids {
		delete(m.rows, id)
	}
	m.mu.Unlock()
	return nil
}

// FSIndex keeps rows as JSON files in a filesystem namespace, so a durable
// backend (fs.LocalBackend) makes the index durable too.
type FSIndex struct {
	store *fs.FileSystem
}

// NewFSIndex builds an index stored in backend under namespace "tool-results-index".
func NewFSIndex(backend fs.Backend) (*FSIndex, error) {
	store, err := fs.New(backend, fs.WithNamespace("tool-results-index"),
		fs.WithMaxFileBytes(MaxResultBytes), fs.WithMaxNamespaceBytes(MaxSessionNamespaceBytes))
	if err != nil {
		return nil, err
	}
	return &FSIndex{store: store}, nil
}

func rowPath(id string) string { return "rows/" + id + ".json" }

// Upsert implements Index.
func (x *FSIndex) Upsert(row Row) error {
	data, err := json.Marshal(row)
	if err != nil {
		return err
	}
	_, err = x.store.Write(rowPath(row.ResultID), string(data))
	return err
}

// Get implements Index.
func (x *FSIndex) Get(id string) (*Row, error) {
	if _, err := fs.NormalizePath(rowPath(id)); err != nil {
		return nil, nil
	}
	data, ok, err := x.store.Read(rowPath(id))
	if err != nil || !ok {
		return nil, err
	}
	var r Row
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (x *FSIndex) all() ([]Row, error) {
	metas, err := x.store.List("rows")
	if err != nil {
		return nil, err
	}
	out := make([]Row, 0, len(metas))
	for _, m := range metas {
		data, ok, err := x.store.Read(m.Path)
		if err != nil || !ok {
			continue
		}
		var r Row
		if json.Unmarshal([]byte(data), &r) == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

// ListForSession implements Index.
func (x *FSIndex) ListForSession(sessionID string, limit int) ([]Row, error) {
	rows, err := x.all()
	if err != nil {
		return nil, err
	}
	var out []Row
	for _, r := range rows {
		if r.SessionID == sessionID {
			out = append(out, r)
		}
	}
	return sortNewest(out, limit), nil
}

// Expired implements Index.
func (x *FSIndex) Expired(now int64) ([]Row, error) {
	rows, err := x.all()
	if err != nil {
		return nil, err
	}
	var out []Row
	for _, r := range rows {
		if r.ExpiresAt > 0 && r.ExpiresAt <= now {
			out = append(out, r)
		}
	}
	return out, nil
}

// Delete implements Index.
func (x *FSIndex) Delete(ids []string) error {
	for _, id := range ids {
		if _, err := x.store.Delete(rowPath(id), nil); err != nil {
			return err
		}
	}
	return nil
}
