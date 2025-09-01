package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/memory"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/reasoning"
	"github.com/pterm/pterm"

	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
	"github.com/google/uuid"
)

type AgentConfig struct {
	Context        context.Context
	Model          models.AgnoModelInterface
	Name           string
	Role           string
	Description    string
	Goal           string
	Instructions   string
	ContextData    map[string]interface{}
	ExpectedOutput string
	Tools          []toolkit.Tool
	Stream         bool
	Markdown       bool
	ShowToolsCall  bool
	Debug          bool
	//--- Agent Reasoning ---
	// Enable reasoning by working through the problem step by step.
	Reasoning         bool
	ReasoningModel    models.AgnoModelInterface
	ReasoningAgent    models.AgentInterface
	ReasoningMinSteps int
	ReasoningMaxSteps int

	// Memory and Storage Configuration
	Memory                 memory.MemoryManager
	Storage                storage.AgentStorage
	SessionID              string
	UserID                 string
	AddHistoryToMessages   bool
	NumHistoryRuns         int
	EnableUserMemories     bool
	EnableAgenticMemory    bool
	EnableSessionSummaries bool
	ReadChatHistory        bool

	//knowledge
	Knowledge knowledge.Knowledge
}

type Agent struct {
	ctx                    context.Context
	model                  models.AgnoModelInterface
	name                   string
	role                   string
	description            string
	goal                   string
	instructions           string
	additional_information []string
	contextData            map[string]interface{}
	expected_output        string
	tools                  []toolkit.Tool
	stream                 bool
	markdown               bool
	showToolsCall          bool
	debug                  bool

	// Memory and Storage
	memory                 memory.MemoryManager
	storage                storage.AgentStorage
	sessionID              string
	userID                 string
	addHistoryToMessages   bool
	numHistoryRuns         int
	enableUserMemories     bool
	enableAgenticMemory    bool
	enableSessionSummaries bool
	readChatHistory        bool

	// Session state
	messages []models.Message
	runs     []*storage.AgentRun

	// Knowledge
	knowledge knowledge.Knowledge

	// Reasoning
	reasoning         bool
	reasoningModel    models.AgnoModelInterface
	reasoningAgent    models.AgentInterface
	reasoningMinSteps int
	reasoningMaxSteps int
}

func NewAgent(config AgentConfig) (*Agent, error) {
	config.Context = context.WithValue(config.Context, models.DebugKey, config.Debug)
	config.Context = context.WithValue(config.Context, models.ShowToolsCallKey, config.ShowToolsCall)
	if config.Reasoning {
		config.Context = context.WithValue(config.Context, "reasoning", true)
	}

	// Generate session ID if not provided
	sessionID := config.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	//set mim and max for steps
	if config.ReasoningMinSteps <= 0 {
		config.ReasoningMinSteps = 1
	}
	if config.ReasoningMaxSteps <= 0 {
		config.ReasoningMaxSteps = 3
	}

	agent := &Agent{
		ctx:             config.Context,
		model:           config.Model,
		name:            config.Name,
		role:            config.Role,
		description:     config.Description,
		goal:            config.Goal,
		instructions:    config.Instructions,
		expected_output: config.ExpectedOutput,
		contextData:     config.ContextData,
		tools:           config.Tools,
		stream:          config.Stream,
		markdown:        config.Markdown,
		showToolsCall:   config.ShowToolsCall,
		debug:           config.Debug,

		// Memory and Storage
		memory:                 config.Memory,
		storage:                config.Storage,
		sessionID:              sessionID,
		userID:                 config.UserID,
		addHistoryToMessages:   config.AddHistoryToMessages,
		numHistoryRuns:         config.NumHistoryRuns,
		enableUserMemories:     config.EnableUserMemories,
		enableAgenticMemory:    config.EnableAgenticMemory,
		enableSessionSummaries: config.EnableSessionSummaries,
		readChatHistory:        config.ReadChatHistory,

		// Initialize session state
		messages: []models.Message{},
		runs:     []*storage.AgentRun{},

		//knowledge
		knowledge: config.Knowledge,

		// Reasoning
		reasoning:         config.Reasoning,
		reasoningModel:    config.ReasoningModel,
		reasoningAgent:    config.ReasoningAgent,
		reasoningMinSteps: config.ReasoningMinSteps,
		reasoningMaxSteps: config.ReasoningMaxSteps,
	}

	// Load existing session if storage is provided
	if agent.storage != nil {
		agent.loadSession()
	}

	return agent, nil
}

// GetName returns the agent's name (implements TeamMember interface)
func (a *Agent) GetName() string {
	if a.name != "" {
		return a.name
	}
	return "Agent"
}

