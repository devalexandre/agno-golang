package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultToolsConfig controls which default tools are enabled
type DefaultToolsConfig struct {
	EnableReadChatHistory     bool
	EnableUpdateKnowledge     bool
	EnableReadToolCallHistory bool
}

// CreateDefaultTools creates the default tools based on config
func CreateDefaultTools(agent *Agent, config DefaultToolsConfig) []toolkit.Tool {
	var tools []toolkit.Tool

	if config.EnableReadChatHistory {
		if tool := NewReadChatHistoryTool(agent); tool != nil {
			tools = append(tools, tool)
		}
	}

	if config.EnableUpdateKnowledge {
		if tool := NewUpdateKnowledgeTool(agent); tool != nil {
			tools = append(tools, tool)
		}
	}

	if config.EnableReadToolCallHistory {
		if tool := NewReadToolCallHistoryTool(agent); tool != nil {
			tools = append(tools, tool)
		}
	}

	return tools
}

// ==================== ReadChatHistory Tool ====================

// ReadChatHistoryToolkit provides tools to read conversation history
type ReadChatHistoryToolkit struct {
	toolkit.Toolkit
	agent *Agent
}

// NewReadChatHistoryTool creates a new chat history reading tool
func NewReadChatHistoryTool(agent *Agent) toolkit.Tool {
	if agent.db == nil {
		return nil
	}

	rcht := &ReadChatHistoryToolkit{
		agent: agent,
	}

	tk := toolkit.NewToolkit()
	tk.Name = "chat_history"
	tk.Description = "Read conversation history from the current session"

	tk.Register("read", rcht, rcht.ReadHistory, ReadHistoryParams{})
	tk.Register("search", rcht, rcht.SearchHistory, SearchHistoryParams{})

	rcht.Toolkit = tk
	return &tk
}

// ReadHistoryParams defines parameters for reading history
type ReadHistoryParams struct {
	Limit int `json:"limit" jsonschema:"description=Number of recent messages to return (default 10)"`
}

// ReadHistory reads recent conversation history
func (rcht *ReadChatHistoryToolkit) ReadHistory(params ReadHistoryParams) (string, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}

	runs, err := rcht.agent.db.GetRunsForSession(rcht.agent.ctx, rcht.agent.sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get chat history: %w", err)
	}

	if len(runs) == 0 {
		return "No chat history found", nil
	}

	// Get the last N runs
	start := len(runs) - params.Limit
	if start < 0 {
		start = 0
	}
	recentRuns := runs[start:]

	// Format the history
	var history strings.Builder
	history.WriteString(fmt.Sprintf("Last %d conversation turns:\n\n", len(recentRuns)))

	for i, run := range recentRuns {
		history.WriteString(fmt.Sprintf("Turn %d:\n", i+1))
		if run.UserMessage != "" {
			history.WriteString(fmt.Sprintf("User: %s\n", run.UserMessage))
		}
		if run.AgentMessage != "" {
			history.WriteString(fmt.Sprintf("Agent: %s\n", run.AgentMessage))
		}
		history.WriteString("\n")
	}

	return history.String(), nil
}

// SearchHistoryParams defines parameters for searching history
type SearchHistoryParams struct {
	Query string `json:"query" jsonschema:"required,description=Search query to find in history"`
}

// SearchHistory searches for specific content in conversation history
func (rcht *ReadChatHistoryToolkit) SearchHistory(params SearchHistoryParams) (string, error) {
	runs, err := rcht.agent.db.GetRunsForSession(rcht.agent.ctx, rcht.agent.sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get chat history: %w", err)
	}

	if len(runs) == 0 {
		return "No chat history found", nil
	}

	queryLower := strings.ToLower(params.Query)
	var matches []string

	for i, run := range runs {
		userMatch := strings.Contains(strings.ToLower(run.UserMessage), queryLower)
		agentMatch := strings.Contains(strings.ToLower(run.AgentMessage), queryLower)

		if userMatch || agentMatch {
			match := fmt.Sprintf("Turn %d:\n", i+1)
			if userMatch {
				match += fmt.Sprintf("User: %s\n", run.UserMessage)
			}
			if agentMatch {
				match += fmt.Sprintf("Agent: %s\n", run.AgentMessage)
			}
			matches = append(matches, match)
		}
	}

	if len(matches) == 0 {
		return fmt.Sprintf("No matches found for '%s'", params.Query), nil
	}

	return fmt.Sprintf("Found %d matches for '%s':\n\n%s", len(matches), params.Query, strings.Join(matches, "\n")), nil
}

// ==================== UpdateKnowledge Tool ====================

// UpdateKnowledgeToolkit provides tools to update the knowledge base
type UpdateKnowledgeToolkit struct {
	toolkit.Toolkit
	agent *Agent
}

// NewUpdateKnowledgeTool creates a new knowledge update tool
func NewUpdateKnowledgeTool(agent *Agent) toolkit.Tool {
	if agent.knowledge == nil {
		return nil
	}

	ukt := &UpdateKnowledgeToolkit{
		agent: agent,
	}

	tk := toolkit.NewToolkit()
	tk.Name = "knowledge"
	tk.Description = "Update and manage the knowledge base"

	tk.Register("add", ukt, ukt.AddKnowledge, AddKnowledgeParams{})
	tk.Register("search", ukt, ukt.SearchKnowledge, SearchKnowledgeParams{})

	ukt.Toolkit = tk
	return &tk
}

