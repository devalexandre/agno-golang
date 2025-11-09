package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// MathToolkit exemplo de toolkit com operações matemáticas
type MathToolkit struct {
	toolkit.Toolkit
}

// MathParams parâmetros para operações matemáticas
type MathParams struct {
	A float64 `json:"a" description:"Primeiro número"`
	B float64 `json:"b" description:"Segundo número"`
}

// Add soma dois números
func (mt *MathToolkit) Add(params MathParams) (float64, error) {
	return params.A + params.B, nil
}

// Subtract subtrai dois números
func (mt *MathToolkit) Subtract(params MathParams) (float64, error) {
	return params.A - params.B, nil
}

// Multiply multiplica dois números
func (mt *MathToolkit) Multiply(params MathParams) (float64, error) {
	return params.A * params.B, nil
}

// Divide divide dois números
func (mt *MathToolkit) Divide(params MathParams) (float64, error) {
	if params.B == 0 {
		return 0, fmt.Errorf("divisão por zero")
	}
	return params.A / params.B, nil
}

// StringToolkit exemplo de toolkit com operações de string
type StringToolkit struct {
	toolkit.Toolkit
}

// StringParams parâmetros para operações de string
type StringParams struct {
	Text string `json:"text" description:"Texto para processar"`
}

// Uppercase converte texto para maiúsculas
func (st *StringToolkit) Uppercase(params StringParams) (string, error) {
	return fmt.Sprintf("UPPERCASE: %s", params.Text), nil
}

// Lowercase converte texto para minúsculas
func (st *StringToolkit) Lowercase(params StringParams) (string, error) {
	return fmt.Sprintf("lowercase: %s", params.Text), nil
}

// Reverse inverte o texto
func (st *StringToolkit) Reverse(params StringParams) (string, error) {
	runes := []rune(params.Text)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return fmt.Sprintf("reversed: %s", string(runes)), nil
}

func main() {
	ctx := context.Background()

	// Usar Ollama Cloud (descomente para usar)
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatalf("OLLAMA_API_KEY não configurada")
	}
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama Cloud model: %v", err)
	}

	// Criar toolkits
	mathToolkit := &MathToolkit{
		Toolkit: toolkit.NewToolkit(),
	}
	mathToolkit.Name = "math"
	mathToolkit.Description = "Operações matemáticas básicas"

	stringToolkit := &StringToolkit{
		Toolkit: toolkit.NewToolkit(),
	}
	stringToolkit.Name = "string"
	stringToolkit.Description = "Operações com strings"

	// Register math methods using exact Go method names
	mathToolkit.Register("Add", "Add two numbers", mathToolkit, mathToolkit.Add, MathParams{})
	mathToolkit.Register("Subtract", "Subtract the second number from the first", mathToolkit, mathToolkit.Subtract, MathParams{})
	mathToolkit.Register("Multiply", "Multiply two numbers", mathToolkit, mathToolkit.Multiply, MathParams{})
	mathToolkit.Register("Divide", "Divide the first number by the second (returns error on division by zero)", mathToolkit, mathToolkit.Divide, MathParams{})

	// Register string methods using exact Go method names
	stringToolkit.Register("Uppercase", "Convert a string to uppercase", stringToolkit, stringToolkit.Uppercase, StringParams{})
	stringToolkit.Register("Lowercase", "Convert a string to lowercase", stringToolkit, stringToolkit.Lowercase, StringParams{})
	stringToolkit.Register("Reverse", "Reverse the characters in a string", stringToolkit, stringToolkit.Reverse, StringParams{})

	// Criar agent com ferramentas
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:     ctx,
		Model:       model,
		Name:        "Advanced Tool Calling Agent",
		Description: "Agent que demonstra tool calling avançado",
		Tools: []toolkit.Tool{
			mathToolkit,
			stringToolkit,
		},
		Debug: true,
	})
	if err != nil {
		log.Fatalf("Erro ao criar agent: %v", err)
	}

	// Exemplo 1: Execução paralela de múltiplas chamadas
	fmt.Println("\n=== Exemplo 1: Execução Paralela ===")
	demonstrateParallelExecution(ag)

	// Exemplo 2: Retry com backoff exponencial
	fmt.Println("\n=== Exemplo 2: Retry com Backoff Exponencial ===")
	demonstrateRetryWithBackoff(ag)

	// Exemplo 3: Validação de argumentos
	fmt.Println("\n=== Exemplo 3: Validação de Argumentos ===")
	demonstrateArgumentValidation(ag)

	// Exemplo 4: Tratamento de erros
	fmt.Println("\n=== Exemplo 4: Tratamento de Erros ===")
	demonstrateErrorHandling(ag)

	// Exemplo 5: Batch de chamadas
	fmt.Println("\n=== Exemplo 5: Batch de Chamadas ===")
	demonstrateBatchExecution(ag)
}

