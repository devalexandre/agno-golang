package inmemory

import (
	"context"
	"sort"
	"sync"

	"github.com/devalexandre/agno-golang/agno/storage"
)

// metricsData holds the metrics tables of an InMemoryDB.
type metricsData struct {
	mu    sync.RWMutex
	rows  map[string]storage.ComponentMetricsRow
	state *storage.MetricsState
}

// SearchSessions implements storage.SessionSearcher.
func (d *InMemoryDB) SearchSessions(ctx context.Context, filter *storage.SessionFilter, opts storage.SessionSearchOptions) ([]*storage.AgentSession, int, error) {
	all, err := d.GetAllSessions(ctx, "")
	if err != nil {
		return nil, 0, err
	}
	return storage.FilterSortPaginate(all, filter, opts)
}

// metricsBackend adapts InMemoryDB to storage.MetricsBackend.
type metricsBackend struct{ d *InMemoryDB }

func (m metricsBackend) ListSessionsForMetrics(_ context.Context, f storage.MetricsSourceFilter) ([]*storage.AgentSession, error) {
	m.d.mu.RLock()
	defer m.d.mu.RUnlock()
	var out []*storage.AgentSession
	for _, s := range m.d.sessions {
		if f.MatchSession(s) {
			out = append(out, cloneSession(s))
		}
	}
	return out, nil
}

func (m metricsBackend) ListRunsForMetrics(_ context.Context, f storage.MetricsSourceFilter) ([]*storage.AgentRun, error) {
	m.d.mu.RLock()
	defer m.d.mu.RUnlock()
	var out []*storage.AgentRun
	for _, r := range m.d.runs {
		if f.MatchRun(r) {
			out = append(out, cloneRun(r))
		}
	}
	return out, nil
}

func (m metricsBackend) ReadSession(ctx context.Context, id string) (*storage.AgentSession, error) {
	return m.d.ReadSession(ctx, id)
}

func (m metricsBackend) ReplaceComponentMetrics(_ context.Context, day string, rows []storage.ComponentMetricsRow) error {
	md := m.d.metrics
	md.mu.Lock()
	defer md.mu.Unlock()
	for id, r := range md.rows {
		if r.Date == day {
			delete(md.rows, id)
		}
	}
	for _, r := range rows {
		md.rows[r.ID] = r
	}
	return nil
}

func (m metricsBackend) ListComponentMetrics(_ context.Context, startDay, endDay string) ([]storage.ComponentMetricsRow, error) {
	md := m.d.metrics
	md.mu.RLock()
	defer md.mu.RUnlock()
	var out []storage.ComponentMetricsRow
	for _, r := range md.rows {
		if (startDay == "" || r.Date >= startDay) && (endDay == "" || r.Date <= endDay) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m metricsBackend) GetMetricsState(context.Context) (*storage.MetricsState, error) {
	md := m.d.metrics
	md.mu.RLock()
	defer md.mu.RUnlock()
	if md.state == nil {
		return nil, nil
	}
	cp := *md.state
	return &cp, nil
}

func (m metricsBackend) SetMetricsState(_ context.Context, st *storage.MetricsState) error {
	md := m.d.metrics
	md.mu.Lock()
	defer md.mu.Unlock()
	cp := *st
	md.state = &cp
	return nil
}

var (
	_ storage.SessionSearcher = (*InMemoryDB)(nil)
	_ storage.MetricsStore    = (*InMemoryDB)(nil)
)
