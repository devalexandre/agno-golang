package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// ToolCallConfig configura o comportamento de chamadas de ferramentas
type ToolCallConfig struct {
	// MaxParallelCalls define o número máximo de chamadas paralelas
	MaxParallelCalls int
	// RetryAttempts define o número de tentativas de retry
	RetryAttempts int
	// RetryDelay define o delay inicial entre retries em milissegundos
	RetryDelay int
	// UseExponentialBackoff ativa backoff exponencial
	UseExponentialBackoff bool
	// ValidateArguments ativa validação de argumentos
	ValidateArguments bool
	// TimeoutPerCall define timeout por chamada em segundos
	TimeoutPerCall int
}

// ToolCallResult representa o resultado de uma chamada de ferramenta
type ToolCallResult struct {
	ToolName   string
	MethodName string
	Arguments  map[string]interface{}
	Result     interface{}
	Error      error
	Duration   time.Duration
	Attempt    int
	Success    bool
}

// ToolCallRequest representa uma requisição de chamada de ferramenta
type ToolCallRequest struct {
	ToolName   string
	MethodName string
	Arguments  json.RawMessage
}

// ToolArgumentValidator define a interface para validação de argumentos
type ToolArgumentValidator interface {
	ValidateArguments(toolName, methodName string, args map[string]interface{}) error
}

// DefaultToolArgumentValidator implementa validação básica de argumentos
type DefaultToolArgumentValidator struct {
	tools []toolkit.Tool
}

// NewDefaultToolArgumentValidator cria um novo validador padrão
func NewDefaultToolArgumentValidator(tools []toolkit.Tool) *DefaultToolArgumentValidator {
	return &DefaultToolArgumentValidator{
		tools: tools,
	}
}

// ValidateArguments valida os argumentos contra o schema da ferramenta
func (v *DefaultToolArgumentValidator) ValidateArguments(toolName, methodName string, args map[string]interface{}) error {
	// Encontrar a ferramenta
	var tool toolkit.Tool
	for _, t := range v.tools {
		if t.GetName() == toolName {
			tool = t
			break
		}
	}

	if tool == nil {
		return fmt.Errorf("tool '%s' not found", toolName)
	}

	// Obter o schema da ferramenta
	fullMethodName := toolName + "_" + methodName
	schema := tool.GetParameterStruct(fullMethodName)

	// Validar campos obrigatórios
	if required, ok := schema["required"].([]string); ok {
		for _, field := range required {
			if _, exists := args[field]; !exists {
				return fmt.Errorf("required argument '%s' is missing for %s.%s", field, toolName, methodName)
			}
		}
	}

	// Validar tipos de argumentos
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for argName, argValue := range args {
			if prop, exists := properties[argName]; exists {
				if propMap, ok := prop.(map[string]interface{}); ok {
					if expectedType, ok := propMap["type"].(string); ok {
						if !validateArgumentType(argValue, expectedType) {
							return fmt.Errorf("argument '%s' has invalid type for %s.%s, expected %s", argName, toolName, methodName, expectedType)
						}
					}
				}
			}
		}
	}

	return nil
}

// validateArgumentType valida o tipo de um argumento
func validateArgumentType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		switch value.(type) {
		case float64, int, int64, float32:
			return true
		case string:
			// Tentar converter string para número
			_, err := json.Number(value.(string)).Float64()
			return err == nil
		}
		return false
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		return true
	}
}

// ExecuteToolCallsParallel executa múltiplas chamadas de ferramentas em paralelo
func (a *Agent) ExecuteToolCallsParallel(ctx context.Context, requests []ToolCallRequest, config ToolCallConfig) []ToolCallResult {
	if config.MaxParallelCalls <= 0 {
		config.MaxParallelCalls = 5 // Default
	}

	results := make([]ToolCallResult, len(requests))
	semaphore := make(chan struct{}, config.MaxParallelCalls)
	var wg sync.WaitGroup

	for i, req := range requests {
		wg.Add(1)
		go func(index int, request ToolCallRequest) {
			defer wg.Done()

			// Adquirir slot do semáforo
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Executar com timeout
			callCtx := ctx
			if config.TimeoutPerCall > 0 {
				var cancel context.CancelFunc
				callCtx, cancel = context.WithTimeout(ctx, time.Duration(config.TimeoutPerCall)*time.Second)
				defer cancel()
			}

			result := a.executeToolCallWithRetry(callCtx, request, config)
			results[index] = result
		}(i, req)
	}

	wg.Wait()
	return results
}