// demonstrateParallelExecution demonstra execução paralela de ferramentas
func demonstrateParallelExecution(ag *agent.Agent) {
	ctx := context.Background()

	// Criar múltiplas requisições
	requests := []agent.ToolCallRequest{
		{
			ToolName:   "math",
			MethodName: "Add",
			Arguments:  json.RawMessage(`{"a": 5, "b": 3}`),
		},
		{
			ToolName:   "math",
			MethodName: "Multiply",
			Arguments:  json.RawMessage(`{"a": 3, "b": 4}`),
		},
		{
			ToolName:   "string",
			MethodName: "Uppercase",
			Arguments:  json.RawMessage(`{"text": "hello world"}`),
		},
		{
			ToolName:   "math",
			MethodName: "Divide",
			Arguments:  json.RawMessage(`{"a": 20, "b": 4}`),
		},
	}

	config := agent.ToolCallConfig{
		MaxParallelCalls:      2,
		RetryAttempts:         0,
		ValidateArguments:     true,
		UseExponentialBackoff: false,
	}

	fmt.Println("Executando 4 chamadas em paralelo (máx 2 simultâneas)...")
	results := ag.ExecuteToolCallsParallel(ctx, requests, config)

	for i, result := range results {
		fmt.Printf("\nChamada %d: %s.%s\n", i+1, result.ToolName, result.MethodName)
		fmt.Printf("  Sucesso: %v\n", result.Success)
		fmt.Printf("  Duração: %v\n", result.Duration)
		if result.Success {
			fmt.Printf("  Resultado: %v\n", result.Result)
		} else {
			fmt.Printf("  Erro: %v\n", result.Error)
		}
	}

	// Exibir estatísticas
	stats := agent.GetToolCallStats(results)
	fmt.Printf("\nEstatísticas:\n")
	fmt.Printf("  Total: %d\n", stats["total_calls"])
	fmt.Printf("  Sucesso: %d\n", stats["successful"])
	fmt.Printf("  Falhas: %d\n", stats["failed"])
	fmt.Printf("  Duração total: %v\n", stats["total_duration"])
	fmt.Printf("  Duração média: %v\n", stats["average_duration"])
}

// demonstrateRetryWithBackoff demonstra retry com backoff exponencial
func demonstrateRetryWithBackoff(ag *agent.Agent) {
	ctx := context.Background()

	requests := []agent.ToolCallRequest{
		{
			ToolName:   "math",
			MethodName: "Add",
			Arguments:  json.RawMessage(`{"a": 100, "b": 200}`),
		},
	}

	config := agent.ToolCallConfig{
		MaxParallelCalls:      1,
		RetryAttempts:         2,
		RetryDelay:            100, // 100ms
		ValidateArguments:     true,
		UseExponentialBackoff: true,
	}

	fmt.Println("Executando com retry e backoff exponencial...")
	results := ag.ExecuteToolCallsParallel(ctx, requests, config)

	for _, result := range results {
		fmt.Printf("Tentativa: %d\n", result.Attempt)
		fmt.Printf("Sucesso: %v\n", result.Success)
		fmt.Printf("Duração: %v\n", result.Duration)
		if result.Success {
			fmt.Printf("Resultado: %v\n", result.Result)
		}
	}
}

