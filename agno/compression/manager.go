// Package compression ports agno/compression: compressing tool-call results
// with a (usually cheaper) model to save context while keeping the facts.
//
// This differs from AgentConfig.EnableSemanticCompression, which compresses
// the agent's own prompt parts (instructions, knowledge, context) by token
// count. The Manager here targets tool results: it can compress the tool
// messages of a transcript once a count/size threshold is hit (Python's
// should_compress/compress), or compress each oversized result as it is
// produced via Wrap.
package compression

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/devalexandre/agno-golang/agno/metrics"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultCompressionPrompt is the system prompt used for compression.
const DefaultCompressionPrompt = `You are compressing tool call results to save context space while preserving critical information.

Your goal: Extract only the essential information from the tool output.

ALWAYS PRESERVE:
• Specific facts: numbers, statistics, amounts, prices, quantities, metrics
• Temporal data: dates, times, timestamps (use short format: "Oct 21 2025")
• Entities: people, companies, products, locations, organizations
• Identifiers: URLs, IDs, codes, technical identifiers, versions
• Key quotes, citations, sources (if relevant to agent's task)

COMPRESS TO ESSENTIALS:
• Descriptions: keep only key attributes
• Explanations: distill to core insight
• Lists: focus on most relevant items based on agent context
• Background: minimal context only if critical

REMOVE ENTIRELY:
• Introductions, conclusions, transitions
• Hedging language ("might", "possibly", "appears to")
• Meta-commentary ("According to", "The results show")
• Formatting artifacts (markdown, HTML, JSON structure)
• Redundant or repetitive information
• Generic background not relevant to agent's task
• Promotional language, filler words

Be concise while retaining all critical facts.`

// TokenCounter counts tokens of a transcript (optional, for TokenLimit).
type TokenCounter func(messages []models.Message) int

// Manager compresses tool results with a model.
type Manager struct {
	Model models.AgnoModelInterface
	// CompressToolResults toggles compression (default true via NewManager).
	CompressToolResults bool
	// ToolResultsLimit: compress once this many uncompressed tool results
	// exist (default 3 when TokenLimit is unset).
	ToolResultsLimit int
	// TokenLimit: compress once the transcript reaches this many tokens
	// (requires CountTokens).
	TokenLimit  int
	CountTokens TokenCounter
	// Instructions overrides DefaultCompressionPrompt.
	Instructions string
	// MinChars: Wrap only compresses results longer than this (default 2000).
	MinChars int

	mu         sync.Mutex
	stats      map[string]int
	compressed map[string]bool // tool_call_id (or content hash) already compressed
}

// NewManager builds a Manager with Python's defaults.
func NewManager(model models.AgnoModelInterface) *Manager {
	return &Manager{Model: model, CompressToolResults: true, ToolResultsLimit: 3, MinChars: 2000}
}

// Stats returns tool_results_compressed, original_size, compressed_size.
func (m *Manager) Stats() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]int{}
	for k, v := range m.stats {
		out[k] = v
	}
	return out
}

func msgKey(i int, msg models.Message) string {
	if msg.ToolCallID != nil && *msg.ToolCallID != "" {
		return *msg.ToolCallID
	}
	return fmt.Sprintf("idx:%d:%d", i, len(msg.Content))
}

func (m *Manager) isCompressed(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.compressed[key]
}

func (m *Manager) uncompressed(messages []models.Message) []int {
	var idx []int
	for i, msg := range messages {
		if msg.Role == models.TypeToolRole && !m.isCompressed(msgKey(i, msg)) {
			idx = append(idx, i)
		}
	}
	return idx
}

// ShouldCompress reports whether the transcript has hit a threshold.
func (m *Manager) ShouldCompress(messages []models.Message) bool {
	if !m.CompressToolResults {
		return false
	}
	if m.TokenLimit > 0 && m.CountTokens != nil && m.CountTokens(messages) >= m.TokenLimit {
		return true
	}
	limit := m.ToolResultsLimit
	if limit == 0 && m.TokenLimit == 0 {
		limit = 3
	}
	return limit > 0 && len(m.uncompressed(messages)) >= limit
}

// CompressText compresses one tool result. On model failure the original
// content is returned (with the error).
func (m *Manager) CompressText(ctx context.Context, toolName, content string, runMetrics *metrics.RunMetrics) (string, error) {
	if m.Model == nil {
		return content, fmt.Errorf("compression: no compression model available")
	}
	if toolName == "" {
		toolName = "unknown"
	}
	prompt := m.Instructions
	if prompt == "" {
		prompt = DefaultCompressionPrompt
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resp, err := m.Model.Invoke(ctx, []models.Message{
		{Role: models.TypeSystemRole, Content: prompt},
		{Role: models.TypeUserRole, Content: "Tool Results to Compress: Tool: " + toolName + "\n" + content + "\n"},
	})
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		if err == nil {
			err = fmt.Errorf("compression: empty response")
		}
		return content, err
	}
	if runMetrics != nil && resp.Metrics != nil {
		runMetrics.AccumulateModel(resp.Metrics, m.Model.GetID(), "", metrics.ModelTypeCompression)
	}
	m.record(len(content), len(resp.Content))
	return resp.Content, nil
}

func (m *Manager) record(orig, comp int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stats == nil {
		m.stats = map[string]int{}
	}
	m.stats["tool_results_compressed"]++
	m.stats["original_size"] += orig
	m.stats["compressed_size"] += comp
}

// Compress compresses every uncompressed tool message in place.
func (m *Manager) Compress(ctx context.Context, messages []models.Message, runMetrics *metrics.RunMetrics) {
	if !m.CompressToolResults {
		return
	}
	for _, i := range m.uncompressed(messages) {
		key := msgKey(i, messages[i])
		out, err := m.CompressText(ctx, "", messages[i].Content, runMetrics)
		if err != nil {
			log.Printf("compression: failed for tool message %d: %v", i, err)
			continue
		}
		messages[i].Content = out
		m.mu.Lock()
		if m.compressed == nil {
			m.compressed = map[string]bool{}
		}
		m.compressed[msgKey(i, messages[i])] = true
		m.compressed[key] = true
		m.mu.Unlock()
	}
}

// compressingTool compresses results longer than MinChars as they are produced.
type compressingTool struct {
	toolkit.Tool
	m       *Manager
	ctx     context.Context
	metrics func() *metrics.RunMetrics
}

// Wrap returns tool whose oversized string results are compressed. metricsFn
// (optional) returns the run's collector for compression-model usage.
func (m *Manager) Wrap(ctx context.Context, tool toolkit.Tool, metricsFn func() *metrics.RunMetrics) toolkit.Tool {
	return &compressingTool{Tool: tool, m: m, ctx: ctx, metrics: metricsFn}
}

func (c *compressingTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	result, err := c.Tool.Execute(methodName, input)
	if err != nil || !c.m.CompressToolResults {
		return result, err
	}
	var text string
	switch v := result.(type) {
	case string:
		text = v
	default:
		data, mErr := json.Marshal(v)
		if mErr != nil {
			return result, nil
		}
		text = string(data)
	}
	minChars := c.m.MinChars
	if minChars <= 0 {
		minChars = 2000
	}
	if len(text) <= minChars {
		return result, nil
	}
	var rm *metrics.RunMetrics
	if c.metrics != nil {
		rm = c.metrics()
	}
	out, cErr := c.m.CompressText(c.ctx, methodName, text, rm)
	if cErr != nil {
		return result, nil
	}
	return out, nil
}
