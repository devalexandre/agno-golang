package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Metrics — a port of Python BaseDb.calculate_metrics / get_metrics /
// get_metrics_by_date / get_component_metrics / get_metrics_totals.
//
// Backends store "component metrics" rows: one row per day, user and
// component (agent, team or workflow) holding session/run counts, token
// counts, cost and model usage. Aggregate rows (Python agno_metrics shape)
// are derived from them on read. MetricsEngine implements the whole API on
// top of a small MetricsBackend, so every backend shares one calculation.

// MetricsTokenFields are the token counters a model call reports (Python METRICS_TOKEN_FIELDS).
var MetricsTokenFields = []string{
	"input_tokens",
	"output_tokens",
	"total_tokens",
	"audio_input_tokens",
	"audio_output_tokens",
	"audio_total_tokens",
	"cache_read_tokens",
	"cache_write_tokens",
	"reasoning_tokens",
}

// MetricsDateLayout is the layout of metrics dates (UTC days).
const MetricsDateLayout = "2006-01-02"

// ModelMetric counts the runs that used one model.
type ModelMetric struct {
	ModelID       string   `json:"model_id"`
	ModelProvider string   `json:"model_provider"`
	Count         int64    `json:"count"`
	Cost          *float64 `json:"cost,omitempty"`
}

// ComponentMetricsRow is one row of the component metrics table: the totals
// of one day, user and agent/team/workflow.
type ComponentMetricsRow struct {
	ID                string           `json:"id"`
	AggregationPeriod string           `json:"aggregation_period"`
	Date              string           `json:"date"`
	UserID            string           `json:"user_id"`
	ComponentType     string           `json:"component_type"`
	ComponentID       string           `json:"component_id"`
	SessionsCount     int64            `json:"sessions_count"`
	RunsCount         int64            `json:"runs_count"`
	Tokens            map[string]int64 `json:"tokens"`
	Cost              *float64         `json:"cost"`
	ModelMetrics      []ModelMetric    `json:"model_metrics"`
	CreatedAt         int64            `json:"created_at"`
	UpdatedAt         int64            `json:"updated_at"`
}

// ComponentMetricsRowID is the deterministic id of a component metrics row.
func ComponentMetricsRowID(r ComponentMetricsRow) string {
	return fmt.Sprintf("%s_%s_%s_%s_%s", r.Date, r.UserID, r.ComponentType, r.ComponentID, r.AggregationPeriod)
}

// MetricsRow is an aggregate metrics row (Python agno_metrics shape): the
// totals of one day, for one owner (UserID nil = unowned) or, from
// GetMetricsByDate, for everyone.
type MetricsRow struct {
	ID                    string           `json:"id"`
	Date                  string           `json:"date"`
	AggregationPeriod     string           `json:"aggregation_period"`
	UserID                *string          `json:"user_id"`
	UsersCount            int64            `json:"users_count"`
	AgentSessionsCount    int64            `json:"agent_sessions_count"`
	TeamSessionsCount     int64            `json:"team_sessions_count"`
	WorkflowSessionsCount int64            `json:"workflow_sessions_count"`
	AgentRunsCount        int64            `json:"agent_runs_count"`
	TeamRunsCount         int64            `json:"team_runs_count"`
	WorkflowRunsCount     int64            `json:"workflow_runs_count"`
	TokenMetrics          map[string]int64 `json:"token_metrics"`
	Cost                  *float64         `json:"cost"`
	ModelMetrics          []ModelMetric    `json:"model_metrics"`
	Completed             bool             `json:"completed"`
	CreatedAt             int64            `json:"created_at"`
	UpdatedAt             int64            `json:"updated_at"`
}

// ToDict renders the row in the Python shape, with cost inside token_metrics.
func (m MetricsRow) ToDict() map[string]interface{} {
	tm := map[string]interface{}{}
	for k, v := range m.TokenMetrics {
		tm[k] = v
	}
	if m.Cost != nil {
		tm["cost"] = *m.Cost
	} else {
		tm["cost"] = nil
	}
	var uid interface{}
	if m.UserID != nil {
		uid = *m.UserID
	}
	return map[string]interface{}{
		"id": m.ID, "date": m.Date, "aggregation_period": m.AggregationPeriod, "user_id": uid,
		"users_count": m.UsersCount, "agent_sessions_count": m.AgentSessionsCount,
		"team_sessions_count": m.TeamSessionsCount, "workflow_sessions_count": m.WorkflowSessionsCount,
		"agent_runs_count": m.AgentRunsCount, "team_runs_count": m.TeamRunsCount,
		"workflow_runs_count": m.WorkflowRunsCount, "token_metrics": tm, "model_metrics": m.ModelMetrics,
		"completed": m.Completed, "created_at": m.CreatedAt, "updated_at": m.UpdatedAt,
	}
}