// GetRole returns the agent's role (implements TeamMember interface)
func (a *Agent) GetRole() string {
	if a.role != "" {
		return a.role
	}
	return "Assistant"
}

func (a *Agent) Run(prompt string) (models.RunResponse, error) {
	var messages []models.Message

	// Add system message and history normally
	baseMessages := a.prepareMessages(prompt)
	for _, msg := range baseMessages {
		if msg.Role == models.TypeUserRole {
			messages = append(messages, msg)
		} else {
			messages = append([]models.Message{msg}, messages...)
		}
	}

	//hide if reasoning
	var bx *pterm.SpinnerPrinter
	if !a.reasoning {
		bx = utils.ThinkingPanel(prompt)
	}

	// use default reasoning agent
	if a.reasoningAgent == nil && a.reasoning && a.reasoningModel != nil {
		a.reasoningAgent = NewReasoningAgent(a.ctx, a.reasoningModel, a.tools, a.reasoningMinSteps, a.reasoningMaxSteps)
	}
	// Reasoning: insert each step as "assistant" message before user prompt
	if a.reasoning && a.reasoningModel != nil && a.reasoningAgent != nil {
		reasoningInterface, ok := a.reasoningAgent.(interface {
			Reason(prompt string) ([]models.ReasoningStep, error)
		})
		if ok {
			reasoningSteps, err := reasoningInterface.Reason(prompt)
			if err == nil && len(reasoningSteps) > 0 {
				var allStepsMsg string
				for _, step := range reasoningSteps {
					stepMsg := ""
					if step.Title != "" {
						stepMsg += "**" + step.Title + "**\n"
					}
					if step.Reasoning != "" {
						stepMsg += step.Reasoning + "\n"
					}
					if step.Action != "" {
						stepMsg += "Action: " + step.Action + "\n"
					}
					if step.Result != "" {
						stepMsg += "Result: " + step.Result + "\n"
					}
					allStepsMsg += stepMsg + "\n"
				}
				messages = append(messages, models.Message{
					Role:    "assistant",
					Content: allStepsMsg,
				})
				//reasoningContent = allStepsMsg
				//utils.ReasoningPanel(reasoningContent)
			}
		}
	}

	resp, err := a.model.Invoke(a.ctx, messages, models.WithTools(a.tools))

	if err != nil {
		return models.RunResponse{}, err
	}

	if !a.reasoning {
		utils.ResponsePanel(resp.Content, bx, time.Now(), a.markdown)
	}

	// Save run to storage if enabled
	if a.storage != nil {
		if err := a.saveRun(prompt, resp.Content, messages); err != nil && a.debug {
			fmt.Printf("Warning: Failed to save run: %v\n", err)
		}
	}

	// Process memories if enabled
	if a.memory != nil {
		if err := a.processMemories(prompt, resp.Content); err != nil && a.debug {
			fmt.Printf("Warning: Failed to process memories: %v\n", err)
		}
	}

	// Update message history for next interaction
	if a.addHistoryToMessages {
		a.messages = append(a.messages, models.Message{
			Role:    "user",
			Content: prompt,
		})
		a.messages = append(a.messages, models.Message{
			Role:    "assistant",
			Content: resp.Content,
		})

		// Keep only recent messages based on history limit
		if a.numHistoryRuns > 0 {
			maxMessages := a.numHistoryRuns * 2 // user + assistant per run
			if len(a.messages) > maxMessages {
				a.messages = a.messages[len(a.messages)-maxMessages:]
			}
		}
	}

	return models.RunResponse{
		TextContent: resp.Content,
		ContentType: "text",
		Event:       "RunResponse",
		Messages: []models.Message{
			{
				Role:     models.Role(resp.Role),
				Content:  resp.Content,
				Thinking: resp.Thinking,
			},
		},
		Model:     resp.Model,
		CreatedAt: time.Now().Unix(),
	}, nil
}

// create Print with stream func is optional
func (a *Agent) PrintResponse(prompt string, stream bool, markdown bool) {
	fmt.Println("Running agent  stream:", stream, "markdown:", markdown)
	a.stream = stream
	a.markdown = markdown
	if stream {
		a.print_stream_response(prompt, markdown)
	} else {
		a.print_response(prompt, markdown)
	}
}

