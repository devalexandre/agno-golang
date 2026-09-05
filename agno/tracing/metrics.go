package tracing

import (
	"sync"
	"time"
)

// Usage describes the resource consumption of a single model call.
type Usage struct {
	InputTokens     int64
	OutputTokens    int64
	TotalTokens     int64
	CachedTokens    int64
	ReasoningTokens int64

	// Cost of the call in Currency (defaults to "USD" when empty).
	Cost     float64
	Currency string

	// Duration of the model call.
	Duration time.Duration
}

// normalized returns a copy of u with derived fields filled in.
func (u Usage) normalized() Usage {
	if u.TotalTokens == 0 {
		u.TotalTokens = u.InputTokens + u.OutputTokens
	}
	if u.Currency == "" {
		u.Currency = DefaultCurrency
	}
	return u
}

// Attributes renders the usage as canonical span attributes.
func (u Usage) Attributes() map[string]any {
	n := u.normalized()
	attrs := make(map[string]any, 8)
	putInt(attrs, AttrTokensInput, n.InputTokens)
	putInt(attrs, AttrTokensOutput, n.OutputTokens)
	putInt(attrs, AttrTokensTotal, n.TotalTokens)
	putInt(attrs, AttrTokensCached, n.CachedTokens)
	putInt(attrs, AttrTokensReasoning, n.ReasoningTokens)
	putFloat(attrs, AttrCost, n.Cost)
	if n.Cost != 0 {
		attrs[AttrCostCurrency] = n.Currency
	}
	putFloat(attrs, AttrLatencyMS, float64(n.Duration.Milliseconds()))
	return attrs
}

// DefaultCurrency is used when a Usage does not specify one.
const DefaultCurrency = "USD"

// Metrics is the consolidated view of a run or a session: tokens, cost,
// latency and call counters.
type Metrics struct {
	RunID     string
	SessionID string
	AgentID   string

	InputTokens     int64
	OutputTokens    int64
	TotalTokens     int64
	CachedTokens    int64
	ReasoningTokens int64

	Cost     float64
	Currency string

	ModelCalls int64
	ToolCalls  int64
	Errors     int64

	// Duration is the wall-clock duration of the run/session. It is set when
	// the run ends; before that it reflects the time elapsed so far.
	Duration time.Duration

	StartTime time.Time
	EndTime   time.Time
}

// Add merges other into m (token/cost/counter fields are summed, the time
// window is widened).
func (m *Metrics) Add(other Metrics) {
	m.InputTokens += other.InputTokens
	m.OutputTokens += other.OutputTokens
	m.TotalTokens += other.TotalTokens
	m.CachedTokens += other.CachedTokens
	m.ReasoningTokens += other.ReasoningTokens
	m.Cost += other.Cost
	m.ModelCalls += other.ModelCalls
	m.ToolCalls += other.ToolCalls
	m.Errors += other.Errors

	if m.Currency == "" {
		m.Currency = other.Currency
	}
	if !other.StartTime.IsZero() && (m.StartTime.IsZero() || other.StartTime.Before(m.StartTime)) {
		m.StartTime = other.StartTime
	}
	if other.EndTime.After(m.EndTime) {
		m.EndTime = other.EndTime
	}
	if !m.StartTime.IsZero() && m.EndTime.After(m.StartTime) {
		m.Duration = m.EndTime.Sub(m.StartTime)
	}
}

// AddUsage accumulates the usage of one model call.
func (m *Metrics) AddUsage(u Usage) {
	n := u.normalized()
	m.InputTokens += n.InputTokens
	m.OutputTokens += n.OutputTokens
	m.TotalTokens += n.TotalTokens
	m.CachedTokens += n.CachedTokens
	m.ReasoningTokens += n.ReasoningTokens
	m.Cost += n.Cost
	if m.Currency == "" {
		m.Currency = n.Currency
	}
	m.ModelCalls++
}

// Attributes renders the metrics as canonical span attributes.
func (m Metrics) Attributes() map[string]any {
	attrs := make(map[string]any, 12)
	putString(attrs, AttrRunID, m.RunID)
	putString(attrs, AttrSessionID, m.SessionID)
	putString(attrs, AttrAgentID, m.AgentID)
	putInt(attrs, AttrTokensInput, m.InputTokens)
	putInt(attrs, AttrTokensOutput, m.OutputTokens)
	putInt(attrs, AttrTokensTotal, m.TotalTokens)
	putInt(attrs, AttrTokensCached, m.CachedTokens)
	putInt(attrs, AttrTokensReasoning, m.ReasoningTokens)
	putFloat(attrs, AttrCost, m.Cost)
	if m.Cost != 0 {
		attrs[AttrCostCurrency] = m.Currency
	}
	putInt(attrs, AttrToolCallCount, m.ToolCalls)
	putFloat(attrs, AttrLatencyMS, float64(m.Duration.Milliseconds()))
	return attrs
}