// MetricsTotals is the total of a date range (Python get_metrics_totals).
type MetricsTotals struct {
	UsersCount            int64            `json:"users_count"`
	AgentSessionsCount    int64            `json:"agent_sessions_count"`
	TeamSessionsCount     int64            `json:"team_sessions_count"`
	WorkflowSessionsCount int64            `json:"workflow_sessions_count"`
	AgentRunsCount        int64            `json:"agent_runs_count"`
	TeamRunsCount         int64            `json:"team_runs_count"`
	WorkflowRunsCount     int64            `json:"workflow_runs_count"`
	TokenMetrics          map[string]int64 `json:"token_metrics"`
	Cost                  *float64         `json:"cost"`
	ModelMetrics          []ModelMetric    `json:"model_metrics"`
}

// MetricsState is the persisted state of metrics calculation (Python agno_metrics_refreshes).
type MetricsState struct {
	LastCalculatedAt int64  `json:"last_calculated_at"`
	Status           string `json:"status"` // "running", "completed" or "failed"
	StartedAt        int64  `json:"started_at"`
	FinishedAt       int64  `json:"finished_at"`
	Error            string `json:"error,omitempty"`
}

// MetricsSourceFilter selects the sessions/runs read by a calculation. Zero
// values are unbounded. CreatedTo is exclusive.
type MetricsSourceFilter struct {
	CreatedFrom  int64
	CreatedTo    int64
	UpdatedSince int64
}

// MatchSession reports whether a session passes the filter.
func (f MetricsSourceFilter) MatchSession(s *AgentSession) bool {
	return f.match(s.CreatedAt, s.UpdatedAt)
}

// MatchRun reports whether a run passes the filter.
func (f MetricsSourceFilter) MatchRun(r *AgentRun) bool {
	return f.match(r.CreatedAt.Unix(), r.UpdatedAt.Unix())
}

func (f MetricsSourceFilter) match(created, updated int64) bool {
	if f.CreatedFrom != 0 && created < f.CreatedFrom {
		return false
	}
	if f.CreatedTo != 0 && created >= f.CreatedTo {
		return false
	}
	if f.UpdatedSince != 0 && updated < f.UpdatedSince && created < f.UpdatedSince {
		return false
	}
	return true
}

// MetricsBackend is what a backend implements to get the metrics API from MetricsEngine.
type MetricsBackend interface {
	ListSessionsForMetrics(ctx context.Context, f MetricsSourceFilter) ([]*AgentSession, error)
	ListRunsForMetrics(ctx context.Context, f MetricsSourceFilter) ([]*AgentRun, error)
	ReadSession(ctx context.Context, sessionID string) (*AgentSession, error)
	// ReplaceComponentMetrics replaces every component row of day with rows.
	ReplaceComponentMetrics(ctx context.Context, day string, rows []ComponentMetricsRow) error
	// ListComponentMetrics returns rows with startDay <= date <= endDay ("" = unbounded).
	ListComponentMetrics(ctx context.Context, startDay, endDay string) ([]ComponentMetricsRow, error)
	GetMetricsState(ctx context.Context) (*MetricsState, error)
	SetMetricsState(ctx context.Context, st *MetricsState) error
}

// MetricsStore is the metrics API (Python BaseDb metrics methods).
type MetricsStore interface {
	CalculateMetrics(ctx context.Context, rebuildFrom *time.Time) ([]MetricsRow, error)
	GetMetrics(ctx context.Context, start, end *time.Time, userID *string) ([]MetricsRow, int64, error)
	GetMetricsByDate(ctx context.Context, start, end *time.Time) ([]MetricsRow, int64, error)
	GetComponentMetrics(ctx context.Context, start, end time.Time, userID *string) ([]ComponentMetricsRow, int64, error)
	GetMetricsTotals(ctx context.Context, start, end time.Time, userID *string) (*MetricsTotals, int64, error)
	GetMetricsRefreshStatus(ctx context.Context) (*MetricsState, error)
}