// AddKnowledgeParams defines parameters for adding knowledge
type AddKnowledgeParams struct {
	Content  string                 `json:"content" jsonschema:"required,description=Content to add to knowledge base"`
	Metadata map[string]interface{} `json:"metadata" jsonschema:"description=Optional metadata for the content"`
}

// AddKnowledge adds content to the knowledge base
func (ukt *UpdateKnowledgeToolkit) AddKnowledge(params AddKnowledgeParams) (string, error) {
	doc := document.Document{
		Content:   params.Content,
		Metadata:  params.Metadata,
		ID:        fmt.Sprintf("doc_%d", time.Now().UnixNano()),
		CreatedAt: time.Now(),
	}

	if err := ukt.agent.knowledge.LoadDocument(ukt.agent.ctx, doc); err != nil {
		return "", fmt.Errorf("failed to add knowledge: %w", err)
	}

	return fmt.Sprintf("Successfully added content to knowledge base (ID: %s)", doc.ID), nil
}

// SearchKnowledgeParams defines parameters for searching knowledge
type SearchKnowledgeParams struct {
	Query string `json:"query" jsonschema:"required,description=Search query"`
	Limit int    `json:"limit" jsonschema:"description=Maximum number of results (default 5)"`
}

// SearchKnowledge searches the knowledge base
func (ukt *UpdateKnowledgeToolkit) SearchKnowledge(params SearchKnowledgeParams) (string, error) {
	if params.Limit <= 0 {
		params.Limit = 5
	}

	results, err := ukt.agent.knowledge.Search(ukt.agent.ctx, params.Query, params.Limit)
	if err != nil {
		return "", fmt.Errorf("failed to search knowledge: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No results found for '%s'", params.Query), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d results:\n\n", len(results)))

	for i, result := range results {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Document.Content))
		if result.Score > 0 {
			output.WriteString(fmt.Sprintf("   (Relevance: %.2f)\n", result.Score))
		}
		output.WriteString("\n")
	}

	return output.String(), nil
}

// ==================== ReadToolCallHistory Tool ====================

// ReadToolCallHistoryToolkit provides tools to read tool call history
type ReadToolCallHistoryToolkit struct {
	toolkit.Toolkit
	agent *Agent
}

// NewReadToolCallHistoryTool creates a new tool call history reading tool
func NewReadToolCallHistoryTool(agent *Agent) toolkit.Tool {
	rth := &ReadToolCallHistoryToolkit{
		agent: agent,
	}

	tk := toolkit.NewToolkit()
	tk.Name = "tool_history"
	tk.Description = "Read history of tool calls made during the conversation"

	tk.Register("read", rth, rth.ReadToolHistory, ReadToolHistoryParams{})
	tk.Register("stats", rth, rth.GetToolStats, GetToolStatsParams{})

	rth.Toolkit = tk
	return &tk
}

// ReadToolHistoryParams defines parameters for reading tool history
type ReadToolHistoryParams struct {
	Limit int `json:"limit" jsonschema:"description=Number of recent tool calls to return (default 10)"`
}

// ReadToolHistory reads recent tool call history from messages
func (rth *ReadToolCallHistoryToolkit) ReadToolHistory(params ReadToolHistoryParams) (string, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}

	// Get tool calls from message history
	var toolCalls []struct {
		Index    int
		ToolName string
		Args     string
	}

	for i, msg := range rth.agent.messages {
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, struct {
					Index    int
					ToolName string
					Args     string
				}{
					Index:    i,
					ToolName: tc.Function.Name,
					Args:     tc.Function.Arguments,
				})
			}
		}
	}

	if len(toolCalls) == 0 {
		return "No tool calls found in history", nil
	}

	// Get the last N tool calls
	start := len(toolCalls) - params.Limit
	if start < 0 {
		start = 0
	}
	recentCalls := toolCalls[start:]

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Last %d tool calls:\n\n", len(recentCalls)))

	for i, tc := range recentCalls {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, tc.ToolName))

		// Try to pretty-print args
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Args), &args); err == nil {
			argsJSON, _ := json.MarshalIndent(args, "   ", "  ")
			output.WriteString(fmt.Sprintf("   Args: %s\n", string(argsJSON)))
		} else {
			output.WriteString(fmt.Sprintf("   Args: %s\n", tc.Args))
		}
		output.WriteString("\n")
	}

	return output.String(), nil
}

// GetToolStatsParams defines parameters for getting tool stats
type GetToolStatsParams struct{}

// GetToolStats returns statistics about tool usage
func (rth *ReadToolCallHistoryToolkit) GetToolStats(params GetToolStatsParams) (string, error) {
	// Count tool calls by tool name
	toolCounts := make(map[string]int)
	totalCalls := 0

	for _, msg := range rth.agent.messages {
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				toolCounts[tc.Function.Name]++
				totalCalls++
			}
		}
	}

	if totalCalls == 0 {
		return "No tool calls found", nil
	}

	var output strings.Builder
	output.WriteString("Tool Call Statistics:\n\n")
	output.WriteString(fmt.Sprintf("Total tool calls: %d\n\n", totalCalls))
	output.WriteString("Calls by tool:\n")

	for tool, count := range toolCounts {
		percentage := float64(count) / float64(totalCalls) * 100
		output.WriteString(fmt.Sprintf("  %s: %d (%.1f%%)\n", tool, count, percentage))
	}

	return output.String(), nil
}
