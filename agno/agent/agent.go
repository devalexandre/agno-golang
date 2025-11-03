package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/memory"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/reasoning"

	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
	"github.com/google/uuid"
	gpt3encoder "github.com/samber/go-gpt-3-encoder"
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
	Memory                  memory.MemoryManager
	Storage                 storage.AgentStorage
	SessionID               string
	UserID                  string
	AddHistoryToMessages    bool
	NumHistoryRuns          int
	MaxToolCallsFromHistory int
	EnableUserMemories      bool
	EnableAgenticMemory     bool
	EnableSessionSummaries  bool
	ReadChatHistory         bool

	//knowledge
	Knowledge             knowledge.Knowledge
	KnowledgeMaxDocuments int

	//Enable Semantic Compression
	EnableSemanticCompression bool
	SemanticMaxTokens         int
	SemanticModel             models.AgnoModelInterface
	SemanticAgent             models.AgentInterface

	// Input/Output Schema
	// InputSchema provides validation for agent input
	// Pass a struct instance to define the expected input structure
	InputSchema interface{}
	// OutputSchema forces the agent to return structured JSON matching the schema
	// Pass a pointer to a struct to define the expected output structure
	// The struct will be filled automatically with the parsed response
	OutputSchema interface{}
	// OutputModel is a separate AI model used specifically for parsing the output JSON
	// This allows using a different model (e.g., faster/cheaper) for JSON generation
	// Similar to how SemanticModel is used for compression
	OutputModel models.AgnoModelInterface
	// OutputModelPrompt allows customizing the prompt used by the OutputModel
	// If not provided, a default prompt will be used
	OutputModelPrompt string
	// ParseResponse controls whether to parse the response into the OutputSchema
	ParseResponse bool
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
	memory                  memory.MemoryManager
	storage                 storage.AgentStorage
	sessionID               string
	userID                  string
	addHistoryToMessages    bool
	numHistoryRuns          int
	maxToolCallsFromHistory int
	enableUserMemories      bool
	enableAgenticMemory     bool
	enableSessionSummaries  bool
	readChatHistory         bool

	// Session state
	messages []models.Message
	runs     []*storage.AgentRun

	// Knowledge
	knowledge             knowledge.Knowledge
	knowledgeMaxDocuments int

	// Reasoning
	reasoning         bool
	reasoningModel    models.AgnoModelInterface
	reasoningAgent    models.AgentInterface
	reasoningMinSteps int
	reasoningMaxSteps int

	// Semantic Compression
	semanticModel             models.AgnoModelInterface
	semanticAgent             models.AgentInterface
	semanticMaxTokens         int
	enableSemanticCompression bool

	// Input/Output Schema
	inputSchema       interface{}
	outputSchema      interface{}
	outputModel       models.AgnoModelInterface
	outputModelPrompt string
	parseResponse     bool
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

	if config.KnowledgeMaxDocuments <= 0 {
		config.KnowledgeMaxDocuments = 5
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
		memory:                  config.Memory,
		storage:                 config.Storage,
		sessionID:               sessionID,
		userID:                  config.UserID,
		addHistoryToMessages:    config.AddHistoryToMessages,
		numHistoryRuns:          config.NumHistoryRuns,
		maxToolCallsFromHistory: config.MaxToolCallsFromHistory,
		enableUserMemories:      config.EnableUserMemories,
		enableAgenticMemory:     config.EnableAgenticMemory,
		enableSessionSummaries:  config.EnableSessionSummaries,
		readChatHistory:         config.ReadChatHistory,

		// Initialize session state
		messages: []models.Message{},
		runs:     []*storage.AgentRun{},

		//knowledge
		knowledge:             config.Knowledge,
		knowledgeMaxDocuments: config.KnowledgeMaxDocuments,

		// Reasoning
		reasoning:         config.Reasoning,
		reasoningModel:    config.ReasoningModel,
		reasoningAgent:    config.ReasoningAgent,
		reasoningMinSteps: config.ReasoningMinSteps,
		reasoningMaxSteps: config.ReasoningMaxSteps,

		// Semantic Compression
		semanticModel:             config.SemanticModel,
		semanticAgent:             config.SemanticAgent,
		semanticMaxTokens:         config.SemanticMaxTokens,
		enableSemanticCompression: config.EnableSemanticCompression,

		// Input/Output Schema
		inputSchema:       config.InputSchema,
		outputSchema:      config.OutputSchema,
		outputModel:       config.OutputModel,
		outputModelPrompt: config.OutputModelPrompt,
		parseResponse:     config.ParseResponse,
	}

	// Set default for ParseResponse
	if agent.parseResponse == false && agent.outputSchema != nil {
		agent.parseResponse = true
	}

	if agent.enableSemanticCompression && agent.semanticModel == nil && agent.semanticAgent == nil {
		return nil, fmt.Errorf("semantic compression is enabled but no semantic model or agent provided")
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

// GetModel returns the agent's model
func (a *Agent) GetModel() models.AgnoModelInterface {
	return a.model
}

// GetKnowledge returns the agent's knowledge base
func (a *Agent) GetKnowledge() knowledge.Knowledge {
	return a.knowledge
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// validateInput validates the input against the input schema if configured
func (a *Agent) validateInput(input interface{}) error {
	if a.inputSchema == nil {
		return nil
	}

	// Convert input to JSON and then validate by unmarshaling into schema type
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	// Create a new instance of the schema type
	schemaType := reflect.TypeOf(a.inputSchema)
	if schemaType.Kind() == reflect.Ptr {
		schemaType = schemaType.Elem()
	}

	schemaInstance := reflect.New(schemaType).Interface()

	// Unmarshal and validate
	if err := json.Unmarshal(inputJSON, schemaInstance); err != nil {
		return fmt.Errorf("input validation failed: %w", err)
	}

	return nil
}

// prepareInputWithSchema prepares input according to input schema if configured
func (a *Agent) prepareInputWithSchema(input interface{}) (string, error) {
	if a.inputSchema == nil {
		// If input is a string, return it directly
		if str, ok := input.(string); ok {
			return str, nil
		}
		// Otherwise, marshal to JSON
		data, err := json.Marshal(input)
		if err != nil {
			return "", fmt.Errorf("failed to marshal input: %w", err)
		}
		return string(data), nil
	}

	// Validate input first
	if err := a.validateInput(input); err != nil {
		return "", err
	}

	// Marshal validated input to string
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal validated input: %w", err)
	}

	return string(data), nil
}

// addOutputSchemaToPrompt adds output schema instructions to the system prompt
func (a *Agent) addOutputSchemaToPrompt(systemPrompt string) (string, error) {
	// If using OutputModel, don't add schema instructions to main model
	// The OutputModel will handle JSON formatting
	if a.outputModel != nil {
		return systemPrompt, nil
	}

	// Only add schema instructions if OutputSchema is configured and no OutputModel
	if a.outputSchema == nil {
		return systemPrompt, nil
	}

	schema, err := GenerateJSONSchema(a.outputSchema)
	if err != nil {
		return "", fmt.Errorf("failed to generate output schema: %w", err)
	}

	schemaJSON, err := schema.ToJSONString()
	if err != nil {
		return "", fmt.Errorf("failed to convert schema to JSON: %w", err)
	}

	// Check if the output schema is a slice/array
	schemaType := reflect.TypeOf(a.outputSchema)
	if schemaType.Kind() == reflect.Ptr {
		schemaType = schemaType.Elem()
	}
	isArray := schemaType.Kind() == reflect.Slice

	var outputInstructions string
	if isArray {
		// Instructions for array output
		outputInstructions = fmt.Sprintf(`

## Output Format
The block below is the JSON Schema (for reference). DO NOT return the JSON Schema itself.
Instead, RETURN a JSON ARRAY that CONFORMS to this schema.

%s

CRITICAL RULES (read carefully):
- Return ONLY a JSON ARRAY (starts with [ and ends with ]).
- Each element in the array must be an object matching the item schema.
- Do NOT wrap the JSON in backticks or triple backtick markers.
- Do NOT include any text before or after the JSON array.
- Do NOT return separate objects - they must be inside a single array.
- Your entire response must be valid JSON and parseable as an array.

Example of correct format for array:
[{"field1": "value1", "field2": ["item1"]}, {"field1": "value2", "field2": ["item2"]}]

DO NOT use markdown formatting like code blocks.
`, schemaJSON)
	} else {
		// Instructions for object output
		outputInstructions = fmt.Sprintf(`

## Output Format
The block below is the JSON Schema (for reference). DO NOT return the JSON Schema itself.
Instead, RETURN a single JSON object that CONFORMS to this schema.

%s

CRITICAL RULES (read carefully):
- Return ONLY the JSON object instance that matches the schema (no schema, no explanations).
- Do NOT wrap the JSON in backticks or triple backtick markers.
- Do NOT include any text before or after the JSON.
- Include all required fields and use the correct types.
- Your entire response must be valid JSON and parseable.

If you understand, immediately produce an example JSON object that follows the schema (populate fields meaningfully).

Example of correct format:
{"field1": "value1", "field2": ["item1", "item2"]}

DO NOT use markdown formatting like code blocks.
`, schemaJSON)
	}

	return systemPrompt + outputInstructions, nil
}

// parseOutputWithSchema parses the response according to output schema if configured
func (a *Agent) parseOutputWithSchema(response string) (interface{}, error) {
	if a.outputSchema == nil || !a.parseResponse {
		return response, nil
	}

	originalResponse := response // Keep original for debugging

	// Clean the response - remove markdown code blocks if present
	cleaned := strings.TrimSpace(response)

	// Remove markdown code blocks (```json ... ``` or ``` ... ```)
	if strings.Contains(cleaned, "```") {
		// Find the start of JSON (after opening backticks)
		startIdx := strings.Index(cleaned, "```")
		if startIdx != -1 {
			// Skip the opening ``` and optional "json"
			cleaned = cleaned[startIdx+3:]
			if strings.HasPrefix(cleaned, "json") {
				cleaned = cleaned[4:]
			}
			cleaned = strings.TrimSpace(cleaned)

			// Find the end (closing backticks)
			endIdx := strings.Index(cleaned, "```")
			if endIdx != -1 {
				cleaned = cleaned[:endIdx]
			}
		}
	}

	cleaned = strings.TrimSpace(cleaned)

	// If debug mode, show what we're trying to parse
	if a.debug {
		fmt.Printf("\n=== DEBUG: Output Parsing ===\n")
		fmt.Printf("Original response length: %d\n", len(originalResponse))
		fmt.Printf("Cleaned response length: %d\n", len(cleaned))
		fmt.Printf("Original response preview (first 200 chars):\n%s\n", truncateString(originalResponse, 200))
		fmt.Printf("Cleaned response preview (first 200 chars):\n%s\n", truncateString(cleaned, 200))
		fmt.Printf("===========================\n\n")
	}

	// Get schema type
	schemaType := reflect.TypeOf(a.outputSchema)
	isPointer := schemaType.Kind() == reflect.Ptr

	if isPointer {
		schemaType = schemaType.Elem()
	}

	// Handle slice types differently
	if schemaType.Kind() == reflect.Slice {
		var result interface{}

		if isPointer {
			// If outputSchema is a pointer, unmarshal directly into it
			if err := json.Unmarshal([]byte(cleaned), a.outputSchema); err != nil {
				preview := truncateString(cleaned, 500)
				return nil, fmt.Errorf("failed to parse response into output schema (slice): %w\nResponse preview: %s", err, preview)
			}
			result = a.outputSchema
		} else {
			// For slices without pointer, create a new slice
			result = reflect.New(schemaType).Interface()
			if err := json.Unmarshal([]byte(cleaned), result); err != nil {
				preview := truncateString(cleaned, 500)
				return nil, fmt.Errorf("failed to parse response into output schema (slice): %w\nResponse preview: %s", err, preview)
			}
		}

		return result, nil
	}

	// For structs
	var result interface{}

	if isPointer {
		// If outputSchema is a pointer, unmarshal directly into it
		if err := json.Unmarshal([]byte(cleaned), a.outputSchema); err != nil {
			preview := truncateString(cleaned, 500)
			return nil, fmt.Errorf("failed to parse response into output schema: %w\nResponse preview: %s", err, preview)
		}
		result = a.outputSchema
	} else {
		// For structs without pointer, create a new instance
		result = reflect.New(schemaType).Interface()
		if err := json.Unmarshal([]byte(cleaned), result); err != nil {
			preview := truncateString(cleaned, 500)
			return nil, fmt.Errorf("failed to parse response into output schema: %w\nResponse preview: %s", err, preview)
		}
	}

	return result, nil
}

// ApplyOutputFormatting applies output formatting using OutputModel if configured
// Similar to ApplySemanticCompression, this method handles the logic of using
// a separate model for JSON formatting or falling back to direct parsing
func (a *Agent) ApplyOutputFormatting(response string) (interface{}, error) {
	if a.outputSchema == nil || !a.parseResponse {
		return response, nil
	}

	// If OutputModel is configured, use it for JSON formatting
	if a.outputModel != nil {
		return a.formatWithOutputModel(response)
	}

	// Otherwise, parse directly from the response
	return a.parseOutputWithSchema(response)
}

// formatWithOutputModel uses the OutputModel to convert response to structured JSON
func (a *Agent) formatWithOutputModel(response string) (interface{}, error) {
	if a.debug {
		fmt.Printf("\n=== DEBUG: Using OutputModel for JSON formatting ===\n")
		fmt.Printf("Original response length: %d\n", len(response))
		fmt.Printf("OutputModel: %T\n", a.outputModel)
		fmt.Printf("===================================================\n\n")
	}

	// Generate schema for the output model
	schema, err := GenerateJSONSchema(a.outputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to generate output schema: %w", err)
	}

	schemaJSON, err := schema.ToJSONString()
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to JSON: %w", err)
	}

	// Prepare prompt for the output model
	var systemPrompt string
	if a.outputModelPrompt != "" {
		systemPrompt = a.outputModelPrompt
	} else {
		systemPrompt = fmt.Sprintf(`You are a JSON formatting assistant. Your task is to convert the provided text into valid JSON that matches the specified schema.

Schema:
%s

CRITICAL RULES:
- Return ONLY valid JSON matching the schema
- Do NOT wrap in backticks or code blocks
- Do NOT add any explanations
- Extract relevant information from the text and structure it according to the schema
- If information is missing, use reasonable defaults or empty values`, schemaJSON)
	}

	userPrompt := fmt.Sprintf("Convert the following text to JSON:\n\n%s", response)

	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: systemPrompt,
		},
		{
			Role:    models.TypeUserRole,
			Content: userPrompt,
		},
	}

	// Invoke the output model
	resp, err := a.outputModel.Invoke(a.ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("output model invocation failed: %w", err)
	}

	// Clean the JSON response
	cleaned := strings.TrimSpace(resp.Content)

	// Remove markdown code blocks if present
	if strings.Contains(cleaned, "```") {
		startIdx := strings.Index(cleaned, "```")
		if startIdx != -1 {
			cleaned = cleaned[startIdx+3:]
			cleaned = strings.TrimPrefix(cleaned, "json")
			cleaned = strings.TrimSpace(cleaned)

			endIdx := strings.Index(cleaned, "```")
			if endIdx != -1 {
				cleaned = cleaned[:endIdx]
			}
		}
	}

	cleaned = strings.TrimSpace(cleaned)

	if a.debug {
		fmt.Printf("\n=== DEBUG: OutputModel Response ===\n")
		fmt.Printf("Cleaned JSON length: %d\n", len(cleaned))
		fmt.Printf("JSON preview (first 500 chars):\n%s\n", truncateString(cleaned, 500))
		fmt.Printf("==================================\n\n")
	}

	// Parse the JSON into the output schema
	return a.unmarshalIntoSchema(cleaned)
}