// MetricsEngine implements MetricsStore over a MetricsBackend.
type MetricsEngine struct {
	Backend MetricsBackend
	// Now is the clock (defaults to time.Now).
	Now     func() time.Time
	running sync.Mutex
}

// NewMetricsEngine creates a MetricsEngine.
func NewMetricsEngine(b MetricsBackend) *MetricsEngine { return &MetricsEngine{Backend: b} }

func (e *MetricsEngine) now() time.Time {
	if e.Now != nil {
		return e.Now().UTC()
	}
	return time.Now().UTC()
}

// MetricsDay returns the UTC day of a unix timestamp.
func MetricsDay(ts int64) string { return time.Unix(ts, 0).UTC().Format(MetricsDateLayout) }

func dayBounds(day string) (int64, int64) {
	t, _ := time.Parse(MetricsDateLayout, day)
	return t.Unix(), t.Add(24 * time.Hour).Unix()
}

// CalculateMetrics recalculates every day with sessions/runs created or
// updated since the last calculation (or every day from rebuildFrom to
// today). It returns the owner rows of the calculated days, or nil when
// another calculation is running.
func (e *MetricsEngine) CalculateMetrics(ctx context.Context, rebuildFrom *time.Time) ([]MetricsRow, error) {
	if !e.running.TryLock() {
		return nil, nil
	}
	defer e.running.Unlock()

	start := e.now()
	st, err := e.Backend.GetMetricsState(ctx)
	if err != nil {
		return nil, err
	}
	if st == nil {
		st = &MetricsState{}
	}
	since := st.LastCalculatedAt
	st.Status, st.StartedAt, st.FinishedAt, st.Error = "running", start.Unix(), 0, ""
	if err := e.Backend.SetMetricsState(ctx, st); err != nil {
		return nil, err
	}

	days, err := e.calculate(ctx, since, rebuildFrom, start)
	st.FinishedAt = e.now().Unix()
	if err != nil {
		st.Status, st.Error = "failed", err.Error()
		_ = e.Backend.SetMetricsState(ctx, st)
		return nil, err
	}
	st.Status, st.LastCalculatedAt = "completed", start.Unix()
	if err := e.Backend.SetMetricsState(ctx, st); err != nil {
		return nil, err
	}
	if len(days) == 0 {
		days = []string{start.Format(MetricsDateLayout)}
	}
	first, _ := time.Parse(MetricsDateLayout, days[0])
	last, _ := time.Parse(MetricsDateLayout, days[len(days)-1])
	rows, _, err := e.GetMetrics(ctx, &first, &last, nil)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, d := range days {
		want[d] = true
	}
	out := rows[:0]
	for _, r := range rows {
		if want[r.Date] {
			out = append(out, r)
		}
	}
	return out, nil
}

func (e *MetricsEngine) calculate(ctx context.Context, since int64, rebuildFrom *time.Time, now time.Time) ([]string, error) {
	daySet := map[string]bool{}
	if rebuildFrom != nil {
		for d := rebuildFrom.UTC().Truncate(24 * time.Hour); !d.After(now); d = d.Add(24 * time.Hour) {
			daySet[d.Format(MetricsDateLayout)] = true
		}
	}
	filter := MetricsSourceFilter{UpdatedSince: since}
	sessions, err := e.Backend.ListSessionsForMetrics(ctx, filter)
	if err != nil {
		return nil, err
	}
	runs, err := e.Backend.ListRunsForMetrics(ctx, filter)
	if err != nil {
		return nil, err
	}
	for _, s := range sessions {
		daySet[MetricsDay(s.CreatedAt)] = true
	}
	for _, r := range runs {
		daySet[MetricsDay(r.CreatedAt.Unix())] = true
	}
	days := make([]string, 0, len(daySet))
	for d := range daySet {
		days = append(days, d)
	}
	sort.Strings(days)

	cache := map[string]*AgentSession{}
	lookup := func(id string) *AgentSession {
		if s, ok := cache[id]; ok {
			return s
		}
		s, _ := e.Backend.ReadSession(ctx, id)
		cache[id] = s
		return s
	}
	ts := now.Unix()
	for _, day := range days {
		from, to := dayBounds(day)
		f := MetricsSourceFilter{CreatedFrom: from, CreatedTo: to}
		daySessions, err := e.Backend.ListSessionsForMetrics(ctx, f)
		if err != nil {
			return nil, err
		}
		dayRuns, err := e.Backend.ListRunsForMetrics(ctx, f)
		if err != nil {
			return nil, err
		}
		for _, s := range daySessions {
			cache[s.SessionID] = s
		}
		rows := CalculateComponentMetrics(day, daySessions, dayRuns, lookup, ts)
		if err := e.Backend.ReplaceComponentMetrics(ctx, day, rows); err != nil {
			return nil, err
		}
	}
	return days, nil
}