func (a *Agent) print_response(prompt string, markdown bool) {
	start := time.Now()
	messages := a.prepareMessages(prompt)

	if a.debug {
		fmt.Printf("DEBUG: Prepared %d messages for model\n", len(messages))
		for i, msg := range messages {
			fmt.Printf("DEBUG: Message %d - Role: %s, Content length: %d\n", i, msg.Role, len(msg.Content))
		}
		fmt.Printf("DEBUG: Using %d tools\n", len(a.tools))
	}

	spinnerResponse := utils.ThinkingPanel(prompt)

	if a.debug {
		fmt.Println("DEBUG: Calling model.Invoke...")
	}

	resp, err := a.model.Invoke(a.ctx, messages, models.WithTools(a.tools))
	if err != nil {
		fmt.Printf("ERROR: Model invoke failed: %v\n", err)
		return
	}

	if a.debug {
		fmt.Printf("DEBUG: Model response received - Content length: %d\n", len(resp.Content))
		fmt.Printf("DEBUG: Response content preview: %.100s...\n", resp.Content)
		fmt.Printf("DEBUG: Response type: %T\n", resp)
		fmt.Printf("DEBUG: Response role: %s\n", resp.Role)
		fmt.Printf("DEBUG: Response model: %s\n", resp.Model)
	}

	utils.ResponsePanel(resp.Content, spinnerResponse, start, markdown)

	if a.debug {
		fmt.Println("DEBUG: ResponsePanel called")
		fmt.Printf("DEBUG: Final response content:\n%s\n", resp.Content)
	}
}

func (a *Agent) print_stream_response(prompt string, markdown bool) {
	start := time.Now()
	messages := a.prepareMessages(prompt)
	// Thinking
	spinnerResponse := utils.ThinkingPanel(prompt)
	contentChan := utils.StartSimplePanel(spinnerResponse, start, markdown)
	defer close(contentChan)

	// Response
	responseTile := fmt.Sprintf("Response (%.1fs)\n\n", time.Since(start).Seconds())
	fullResponse := ""
	var streamBuffer string // Mover para fora do callback
	showResponse := false
	callOptions := []models.Option{
		models.WithTools(a.tools),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			if !showResponse {
				contentChan <- utils.ContentUpdateMsg{
					PanelName: "Response",
					Content:   responseTile,
				}
				showResponse = true
			}

			// Adicionar chunk ao buffer
			streamBuffer += string(chunk)
			fullResponse += string(chunk)

			// Verificar se devemos fazer flush do buffer
			shouldFlush := false

			// Flush if finding period, exclamation or question mark
			if strings.Contains(streamBuffer, ".") ||
				strings.Contains(streamBuffer, "!") ||
				strings.Contains(streamBuffer, "?") {
				shouldFlush = true
			}

			// Flush se buffer ficar muito grande (mais de 50 caracteres)
			if len(streamBuffer) > 50 {
				shouldFlush = true
			}

			if shouldFlush {
				// Send accumulated content
				contentChan <- utils.ContentUpdateMsg{
					PanelName: "Response",
					Content:   streamBuffer,
				}
				streamBuffer = "" // Limpar buffer
			}

			return nil
		}),
	}

	err := a.model.InvokeStream(a.ctx, messages, callOptions...)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Flush any remaining content in buffer
	if streamBuffer != "" {
		contentChan <- utils.ContentUpdateMsg{
			PanelName: "Response",
			Content:   streamBuffer,
		}
	}
}

