package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/utils"
)

// RunStatus represents the status of a workflow run
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// WorkflowRunEvent represents events that can occur during workflow execution
type WorkflowRunEvent string

const (
	WorkflowStartedEvent             WorkflowRunEvent = "WorkflowStarted"
	WorkflowCompletedEvent           WorkflowRunEvent = "WorkflowCompleted"
	StepStartedEvent                 WorkflowRunEvent = "StepStarted"
	StepCompletedEvent               WorkflowRunEvent = "StepCompleted"
	StepOutputEvent                  WorkflowRunEvent = "StepOutput"
	StepsExecutionStartedEvent       WorkflowRunEvent = "StepsExecutionStarted"
	StepsExecutionCompletedEvent     WorkflowRunEvent = "StepsExecutionCompleted"
	LoopExecutionStartedEvent        WorkflowRunEvent = "LoopExecutionStarted"
	LoopExecutionCompletedEvent      WorkflowRunEvent = "LoopExecutionCompleted"
	LoopIterationStartedEvent        WorkflowRunEvent = "LoopIterationStarted"
	LoopIterationCompletedEvent      WorkflowRunEvent = "LoopIterationCompleted"
	ParallelExecutionStartedEvent    WorkflowRunEvent = "ParallelExecutionStarted"
	ParallelExecutionCompletedEvent  WorkflowRunEvent = "ParallelExecutionCompleted"
	ConditionExecutionStartedEvent   WorkflowRunEvent = "ConditionExecutionStarted"
	ConditionExecutionCompletedEvent WorkflowRunEvent = "ConditionExecutionCompleted"
	RouterExecutionStartedEvent      WorkflowRunEvent = "RouterExecutionStarted"
	RouterExecutionCompletedEvent    WorkflowRunEvent = "RouterExecutionCompleted"
)

