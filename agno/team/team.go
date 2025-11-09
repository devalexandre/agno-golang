package team

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/memory"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// TeamMode defines how team members collaborate
type TeamMode string

const (
	// RouteMode: Team leader routes requests to the most appropriate member
	RouteMode TeamMode = "route"

	// CoordinateMode: Team leader delegates tasks and synthesizes responses
	CoordinateMode TeamMode = "coordinate"

	// CollaborateMode: All members work on same task, leader synthesizes
	CollaborateMode TeamMode = "collaborate"
)

// TeamMember represents a member that can be either an Agent or another Team
type TeamMember interface {
	GetName() string
	GetRole() string
	Run(prompt string) (models.RunResponse, error)
	RunStream(prompt string, fn func([]byte) error) error
}

// TeamConfig holds the configuration for creating a Team
type TeamConfig struct {
	Context      context.Context
	Name         string
	Role         string
	Description  string
	Instructions []string
	Model        models.AgnoModelInterface // Team leader model
	Members      []TeamMember
	Mode         TeamMode
	Tools        []toolkit.Tool

	// Memory Configuration
	Memory                 memory.MemoryManager
	SessionID              string
	UserID                 string
	AddHistoryToMessages   bool
	NumHistoryRuns         int
	EnableUserMemories     bool
	EnableAgenticMemory    bool
	EnableSessionSummaries bool
	ReadChatHistory        bool

	// Storage Configuration (new)
	Storage storage.Storage

	// Display Options
	ShowMembersResponses bool
	ShowToolCalls        bool
	Markdown             bool
	Debug                bool
	Stream               bool
	Async                bool // Execute members concurrently when possible
}

// Team represents a multi-agent system
type Team struct {
	ctx          context.Context
	name         string
	role         string
	description  string
	instructions []string
	model        models.AgnoModelInterface
	members      []TeamMember
	mode         TeamMode
	tools        []toolkit.Tool

	// Memory
	memory                 memory.MemoryManager
	sessionID              string
	userID                 string
	addHistoryToMessages   bool
	numHistoryRuns         int
	enableUserMemories     bool
	enableAgenticMemory    bool
	enableSessionSummaries bool
	readChatHistory        bool

	// Storage
	storage storage.Storage

	// Display Options
	showMembersResponses bool
	showToolCalls        bool
	markdown             bool
	debug                bool
	stream               bool
	async                bool

	// Session state
	messages []models.Message
}

// NewTeam creates a new Team instance
func NewTeam(config TeamConfig) *Team {
	// Set default mode if not specified
	if config.Mode == "" {
		config.Mode = CoordinateMode
	}

	// Set context values for debugging
	config.Context = context.WithValue(config.Context, models.DebugKey, config.Debug)
	config.Context = context.WithValue(config.Context, models.ShowToolsCallKey, config.ShowToolCalls)

	team := &Team{
		ctx:          config.Context,
		name:         config.Name,
		role:         config.Role,
		description:  config.Description,
		instructions: config.Instructions,
		model:        config.Model,
		members:      config.Members,
		mode:         config.Mode,
		tools:        config.Tools,

		// Memory
		memory:                 config.Memory,
		sessionID:              config.SessionID,
		userID:                 config.UserID,
		addHistoryToMessages:   config.AddHistoryToMessages,
		numHistoryRuns:         config.NumHistoryRuns,
		enableUserMemories:     config.EnableUserMemories,
		enableAgenticMemory:    config.EnableAgenticMemory,
		enableSessionSummaries: config.EnableSessionSummaries,
		readChatHistory:        config.ReadChatHistory,

		// Storage
		storage: config.Storage,

		// Display Options
		showMembersResponses: config.ShowMembersResponses,
		showToolCalls:        config.ShowToolCalls,
		markdown:             config.Markdown,
		debug:                config.Debug,
		stream:               config.Stream,
		async:                config.Async,

		// Initialize session state
		messages: []models.Message{},
	}

	// Load existing session if memory or storage is provided
	if (team.memory != nil || team.storage != nil) && team.sessionID != "" {
		team.loadSession()
	}

	return team
}

// GetName returns the team name
func (t *Team) GetName() string {
	return t.name
}