// MetricsCollector aggregates Metrics per run and per session. It is safe for
// concurrent use.
type MetricsCollector struct {
	mu       sync.RWMutex
	runs     map[string]*Metrics
	sessions map[string]*Metrics
	now      func() time.Time
}

// NewMetricsCollector creates an empty collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		runs:     make(map[string]*Metrics),
		sessions: make(map[string]*Metrics),
		now:      time.Now,
	}
}

// StartRun registers a run and marks its start time. Calling it twice for the
// same run ID is a no-op for the start time.
func (c *MetricsCollector) StartRun(runID, sessionID, agentID string) {
	if runID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	m := c.runLocked(runID)
	m.SessionID = sessionID
	m.AgentID = agentID
	if m.StartTime.IsZero() {
		m.StartTime = c.now()
	}
	if sessionID != "" {
		s := c.sessionLocked(sessionID)
		if s.StartTime.IsZero() {
			s.StartTime = m.StartTime
		}
	}
}

// RecordUsage accumulates the usage of a model call into the run and its
// session.
func (c *MetricsCollector) RecordUsage(runID string, u Usage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := c.runLocked(runID)
	m.AddUsage(u)
	if s := c.sessionOf(m); s != nil {
		s.AddUsage(u)
	}
}

// RecordToolCall accumulates one tool execution into the run and its session.
func (c *MetricsCollector) RecordToolCall(runID string, duration time.Duration, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := c.runLocked(runID)
	m.ToolCalls++
	if err != nil {
		m.Errors++
	}
	if s := c.sessionOf(m); s != nil {
		s.ToolCalls++
		if err != nil {
			s.Errors++
		}
	}
	_ = duration // tool latency is carried by the span; kept for API symmetry.
}

// RecordError increments the error counter of the run and its session.
func (c *MetricsCollector) RecordError(runID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := c.runLocked(runID)
	m.Errors++
	if s := c.sessionOf(m); s != nil {
		s.Errors++
	}
}

// EndRun closes the run, computes its duration and returns the consolidated
// metrics.
func (c *MetricsCollector) EndRun(runID string) Metrics {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := c.runLocked(runID)
	m.EndTime = c.now()
	if m.StartTime.IsZero() {
		m.StartTime = m.EndTime
	}
	m.Duration = m.EndTime.Sub(m.StartTime)

	if s := c.sessionOf(m); s != nil {
		s.EndTime = m.EndTime
		if s.StartTime.IsZero() {
			s.StartTime = m.StartTime
		}
		s.Duration = s.EndTime.Sub(s.StartTime)
	}
	return *m
}

// RunMetrics returns the metrics of a run.
func (c *MetricsCollector) RunMetrics(runID string) (Metrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m, ok := c.runs[runID]
	if !ok {
		return Metrics{}, false
	}
	return *m, true
}

// SessionMetrics returns the metrics aggregated across every run of a session.
func (c *MetricsCollector) SessionMetrics(sessionID string) (Metrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s, ok := c.sessions[sessionID]
	if !ok {
		return Metrics{}, false
	}
	return *s, true
}

// Reset drops every run and session recorded so far.
func (c *MetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runs = make(map[string]*Metrics)
	c.sessions = make(map[string]*Metrics)
}

// runLocked returns (creating if needed) the metrics of a run.
// The caller must hold the write lock.
func (c *MetricsCollector) runLocked(runID string) *Metrics {
	m, ok := c.runs[runID]
	if !ok {
		m = &Metrics{RunID: runID, Currency: DefaultCurrency, StartTime: c.now()}
		c.runs[runID] = m
	}
	return m
}

// sessionLocked returns (creating if needed) the metrics of a session.
// The caller must hold the write lock.
func (c *MetricsCollector) sessionLocked(sessionID string) *Metrics {
	s, ok := c.sessions[sessionID]
	if !ok {
		s = &Metrics{SessionID: sessionID, Currency: DefaultCurrency, StartTime: c.now()}
		c.sessions[sessionID] = s
	}
	return s
}

// sessionOf returns the session metrics of a run, or nil when the run has no
// session attached. The caller must hold the write lock.
func (c *MetricsCollector) sessionOf(m *Metrics) *Metrics {
	if m.SessionID == "" {
		return nil
	}
	return c.sessionLocked(m.SessionID)
}