// WorkflowRunResponse represents the response from a workflow run
type WorkflowRunResponse struct {
	RunID      string                 `json:"run_id"`
	WorkflowID string                 `json:"workflow_id"`
	Status     RunStatus              `json:"status"`
	Content    interface{}            `json:"content,omitempty"`
	Event      WorkflowRunEvent       `json:"event,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Metrics    *WorkflowMetrics       `json:"metrics,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// WorkflowRunResponseEvent represents an event during workflow execution
type WorkflowRunResponseEvent struct {
	Event     WorkflowRunEvent       `json:"event"`
	Data      interface{}            `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowSession represents session storage for workflows
type WorkflowSession struct {
	SessionID      string                 `json:"session_id"`
	WorkflowID     string                 `json:"workflow_id"`
	State          map[string]interface{} `json:"state"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	LastAccessedAt time.Time              `json:"last_accessed_at"`
}

// Storage interface for workflow persistence
type Storage interface {
	SaveSession(ctx context.Context, session *WorkflowSession) error
	LoadSession(ctx context.Context, sessionID string) (*WorkflowSession, error)
	DeleteSession(ctx context.Context, sessionID string) error
}

// WorkflowSteps represents the steps configuration for a workflow
type WorkflowSteps interface{}

// Workflow represents a pipeline-based workflow execution
type Workflow struct {
	// Workflow identification
	WorkflowID  string
	Name        string
	Description string

	// Workflow configuration
	Steps WorkflowSteps

	// Storage
	Storage Storage

	// Session management
	SessionID            string
	SessionName          string
	UserID               string
	WorkflowSessionID    string
	WorkflowSessionState map[string]interface{}

	// Runtime state
	RunID       string
	RunResponse *WorkflowRunResponse

	// Workflow session for storage
	WorkflowSession *WorkflowSession
	DebugMode       bool

	// Streaming configuration
	Stream                  bool
	StreamIntermediateSteps bool

	// Event handling
	StoreEvents  bool
	EventsToSkip []WorkflowRunEvent

	// Input validation
	InputSchema interface{}

	// WebSocket streaming
	WebSocketHandler WebSocketHandler

	// Internal state
	mu            sync.RWMutex
	stepOutputs   map[string]*StepOutput
	metrics       *WorkflowMetrics
	eventHandlers map[WorkflowRunEvent][]func(*WorkflowRunResponseEvent)
}

// NewWorkflow creates a new Workflow instance
func NewWorkflow(options ...WorkflowOption) *Workflow {
	w := &Workflow{
		WorkflowSessionState: make(map[string]interface{}),
		stepOutputs:          make(map[string]*StepOutput),
		eventHandlers:        make(map[WorkflowRunEvent][]func(*WorkflowRunResponseEvent)),
		EventsToSkip:         []WorkflowRunEvent{},
	}

	for _, opt := range options {
		opt(w)
	}

	// Generate workflow ID if not provided
	if w.WorkflowID == "" {
		w.WorkflowID = GenerateID()
	}

	// Initialize metrics
	w.metrics = &WorkflowMetrics{
		WorkflowID:    w.WorkflowID,
		StepMetrics:   make(map[string]*StepMetrics),
		CustomMetrics: make(map[string]interface{}),
	}

	return w
}

// WorkflowOption is a functional option for configuring a Workflow
type WorkflowOption func(*Workflow)

// WithWorkflowID sets the workflow ID
func WithWorkflowID(id string) WorkflowOption {
	return func(w *Workflow) {
		w.WorkflowID = id
	}
}

// WithWorkflowName sets the workflow name
func WithWorkflowName(name string) WorkflowOption {
	return func(w *Workflow) {
		w.Name = name
	}
}

// WithWorkflowDescription sets the workflow description
func WithWorkflowDescription(desc string) WorkflowOption {
	return func(w *Workflow) {
		w.Description = desc
	}
}

// WithWorkflowSteps sets the workflow steps
func WithWorkflowSteps(steps WorkflowSteps) WorkflowOption {
	return func(w *Workflow) {
		w.Steps = steps
	}
}

// WithStorage sets the storage backend
func WithStorage(storage Storage) WorkflowOption {
	return func(w *Workflow) {
		w.Storage = storage
	}
}

// WithSessionID sets the session ID
func WithSessionID(id string) WorkflowOption {
	return func(w *Workflow) {
		w.SessionID = id
	}
}

// WithUserID sets the user ID
func WithUserID(id string) WorkflowOption {
	return func(w *Workflow) {
		w.UserID = id
	}
}

// WithDebugMode enables debug mode
func WithDebugMode(debug bool) WorkflowOption {
	return func(w *Workflow) {
		w.DebugMode = debug
	}
}

// WithStreaming configures streaming options
func WithStreaming(stream bool, intermediateSteps bool) WorkflowOption {
	return func(w *Workflow) {
		w.Stream = stream
		w.StreamIntermediateSteps = intermediateSteps
	}
}

// WithEventStorage configures event storage
func WithEventStorage(store bool, skip ...WorkflowRunEvent) WorkflowOption {
	return func(w *Workflow) {
		w.StoreEvents = store
		w.EventsToSkip = skip
	}
}

// WithInputSchema sets the input schema for validation
func WithInputSchema(schema interface{}) WorkflowOption {
	return func(w *Workflow) {
		w.InputSchema = schema
	}
}

// WithWebSocketHandler sets the WebSocket handler for real-time event streaming
func WithWebSocketHandler(handler WebSocketHandler) WorkflowOption {
	return func(w *Workflow) {
		w.WebSocketHandler = handler
	}
}

// Run executes the workflow with the given input
func (w *Workflow) Run(ctx context.Context, input interface{}) (*WorkflowRunResponse, error) {
	// Validate input if schema is configured
	if err := w.validateInput(input); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Initialize run
	w.RunID = GenerateID()
	w.metrics.RunID = w.RunID
	w.metrics.StartTime = time.Now()

	// Create workflow execution input
	execInput := w.createExecutionInput(input)

	// Emit workflow started event
	w.emitEvent(&WorkflowRunResponseEvent{
		Event:     WorkflowStartedEvent,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"workflow_id": w.WorkflowID,
			"run_id":      w.RunID,
			"input":       execInput.ToMap(),
		},
	})

	// Execute workflow steps
	var finalOutput *StepOutput
	var err error

	switch steps := w.Steps.(type) {
	case []*Step:
		finalOutput, err = w.executeStepSequence(ctx, steps, execInput)
	case []interface{}:
		finalOutput, err = w.executeInterfaceSequence(ctx, steps, execInput)
	case []ExecutorFunc:
		// Convert to interface slice
		interfaceSteps := make([]interface{}, len(steps))
		for i, s := range steps {
			interfaceSteps[i] = s
		}
		finalOutput, err = w.executeInterfaceSequence(ctx, interfaceSteps, execInput)
	case ExecutorFunc:
		finalOutput, err = w.executeFunctionWorkflow(ctx, steps, execInput)
	case func(*StepInput) (*StepOutput, error):
		finalOutput, err = w.executeFunctionWorkflow(ctx, ExecutorFunc(steps), execInput)
	default:
		err = fmt.Errorf("unsupported workflow steps type: %T", steps)
	}

	// Update metrics
	w.metrics.EndTime = time.Now()
	w.metrics.DurationMs = w.metrics.EndTime.Sub(w.metrics.StartTime).Milliseconds()

	// Determine status
	status := RunStatusCompleted
	if err != nil {
		status = RunStatusFailed
		w.metrics.Success = false
		w.metrics.Error = err.Error()
	} else {
		w.metrics.Success = true
	}

	// Create response
	var content interface{}
	if finalOutput != nil {
		content = finalOutput.Content
	}

	response := &WorkflowRunResponse{
		RunID:      w.RunID,
		WorkflowID: w.WorkflowID,
		Status:     status,
		Content:    content,
		Metrics:    w.metrics,
		CreatedAt:  w.metrics.StartTime,
		UpdatedAt:  w.metrics.EndTime,
	}

	// Create final output if nil
	if finalOutput == nil {
		finalOutput = &StepOutput{
			Content: "Workflow completed with no output",
		}
	}

	// Emit workflow completed event
	w.emitEvent(&WorkflowRunResponseEvent{
		Event:     WorkflowCompletedEvent,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"workflow_id": w.WorkflowID,
			"run_id":      w.RunID,
			"status":      status,
			"metrics":     w.metrics.ToMap(),
		},
	})

	// Save session if storage is configured
	if w.Storage != nil && w.SessionID != "" {
		w.saveSession(ctx)
	}

	w.RunResponse = response
	return response, err
}

