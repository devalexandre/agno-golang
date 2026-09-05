package tracing

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageNormalizationAndAttributes(t *testing.T) {
	u := Usage{InputTokens: 30, OutputTokens: 12, Cost: 0.5, Duration: 1500 * time.Millisecond}
	attrs := u.Attributes()

	assert.Equal(t, int64(30), attrs[AttrTokensInput])
	assert.Equal(t, int64(12), attrs[AttrTokensOutput])
	assert.Equal(t, int64(42), attrs[AttrTokensTotal], "total tokens must be derived when absent")
	assert.Equal(t, 0.5, attrs[AttrCost])
	assert.Equal(t, DefaultCurrency, attrs[AttrCostCurrency])
	assert.Equal(t, float64(1500), attrs[AttrLatencyMS])

	// Zero-valued fields are omitted.
	_, ok := attrs[AttrTokensCached]
	assert.False(t, ok)
}

func TestMetricsCollectorAggregatesRunAndSession(t *testing.T) {
	c := NewMetricsCollector()

	c.StartRun("run-1", "sess-1", "agent-1")
	c.RecordUsage("run-1", Usage{InputTokens: 100, OutputTokens: 50, Cost: 0.01})
	c.RecordUsage("run-1", Usage{InputTokens: 20, OutputTokens: 5, CachedTokens: 10, Cost: 0.002})
	c.RecordToolCall("run-1", 5*time.Millisecond, nil)
	c.RecordToolCall("run-1", 5*time.Millisecond, errors.New("failed"))
	run := c.EndRun("run-1")

	assert.Equal(t, "run-1", run.RunID)
	assert.Equal(t, "sess-1", run.SessionID)
	assert.Equal(t, "agent-1", run.AgentID)
	assert.Equal(t, int64(120), run.InputTokens)
	assert.Equal(t, int64(55), run.OutputTokens)
	assert.Equal(t, int64(175), run.TotalTokens)
	assert.Equal(t, int64(10), run.CachedTokens)
	assert.InDelta(t, 0.012, run.Cost, 1e-9)
	assert.Equal(t, int64(2), run.ModelCalls)
	assert.Equal(t, int64(2), run.ToolCalls)
	assert.Equal(t, int64(1), run.Errors)
	assert.GreaterOrEqual(t, run.Duration, time.Duration(0))

	// A second run in the same session rolls up into the session metrics.
	c.StartRun("run-2", "sess-1", "agent-1")
	c.RecordUsage("run-2", Usage{InputTokens: 10, OutputTokens: 10, Cost: 0.001})
	c.EndRun("run-2")

	session, ok := c.SessionMetrics("sess-1")
	require.True(t, ok)
	assert.Equal(t, int64(130), session.InputTokens)
	assert.Equal(t, int64(65), session.OutputTokens)
	assert.Equal(t, int64(195), session.TotalTokens)
	assert.Equal(t, int64(3), session.ModelCalls)
	assert.Equal(t, int64(2), session.ToolCalls)
	assert.InDelta(t, 0.013, session.Cost, 1e-9)

	_, ok = c.SessionMetrics("unknown")
	assert.False(t, ok)

	stored, ok := c.RunMetrics("run-1")
	require.True(t, ok)
	assert.Equal(t, run.TotalTokens, stored.TotalTokens)

	c.Reset()
	_, ok = c.RunMetrics("run-1")
	assert.False(t, ok)
}

func TestMetricsCollectorConcurrentUse(t *testing.T) {
	c := NewMetricsCollector()
	c.StartRun("run-1", "sess-1", "agent-1")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.RecordUsage("run-1", Usage{InputTokens: 1, OutputTokens: 1, Cost: 0.001})
			c.RecordToolCall("run-1", time.Millisecond, nil)
		}()
	}
	wg.Wait()

	run := c.EndRun("run-1")
	assert.Equal(t, int64(50), run.InputTokens)
	assert.Equal(t, int64(50), run.ModelCalls)
	assert.Equal(t, int64(50), run.ToolCalls)
	assert.Equal(t, int64(100), run.TotalTokens)
}

func TestMetricsAddMergesWindows(t *testing.T) {
	start := time.Now()
	a := Metrics{InputTokens: 5, Cost: 1, Currency: DefaultCurrency, StartTime: start, EndTime: start.Add(time.Second)}
	b := Metrics{InputTokens: 7, Cost: 2, ToolCalls: 3, StartTime: start.Add(-time.Second), EndTime: start.Add(2 * time.Second)}

	a.Add(b)

	assert.Equal(t, int64(12), a.InputTokens)
	assert.Equal(t, float64(3), a.Cost)
	assert.Equal(t, int64(3), a.ToolCalls)
	assert.Equal(t, start.Add(-time.Second), a.StartTime)
	assert.Equal(t, start.Add(2*time.Second), a.EndTime)
	assert.Equal(t, 3*time.Second, a.Duration)
}

func TestMetricsAttributes(t *testing.T) {
	m := Metrics{
		RunID:       "run-1",
		SessionID:   "sess-1",
		AgentID:     "agent-1",
		InputTokens: 10,
		TotalTokens: 10,
		Cost:        0.25,
		Currency:    DefaultCurrency,
		ToolCalls:   2,
		Duration:    2 * time.Second,
	}

	attrs := m.Attributes()
	assert.Equal(t, "run-1", attrs[AttrRunID])
	assert.Equal(t, "sess-1", attrs[AttrSessionID])
	assert.Equal(t, "agent-1", attrs[AttrAgentID])
	assert.Equal(t, int64(10), attrs[AttrTokensTotal])
	assert.Equal(t, 0.25, attrs[AttrCost])
	assert.Equal(t, DefaultCurrency, attrs[AttrCostCurrency])
	assert.Equal(t, int64(2), attrs[AttrToolCallCount])
	assert.Equal(t, float64(2000), attrs[AttrLatencyMS])
}
