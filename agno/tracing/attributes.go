package tracing

// Span names used by the instrumentation helpers. They mirror the span names
// emitted by agno-python so traces from both runtimes can share a backend.
const (
	SpanAgentRun    = "agent.run"
	SpanTeamRun     = "team.run"
	SpanWorkflowRun = "workflow.run"
	SpanModelCall   = "model.response"
	SpanToolCall    = "tool.call"
	SpanReasoning   = "reasoning.step"
	SpanMemory      = "memory.operation"
	SpanRetrieval   = "knowledge.retrieval"
)

// Span kinds (the agno-level kind, not the OpenTelemetry SpanKind).
const (
	KindAgent     = "agent"
	KindTeam      = "team"
	KindWorkflow  = "workflow"
	KindLLM       = "llm"
	KindTool      = "tool"
	KindReasoning = "reasoning"
	KindMemory    = "memory"
	KindRetriever = "retriever"
)

// Canonical attribute keys. Every span emitted by agno-golang must use these
// constants instead of raw strings so exporters and dashboards can rely on a
// stable schema.
const (
	// Identity / context.
	AttrSpanKind   = "agno.span.kind"
	AttrAgentID    = "agent.id"
	AttrAgentName  = "agent.name"
	AttrTeamID     = "team.id"
	AttrTeamName   = "team.name"
	AttrWorkflowID = "workflow.id"
	AttrSessionID  = "session.id"
	AttrRunID      = "run.id"
	AttrUserID     = "user.id"

	// Model.
	AttrModelProvider     = "model.provider"
	AttrModelID           = "model.id"
	AttrModelStreaming    = "model.streaming"
	AttrModelTemperature  = "model.temperature"
	AttrModelMaxTokens    = "model.max_tokens"
	AttrModelFinishReason = "model.finish_reason"

	// Tokens.
	AttrTokensInput     = "tokens.input"
	AttrTokensOutput    = "tokens.output"
	AttrTokensTotal     = "tokens.total"
	AttrTokensCached    = "tokens.cached"
	AttrTokensReasoning = "tokens.reasoning"

	// Cost.
	AttrCost         = "cost"
	AttrCostCurrency = "cost.currency"

	// Tools.
	AttrToolName      = "tool.name"
	AttrToolCallID    = "tool.call_id"
	AttrToolArguments = "tool.arguments"
	AttrToolResult    = "tool.result"
	AttrToolCallCount = "tool.call_count"

	// Payloads.
	AttrInput  = "input.value"
	AttrOutput = "output.value"

	// Timing / status.
	AttrLatencyMS    = "latency.ms"
	AttrErrorFlag    = "error"
	AttrErrorMessage = "error.message"

	// Resource attributes.
	AttrServiceName        = "service.name"
	AttrServiceVersion     = "service.version"
	AttrDeploymentEnv      = "deployment.environment"
	AttrTelemetrySDKName   = "telemetry.sdk.name"
	AttrTelemetrySDKLang   = "telemetry.sdk.language"
	AttrTelemetryFramework = "agno.framework"
)

// DefaultServiceName is used when no service name is configured.
const DefaultServiceName = "agno-golang"

// TracerName is the instrumentation scope name reported to OpenTelemetry.
const TracerName = "github.com/devalexandre/agno-golang/agno/tracing"

// putString adds key to attrs only when value is non-empty.
func putString(attrs map[string]any, key, value string) {
	if value != "" {
		attrs[key] = value
	}
}

// putInt adds key to attrs only when value is non-zero.
func putInt(attrs map[string]any, key string, value int64) {
	if value != 0 {
		attrs[key] = value
	}
}

// putFloat adds key to attrs only when value is non-zero.
func putFloat(attrs map[string]any, key string, value float64) {
	if value != 0 {
		attrs[key] = value
	}
}