// executeStepSequence executes a sequence of steps
func (w *Workflow) executeStepSequence(ctx context.Context, steps []*Step, execInput *WorkflowExecutionInput) (*StepOutput, error) {
	var lastOutput *StepOutput
	stepInput := &StepInput{
		Message:             execInput.Message,
		AdditionalData:      execInput.AdditionalData,
		Images:              execInput.Images,
		Videos:              execInput.Videos,
		Audio:               execInput.Audio,
		PreviousStepOutputs: make(map[string]*StepOutput),
	}

	for i, step := range steps {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 🔁 Atualiza PreviousStepOutputs com TODAS as saídas armazenadas no workflow
		w.mu.RLock()
		for k, v := range w.stepOutputs {
			stepInput.PreviousStepOutputs[k] = v
		}
		w.mu.RUnlock()

		// 🔄 Atualiza PreviousStepContent com a saída do passo anterior
		if lastOutput != nil {
			stepInput.PreviousStepContent = lastOutput.Content
		}

		// Get step name for events
		stepName := fmt.Sprintf("step_%d", i)
		if step.Name != "" {
			stepName = step.Name
		}

		// Emit step started event
		w.emitEvent(&WorkflowRunResponseEvent{
			Event:     StepStartedEvent,
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"step_name":  stepName,
				"step_index": i,
			},
		})

		// Execute step
		stepMetrics := &StepMetrics{
			StartTime: time.Now(),
		}

		output, err := step.Execute(ctx, stepInput)

		stepMetrics.EndTime = time.Now()
		stepMetrics.DurationMs = stepMetrics.EndTime.Sub(stepMetrics.StartTime).Milliseconds()

		if err != nil {
			stepMetrics.Success = false
			stepMetrics.Error = err.Error()
			w.metrics.StepsFailed++

			if !step.SkipOnFailure {
				return nil, fmt.Errorf("step '%s' failed: %w", stepName, err)
			}
		} else {
			stepMetrics.Success = true
			w.metrics.StepsSucceeded++
		}

		// Store step output and metrics
		w.mu.Lock()
		w.stepOutputs[stepName] = output
		w.metrics.StepMetrics[stepName] = stepMetrics
		w.metrics.StepsExecuted++
		w.mu.Unlock()

		// Update step input for next iteration
		stepInput.PreviousStepOutputs[stepName] = output

		// Emit step completed event
		w.emitEvent(&WorkflowRunResponseEvent{
			Event:     StepCompletedEvent,
			Timestamp: time.Now(),
			Data:      output,
			Metadata: map[string]interface{}{
				"step_name":  stepName,
				"step_index": i,
				"metrics":    stepMetrics.ToMap(),
			},
		})

		lastOutput = output
	}

	return lastOutput, nil
}

