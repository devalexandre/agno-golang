package tracing

import (
	"context"
	"time"
)

// AgentRun describes the context of an agent/team/workflow run.
type AgentRun struct {
	AgentID    string
	AgentName  string
	TeamID     string
	TeamName   string
	WorkflowID string
	SessionID  string
	RunID      string
	UserID     string

	// Input is the user message/payload that started the run (optional).
	Input string

	// Extra carries additional attributes to attach to the span.
	Extra map[string]any
}

// Attributes renders the run context as canonical span attributes.
func (r AgentRun) Attributes() map[string]any {
	attrs := make(map[string]any, 10+len(r.Extra))
	attrs[AttrSpanKind] = r.spanKind()
	putString(attrs, AttrAgentID, r.AgentID)
	putString(attrs, AttrAgentName, r.AgentName)
	putString(attrs, AttrTeamID, r.TeamID)
	putString(attrs, AttrTeamName, r.TeamName)
	putString(attrs, AttrWorkflowID, r.WorkflowID)
	putString(attrs, AttrSessionID, r.SessionID)
	putString(attrs, AttrRunID, r.RunID)
	putString(attrs, AttrUserID, r.UserID)
	putString(attrs, AttrInput, r.Input)
	for k, v := range r.Extra {
		attrs[k] = v
	}
	return attrs
}

func (r AgentRun) spanKind() string {
	switch {
	case r.WorkflowID != "":
		return KindWorkflow
	case r.TeamID != "":
		return KindTeam
	default:
		return KindAgent
	}
}

func (r AgentRun) spanName() string {
	switch {
	case r.WorkflowID != "":
		return SpanWorkflowRun
	case r.TeamID != "":
		return SpanTeamRun
	default:
		return SpanAgentRun
	}
}

// ModelCall describes a single call to a model provider.
type ModelCall struct {
	Provider  string
	ModelID   string
	AgentID   string
	SessionID string
	RunID     string
	UserID    string

	Streaming   bool
	Temperature float64
	MaxTokens   int64

	// Input is the rendered prompt/messages payload (optional).
	Input string

	// Extra carries additional attributes to attach to the span.
	Extra map[string]any
}

// Attributes renders the model call as canonical span attributes.
func (m ModelCall) Attributes() map[string]any {
	attrs := make(map[string]any, 10+len(m.Extra))
	attrs[AttrSpanKind] = KindLLM
	putString(attrs, AttrModelProvider, m.Provider)
	putString(attrs, AttrModelID, m.ModelID)
	putString(attrs, AttrAgentID, m.AgentID)
	putString(attrs, AttrSessionID, m.SessionID)
	putString(attrs, AttrRunID, m.RunID)
	putString(attrs, AttrUserID, m.UserID)
	putString(attrs, AttrInput, m.Input)
	if m.Streaming {
		attrs[AttrModelStreaming] = true
	}
	putFloat(attrs, AttrModelTemperature, m.Temperature)
	putInt(attrs, AttrModelMaxTokens, m.MaxTokens)
	for k, v := range m.Extra {
		attrs[k] = v
	}
	return attrs
}

// ToolCall describes a single tool execution.
type ToolCall struct {
	Name      string
	CallID    string
	AgentID   string
	SessionID string
	RunID     string

	// Arguments is the serialized argument payload (optional).
	Arguments string

	// Extra carries additional attributes to attach to the span.
	Extra map[string]any
}

// Attributes renders the tool call as canonical span attributes.
func (t ToolCall) Attributes() map[string]any {
	attrs := make(map[string]any, 7+len(t.Extra))
	attrs[AttrSpanKind] = KindTool
	putString(attrs, AttrToolName, t.Name)
	putString(attrs, AttrToolCallID, t.CallID)
	putString(attrs, AttrToolArguments, t.Arguments)
	putString(attrs, AttrAgentID, t.AgentID)
	putString(attrs, AttrSessionID, t.SessionID)
	putString(attrs, AttrRunID, t.RunID)
	for k, v := range t.Extra {
		attrs[k] = v
	}
	return attrs
}