// GetRole returns the team role
func (t *Team) GetRole() string {
	return t.role
}

// Run executes a task using the team
func (t *Team) Run(prompt string) (models.RunResponse, error) {
	var response models.RunResponse
	var err error

	switch t.mode {
	case RouteMode:
		response, err = t.runRouteMode(prompt)
	case CoordinateMode:
		response, err = t.runCoordinateMode(prompt)
	case CollaborateMode:
		response, err = t.runCollaborateMode(prompt)
	default:
		response, err = t.runCoordinateMode(prompt) // Default to coordinate
	}

	// Save to memory and/or storage if successful
	if err == nil && (t.memory != nil || t.storage != nil) {
		t.saveToMemory(prompt, response)
	}

	return response, err
}

// runRouteMode routes the request to the most appropriate team member
func (t *Team) runRouteMode(prompt string) (models.RunResponse, error) {
	// Step 1: Use team leader to decide which member should handle the request
	routingPrompt := t.buildRoutingPrompt(prompt)

	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: routingPrompt,
		},
		{
			Role:    models.TypeUserRole,
			Content: prompt,
		},
	}

	// Get routing decision from team leader
	resp, err := t.model.Invoke(t.ctx, messages)
	if err != nil {
		return models.RunResponse{}, err
	}

	// Step 2: Execute the selected member(s)
	// For now, let's route to the first member (simplified implementation)
	if len(t.members) > 0 {
		memberResponse, err := t.members[0].Run(prompt)
		if err != nil {
			return models.RunResponse{}, err
		}

		return models.RunResponse{
			TextContent: memberResponse.TextContent,
			ContentType: memberResponse.ContentType,
			Event:       "TeamRouteResponse",
			Messages:    memberResponse.Messages,
			Model:       resp.Model,
			CreatedAt:   time.Now().Unix(),
		}, nil
	}

	return models.RunResponse{
		TextContent: "No team members available",
		ContentType: "text",
		Event:       "TeamError",
		CreatedAt:   time.Now().Unix(),
	}, nil
}

// runCoordinateMode delegates tasks to members and synthesizes their outputs
func (t *Team) runCoordinateMode(prompt string) (models.RunResponse, error) {
	// Step 1: Plan the delegation
	planPrompt := t.buildCoordinationPrompt(prompt)

	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: planPrompt,
		},
		{
			Role:    models.TypeUserRole,
			Content: prompt,
		},
	}

	// Get coordination plan from team leader
	_, err := t.model.Invoke(t.ctx, messages)
	if err != nil {
		return models.RunResponse{}, err
	}

	// Step 2: Execute members (simplified - execute all for now)
	memberResponses := []string{}

	for i, member := range t.members {
		memberResp, err := member.Run(prompt)
		if err != nil {
			if t.debug {
				memberResponses = append(memberResponses, fmt.Sprintf("Member %d (%s) error: %v", i+1, member.GetName(), err))
			}
			continue
		}

		if t.showMembersResponses {
			memberResponses = append(memberResponses, fmt.Sprintf("**%s Response:**\n%s", member.GetName(), memberResp.TextContent))
		} else {
			memberResponses = append(memberResponses, memberResp.TextContent)
		}
	}

	// Step 3: Synthesize responses
	synthesisPrompt := t.buildSynthesisPrompt(prompt, memberResponses)

	synthesisMessages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: synthesisPrompt,
		},
		{
			Role:    models.TypeUserRole,
			Content: fmt.Sprintf("Original request: %s\n\nPlease synthesize the following responses into a cohesive answer:\n\n%s", prompt, strings.Join(memberResponses, "\n\n---\n\n")),
		},
	}

	finalResp, err := t.model.Invoke(t.ctx, synthesisMessages)
	if err != nil {
		return models.RunResponse{}, err
	}

	return models.RunResponse{
		TextContent: finalResp.Content,
		ContentType: "text",
		Event:       "TeamCoordinateResponse",
		Messages: []models.Message{
			{
				Role:    models.Role(finalResp.Role),
				Content: finalResp.Content,
			},
		},
		Model:     finalResp.Model,
		CreatedAt: time.Now().Unix(),
	}, nil
}