// unmarshalIntoSchema unmarshals JSON string into the output schema struct
func (a *Agent) unmarshalIntoSchema(jsonStr string) (interface{}, error) {
	// Get schema type
	schemaType := reflect.TypeOf(a.outputSchema)
	isPointer := schemaType.Kind() == reflect.Ptr

	if isPointer {
		schemaType = schemaType.Elem()
	}

	// Handle slice types
	if schemaType.Kind() == reflect.Slice {
		var result interface{}

		if isPointer {
			if err := json.Unmarshal([]byte(jsonStr), a.outputSchema); err != nil {
				preview := truncateString(jsonStr, 500)
				return nil, fmt.Errorf("failed to parse output model response (slice): %w\nResponse preview: %s", err, preview)
			}
			result = a.outputSchema
		} else {
			result = reflect.New(schemaType).Interface()
			if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
				preview := truncateString(jsonStr, 500)
				return nil, fmt.Errorf("failed to parse output model response (slice): %w\nResponse preview: %s", err, preview)
			}
		}

		return result, nil
	}

	// Handle struct types
	var result interface{}

	if isPointer {
		if err := json.Unmarshal([]byte(jsonStr), a.outputSchema); err != nil {
			preview := truncateString(jsonStr, 500)
			return nil, fmt.Errorf("failed to parse output model response: %w\nResponse preview: %s", err, preview)
		}
		result = a.outputSchema
	} else {
		result = reflect.New(schemaType).Interface()
		if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
			preview := truncateString(jsonStr, 500)
			return nil, fmt.Errorf("failed to parse output model response: %w\nResponse preview: %s", err, preview)
		}
	}

	return result, nil
}