// executeToolCallWithRetry executa uma chamada de ferramenta com retry automático
func (a *Agent) executeToolCallWithRetry(ctx context.Context, req ToolCallRequest, config ToolCallConfig) ToolCallResult {
	result := ToolCallResult{
		ToolName:   req.ToolName,
		MethodName: req.MethodName,
		Attempt:    0,
	}

	// Parse argumentos
	var args map[string]interface{}
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		result.Error = fmt.Errorf("failed to parse arguments: %w", err)
		return result
	}

	result.Arguments = args

	// Validar argumentos se configurado
	if config.ValidateArguments {
		validator := NewDefaultToolArgumentValidator(a.tools)
		if err := validator.ValidateArguments(req.ToolName, req.MethodName, args); err != nil {
			result.Error = fmt.Errorf("argument validation failed: %w", err)
			return result
		}
	}

	// Executar com retry
	maxAttempts := config.RetryAttempts + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		result.Attempt = attempt + 1

		// Verificar contexto cancelado
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result
		default:
		}

		// Executar chamada de ferramenta
		start := time.Now()
		toolResult, err := a.executeToolCall(ctx, req)
		result.Duration = time.Since(start)

		if err == nil {
			result.Result = toolResult
			result.Success = true
			return result
		}

		result.Error = err

		// Se foi a última tentativa, retornar erro
		if attempt == maxAttempts-1 {
			return result
		}

		// Calcular delay para próxima tentativa
		delay := calculateRetryDelay(attempt, config.RetryDelay, config.UseExponentialBackoff)

		// Aguardar antes de retry
		select {
		case <-time.After(delay):
			// Continuar para próxima tentativa
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result
		}
	}

	return result
}

// executeToolCall executa uma única chamada de ferramenta
func (a *Agent) executeToolCall(ctx context.Context, req ToolCallRequest) (interface{}, error) {
	// Encontrar a ferramenta
	var tool toolkit.Tool
	for _, t := range a.tools {
		if t.GetName() == req.ToolName {
			tool = t
			break
		}
	}

	if tool == nil {
		return nil, fmt.Errorf("tool '%s' not found", req.ToolName)
	}

	// Executar a ferramenta
	fullMethodName := req.ToolName + "_" + req.MethodName
	result, err := tool.Execute(fullMethodName, req.Arguments)

	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	return result, nil
}

// calculateRetryDelay calcula o delay para retry com backoff exponencial opcional
func calculateRetryDelay(attempt int, baseDelayMs int, useExponentialBackoff bool) time.Duration {
	if baseDelayMs <= 0 {
		baseDelayMs = 100 // Default 100ms
	}

	delay := baseDelayMs
	if useExponentialBackoff {
		// Backoff exponencial: 2^attempt * baseDelay
		delay = baseDelayMs * int(math.Pow(2, float64(attempt)))
	}

	// Adicionar jitter (±10%)
	jitter := int(float64(delay) * 0.1)
	delay = delay - jitter/2 + (jitter / 2)

	return time.Duration(delay) * time.Millisecond
}

// ExecuteToolCallsSequential executa chamadas de ferramentas sequencialmente
func (a *Agent) ExecuteToolCallsSequential(ctx context.Context, requests []ToolCallRequest, config ToolCallConfig) []ToolCallResult {
	results := make([]ToolCallResult, len(requests))

	for i, req := range requests {
		// Verificar contexto cancelado
		select {
		case <-ctx.Done():
			results[i] = ToolCallResult{
				ToolName:   req.ToolName,
				MethodName: req.MethodName,
				Error:      ctx.Err(),
			}
			continue
		default:
		}

		results[i] = a.executeToolCallWithRetry(ctx, req, config)
	}

	return results
}

// ToolCallBatch agrupa múltiplas chamadas de ferramentas
type ToolCallBatch struct {
	ID       string
	Requests []ToolCallRequest
	Results  []ToolCallResult
	Config   ToolCallConfig
	Status   string // "pending", "running", "completed", "failed"
	Error    error
	Duration time.Duration
}