func (a *Agent) prepareMessages(prompt string) []models.Message {
	systemMessage := ""

	if a.goal != "" {
		systemMessage += fmt.Sprintf("<goal>\n%s\n</goal>\n", a.goal)
	}

	if a.description != "" {
		systemMessage += fmt.Sprintf("<description>\n%s\n</description>\n", a.description)
	}

	if a.instructions != "" {
		systemMessage += fmt.Sprintf("<instructions>\n%s\n</instructions>\n", a.instructions)
	}

	if a.expected_output != "" {
		systemMessage += fmt.Sprintf("<expected_output>\n%s\n</expected_output>\n", a.expected_output)
	}

	// Add user memories if enabled and available
	if a.enableUserMemories && a.memory != nil && a.userID != "" {
		userMemories, err := a.memory.GetUserMemories(a.ctx, a.userID)
		if err == nil && len(userMemories) > 0 {
			memoryContent := ""
			// Limit to recent memories (last 10)
			maxMemories := 10
			if len(userMemories) > maxMemories {
				userMemories = userMemories[len(userMemories)-maxMemories:]
			}
			for _, memory := range userMemories {
				memoryContent += fmt.Sprintf("- %s\n", memory.Memory)
			}
			systemMessage += fmt.Sprintf("<user_memories>\nWhat I know about the user:\n%s</user_memories>\n", memoryContent)
		}
	}

	if a.markdown {
		a.additional_information = append(a.additional_information, "Use markdown to format your answers.")
	}

	//if have Knowledge, search for relevant documents
	if a.knowledge != nil {
		relevantDocs, err := a.knowledge.Search(a.ctx, prompt, 5)
		if err == nil && len(relevantDocs) > 0 {
			docContent := ""
			for _, doc := range relevantDocs {
				snippet := doc.Document.Content
				if len(snippet) > 200 {
					snippet = snippet[:200] + "..."
				}
				docContent += fmt.Sprintf("- %s\n", snippet)
			}
			systemMessage += fmt.Sprintf("<knowledge>\nRelevant information I found:\n%s</knowledge>\n", docContent)
		}
	}

	if len(a.additional_information) > 0 {
		systemMessage += fmt.Sprintf("<additional_information>\n%s\n</additional_information>\n", strings.Join(a.additional_information, "\n"))
	}

	if len(a.contextData) > 0 {
		contextStr := utils.PrettyPrintMap(a.contextData)
		systemMessage += fmt.Sprintf("<context>\n%s\n</context>\n", contextStr)
	}

	if a.debug {
		utils.DebugPanel(systemMessage)
	}

	messages := []models.Message{}

	if systemMessage != "" {
		messages = append(messages, models.Message{
			Role:    models.TypeSystemRole,
			Content: systemMessage,
		})
	}

	// Add chat history if enabled
	if a.addHistoryToMessages && len(a.messages) > 0 {
		messages = append(messages, a.messages...)
	}

	messages = append(messages, models.Message{
		Role:    models.TypeUserRole,
		Content: prompt,
	})

	return messages
}

// loadSession loads existing session data from storage
func (a *Agent) loadSession() error {
	if a.storage == nil || a.sessionID == "" {
		return nil
	}

	// Load session
	_, err := a.storage.ReadSession(a.ctx, a.sessionID)
	if err != nil {
		// Session doesn't exist, create new one
		if err.Error() == "session not found" {
			session := &storage.AgentSession{
				Session: storage.Session{
					SessionID:   a.sessionID,
					UserID:      a.userID,
					Memory:      make(map[string]interface{}),
					SessionData: make(map[string]interface{}),
					ExtraData:   make(map[string]interface{}),
					CreatedAt:   time.Now().Unix(),
					UpdatedAt:   time.Now().Unix(),
				},
				AgentID:   "default-agent",
				AgentData: make(map[string]interface{}),
			}
			if err := a.storage.CreateSession(a.ctx, session); err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
		} else {
			return fmt.Errorf("failed to load session: %w", err)
		}
	}

	// Load runs if history is enabled
	if a.addHistoryToMessages {
		runs, err := a.storage.GetRunsForSession(a.ctx, a.sessionID)
		if err != nil {
			return fmt.Errorf("failed to load session runs: %w", err)
		}

		// Keep only the most recent runs based on numHistoryRuns
		if a.numHistoryRuns > 0 && len(runs) > a.numHistoryRuns {
			runs = runs[len(runs)-a.numHistoryRuns:]
		}

		a.runs = runs

		// Build message history from runs
		a.buildMessageHistoryFromRuns()
	}

	return nil
}

// buildMessageHistoryFromRuns reconstructs message history from stored runs
func (a *Agent) buildMessageHistoryFromRuns() {
	a.messages = []models.Message{}

	for _, run := range a.runs {
		// Add user message
		if run.UserMessage != "" {
			a.messages = append(a.messages, models.Message{
				Role:    "user",
				Content: run.UserMessage,
			})
		}

		// Add assistant response
		if run.AgentMessage != "" {
			a.messages = append(a.messages, models.Message{
				Role:    "assistant",
				Content: run.AgentMessage,
			})
		}
	}
}