func (a *Agent) Run(input interface{}) (models.RunResponse, error) {
	var messages []models.Message

	// Prepare input according to schema if configured
	prompt, err := a.prepareInputWithSchema(input)
	if err != nil {
		return models.RunResponse{}, fmt.Errorf("failed to prepare input: %w", err)
	}

	// Add system message and history normally
	baseMessages := a.prepareMessages(prompt)
	for _, msg := range baseMessages {
		if msg.Role == models.TypeUserRole {
			messages = append(messages, msg)
		} else {
			messages = append([]models.Message{msg}, messages...)
		}
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
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Keep only recent messages based on history limit
		if a.numHistoryRuns > 0 {
			maxMessages := a.numHistoryRuns * 2 // user + assistant per run
			if len(a.messages) > maxMessages {
				a.messages = a.messages[len(a.messages)-maxMessages:]
			}
		}
	}

	// Parse output using ApplyOutputFormatting method
	// This provides TWO outputs when OutputModel is configured:
	// 1. resp.Content (TextContent) = Original creative response from main model
	// 2. parsedContent (Output) = Structured JSON formatted by OutputModel
	// This allows using expensive models for content and cheap models for formatting
	parsedContent, err := a.ApplyOutputFormatting(resp.Content)
	if err != nil {
		return models.RunResponse{}, err
	}

	var outputContent interface{}
	if parsedContent != resp.Content {
		// Output was parsed/formatted
		outputContent = parsedContent
	}

	return models.RunResponse{
		TextContent:  resp.Content, // Original response from main model
		ContentType:  "text",
		Event:        "RunResponse",
		ParsedOutput: parsedContent, // Deprecated: kept for backwards compatibility
		Output:       outputContent, // Structured output (pointer to filled struct)
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

// filterToolCallsFromHistory filters the tool calls from message history based on maxToolCallsFromHistory
func (a *Agent) filterToolCallsFromHistory(messages []models.Message) []models.Message {
	if a.maxToolCallsFromHistory <= 0 {
		// If no limit is set, return all messages as is
		return messages
	}

	// Count tool calls from the end of the messages (most recent first)
	var filteredMessages []models.Message
	toolCallCount := 0
	limitReached := false

	// Process messages in reverse order to count from most recent
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		// Check if this message contains tool calls
		if len(msg.ToolCalls) > 0 {
			if limitReached {
				// Skip messages with tool calls after limit is reached
				continue
			}

			// Check if adding these tool calls would exceed the limit
			if toolCallCount+len(msg.ToolCalls) > a.maxToolCallsFromHistory {
				// Calculate how many tool calls we can still include
				remainingSlots := a.maxToolCallsFromHistory - toolCallCount
				if remainingSlots > 0 {
					// Create a copy of the message with limited tool calls
					limitedMsg := msg
					limitedMsg.ToolCalls = msg.ToolCalls[len(msg.ToolCalls)-remainingSlots:]
					filteredMessages = append([]models.Message{limitedMsg}, filteredMessages...)
				}
				// Mark that we've reached the limit
				limitReached = true
			} else {
				// Add all tool calls from this message
				filteredMessages = append([]models.Message{msg}, filteredMessages...)
				toolCallCount += len(msg.ToolCalls)
			}
		} else {
			// Message has no tool calls, always include it
			filteredMessages = append([]models.Message{msg}, filteredMessages...)
		}
	}

	return filteredMessages
}

func (a *Agent) prepareMessages(prompt string) []models.Message {
	systemMessage := ""
	originalSystemMessage := ""
	originalPrompt := prompt

	if a.goal != "" {
		systemMessage += fmt.Sprintf("<goal>\n%s\n</goal>\n", a.ApplySemanticCompression(a.goal))
		originalSystemMessage += fmt.Sprintf("<goal>\n%s\n</goal>\n", a.goal)
	}

	if a.description != "" {
		systemMessage += fmt.Sprintf("<description>\n%s\n</description>\n", a.ApplySemanticCompression(a.description))
		originalSystemMessage += fmt.Sprintf("<description>\n%s\n</description>\n", a.description)
	}

	if a.instructions != "" {
		systemMessage += fmt.Sprintf("<instructions>\n%s\n</instructions>\n", a.ApplySemanticCompression(a.instructions))
		originalSystemMessage += fmt.Sprintf("<instructions>\n%s\n</instructions>\n", a.instructions)
	}

	if a.expected_output != "" {
		systemMessage += fmt.Sprintf("<expected_output>\n%s\n</expected_output>\n", a.ApplySemanticCompression(a.expected_output))
		originalSystemMessage += fmt.Sprintf("<expected_output>\n%s\n</expected_output>\n", a.expected_output)
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
			originalSystemMessage += fmt.Sprintf("<user_memories>\nWhat I know about the user:\n%s</user_memories>\n", memoryContent)
		}
	}

	if a.markdown {
		a.additional_information = append(a.additional_information, "Use markdown to format your answers.")
	}

	//if have Knowledge, search for relevant documents
	if a.knowledge != nil {
		relevantDocs, err := a.knowledge.Search(a.ctx, prompt, a.knowledgeMaxDocuments)
		if err == nil && len(relevantDocs) > 0 {
			docContent := ""
			for _, doc := range relevantDocs {
				snippet := doc.Document.Content
				if len(snippet) > 200 {
					snippet = snippet[:200] + "..."
				}
				docContent += fmt.Sprintf("- %s\n", snippet)
			}
			systemMessage += fmt.Sprintf("<knowledge>\nRelevant information I found:\n%s</knowledge>\n", a.ApplySemanticCompression(docContent))
			originalSystemMessage += fmt.Sprintf("<knowledge>\nRelevant information I found:\n%s</knowledge>\n", docContent)
		}
	}

	if len(a.additional_information) > 0 {
		systemMessage += fmt.Sprintf("<additional_information>\n%s\n</additional_information>\n", strings.Join(a.additional_information, "\n"))
	}

	if len(a.contextData) > 0 {
		contextStr := utils.PrettyPrintMap(a.contextData)
		systemMessage += fmt.Sprintf("<context>\n%s\n</context>\n", a.ApplySemanticCompression(contextStr))
		originalSystemMessage += fmt.Sprintf("<context>\n%s\n</context>\n", contextStr)
	}

	// Add output schema or output model instructions if configured
	if a.outputSchema != nil {
		schemaInstructions, err := a.addOutputSchemaToPrompt("")
		if err == nil {
			systemMessage += schemaInstructions
			originalSystemMessage += schemaInstructions
		}
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
		historyMessages := a.filterToolCallsFromHistory(a.messages)
		messages = append(messages, historyMessages...)
	}

	compressedPrompt := a.ApplySemanticCompression(prompt)

	if a.debug && a.enableSemanticCompression {
		encoder, _ := gpt3encoder.NewEncoder()
		// Check token length
		tokensSemantic, err := encoder.Encode(systemMessage)
		if err != nil {
			log.Printf("ERROR: Token encoding tokensSemantic failed: %v\n", err)
		}

		tokensOriginal, err := encoder.Encode(originalSystemMessage)
		if err != nil {
			log.Printf("ERROR: Token encoding tokensOriginal failed: %v\n", err)
		}

		fmt.Println("--------------------------------------System Compression-------------------------------------------------------------")
		fmt.Printf("DEBUG: Original Message System \n\n %s\n\n", originalSystemMessage)
		fmt.Printf("DEBUG: Applying semantic compression original message tokens: %d \n", len(tokensOriginal))
		// Check for token length reduction
		fmt.Printf("DEBUG: Compressed Message \n\n %s \n\n", systemMessage)
		fmt.Printf("DEBUG: Applying semantic compression compressed message tokens: %d\n", len(tokensSemantic))
		fmt.Println("--------------------------------------------------------------------------------------------------------------------------")

		tokensPromptSemantic, _ := encoder.Encode(compressedPrompt)
		tokensPromptOriginal, _ := encoder.Encode(originalPrompt)

		fmt.Println("--------------------------------------Prompt Compression-------------------------------------------------------------")
		fmt.Printf("DEBUG: Original Prompt \n\n %s\n\n", originalPrompt)
		fmt.Printf("DEBUG: Applying semantic compression original prompt tokens: %d \n", len(tokensPromptOriginal))
		// Check for token length reduction
		fmt.Printf("DEBUG: Compressed Prompt \n\n %s \n\n", compressedPrompt)
		fmt.Printf("DEBUG: Applying semantic compression compressed prompt tokens: %d\n", len(tokensPromptSemantic))
		fmt.Println("--------------------------------------------------------------------------------------------------------------------------")

	}

	messages = append(messages, models.Message{
		Role:    models.TypeUserRole,
		Content: compressedPrompt,
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
	messages := a.prepareMessages(prompt)

	// Collect streaming content for memory processing
	var fullResponse strings.Builder

	opts := []models.Option{
		models.WithTools(a.tools),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			// Collect content for memory processing
			fullResponse.Write(chunk)

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

func (a *Agent) ApplySemanticCompression(message string) string {
	if !a.enableSemanticCompression {
		return message
	}

	encoder, _ := gpt3encoder.NewEncoder()
	// Check token length
	tokens, _ := encoder.Encode(message)
	if a.debug {
		fmt.Printf("DEBUG: Applying semantic compression to %d tokens\n", tokens)
	}
	if a.semanticMaxTokens == 0 || len(tokens) < a.semanticMaxTokens {
		// No need to compress
		return message
	}
	var semanticAgent *Agent
	var err error
	var msgcompressed string

	if a.semanticModel != nil && a.semanticAgent == nil {

		semanticAgent, err = NewAgent(AgentConfig{
			Context:      a.ctx,
			Name:         "SemanticCompressor",
			Description:  "Semantic text compression agent.",
			Instructions: "Replace the input text with an ultra-concise version using abbreviations, technical notation, and minimal wording. Preserve all essential facts (dates, versions, IDs, deadlines). Return only the compressed result in the same language as the input. Do not add explanations or comments.",
			Model:        a.semanticModel,
			Markdown:     false,
			Debug:        false,
		})

		if err != nil {
			log.Fatalf("Failed to create assistant agent: %v", err)
		}
	}

	if a.semanticAgent != nil && a.semanticModel == nil {

		newmsg, err := a.semanticAgent.Run(message)
		if err != nil {
			if a.debug {
				fmt.Printf("Warning: Semantic compression failed for message: %v\n", err)
			}

		}
		msgcompressed = newmsg.Messages[0].Content
	}

	if a.semanticModel != nil && a.semanticAgent == nil {

		newmsg, err := semanticAgent.Run(message)
		if err != nil {
			if a.debug {
				fmt.Printf("Warning: Semantic compression failed for message: %v\n", err)
			}

		}
		msgcompressed = newmsg.Messages[0].Content
	}

	return msgcompressed
}