// StartAgentRun starts a span for an agent (or team/workflow) run, already
// populated with the canonical attributes. A nil tracer falls back to the
// global tracer. The caller must end the returned span, typically with
// EndAgentRun.
func StartAgentRun(ctx context.Context, t Tracer, run AgentRun) (context.Context, Span) {
	return tracerOrGlobal(t).StartSpan(ctx, run.spanName(), run.Attributes())
}

// StartModelCall starts a span for a model call, already populated with the
// canonical attributes.
func StartModelCall(ctx context.Context, t Tracer, call ModelCall) (context.Context, Span) {
	return tracerOrGlobal(t).StartSpan(ctx, SpanModelCall, call.Attributes())
}

// StartToolCall starts a span for a tool execution, already populated with the
// canonical attributes.
func StartToolCall(ctx context.Context, t Tracer, call ToolCall) (context.Context, Span) {
	return tracerOrGlobal(t).StartSpan(ctx, SpanToolCall, call.Attributes())
}

// EndAgentRun attaches the consolidated metrics (and the error, if any) to the
// run span and ends it.
//
// Identity fields are recorded as attributes, while tokens, cost, tool-call
// count and latency go through Span.AddMetric so every numeric value is
// published exactly once, with a single type.
func EndAgentRun(span Span, metrics Metrics, output string, err error) {
	if span == nil {
		return
	}
	attrs := make(map[string]any, 4)
	putString(attrs, AttrRunID, metrics.RunID)
	putString(attrs, AttrSessionID, metrics.SessionID)
	putString(attrs, AttrAgentID, metrics.AgentID)
	if metrics.Cost != 0 {
		putString(attrs, AttrCostCurrency, metrics.Currency)
	}
	SetAttributes(span, attrs)

	addMetrics(span, map[string]float64{
		AttrTokensInput:     float64(metrics.InputTokens),
		AttrTokensOutput:    float64(metrics.OutputTokens),
		AttrTokensTotal:     float64(metrics.TotalTokens),
		AttrTokensCached:    float64(metrics.CachedTokens),
		AttrTokensReasoning: float64(metrics.ReasoningTokens),
		AttrCost:            metrics.Cost,
		AttrToolCallCount:   float64(metrics.ToolCalls),
		AttrLatencyMS:       float64(metrics.Duration.Milliseconds()),
	})
	finish(span, output, err)
}

// EndModelCall attaches usage (tokens, cost, latency) to the model span and
// ends it.
func EndModelCall(span Span, usage Usage, output string, err error) {
	if span == nil {
		return
	}
	u := usage.normalized()
	if u.Cost != 0 {
		span.SetAttribute(AttrCostCurrency, u.Currency)
	}
	addMetrics(span, map[string]float64{
		AttrTokensInput:     float64(u.InputTokens),
		AttrTokensOutput:    float64(u.OutputTokens),
		AttrTokensTotal:     float64(u.TotalTokens),
		AttrTokensCached:    float64(u.CachedTokens),
		AttrTokensReasoning: float64(u.ReasoningTokens),
		AttrCost:            u.Cost,
		AttrLatencyMS:       float64(u.Duration.Milliseconds()),
	})
	finish(span, output, err)
}

// EndToolCall attaches the result (and the error, if any) to the tool span and
// ends it.
func EndToolCall(span Span, result string, duration time.Duration, err error) {
	if span == nil {
		return
	}
	if result != "" {
		span.SetAttribute(AttrToolResult, result)
	}
	addMetrics(span, map[string]float64{AttrLatencyMS: float64(duration.Milliseconds())})
	finish(span, "", err)
}

// addMetrics publishes every non-zero numeric value on the span.
func addMetrics(span Span, values map[string]float64) {
	for name, value := range values {
		if value == 0 {
			continue
		}
		span.AddMetric(name, value)
	}
}

// finish records the output/error on the span and ends it.
func finish(span Span, output string, err error) {
	if output != "" {
		span.SetAttribute(AttrOutput, output)
	}
	if err != nil {
		span.SetAttribute(AttrErrorFlag, true)
		span.SetAttribute(AttrErrorMessage, err.Error())
		span.RecordError(err)
	}
	span.End()
}