// runCollaborateMode gives all members the same task and synthesizes their outputs
func (t *Team) runCollaborateMode(prompt string) (models.RunResponse, error) {
	// Step 1: Execute all members with the same task
	memberResponses := []string{}

	// Execute members concurrently if async is enabled
	if t.async {
		return t.runCollaborateModeAsync(prompt)
	}

	// Sequential execution
	for i, member := range t.members {
		memberResp, err := member.Run(prompt)
		if err != nil {
			if t.debug {
				memberResponses = append(memberResponses, fmt.Sprintf("Member %d (%s) error: %v", i+1, member.GetName(), err))
			}
			continue
		}

		if t.showMembersResponses {
			memberResponses = append(memberResponses, fmt.Sprintf("**%s Response:**\n%s", member.GetName(), memberResp.TextContent))
		} else {
			memberResponses = append(memberResponses, memberResp.TextContent)
		}
	}

	// Step 2: Detect and resolve conflicts if needed
	hasConflicts, conflictAnalysis := t.detectConflicts(memberResponses)

	var finalContent string
	if hasConflicts {
		// Resolve conflicts before synthesis
		resolvedContent, err := t.resolveConflicts(prompt, memberResponses, conflictAnalysis)
		if err != nil {
			if t.debug {
				fmt.Printf("Conflict resolution failed, proceeding with standard synthesis: %v\n", err)
			}
			// Fall back to standard synthesis
			finalContent, err = t.synthesizeResponses(prompt, memberResponses, t.buildCollaborationSynthesisPrompt(prompt))
			if err != nil {
				return models.RunResponse{}, err
			}
		} else {
			finalContent = resolvedContent
		}
	} else {
		// No conflicts, proceed with standard synthesis
		var err error
		finalContent, err = t.synthesizeResponses(prompt, memberResponses, t.buildCollaborationSynthesisPrompt(prompt))
		if err != nil {
			return models.RunResponse{}, err
		}
	}

	return models.RunResponse{
		TextContent: finalContent,
		ContentType: "text",
		Event:       "TeamCollaborateResponse",
		Messages: []models.Message{
			{
				Role:    models.TypeAssistantRole,
				Content: finalContent,
			},
		},
		Model:     t.model.GetID(),
		CreatedAt: time.Now().Unix(),
	}, nil
}

// RunStream executes a task using the team with streaming response
func (t *Team) RunStream(prompt string, fn func([]byte) error) error {
	// For now, convert to regular Run and stream the final result
	response, err := t.Run(prompt)
	if err != nil {
		return err
	}

	// Stream the response content in chunks
	content := response.TextContent
	chunkSize := 50 // characters per chunk

	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}

		chunk := content[i:end]
		if err := fn([]byte(chunk)); err != nil {
			return err
		}

		// Small delay to simulate streaming
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// Helper methods for building prompts

// buildRoutingPrompt creates a prompt for routing decisions
func (t *Team) buildRoutingPrompt(prompt string) string {
	membersInfo := ""
	for i, member := range t.members {
		membersInfo += fmt.Sprintf("%d. %s - %s\n", i+1, member.GetName(), member.GetRole())
	}

	routingPrompt := fmt.Sprintf(`You are a team leader responsible for routing user requests to the most appropriate team member.

Team Members:
%s

Your task is to analyze the user's request and determine which team member is best suited to handle it.

Instructions:
- Read the user's request carefully
- Consider each team member's role and expertise
- Route the request to the member who can best address the user's needs
- If multiple members could help, choose the most specialized one
- Respond with the member's name and a brief explanation of why they were chosen

Team Description: %s
Team Instructions: %s`, membersInfo, t.description, strings.Join(t.instructions, "\n"))

	return routingPrompt
}

// buildCoordinationPrompt creates a prompt for coordination planning
func (t *Team) buildCoordinationPrompt(prompt string) string {
	membersInfo := ""
	for i, member := range t.members {
		membersInfo += fmt.Sprintf("%d. %s - %s\n", i+1, member.GetName(), member.GetRole())
	}

	coordinationPrompt := fmt.Sprintf(`You are a team leader responsible for coordinating team members to complete complex tasks.

Team Members:
%s

Your task is to create a coordination plan that assigns specific sub-tasks to appropriate team members.

Instructions:
- Break down the user's request into sub-tasks if needed
- Assign tasks to team members based on their expertise
- Consider dependencies between tasks
- Plan the sequence of execution
- Ensure all aspects of the request are covered

Team Description: %s
Team Instructions: %s`, membersInfo, t.description, strings.Join(t.instructions, "\n"))

	return coordinationPrompt
}

