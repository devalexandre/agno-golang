// Package metrics ports agno/metrics.py: typed token/cost/timing metrics at
// the message, model, run and session levels, with a per-model-type
// breakdown (model, output_model, memory_model, compression_model,
// compaction_model, ...).
//
// The package has no dependencies on the rest of agno so models, agent and
// team can all import it. RunResponse.Metrics (a map) stays the wire format;
// RunMetrics.ToMap produces it.
package metrics

import (
	"encoding/json"
	"sync"
	"time"
)

// ModelType identifies the functional role of a model within a run.
type ModelType string

const (
	ModelTypeModel          ModelType = "model"
	ModelTypeOutputModel    ModelType = "output_model"
	ModelTypeParserModel    ModelType = "parser_model"
	ModelTypeMemoryModel    ModelType = "memory_model"
	ModelTypeReasoningModel ModelType = "reasoning_model"
	ModelTypeSessionSummary ModelType = "session_summary_model"
	ModelTypeLearningModel  ModelType = "learning_model"
	ModelTypeCompression    ModelType = "compression_model"
	ModelTypeCompaction     ModelType = "compaction_model"
	ModelTypeFollowup       ModelType = "followup_model"
)

// Tokens are the token counters shared by every metrics level.
type Tokens struct {
	InputTokens       int      `json:"input_tokens,omitempty"`
	OutputTokens      int      `json:"output_tokens,omitempty"`
	TotalTokens       int      `json:"total_tokens,omitempty"`
	AudioInputTokens  int      `json:"audio_input_tokens,omitempty"`
	AudioOutputTokens int      `json:"audio_output_tokens,omitempty"`
	AudioTotalTokens  int      `json:"audio_total_tokens,omitempty"`
	CacheReadTokens   int      `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens  int      `json:"cache_write_tokens,omitempty"`
	ReasoningTokens   int      `json:"reasoning_tokens,omitempty"`
	Cost              *float64 `json:"cost,omitempty"`
}

// Add sums other into t.
func (t *Tokens) Add(other Tokens) {
	t.InputTokens += other.InputTokens
	t.OutputTokens += other.OutputTokens
	t.TotalTokens += other.TotalTokens
	t.AudioInputTokens += other.AudioInputTokens
	t.AudioOutputTokens += other.AudioOutputTokens
	t.AudioTotalTokens += other.AudioTotalTokens
	t.CacheReadTokens += other.CacheReadTokens
	t.CacheWriteTokens += other.CacheWriteTokens
	t.ReasoningTokens += other.ReasoningTokens
	t.Cost = addCost(t.Cost, other.Cost)
}

func addCost(a, b *float64) *float64 {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		v := *b
		return &v
	case b == nil:
		v := *a
		return &v
	}
	v := *a + *b
	return &v
}

func mergeNumeric(dst map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return dst
	}
	if dst == nil {
		dst = make(map[string]interface{}, len(src))
	}
	for k, v := range src {
		if cur, ok := dst[k]; ok {
			if a, ok := asFloat(cur); ok {
				if b, ok := asFloat(v); ok {
					dst[k] = a + b
					continue
				}
			}
		}
		dst[k] = v
	}
	return dst
}

func asFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	}
	return 0, false
}

// MessageMetrics are the usage numbers of one model call. Providers set them
// on models.MessageResponse.Metrics.
type MessageMetrics struct {
	Tokens
	Duration         time.Duration          `json:"duration,omitempty"`
	TimeToFirstToken time.Duration          `json:"time_to_first_token,omitempty"`
	ProviderMetrics  map[string]interface{} `json:"provider_metrics,omitempty"`
}

// Add returns the sum of two message metrics (first non-zero TTFT is kept).
func (m MessageMetrics) Add(other MessageMetrics) MessageMetrics {
	out := m
	out.ProviderMetrics = mergeNumeric(copyMap(m.ProviderMetrics), other.ProviderMetrics)
	out.Tokens.Add(other.Tokens)
	out.Duration += other.Duration
	if out.TimeToFirstToken == 0 {
		out.TimeToFirstToken = other.TimeToFirstToken
	}
	return out
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ModelMetrics aggregates usage for one (provider, id) model.
type ModelMetrics struct {
	Tokens
	ID              string                 `json:"id,omitempty"`
	Provider        string                 `json:"provider,omitempty"`
	ProviderMetrics map[string]interface{} `json:"provider_metrics,omitempty"`
}

// Accumulate adds other's token counts, cost and provider metrics.
func (m *ModelMetrics) Accumulate(other ModelMetrics) {
	m.Tokens.Add(other.Tokens)
	m.ProviderMetrics = mergeNumeric(m.ProviderMetrics, other.ProviderMetrics)
}

func (m ModelMetrics) clone() ModelMetrics {
	out := m
	out.Tokens.Cost = addCost(m.Cost, nil)
	out.ProviderMetrics = copyMap(m.ProviderMetrics)
	return out
}

// Details is the per-model-type breakdown.
type Details map[string][]ModelMetrics

// add finds-or-creates the (provider,id) entry under modelType and accumulates mm.
func (d Details) add(modelType string, mm ModelMetrics) {
	list := d[modelType]
	for i := range list {
		if list[i].Provider == mm.Provider && list[i].ID == mm.ID {
			list[i].Accumulate(mm)
			return
		}
	}
	d[modelType] = append(list, mm.clone())
}

func (d Details) merge(other Details) {
	for mt, list := range other {
		for _, mm := range list {
			d.add(mt, mm)
		}
	}
}

// ToolCallMetrics hold tool execution timing.
type ToolCallMetrics struct {
	StartTime time.Time     `json:"start_time,omitempty"`
	EndTime   time.Time     `json:"end_time,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// Start records the start time.
func (t *ToolCallMetrics) Start() { t.StartTime = time.Now() }

// Stop records the end time and duration.
func (t *ToolCallMetrics) Stop() {
	t.EndTime = time.Now()
	if !t.StartTime.IsZero() {
		t.Duration = t.EndTime.Sub(t.StartTime)
	}
}

// RunMetrics are run-level metrics with a per-model breakdown. Safe for
// concurrent accumulation.
type RunMetrics struct {
	Tokens
	TimeToFirstToken  time.Duration          `json:"time_to_first_token,omitempty"`
	Duration          time.Duration          `json:"duration,omitempty"`
	Details           Details                `json:"details,omitempty"`
	AdditionalMetrics map[string]interface{} `json:"additional_metrics,omitempty"`

	mu        sync.Mutex
	startedAt time.Time
}

// NewRunMetrics returns empty run metrics.
func NewRunMetrics() *RunMetrics { return &RunMetrics{} }

// StartTimer starts (or restarts) the run timer.
func (r *RunMetrics) StartTimer() {
	r.mu.Lock()
	r.startedAt = time.Now()
	r.mu.Unlock()
}

// StopTimer sets Duration from the timer.
func (r *RunMetrics) StopTimer() {
	r.mu.Lock()
	if !r.startedAt.IsZero() {
		r.Duration = time.Since(r.startedAt)
	}
	r.mu.Unlock()
}

// SetTimeToFirstToken records TTFT once.
func (r *RunMetrics) SetTimeToFirstToken() {
	r.mu.Lock()
	if r.TimeToFirstToken == 0 && !r.startedAt.IsZero() {
		r.TimeToFirstToken = time.Since(r.startedAt)
	}
	r.mu.Unlock()
}

// AccumulateModel adds one model call's usage under modelType for the
// (provider, id) model, and to the top-level counters. Port of
// accumulate_model_metrics.
func (r *RunMetrics) AccumulateModel(usage *MessageMetrics, modelID, provider string, modelType ModelType) {
	if r == nil || usage == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Details == nil {
		r.Details = Details{}
	}
	mm := ModelMetrics{Tokens: usage.Tokens, ID: modelID, Provider: provider, ProviderMetrics: usage.ProviderMetrics}
	r.Details.add(string(modelType), mm)
	r.Tokens.Add(usage.Tokens)
}

// AccumulateToolLoop adds every model call of a tool loop (one per entry).
// Port of accumulate_tool_loop_metrics.
func (r *RunMetrics) AccumulateToolLoop(usages []*MessageMetrics, modelID, provider string, modelType ModelType) {
	for _, u := range usages {
		if u != nil && u.TotalTokens > 0 {
			r.AccumulateModel(u, modelID, provider, modelType)
		}
	}
}

// AccumulateChild merges a child agent/eval's metrics under "{prefix}_<type>"
// keys. Port of accumulate_eval_metrics.
func (r *RunMetrics) AccumulateChild(child *RunMetrics, prefix string) {
	if r == nil || child == nil {
		return
	}
	snap := child.Snapshot()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Details == nil {
		r.Details = Details{}
	}
	for mt, list := range snap.Details {
		key := mt
		if len(mt) < len(prefix)+1 || mt[:len(prefix)+1] != prefix+"_" {
			key = prefix + "_" + mt
		}
		for _, mm := range list {
			r.Details.add(key, mm)
		}
	}
	r.Tokens.Add(snap.Tokens)
	if prefix == "eval" && snap.Duration > 0 {
		if r.AdditionalMetrics == nil {
			r.AdditionalMetrics = map[string]interface{}{}
		}
		prev, _ := asFloat(r.AdditionalMetrics["eval_duration"])
		r.AdditionalMetrics["eval_duration"] = prev + snap.Duration.Seconds()
	}
}

// MergeBackground merges isolated collectors (memory, learning tasks) into r.
func (r *RunMetrics) MergeBackground(background ...*RunMetrics) {
	for _, bg := range background {
		if bg == nil || bg == r {
			continue
		}
		snap := bg.Snapshot()
		r.mu.Lock()
		if r.Details == nil {
			r.Details = Details{}
		}
		r.Details.merge(snap.Details)
		r.Tokens.Add(snap.Tokens)
		r.AdditionalMetrics = mergeNumeric(r.AdditionalMetrics, snap.AdditionalMetrics)
		r.mu.Unlock()
	}
}

// Add returns the sum of r and other (details aggregated by type/provider/id,
// earliest TTFT kept, durations summed).
func (r *RunMetrics) Add(other *RunMetrics) *RunMetrics {
	a := r.Snapshot()
	out := &RunMetrics{Tokens: a.Tokens, TimeToFirstToken: a.TimeToFirstToken, Duration: a.Duration,
		AdditionalMetrics: copyMap(a.AdditionalMetrics)}
	if len(a.Details) > 0 {
		out.Details = Details{}
		out.Details.merge(a.Details)
	}
	if other == nil {
		return out
	}
	b := other.Snapshot()
	out.Tokens.Add(b.Tokens)
	out.Duration += b.Duration
	if out.TimeToFirstToken == 0 || (b.TimeToFirstToken > 0 && b.TimeToFirstToken < out.TimeToFirstToken) {
		out.TimeToFirstToken = b.TimeToFirstToken
	}
	if len(b.Details) > 0 {
		if out.Details == nil {
			out.Details = Details{}
		}
		out.Details.merge(b.Details)
	}
	out.AdditionalMetrics = mergeNumeric(out.AdditionalMetrics, b.AdditionalMetrics)
	return out
}

// Snapshot returns a deep copy safe to read without locking.
func (r *RunMetrics) Snapshot() *RunMetrics {
	if r == nil {
		return &RunMetrics{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := &RunMetrics{Tokens: r.Tokens, TimeToFirstToken: r.TimeToFirstToken, Duration: r.Duration,
		AdditionalMetrics: copyMap(r.AdditionalMetrics)}
	out.Cost = addCost(r.Cost, nil)
	if r.Details != nil {
		out.Details = Details{}
		for k, list := range r.Details {
			cp := make([]ModelMetrics, len(list))
			for i, mm := range list {
				cp[i] = mm.clone()
			}
			out.Details[k] = cp
		}
	}
	return out
}

// ToMap renders the metrics in the dictionary shape of Python's
// RunMetrics.to_dict (zero values dropped, durations in seconds).
func (r *RunMetrics) ToMap() map[string]interface{} {
	s := r.Snapshot()
	m := tokensMap(s.Tokens)
	if s.Duration > 0 {
		m["duration"] = s.Duration.Seconds()
	}
	if s.TimeToFirstToken > 0 {
		m["time_to_first_token"] = s.TimeToFirstToken.Seconds()
	}
	if len(s.Details) > 0 {
		d := make(map[string]interface{}, len(s.Details))
		for mt, list := range s.Details {
			entries := make([]map[string]interface{}, 0, len(list))
			for _, mm := range list {
				e := tokensMap(mm.Tokens)
				if mm.ID != "" {
					e["id"] = mm.ID
				}
				if mm.Provider != "" {
					e["provider"] = mm.Provider
				}
				if len(mm.ProviderMetrics) > 0 {
					e["provider_metrics"] = mm.ProviderMetrics
				}
				entries = append(entries, e)
			}
			d[mt] = entries
		}
		m["details"] = d
	}
	if len(s.AdditionalMetrics) > 0 {
		m["additional_metrics"] = s.AdditionalMetrics
	}
	return m
}

func tokensMap(t Tokens) map[string]interface{} {
	m := map[string]interface{}{}
	put := func(k string, v int) {
		if v != 0 {
			m[k] = v
		}
	}
	put("input_tokens", t.InputTokens)
	put("output_tokens", t.OutputTokens)
	put("total_tokens", t.TotalTokens)
	put("audio_input_tokens", t.AudioInputTokens)
	put("audio_output_tokens", t.AudioOutputTokens)
	put("audio_total_tokens", t.AudioTotalTokens)
	put("cache_read_tokens", t.CacheReadTokens)
	put("cache_write_tokens", t.CacheWriteTokens)
	put("reasoning_tokens", t.ReasoningTokens)
	if t.Cost != nil {
		m["cost"] = *t.Cost
	}
	return m
}

// RunMetricsFromMap rebuilds RunMetrics from a ToMap/Python to_dict map.
func RunMetricsFromMap(m map[string]interface{}) (*RunMetrics, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Tokens
		Duration          float64                `json:"duration"`
		TimeToFirstToken  float64                `json:"time_to_first_token"`
		Details           Details                `json:"details"`
		AdditionalMetrics map[string]interface{} `json:"additional_metrics"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &RunMetrics{
		Tokens:            raw.Tokens,
		Duration:          time.Duration(raw.Duration * float64(time.Second)),
		TimeToFirstToken:  time.Duration(raw.TimeToFirstToken * float64(time.Second)),
		Details:           raw.Details,
		AdditionalMetrics: raw.AdditionalMetrics,
	}, nil
}

// SessionMetrics aggregate run metrics across a session.
type SessionMetrics struct {
	Tokens
	Details           Details                `json:"details,omitempty"`
	AdditionalMetrics map[string]interface{} `json:"additional_metrics,omitempty"`
}

// AccumulateFromRun adds a run's metrics to the session totals.
func (s *SessionMetrics) AccumulateFromRun(run *RunMetrics) {
	if run == nil {
		return
	}
	snap := run.Snapshot()
	s.Tokens.Add(snap.Tokens)
	s.AdditionalMetrics = mergeNumeric(s.AdditionalMetrics, snap.AdditionalMetrics)
	if len(snap.Details) > 0 {
		if s.Details == nil {
			s.Details = Details{}
		}
		s.Details.merge(snap.Details)
	}
}