// ExecuteToolCallBatch executa um batch de chamadas de ferramentas
func (a *Agent) ExecuteToolCallBatch(ctx context.Context, batch *ToolCallBatch) error {
	batch.Status = "running"
	start := time.Now()

	// Executar em paralelo
	batch.Results = a.ExecuteToolCallsParallel(ctx, batch.Requests, batch.Config)

	batch.Duration = time.Since(start)

	// Verificar se houve erros
	hasErrors := false
	for _, result := range batch.Results {
		if result.Error != nil {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		batch.Status = "failed"
		batch.Error = fmt.Errorf("some tool calls failed")
	} else {
		batch.Status = "completed"
	}

	return batch.Error
}

// GetToolCallStats retorna estatísticas sobre as chamadas de ferramentas
func GetToolCallStats(results []ToolCallResult) map[string]interface{} {
	stats := map[string]interface{}{
		"total_calls":      len(results),
		"successful":       0,
		"failed":           0,
		"total_duration":   time.Duration(0),
		"average_duration": time.Duration(0),
		"max_duration":     time.Duration(0),
		"min_duration":     time.Duration(0),
		"total_retries":    0,
	}

	if len(results) == 0 {
		return stats
	}

	var totalDuration time.Duration
	minDuration := time.Duration(math.MaxInt64)
	maxDuration := time.Duration(0)

	for _, result := range results {
		if result.Success {
			stats["successful"] = stats["successful"].(int) + 1
		} else {
			stats["failed"] = stats["failed"].(int) + 1
		}

		totalDuration += result.Duration
		if result.Duration > maxDuration {
			maxDuration = result.Duration
		}
		if result.Duration < minDuration {
			minDuration = result.Duration
		}

		stats["total_retries"] = stats["total_retries"].(int) + (result.Attempt - 1)
	}

	stats["total_duration"] = totalDuration
	if len(results) > 0 {
		stats["average_duration"] = totalDuration / time.Duration(len(results))
	}
	stats["max_duration"] = maxDuration
	stats["min_duration"] = minDuration

	return stats
}

// ToolCallErrorHandler define a interface para tratamento de erros de ferramentas
type ToolCallErrorHandler interface {
	HandleError(result ToolCallResult) error
}

// DefaultToolCallErrorHandler implementa tratamento padrão de erros
type DefaultToolCallErrorHandler struct {
	debug bool
}

// NewDefaultToolCallErrorHandler cria um novo handler padrão
func NewDefaultToolCallErrorHandler(debug bool) *DefaultToolCallErrorHandler {
	return &DefaultToolCallErrorHandler{
		debug: debug,
	}
}

// HandleError trata erros de chamadas de ferramentas
func (h *DefaultToolCallErrorHandler) HandleError(result ToolCallResult) error {
	if result.Success {
		return nil
	}

	errorMsg := fmt.Sprintf("Tool call failed: %s.%s", result.ToolName, result.MethodName)

	if result.Error != nil {
		errorMsg += fmt.Sprintf(" - Error: %v", result.Error)
	}

	if h.debug {
		errorMsg += fmt.Sprintf(" (Attempt: %d, Duration: %v)", result.Attempt, result.Duration)
	}

	return fmt.Errorf(errorMsg)
}

// ValidateToolCallResponse valida a resposta de uma chamada de ferramenta
func ValidateToolCallResponse(result interface{}, expectedType string) error {
	if result == nil {
		return fmt.Errorf("tool call returned nil result")
	}

	// Validação básica de tipo
	switch expectedType {
	case "string":
		if _, ok := result.(string); !ok {
			return fmt.Errorf("expected string result, got %T", result)
		}
	case "number":
		switch result.(type) {
		case float64, int, int64, float32:
			// OK
		default:
			return fmt.Errorf("expected number result, got %T", result)
		}
	case "boolean":
		if _, ok := result.(bool); !ok {
			return fmt.Errorf("expected boolean result, got %T", result)
		}
	case "object":
		if _, ok := result.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object result, got %T", result)
		}
	case "array":
		if _, ok := result.([]interface{}); !ok {
			return fmt.Errorf("expected array result, got %T", result)
		}
	}

	return nil
}