// buildSynthesisPrompt creates a prompt for synthesizing member responses
func (t *Team) buildSynthesisPrompt(prompt string, memberResponses []string) string {
	synthesisPrompt := fmt.Sprintf(`You are a team leader responsible for synthesizing team member responses into a cohesive final answer.

Original User Request: %s

Your task is to:
- Review all team member responses
- Identify key insights and information
- Resolve any conflicts or contradictions
- Create a comprehensive, well-structured response
- Ensure the final answer directly addresses the user's request

Team Description: %s
Team Instructions: %s

Guidelines:
- Combine the best elements from each response
- Maintain consistency in tone and style
- Include all relevant information
- Present the information in a logical flow
- Cite team members when appropriate`, prompt, t.description, strings.Join(t.instructions, "\n"))

	return synthesisPrompt
}

// buildCollaborationSynthesisPrompt creates a prompt for synthesizing collaborative responses
func (t *Team) buildCollaborationSynthesisPrompt(prompt string) string {
	synthesisPrompt := fmt.Sprintf(`You are a team leader responsible for synthesizing collaborative responses from team members who all worked on the same task.

Original User Request: %s

Your task is to:
- Compare and contrast the different approaches taken by team members
- Identify areas of agreement and disagreement
- Highlight the most valuable insights from each response
- Create a balanced, comprehensive final answer
- Leverage the diversity of perspectives to provide a richer response

Team Description: %s
Team Instructions: %s

Guidelines:
- Show how different perspectives complement each other
- Address any contradictions with balanced analysis
- Highlight unique contributions from each member
- Create a response that's better than any individual response
- Maintain objectivity while leveraging all viewpoints`, prompt, t.description, strings.Join(t.instructions, "\n"))

	return synthesisPrompt
}

// runCollaborateModeAsync executes collaboration mode with concurrent member execution
func (t *Team) runCollaborateModeAsync(prompt string) (models.RunResponse, error) {
	// Channel to collect member responses
	type memberResult struct {
		response string
		name     string
		err      error
	}

	results := make(chan memberResult, len(t.members))

	// Execute all members concurrently
	for _, member := range t.members {
		go func(m TeamMember) {
			resp, err := m.Run(prompt)
			if err != nil {
				results <- memberResult{err: err, name: m.GetName()}
				return
			}

			if t.showMembersResponses {
				results <- memberResult{
					response: fmt.Sprintf("**%s Response:**\n%s", m.GetName(), resp.TextContent),
					name:     m.GetName(),
				}
			} else {
				results <- memberResult{
					response: resp.TextContent,
					name:     m.GetName(),
				}
			}
		}(member)
	}

	// Collect all responses
	memberResponses := []string{}
	for i := 0; i < len(t.members); i++ {
		result := <-results
		if result.err != nil {
			if t.debug {
				memberResponses = append(memberResponses, fmt.Sprintf("Member %s error: %v", result.name, result.err))
			}
			continue
		}
		memberResponses = append(memberResponses, result.response)
	}

	// Synthesize responses
	synthesisPrompt := t.buildCollaborationSynthesisPrompt(prompt)

	synthesisMessages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: synthesisPrompt,
		},
		{
			Role:    models.TypeUserRole,
			Content: fmt.Sprintf("Original request: %s\n\nPlease synthesize the following collaborative responses:\n\n%s", prompt, strings.Join(memberResponses, "\n\n---\n\n")),
		},
	}

	finalResp, err := t.model.Invoke(t.ctx, synthesisMessages)
	if err != nil {
		return models.RunResponse{}, err
	}

	return models.RunResponse{
		TextContent: finalResp.Content,
		ContentType: "text",
		Event:       "TeamCollaborateAsyncResponse",
		Messages: []models.Message{
			{
				Role:    models.Role(finalResp.Role),
				Content: finalResp.Content,
			},
		},
		Model:     finalResp.Model,
		CreatedAt: time.Now().Unix(),
	}, nil
}