// sessionComponent returns the component type and id of a session.
func sessionComponent(s *AgentSession) (string, string) {
	rec := SessionFilterRecord(s)
	st, _ := rec["session_type"].(string)
	key := map[string]string{"agent": "agent_id", "team": "team_id", "workflow": "workflow_id"}[st]
	id, _ := rec[key].(string)
	return st, id
}

func runComponent(r *AgentRun, sessionOf func(string) *AgentSession) (string, string) {
	for _, c := range []struct{ typ, key string }{{"workflow", "workflow_id"}, {"team", "team_id"}, {"agent", "agent_id"}} {
		if v, ok := r.RunData[c.key].(string); ok && v != "" {
			return c.typ, v
		}
	}
	if sessionOf != nil {
		if s := sessionOf(r.SessionID); s != nil {
			return sessionComponent(s)
		}
	}
	return "", ""
}

func metricInt(v interface{}) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int64:
		return x
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	case float32:
		return int64(x)
	case []interface{}:
		var sum int64
		for _, i := range x {
			sum += metricInt(i)
		}
		return sum
	}
	return 0
}

func metricFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case []interface{}:
		var sum float64
		ok := false
		for _, i := range x {
			if f, o := metricFloat(i); o {
				sum += f
				ok = true
			}
		}
		return sum, ok
	}
	return toFloat(v)
}

func firstString(maps []map[string]interface{}, keys ...string) string {
	for _, m := range maps {
		for _, k := range keys {
			if v, ok := m[k].(string); ok && v != "" {
				return v
			}
		}
	}
	return ""
}