// executeInterfaceSequence executes a sequence of mixed step types
func (w *Workflow) executeInterfaceSequence(ctx context.Context, steps []interface{}, execInput *WorkflowExecutionInput) (*StepOutput, error) {
	var lastOutput *StepOutput
	stepInput := &StepInput{
		Message:             execInput.Message,
		AdditionalData:      execInput.AdditionalData,
		Images:              execInput.Images,
		Videos:              execInput.Videos,
		Audio:               execInput.Audio,
		PreviousStepOutputs: make(map[string]*StepOutput),
	}

	// Emit steps execution started event
	w.emitEvent(&WorkflowRunResponseEvent{
		Event:     StepsExecutionStartedEvent,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"total_steps": len(steps),
		},
	})

	for i, item := range steps {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Atualiza o conteúdo anterior com a saída do passo anterior
		// 🔁 Atualiza PreviousStepOutputs com TODAS as saídas armazenadas no workflow
		w.mu.RLock()
		for k, v := range w.stepOutputs {
			stepInput.PreviousStepOutputs[k] = v
		}
		w.mu.RUnlock()

		if lastOutput != nil {
			stepInput.PreviousStepContent = lastOutput.Content
		}

		var output *StepOutput
		var err error

		// Executa o passo com base no tipo
		switch v := item.(type) {
		case *Step:
			output, err = v.Execute(ctx, stepInput)
		case ExecutorFunc:
			output, err = v(stepInput)
		case func(*StepInput) (*StepOutput, error):
			output, err = v(stepInput)
		case *Loop:
			// Executa o loop
			output, err = v.Execute(ctx, stepInput)
			if err != nil {
				return nil, err
			}

			// ✅ Armazena a saída do loop inteiro
			w.mu.Lock()
			w.stepOutputs[v.Name] = output
			w.mu.Unlock()

			// ✅ Armazena todas as saídas internas do loop (ex: "research")
			if v.CollectOutputs {
				for _, innerOutput := range v.outputs {
					if innerOutput.StepName != "" {
						w.mu.Lock()
						w.stepOutputs[innerOutput.StepName] = innerOutput
						w.mu.Unlock()
					}
				}
			}

			// ✅ Atualiza lastOutput para o próximo passo
			lastOutput = output
		case *Parallel:
			output, err = v.Execute(ctx, stepInput)
		case *Condition:
			output, err = v.Execute(ctx, stepInput)
		case *Router:
			output, err = v.Execute(ctx, stepInput)
		default:
			return nil, fmt.Errorf("unsupported step type at index %d: %T", i, v)
		}

		// Determina o nome do passo
		stepName := fmt.Sprintf("step_%d", i)
		if output != nil && output.StepName != "" {
			stepName = output.StepName
		}

		// Trata erro
		if err != nil {
			w.metrics.StepsFailed++
			return nil, err
		}

		// Atualiza métricas
		w.metrics.StepsExecuted++
		w.metrics.StepsSucceeded++

		// Emite eventos
		w.emitEvent(&WorkflowRunResponseEvent{
			Event:     StepStartedEvent,
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"step_name":  stepName,
				"step_index": i,
			},
		})

		w.emitEvent(&WorkflowRunResponseEvent{
			Event:     StepCompletedEvent,
			Timestamp: time.Now(),
			Data:      output,
			Metadata: map[string]interface{}{
				"step_name":  stepName,
				"step_index": i,
			},
		})

		// Armazena a saída do passo atual (incluindo loops, steps, etc)
		if output != nil {
			w.mu.Lock()
			w.stepOutputs[stepName] = output
			if w.metrics.StepMetrics == nil {
				w.metrics.StepMetrics = make(map[string]*StepMetrics)
			}
			w.metrics.StepMetrics[stepName] = &StepMetrics{
				Success: true,
			}
			w.mu.Unlock()

			// Atualiza o PreviousStepOutputs para os próximos passos
			stepInput.PreviousStepOutputs[stepName] = output
			lastOutput = output
		}
	}

	return lastOutput, nil
}