// loadSession loads existing session messages from memory and storage
func (t *Team) loadSession() {
	// Load from memory (session summaries)
	if t.memory != nil && t.sessionID != "" && t.readChatHistory {
		if summary, err := t.memory.GetSessionSummary(context.Background(), t.userID, t.sessionID); err == nil && summary != nil {
			// Add the session summary as system context
			t.messages = append(t.messages, models.Message{
				Role:    models.TypeSystemRole,
				Content: fmt.Sprintf("Previous session summary: %s", summary.Summary),
			})

			if t.debug {
				fmt.Printf("Loaded session summary for session %s\n", t.sessionID)
			}
		} else if t.debug && err != nil {
			fmt.Printf("No session summary found for session %s: %v\n", t.sessionID, err)
		}
	}

	// Load from storage (team session data)
	if t.storage != nil && t.sessionID != "" {
		if sessionData, err := t.storage.Read(t.sessionID, &t.userID); err == nil && sessionData != nil {
			if teamSession, ok := sessionData.(*storage.TeamSession); ok {
				// Restore team state from storage
				if teamData, ok := teamSession.TeamData["messages"]; ok {
					if messagesList, ok := teamData.([]interface{}); ok {
						for _, msgData := range messagesList {
							if msgMap, ok := msgData.(map[string]interface{}); ok {
								role := getStringFromMap(msgMap, "role")
								content := getStringFromMap(msgMap, "content")
								if role != "" && content != "" {
									t.messages = append(t.messages, models.Message{
										Role:    models.Role(role),
										Content: content,
									})
								}
							}
						}
					}
				}

				if t.debug {
					fmt.Printf("Loaded team session data for session %s\n", t.sessionID)
				}
			}
		} else if t.debug && err != nil {
			fmt.Printf("No team session found for session %s: %v\n", t.sessionID, err)
		}
	}
}

// detectConflicts analyzes member responses to identify conflicts
func (t *Team) detectConflicts(responses []string) (bool, string) {
	if len(responses) < 2 {
		return false, ""
	}

	// Use AI to detect conflicts
	analysisPrompt := fmt.Sprintf(`Analyze the following responses from team members and identify any conflicts or contradictions.

Responses:
%s

Identify:
1. Direct contradictions (members saying opposite things)
2. Inconsistent recommendations
3. Conflicting data or facts
4. Different approaches that cannot coexist

If conflicts exist, respond with "CONFLICTS DETECTED" followed by a detailed analysis.
If no significant conflicts exist, respond with "NO CONFLICTS".`, strings.Join(responses, "\n\n---\n\n"))

	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: "You are a conflict detection specialist. Analyze team responses for contradictions and conflicts.",
		},
		{
			Role:    models.TypeUserRole,
			Content: analysisPrompt,
		},
	}

	resp, err := t.model.Invoke(t.ctx, messages)
	if err != nil {
		if t.debug {
			fmt.Printf("Conflict detection failed: %v\n", err)
		}
		return false, ""
	}

	analysis := strings.TrimSpace(resp.Content)
	hasConflicts := strings.Contains(strings.ToUpper(analysis), "CONFLICTS DETECTED")

	return hasConflicts, analysis
}

// resolveConflicts uses AI to resolve conflicts between member responses
func (t *Team) resolveConflicts(prompt string, responses []string, conflictAnalysis string) (string, error) {
	resolutionPrompt := fmt.Sprintf(`You are a conflict resolution specialist for a team of AI agents.

Original Request: %s

Team Member Responses:
%s

Conflict Analysis:
%s

Your task is to resolve these conflicts and provide a unified, coherent response that:
1. Acknowledges the different perspectives
2. Identifies the most accurate or appropriate information
3. Reconciles contradictions with logical reasoning
4. Provides a balanced final answer
5. Explains the resolution approach when necessary

Provide a clear, unified response that addresses the original request while resolving all conflicts.`,
		prompt,
		strings.Join(responses, "\n\n---\n\n"),
		conflictAnalysis)

	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: "You are an expert at resolving conflicts and synthesizing diverse viewpoints into coherent solutions.",
		},
		{
			Role:    models.TypeUserRole,
			Content: resolutionPrompt,
		},
	}

	resp, err := t.model.Invoke(t.ctx, messages)
	if err != nil {
		return "", fmt.Errorf("conflict resolution failed: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

// synthesizeResponses performs standard synthesis without conflict resolution
func (t *Team) synthesizeResponses(prompt string, responses []string, synthesisPrompt string) (string, error) {
	synthesisMessages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: synthesisPrompt,
		},
		{
			Role:    models.TypeUserRole,
			Content: fmt.Sprintf("Original request: %s\n\nPlease synthesize the following collaborative responses:\n\n%s", prompt, strings.Join(responses, "\n\n---\n\n")),
		},
	}

	finalResp, err := t.model.Invoke(t.ctx, synthesisMessages)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(finalResp.Content), nil
}