// demonstrateArgumentValidation demonstra validação de argumentos
func demonstrateArgumentValidation(ag *agent.Agent) {
	ctx := context.Background()

	// Teste 1: Argumentos válidos
	fmt.Println("Teste 1: Argumentos válidos")
	validRequests := []agent.ToolCallRequest{
		{
			ToolName:   "math",
			MethodName: "Add",
			Arguments:  json.RawMessage(`{"a": 10, "b": 5}`),
		},
	}

	config := agent.ToolCallConfig{
		MaxParallelCalls:  1,
		RetryAttempts:     0,
		ValidateArguments: true,
	}

	results := ag.ExecuteToolCallsParallel(ctx, validRequests, config)
	for _, result := range results {
		fmt.Printf("  Sucesso: %v, Resultado: %v\n", result.Success, result.Result)
	}

	// Teste 2: Argumentos inválidos (validação deve rejeitar)
	fmt.Println("\nTeste 2: Argumentos inválidos (esperado falhar na validação)")
	invalidRequests := []agent.ToolCallRequest{
		{
			ToolName:   "math",
			MethodName: "Add",
			Arguments:  json.RawMessage(`{"a": "não é número", "b": 5}`),
		},
	}

	results = ag.ExecuteToolCallsParallel(ctx, invalidRequests, config)
	for _, result := range results {
		fmt.Printf("  Sucesso: %v\n", result.Success)
		if !result.Success {
			fmt.Printf("  Erro de validação (esperado): %v\n", result.Error)
		}
	}
}

// demonstrateErrorHandling demonstra tratamento de erros
func demonstrateErrorHandling(ag *agent.Agent) {
	ctx := context.Background()

	// Requisição que causará erro (divisão por zero)
	requests := []agent.ToolCallRequest{
		{
			ToolName:   "math",
			MethodName: "Divide",
			Arguments:  json.RawMessage(`{"a": 10, "b": 0}`),
		},
	}

	config := agent.ToolCallConfig{
		MaxParallelCalls:  1,
		RetryAttempts:     0,
		ValidateArguments: false,
	}

	fmt.Println("Executando operação que causará erro...")
	results := ag.ExecuteToolCallsParallel(ctx, requests, config)

	handler := agent.NewDefaultToolCallErrorHandler(true)

	for _, result := range results {
		err := handler.HandleError(result)
		if err != nil {
			fmt.Printf("Erro tratado: %v\n", err)
		}
	}
}

// demonstrateBatchExecution demonstra execução em batch
func demonstrateBatchExecution(ag *agent.Agent) {
	ctx := context.Background()

	batch := &agent.ToolCallBatch{
		ID: "batch-001",
		Requests: []agent.ToolCallRequest{
			{
				ToolName:   "math",
				MethodName: "Add",
				Arguments:  json.RawMessage(`{"a": 5, "b": 3}`),
			},
			{
				ToolName:   "math",
				MethodName: "Multiply",
				Arguments:  json.RawMessage(`{"a": 4, "b": 2}`),
			},
			{
				ToolName:   "string",
				MethodName: "Uppercase",
				Arguments:  json.RawMessage(`{"text": "batch processing"}`),
			},
		},
		Config: agent.ToolCallConfig{
			MaxParallelCalls:  2,
			RetryAttempts:     1,
			ValidateArguments: true,
		},
	}

	fmt.Println("Executando batch de chamadas...")
	err := ag.ExecuteToolCallBatch(ctx, batch)

	fmt.Printf("Status do batch: %s\n", batch.Status)
	fmt.Printf("Duração total: %v\n", batch.Duration)

	if err != nil {
		fmt.Printf("Erro no batch: %v\n", err)
	}

	for i, result := range batch.Results {
		fmt.Printf("\nResultado %d:\n", i+1)
		fmt.Printf("  Ferramenta: %s.%s\n", result.ToolName, result.MethodName)
		fmt.Printf("  Sucesso: %v\n", result.Success)
		if result.Success {
			fmt.Printf("  Resultado: %v\n", result.Result)
		} else {
			fmt.Printf("  Erro: %v\n", result.Error)
		}
	}
}