// executeFunctionWorkflow executes a function-based workflow
func (w *Workflow) executeFunctionWorkflow(ctx context.Context, fn ExecutorFunc, execInput *WorkflowExecutionInput) (*StepOutput, error) {
	stepInput := &StepInput{
		Message:        execInput.Message,
		AdditionalData: execInput.AdditionalData,
		Images:         execInput.Images,
		Videos:         execInput.Videos,
		Audio:          execInput.Audio,
	}

	// Emit step started event
	w.emitEvent(&WorkflowRunResponseEvent{
		Event:     StepStartedEvent,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"step_name": "function_workflow",
		},
	})

	output, err := fn(stepInput)

	if err != nil {
		w.metrics.StepsFailed++
	} else {
		w.metrics.StepsExecuted++
		w.metrics.StepsSucceeded++
	}

	// Emit step completed event
	w.emitEvent(&WorkflowRunResponseEvent{
		Event:     StepCompletedEvent,
		Timestamp: time.Now(),
		Data:      output,
		Metadata: map[string]interface{}{
			"step_name": "function_workflow",
		},
	})

	return output, err
}

// createExecutionInput creates a WorkflowExecutionInput from various input types
func (w *Workflow) createExecutionInput(input interface{}) *WorkflowExecutionInput {
	switch v := input.(type) {
	case *WorkflowExecutionInput:
		return v
	case WorkflowExecutionInput:
		return &v
	case string:
		return &WorkflowExecutionInput{Message: v}
	case map[string]interface{}:
		// Check if it's already a structured input
		if msg, ok := v["message"]; ok {
			execInput := &WorkflowExecutionInput{Message: msg}
			if additionalData, ok := v["additional_data"].(map[string]interface{}); ok {
				execInput.AdditionalData = additionalData
			}
			return execInput
		}
		return &WorkflowExecutionInput{Message: v}
	default:
		return &WorkflowExecutionInput{Message: input}
	}
}

// emitEvent emits a workflow event to registered handlers
func (w *Workflow) emitEvent(event *WorkflowRunResponseEvent) {
	// Skip if event should be skipped
	for _, skipEvent := range w.EventsToSkip {
		if event.Event == skipEvent {
			return
		}
	}

	// Store event if configured
	if w.StoreEvents {
		// TODO: Implement event storage
	}

	// Stream event if configured
	if w.Stream && (w.StreamIntermediateSteps || event.Event == WorkflowCompletedEvent) {
		// TODO: Implement streaming
	}
}

// OnEvent registers an event handler for a specific event type
func (w *Workflow) OnEvent(event WorkflowRunEvent, handler func(*WorkflowRunResponseEvent)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.eventHandlers[event] = append(w.eventHandlers[event], handler)
}

// saveSession saves the current workflow session
func (w *Workflow) saveSession(ctx context.Context) error {
	if w.Storage == nil || w.SessionID == "" {
		return nil
	}

	session := &WorkflowSession{
		SessionID:      w.SessionID,
		WorkflowID:     w.WorkflowID,
		State:          w.WorkflowSessionState,
		UpdatedAt:      time.Now(),
		LastAccessedAt: time.Now(),
	}

	if w.WorkflowSession != nil {
		session.CreatedAt = w.WorkflowSession.CreatedAt
	} else {
		session.CreatedAt = time.Now()
	}

	return w.Storage.SaveSession(ctx, session)
}

// loadSession loads a workflow session
func (w *Workflow) loadSession(ctx context.Context) error {
	if w.Storage == nil || w.SessionID == "" {
		return nil
	}

	session, err := w.Storage.LoadSession(ctx, w.SessionID)
	if err != nil {
		return err
	}

	w.WorkflowSession = session
	w.WorkflowSessionState = session.State
	return nil
}

// GetStepOutput returns the output of a specific step
func (w *Workflow) GetStepOutput(stepName string) *StepOutput {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stepOutputs[stepName]
}

// validateInput validates the input against the InputSchema if configured
func (w *Workflow) validateInput(input interface{}) error {
	if w.InputSchema == nil {
		return nil
	}

	// Get the schema type
	schemaType := reflect.TypeOf(w.InputSchema)
	if schemaType == nil {
		return fmt.Errorf("input schema is nil")
	}

	// Handle pointer types
	if schemaType.Kind() == reflect.Ptr {
		schemaType = schemaType.Elem()
	}

	// Get the input type
	inputType := reflect.TypeOf(input)
	if inputType == nil {
		return fmt.Errorf("input is nil, expected %s", schemaType.Name())
	}

	// Handle pointer types in input
	if inputType.Kind() == reflect.Ptr {
		inputType = inputType.Elem()
	}

	// Check if types match
	if inputType != schemaType {
		return fmt.Errorf("input type mismatch: expected %s, got %s", schemaType.Name(), inputType.Name())
	}

	// If input is a struct, validate required fields
	if schemaType.Kind() == reflect.Struct {
		inputValue := reflect.ValueOf(input)
		if inputValue.Kind() == reflect.Ptr {
			inputValue = inputValue.Elem()
		}

		// Iterate through struct fields
		for i := 0; i < schemaType.NumField(); i++ {
			field := schemaType.Field(i)
			fieldValue := inputValue.Field(i)

			// Check for required tag
			if tag := field.Tag.Get("validate"); tag == "required" {
				// Check if field is zero value
				if fieldValue.IsZero() {
					return fmt.Errorf("required field '%s' is missing or zero", field.Name)
				}
			}
		}
	}

	return nil
}