// saveRun saves a completed run to storage
func (a *Agent) saveRun(userMessage, agentResponse string, messages []models.Message) error {
	if a.storage == nil {
		return nil
	}

	// Convert messages to map format for storage
	messagesMaps := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		messagesMaps[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	run := &storage.AgentRun{
		ID:           uuid.New().String(),
		SessionID:    a.sessionID,
		UserID:       a.userID,
		RunName:      fmt.Sprintf("run_%d", time.Now().Unix()),
		RunData:      make(map[string]interface{}),
		UserMessage:  userMessage,
		AgentMessage: agentResponse,
		Messages:     messagesMaps,
		Metrics:      make(map[string]interface{}),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := a.storage.CreateRun(a.ctx, run); err != nil {
		return fmt.Errorf("failed to save run: %w", err)
	}

	// Add to local runs list
	a.runs = append(a.runs, run)

	// Keep only the most recent runs in memory
	if a.numHistoryRuns > 0 && len(a.runs) > a.numHistoryRuns {
		a.runs = a.runs[len(a.runs)-a.numHistoryRuns:]
	}

	return nil
}

// processMemories handles memory extraction and session summarization
func (a *Agent) processMemories(userMessage, agentResponse string) error {
	if a.memory == nil {
		return nil
	}

	// Extract and save user memories if enabled
	if a.enableAgenticMemory && a.userID != "" {
		_, err := a.memory.CreateMemory(a.ctx, a.userID, userMessage, agentResponse)
		if err != nil {
			// Log error but don't fail the whole operation
			if a.debug {
				fmt.Printf("Warning: Failed to create memory: %v\n", err)
			}
		}
	}

	// Generate session summary if enabled
	if a.enableSessionSummaries && a.userID != "" && a.sessionID != "" {
		// Check if we need to create/update session summary
		// This could be done periodically or based on number of interactions
		runCount := len(a.runs)
		if runCount > 0 && runCount%5 == 0 { // Summarize every 5 interactions
			conversation := []map[string]interface{}{}
			for _, run := range a.runs {
				if run.UserMessage != "" {
					conversation = append(conversation, map[string]interface{}{
						"role":    "user",
						"content": run.UserMessage,
					})
				}
				if run.AgentMessage != "" {
					conversation = append(conversation, map[string]interface{}{
						"role":    "assistant",
						"content": run.AgentMessage,
					})
				}
			}

			_, err := a.memory.CreateSessionSummary(a.ctx, a.userID, a.sessionID, conversation)
			if err != nil {
				// Log error but don't fail the whole operation
				if a.debug {
					fmt.Printf("Warning: Failed to create session summary: %v\n", err)
				}
			}
		}
	}

	return nil
}

func (a *Agent) RunStream(prompt string, fn func(chuck []byte) error) error {
	start := time.Now()
	messages := a.prepareMessages(prompt)
	//get debug
	debugmod := a.ctx.Value(models.DebugKey)

	spinnerResponse := utils.ThinkingPanel(prompt)
	contentChan := utils.StartSimplePanel(spinnerResponse, start, a.markdown)
	defer close(contentChan)

	// Thinking
	contentChan <- utils.ContentUpdateMsg{
		PanelName: "Thinking",
		Content:   prompt,
	}

	// Collect streaming content for memory processing
	var fullResponse strings.Builder

	opts := []models.Option{
		models.WithTools(a.tools),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			// Collect content for memory processing
			fullResponse.Write(chunk)

			if debugmod != nil && debugmod.(bool) {
				contentChan <- utils.ContentUpdateMsg{
					PanelName: "Response",
					Content:   fmt.Sprintf("Response (%.1fs)\n\n%s", time.Since(start).Seconds(), string(chunk)),
				}
			}

			return fn(chunk)
		}),
	}

	err := a.model.InvokeStream(a.ctx, messages, opts...)

	// After streaming is complete, process memory and storage
	if err == nil {
		responseContent := fullResponse.String()

		// Save run to storage if enabled
		if a.storage != nil {
			if saveErr := a.saveRun(prompt, responseContent, messages); saveErr != nil && a.debug {
				fmt.Printf("Warning: Failed to save run: %v\n", saveErr)
			}
		}

		// Process memories if enabled
		if a.memory != nil {
			if memErr := a.processMemories(prompt, responseContent); memErr != nil && a.debug {
				fmt.Printf("Warning: Failed to process memories: %v\n", memErr)
			}
		}

		// Update message history for next interaction
		if a.addHistoryToMessages {
			a.messages = append(a.messages, models.Message{
				Role:    "user",
				Content: prompt,
			})
			a.messages = append(a.messages, models.Message{
				Role:    "assistant",
				Content: responseContent,
			})

			// Keep only recent messages based on history limit
			if a.numHistoryRuns > 0 {
				maxMessages := a.numHistoryRuns * 2 // user + assistant per run
				if len(a.messages) > maxMessages {
					a.messages = a.messages[len(a.messages)-maxMessages:]
				}
			}
		}
	}

	return err

}

// Reason executa o reasoning chain usando o modelo configurado.
func (a *Agent) Reason(prompt string) ([]models.ReasoningStep, error) {
	// The model needs to implement the Invoke method.
	invoker := func(ctx context.Context, msgs []string) (string, error) {
		resp, err := a.Run(prompt)
		if err != nil {
			return "", err
		}
		return resp.Messages[0].Thinking, nil
	}

	return reasoning.ReasoningChain(a.ctx, invoker, prompt, a.reasoningMinSteps, a.reasoningMaxSteps)
}