// CalculateComponentMetrics computes the component metrics rows of one day
// from the sessions and runs created on it. sessionOf resolves the session
// of a run whose RunData names no component.
func CalculateComponentMetrics(day string, sessions []*AgentSession, runs []*AgentRun, sessionOf func(string) *AgentSession, now int64) []ComponentMetricsRow {
	type key struct{ user, typ, id string }
	buckets := map[key]*ComponentMetricsRow{}
	get := func(k key) *ComponentMetricsRow {
		b, ok := buckets[k]
		if !ok {
			b = &ComponentMetricsRow{
				AggregationPeriod: "daily", Date: day, UserID: k.user,
				ComponentType: k.typ, ComponentID: k.id, Tokens: map[string]int64{},
				CreatedAt: now, UpdatedAt: now,
			}
			buckets[k] = b
		}
		return b
	}
	for _, s := range sessions {
		typ, id := sessionComponent(s)
		if id == "" {
			continue
		}
		get(key{s.UserID, typ, id}).SessionsCount++
	}
	for _, r := range runs {
		if v, ok := r.RunData["forked_from_session_id"].(string); ok && v != "" {
			continue
		}
		typ, id := runComponent(r, sessionOf)
		if id == "" {
			continue
		}
		b := get(key{r.UserID, typ, id})
		b.RunsCount++
		for _, f := range MetricsTokenFields {
			if v, ok := r.Metrics[f]; ok {
				b.Tokens[f] += metricInt(v)
			}
		}
		var cost *float64
		if c, ok := metricFloat(r.Metrics["cost"]); ok {
			cost = &c
			b.Cost = addCost(b.Cost, cost)
		}
		srcs := []map[string]interface{}{r.Metrics, r.RunData}
		if model := firstString(srcs, "model", "model_id"); model != "" {
			b.ModelMetrics = mergeModelMetrics(b.ModelMetrics, []ModelMetric{{
				ModelID: model, ModelProvider: firstString(srcs, "model_provider", "provider"), Count: 1, Cost: cost,
			}})
		}
	}
	out := make([]ComponentMetricsRow, 0, len(buckets))
	for _, b := range buckets {
		b.ID = ComponentMetricsRowID(*b)
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func addCost(a, b *float64) *float64 {
	if b == nil {
		return a
	}
	if a == nil {
		v := *b
		return &v
	}
	v := *a + *b
	return &v
}

func mergeModelMetrics(dst, extra []ModelMetric) []ModelMetric {
	for _, m := range extra {
		found := false
		for i := range dst {
			if dst[i].ModelID == m.ModelID && dst[i].ModelProvider == m.ModelProvider {
				dst[i].Count += m.Count
				dst[i].Cost = addCost(dst[i].Cost, m.Cost)
				found = true
				break
			}
		}
		if !found {
			cp := m
			cp.Cost = addCost(nil, m.Cost)
			dst = append(dst, cp)
		}
	}
	sort.Slice(dst, func(i, j int) bool {
		if dst[i].ModelID == dst[j].ModelID {
			return dst[i].ModelProvider < dst[j].ModelProvider
		}
		return dst[i].ModelID < dst[j].ModelID
	})
	return dst
}

func metricsDayCompleted(day string, now time.Time) bool {
	d, err := time.Parse(MetricsDateLayout, day)
	if err != nil {
		return false
	}
	return d.Before(now.UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour))
}

func dateRange(start, end *time.Time) (string, string) {
	var s, e string
	if start != nil {
		s = start.UTC().Format(MetricsDateLayout)
	}
	if end != nil {
		e = end.UTC().Format(MetricsDateLayout)
	}
	return s, e
}

func userMatches(rowUser string, userID *string) bool {
	return userID == nil || rowUser == *userID
}

func (e *MetricsEngine) loadRows(ctx context.Context, start, end *time.Time, userID *string) ([]ComponentMetricsRow, int64, error) {
	s, en := dateRange(start, end)
	rows, err := e.Backend.ListComponentMetrics(ctx, s, en)
	if err != nil {
		return nil, 0, err
	}
	var latest int64
	out := rows[:0]
	for _, r := range rows {
		if (s != "" && r.Date < s) || (en != "" && r.Date > en) || !userMatches(r.UserID, userID) {
			continue
		}
		if r.UpdatedAt > latest {
			latest = r.UpdatedAt
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, latest, nil
}

func addRowToMetrics(m *MetricsRow, r ComponentMetricsRow) {
	switch r.ComponentType {
	case "agent":
		m.AgentSessionsCount += r.SessionsCount
		m.AgentRunsCount += r.RunsCount
	case "team":
		m.TeamSessionsCount += r.SessionsCount
		m.TeamRunsCount += r.RunsCount
	case "workflow":
		m.WorkflowSessionsCount += r.SessionsCount
		m.WorkflowRunsCount += r.RunsCount
	}
	for k, v := range r.Tokens {
		m.TokenMetrics[k] += v
	}
	m.Cost = addCost(m.Cost, r.Cost)
	m.ModelMetrics = mergeModelMetrics(m.ModelMetrics, r.ModelMetrics)
	if m.CreatedAt == 0 || (r.CreatedAt != 0 && r.CreatedAt < m.CreatedAt) {
		m.CreatedAt = r.CreatedAt
	}
	if r.UpdatedAt > m.UpdatedAt {
		m.UpdatedAt = r.UpdatedAt
	}
}

func newMetricsRow(day string, now time.Time) *MetricsRow {
	tm := map[string]int64{}
	for _, f := range MetricsTokenFields {
		tm[f] = 0
	}
	return &MetricsRow{Date: day, AggregationPeriod: "daily", TokenMetrics: tm, ModelMetrics: []ModelMetric{}, Completed: metricsDayCompleted(day, now)}
}

// GetMetrics returns one row per day and owner. userID nil returns every
// owner, "" the unowned rows, otherwise that user's rows. It also returns the
// latest updated_at of the rows read.
func (e *MetricsEngine) GetMetrics(ctx context.Context, start, end *time.Time, userID *string) ([]MetricsRow, int64, error) {
	rows, latest, err := e.loadRows(ctx, start, end, userID)
	if err != nil {
		return nil, 0, err
	}
	type k struct{ day, user string }
	byOwner := map[k]*MetricsRow{}
	now := e.now()
	for _, r := range rows {
		m, ok := byOwner[k{r.Date, r.UserID}]
		if !ok {
			m = newMetricsRow(r.Date, now)
			m.ID = fmt.Sprintf("%s_%s_daily", r.Date, r.UserID)
			if r.UserID != "" {
				u := r.UserID
				m.UserID = &u
				m.UsersCount = 1
			}
			byOwner[k{r.Date, r.UserID}] = m
		}
		addRowToMetrics(m, r)
	}
	out := make([]MetricsRow, 0, len(byOwner))
	for _, m := range byOwner {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, latest, nil
}

// GetMetricsByDate returns one row per day totalling every owner.
func (e *MetricsEngine) GetMetricsByDate(ctx context.Context, start, end *time.Time) ([]MetricsRow, int64, error) {
	owners, latest, err := e.GetMetrics(ctx, start, end, nil)
	if err != nil {
		return nil, 0, err
	}
	return AggregateMetricsByDate(owners), latest, nil
}

// AggregateMetricsByDate collapses per-owner rows into one row per day.
func AggregateMetricsByDate(rows []MetricsRow) []MetricsRow {
	byDay := map[string]*MetricsRow{}
	var days []string
	for _, r := range rows {
		agg, ok := byDay[r.Date]
		if !ok {
			cp := r
			cp.ID = r.Date + "_" + r.AggregationPeriod
			cp.UserID = nil
			cp.TokenMetrics = map[string]int64{}
			for k, v := range r.TokenMetrics {
				cp.TokenMetrics[k] = v
			}
			cp.ModelMetrics = mergeModelMetrics(nil, r.ModelMetrics)
			cp.Cost = addCost(nil, r.Cost)
			byDay[r.Date] = &cp
			days = append(days, r.Date)
			continue
		}
		agg.UsersCount += r.UsersCount
		agg.AgentSessionsCount += r.AgentSessionsCount
		agg.TeamSessionsCount += r.TeamSessionsCount
		agg.WorkflowSessionsCount += r.WorkflowSessionsCount
		agg.AgentRunsCount += r.AgentRunsCount
		agg.TeamRunsCount += r.TeamRunsCount
		agg.WorkflowRunsCount += r.WorkflowRunsCount
		for k, v := range r.TokenMetrics {
			agg.TokenMetrics[k] += v
		}
		agg.Cost = addCost(agg.Cost, r.Cost)
		agg.ModelMetrics = mergeModelMetrics(agg.ModelMetrics, r.ModelMetrics)
		if r.CreatedAt != 0 && r.CreatedAt < agg.CreatedAt {
			agg.CreatedAt = r.CreatedAt
		}
		if r.UpdatedAt > agg.UpdatedAt {
			agg.UpdatedAt = r.UpdatedAt
		}
	}
	sort.Strings(days)
	out := make([]MetricsRow, 0, len(days))
	for _, d := range days {
		out = append(out, *byDay[d])
	}
	return out
}

// GetComponentMetrics returns the component rows of the date range.
func (e *MetricsEngine) GetComponentMetrics(ctx context.Context, start, end time.Time, userID *string) ([]ComponentMetricsRow, int64, error) {
	return e.loadRows(ctx, &start, &end, userID)
}

// GetMetricsTotals totals the component rows of the date range.
func (e *MetricsEngine) GetMetricsTotals(ctx context.Context, start, end time.Time, userID *string) (*MetricsTotals, int64, error) {
	rows, latest, err := e.loadRows(ctx, &start, &end, userID)
	if err != nil {
		return nil, 0, err
	}
	m := newMetricsRow("", e.now())
	users := map[string]bool{}
	for _, r := range rows {
		addRowToMetrics(m, r)
		if r.UserID != "" {
			users[r.UserID] = true
		}
	}
	return &MetricsTotals{
		UsersCount:            int64(len(users)),
		AgentSessionsCount:    m.AgentSessionsCount,
		TeamSessionsCount:     m.TeamSessionsCount,
		WorkflowSessionsCount: m.WorkflowSessionsCount,
		AgentRunsCount:        m.AgentRunsCount,
		TeamRunsCount:         m.TeamRunsCount,
		WorkflowRunsCount:     m.WorkflowRunsCount,
		TokenMetrics:          m.TokenMetrics,
		Cost:                  m.Cost,
		ModelMetrics:          m.ModelMetrics,
	}, latest, nil
}

// GetMetricsRefreshStatus returns the state of the most recent calculation (nil before the first).
func (e *MetricsEngine) GetMetricsRefreshStatus(ctx context.Context) (*MetricsState, error) {
	st, err := e.Backend.GetMetricsState(ctx)
	if err != nil || st == nil || st.Status == "" {
		return nil, err
	}
	return st, nil
}

var _ MetricsStore = (*MetricsEngine)(nil)