// GetMetrics returns the workflow metrics
func (w *Workflow) GetMetrics() *WorkflowMetrics {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.metrics
}

// PrintResponse prints the workflow response in a formatted way
func (w *Workflow) PrintResponse(input interface{}, markdown bool) {
	utils.SetMarkdownMode(markdown)

	if w.Stream {
		w.printStreamingResponse(input, markdown)
	} else {
		w.printStaticResponse(input, markdown)
	}
}

// printStaticResponse prints a static workflow response
func (w *Workflow) printStaticResponse(input interface{}, markdown bool) {
	start := time.Now()
	ctx := context.Background()

	// Thinking panel
	spinnerResponse := utils.ThinkingPanel("Executing workflow...")

	response, err := w.Run(ctx, input)
	if err != nil {
		utils.ErrorPanel(err)
		return
	}

	// Response panel
	content := ""
	if response.Content != nil {
		switch v := response.Content.(type) {
		case string:
			content = v
		default:
			// Try to extract text content from complex objects
			if contentMap, ok := v.(map[string]interface{}); ok {
				if textContent, exists := contentMap["text_content"]; exists {
					if textStr, ok := textContent.(string); ok {
						content = textStr
					} else {
						content = fmt.Sprintf("%v", textContent)
					}
				} else if parsedOutput, exists := contentMap["parsed_output"]; exists {
					if parsedStr, ok := parsedOutput.(string); ok {
						content = parsedStr
					} else {
						content = fmt.Sprintf("%v", parsedOutput)
					}
				} else {
					// Fallback: try to find any string field
					for _, value := range contentMap {
						if strValue, ok := value.(string); ok && len(strValue) > 10 {
							content = strValue
							break
						}
					}
					if content == "" {
						data, _ := json.MarshalIndent(v, "", "  ")
						content = string(data)
					}
				}
			} else {
				content = fmt.Sprintf("%v", v)
			}
		}
	}

	utils.ResponsePanel(content, spinnerResponse, start, markdown)
}

// printStreamingResponse prints a streaming workflow response
func (w *Workflow) printStreamingResponse(input interface{}, markdown bool) {
	start := time.Now()
	ctx := context.Background()

	// Thinking panel
	spinnerResponse := utils.ThinkingPanel("Executing workflow...")

	// Start streaming panel
	contentChan := utils.StartSimplePanel(spinnerResponse, start, markdown)

	// Register event handlers for streaming
	w.OnEvent(StepCompletedEvent, func(event *WorkflowRunResponseEvent) {
		if event.Data != nil {
			stepOutput, ok := event.Data.(*StepOutput)
			if ok && stepOutput.Content != nil {
				var content string
				switch v := stepOutput.Content.(type) {
				case string:
					content = v
				default:
					data, _ := json.Marshal(v)
					content = string(data)
				}

				contentChan <- utils.ContentUpdateMsg{
					PanelName: utils.MessageResponse,
					Content:   content,
				}
			}
		}
	})

	// Run workflow
	response, err := w.Run(ctx, input)
	if err != nil {
		contentChan <- utils.ContentUpdateMsg{
			PanelName: utils.MessageError,
			Content:   err.Error(),
		}
		close(contentChan)
		return
	}

	// Send final response if not already streamed
	if response.Content != nil && !w.StreamIntermediateSteps {
		var content string
		switch v := response.Content.(type) {
		case string:
			content = v
		default:
			data, _ := json.MarshalIndent(v, "", "  ")
			content = string(data)
		}

		contentChan <- utils.ContentUpdateMsg{
			PanelName: utils.MessageResponse,
			Content:   content,
		}
	}

	close(contentChan)

	// Small delay to ensure final rendering
	time.Sleep(100 * time.Millisecond)
}
