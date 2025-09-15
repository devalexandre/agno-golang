package os

import (
	"github.com/devalexandre/agno-golang/agno/agent"
)

// Helper functions
func stringPtr(s string) *string {
	return &s
}

// FromAgent converts an agent.Agent to AgentResponse with all detailed configurations
func (ar *AgentResponse) FromAgent(agent *agent.Agent) *AgentResponse {
	if agent == nil {
		return nil
	}

	// Generate deterministic ID from agent name
	agentID := generateDeterministicID("agent", agent.GetName())
	agentName := agent.GetName()
	dbID := "default" // TODO: get actual database ID from agent

	// Build model information
	// TODO: Extract actual model information from agent
	modelResponse := &ModelResponse{
		Name:     stringPtr("gpt-4"),
		Model:    stringPtr("gpt-4"),
		Provider: stringPtr("openai"),
	}

	// Build tools information (simplified for now)
	var toolsInfo *map[string]interface{}
	// TODO: Extract tools from agent
	tools := map[string]interface{}{
		"tools": nil, // Will be populated when we have access to agent tools
	}
	toolsInfo = &tools

	// Build sessions information
	var sessionsInfo *map[string]interface{}
	sessions := map[string]interface{}{
		"session_table":           "agno_sessions",
		"add_history_to_context":  true,
	}
	sessionsInfo = &sessions

	// Build knowledge information
	var knowledgeInfo *map[string]interface{}
	knowledge := map[string]interface{}{
		"knowledge_table": "agno_knowledge",
	}
	knowledgeInfo = &knowledge

	// Build memory information
	var memoryInfo *map[string]interface{}
	memory := map[string]interface{}{
		"enable_agentic_memory": true,
		"model": map[string]interface{}{
			"name":     "OpenAIChat",
			"model":    "gpt-4",
			"provider": "OpenAI",
		},
	}
	memoryInfo = &memory

	// Build default tools information
	var defaultToolsInfo *map[string]interface{}
	defaultTools := map[string]interface{}{
		"read_chat_history": true,
	}
	defaultToolsInfo = &defaultTools

	// Build system message information
	var systemMessageInfo *map[string]interface{}
	systemMessage := map[string]interface{}{
		"description":            agent.GetName(),
		"instructions":           getAgentInstructions(agent),
		"markdown":               true,
		"add_datetime_to_context": true,
	}
	systemMessageInfo = &systemMessage

	// Build streaming information
	var streamingInfo *map[string]interface{}
	streaming := map[string]interface{}{
		"stream":                      true,
		"stream_intermediate_steps":   true,
	}
	streamingInfo = &streaming

	return &AgentResponse{
		ID:            &agentID,
		Name:          &agentName,
		DBID:          &dbID,
		Model:         modelResponse,
		Tools:         toolsInfo,
		Sessions:      sessionsInfo,
		Knowledge:     knowledgeInfo,
		Memory:        memoryInfo,
		DefaultTools:  defaultToolsInfo,
		SystemMessage: systemMessageInfo,
		Streaming:     streamingInfo,
	}
}

// FromAgentSummary creates a simplified AgentSummaryResponse
func AgentSummaryFromAgent(agent *agent.Agent) AgentSummaryResponse {
	if agent == nil {
		return AgentSummaryResponse{}
	}

	agentID := generateDeterministicID("agent", agent.GetName())
	agentName := agent.GetName()
	description := agent.GetName() // TODO: get actual description
	dbID := "agno-storage"

	return AgentSummaryResponse{
		ID:          &agentID,
		Name:        &agentName,
		Description: &description,
		DBID:        &dbID,
	}
}

// Helper functions

func getAgentInstructions(agent *agent.Agent) string {
	// TODO: Extract actual instructions from agent
	// This is a placeholder based on the Python examples
	return `Your mission is to provide comprehensive and actionable support for developers working with the Agno framework. Follow these steps to deliver high-quality assistance:

1. **Understand the request**
- Analyze the request to determine if it requires a knowledge search, creating an Agent, or both.
- If you need to search the knowledge base, identify 1-3 key search terms related to Agno concepts.
- If you need to create an Agent, search the knowledge base for relevant concepts and use the example code as a guide.
- When the user asks for an Agent, they mean an Agno Agent.
- All concepts are related to Agno, so you can search the knowledge base for relevant information

After Analysis, always start the iterative search process. No need to wait for approval from the user.

2. **Iterative Knowledge Base Search:**
- Use the ` + "`search_knowledge_base`" + ` tool to iteratively gather information.
- Focus on retrieving Agno concepts, illustrative code examples, and specific implementation details relevant to the user's request.
- Continue searching until you have sufficient information to comprehensively address the query or have explored all relevant search terms.

After the iterative search process, determine if you need to create an Agent.

3. **Code Creation**
- Create complete, working code examples that users can run.
- Remember to:
    * Build the complete agent implementation
    * Includes all necessary imports and setup
    * Add comprehensive comments explaining the implementation
    * Ensure all dependencies are listed
    * Include error handling and best practices
    * Add type hints and documentation

Key topics to cover:
- Agent architecture, levels, and capabilities.
- Knowledge base integration and memory management strategies.
- Tool creation, integration, and usage.
- Supported models and their configuration.
- Common development patterns and best practices within Agno.

Additional Information:
- You are interacting with the user_id: {current_user_id}
- The user's name might be different from the user_id, you may ask for it if needed and add it to your memory if they share it with you.`
}