// Helper functions for type conversion
func getStringFromMap(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// saveToMemory saves the current interaction to memory and storage
func (t *Team) saveToMemory(prompt string, response models.RunResponse) {
	// Save to memory system
	if t.memory != nil && t.sessionID != "" {
		// Create memory from the interaction if enabled
		if t.enableUserMemories {
			if _, err := t.memory.CreateMemory(context.Background(), t.userID, prompt, response.TextContent); err != nil && t.debug {
				fmt.Printf("Failed to create user memory: %v\n", err)
			}
		}

		// Update session summary if enabled
		if t.enableSessionSummaries {
			// Convert current messages to the format expected by CreateSessionSummary
			messagesMap := make([]map[string]interface{}, 0, len(t.messages)+2)

			// Add existing messages
			for _, msg := range t.messages {
				messagesMap = append(messagesMap, map[string]interface{}{
					"role":    string(msg.Role),
					"content": msg.Content,
				})
			}

			// Add current interaction
			messagesMap = append(messagesMap, map[string]interface{}{
				"role":    string(models.TypeUserRole),
				"content": prompt,
			})
			messagesMap = append(messagesMap, map[string]interface{}{
				"role":    string(models.TypeAssistantRole),
				"content": response.TextContent,
			})

			if _, err := t.memory.CreateSessionSummary(context.Background(), t.userID, t.sessionID, messagesMap); err != nil && t.debug {
				fmt.Printf("Failed to create session summary: %v\n", err)
			} else if t.debug {
				fmt.Printf("Session summary updated for session %s\n", t.sessionID)
			}
		}
	}

	// Save to storage system
	if t.storage != nil && t.sessionID != "" {
		// Add current interaction to messages
		t.messages = append(t.messages,
			models.Message{
				Role:    models.TypeUserRole,
				Content: prompt,
			},
			models.Message{
				Role:    models.TypeAssistantRole,
				Content: response.TextContent,
			},
		)

		// Create or update team session
		teamSession := &storage.TeamSession{
			Session: storage.Session{
				SessionID:   t.sessionID,
				UserID:      t.userID,
				Memory:      make(map[string]interface{}),
				SessionData: make(map[string]interface{}),
				ExtraData:   make(map[string]interface{}),
				CreatedAt:   time.Now().Unix(),
				UpdatedAt:   time.Now().Unix(),
			},
			TeamID:   t.name, // Use team name as team ID
			TeamData: make(map[string]interface{}),
		}

		// Store messages in team data
		messagesData := make([]map[string]interface{}, len(t.messages))
		for i, msg := range t.messages {
			messagesData[i] = map[string]interface{}{
				"role":    string(msg.Role),
				"content": msg.Content,
			}
		}
		teamSession.TeamData["messages"] = messagesData
		teamSession.TeamData["team_name"] = t.name
		teamSession.TeamData["team_role"] = t.role
		teamSession.TeamData["team_mode"] = string(t.mode)

		// Store team configuration
		teamSession.SessionData["mode"] = string(t.mode)
		teamSession.SessionData["member_count"] = len(t.members)
		teamSession.SessionData["last_interaction"] = time.Now().Unix()

		// Upsert the session
		if _, err := t.storage.Upsert(teamSession); err != nil && t.debug {
			fmt.Printf("Failed to save team session: %v\n", err)
		} else if t.debug {
			fmt.Printf("Team session saved for session %s\n", t.sessionID)
		}
	}
}
