package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func main() {
	// ===== Exemplo 1: Agente com Reasoning e Persistência SQLite =====
	fmt.Println("=== Exemplo 1: Agente com Reasoning e Persistência SQLite ===\n")
	exampleWithSQLite()

	// ===== Exemplo 2: Agente com Reasoning e Persistência PostgreSQL =====
	fmt.Println("\n=== Exemplo 2: Agente com Reasoning e Persistência PostgreSQL (Configuração) ===\n")
	exampleWithPostgreSQL()

	// ===== Exemplo 3: Agente com Reasoning e Persistência via Variáveis de Ambiente =====
	fmt.Println("\n=== Exemplo 3: Agente com Reasoning e Persistência via Variáveis de Ambiente ===\n")
	exampleWithEnvironment()
}

// exampleWithSQLite demonstra como usar reasoning persistence com SQLite
func exampleWithSQLite() {
	ctx := context.Background()

	// Criar configuração de persistência SQLite
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeSQLite,
		Database: "/tmp/agno_reasoning.db", // Arquivo local
	}

	// Criar instância de persistência usando a factory
	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		log.Fatalf("Erro ao criar persistência: %v", err)
	}

	fmt.Println("✓ Persistência SQLite criada com sucesso")
	fmt.Printf("  Tipo: %T\n", persistence)

	// Criar modelo Ollama Cloud
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
		log.Fatalf("Erro ao criar modelo Ollama Cloud: %v", err)
	}

	// Criar agente com reasoning e persistência integrada
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:              ctx,
		Model:                model,
		Name:                 "Reasoning Agent",
		Role:                 "Assistant",
		Description:          "Um agente que usa reasoning para resolver problemas complexos",
		Instructions:         "Pense passo a passo e explique seu raciocínio",
		Reasoning:            true,  // Ativar reasoning
		ReasoningModel:       model, // Usar o mesmo modelo para reasoning
		ReasoningMinSteps:    1,
		ReasoningMaxSteps:    3,
		ReasoningPersistence: persistence, // Integrar persistência diretamente
	})

	if err != nil {
		log.Fatalf("Erro ao criar agente: %v", err)
	}

	fmt.Println("✓ Agente com reasoning e persistência criado com sucesso")

	// Usar o agente
	prompt := "Qual é a capital da França?"

	fmt.Printf("\nExecutando agente com prompt: '%s'\n", prompt)
	response, err := ag.Run(prompt)
	if err != nil {
		log.Fatalf("Erro ao executar agente: %v", err)
	}

	fmt.Printf("\nResposta do agente:\n%s\n", response.TextContent)

	// A persistência está integrada no agente e pode ser usada internamente
	fmt.Println("\n✓ Reasoning persistence integrada no agente")
	fmt.Printf("  Persistência disponível: %T\n", persistence)
}

// exampleWithPostgreSQL demonstra como configurar reasoning persistence com PostgreSQL
func exampleWithPostgreSQL() {
	// Configuração para PostgreSQL
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypePostgreSQL,
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "password",
		Database: "agno",
		SSLMode:  "disable",
	}

	fmt.Println("Configuração PostgreSQL:")
	fmt.Printf("  Host: %s\n", config.Host)
	fmt.Printf("  Port: %d\n", config.Port)
	fmt.Printf("  Database: %s\n", config.Database)
	fmt.Printf("  User: %s\n", config.User)

	// Criar persistência (retornará erro se PostgreSQL não estiver disponível)
	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		fmt.Printf("  ⚠ Erro esperado (PostgreSQL não configurado): %v\n", err)
		fmt.Println("  Para usar PostgreSQL em produção:")
		fmt.Println("    1. Instale: go get github.com/vingarcia/ksql")
		fmt.Println("    2. Configure um servidor PostgreSQL")
		fmt.Println("    3. Implemente os métodos de persistência")
	} else {
		fmt.Printf("✓ Persistência PostgreSQL criada: %T\n", persistence)
	}
}

// exampleWithEnvironment demonstra como usar variáveis de ambiente
func exampleWithEnvironment() {
	ctx := context.Background()

	// Configurar variáveis de ambiente (ou usar as já existentes)
	os.Setenv("DB_TYPE", "sqlite")
	os.Setenv("DB_NAME", "/tmp/agno_reasoning_env.db")

	// Criar configuração a partir de variáveis de ambiente
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseType(os.Getenv("DB_TYPE")),
		Database: os.Getenv("DB_NAME"),
		Host:     os.Getenv("DB_HOST"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
	}

	fmt.Println("Configuração a partir de variáveis de ambiente:")
	fmt.Printf("  DB_TYPE: %s\n", config.Type)
	fmt.Printf("  DB_NAME: %s\n", config.Database)

	// Criar persistência
	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		log.Fatalf("Erro ao criar persistência: %v", err)
	}

	fmt.Printf("✓ Persistência criada: %T\n", persistence)

	// Criar modelo Ollama Cloud
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
		log.Fatalf("Erro ao criar modelo Ollama Cloud: %v", err)
	}

	// Criar agente com persistência integrada
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:              ctx,
		Model:                model,
		Name:                 "Environment Agent",
		Reasoning:            true,
		ReasoningModel:       model,
		ReasoningMinSteps:    1,
		ReasoningMaxSteps:    2,
		ReasoningPersistence: persistence, // Integrar persistência
	})

	if err != nil {
		log.Fatalf("Erro ao criar agente: %v", err)
	}

	fmt.Println("✓ Agente com persistência criado com sucesso")

	// Usar o agente
	prompt := "Qual é 2 + 2?"
	fmt.Printf("\nExecutando agente com prompt: '%s'\n", prompt)

	response, err := ag.Run(prompt)
	if err != nil {
		log.Fatalf("Erro ao executar agente: %v", err)
	}

	fmt.Printf("\nResposta:\n%s\n", response.TextContent)
}

// Exemplo de como integrar reasoning persistence em um workflow
func exampleIntegrationWorkflow() {
	ctx := context.Background()

	// 1. Criar persistência
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeSQLite,
		Database: "/tmp/agno_workflow.db",
	}

	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		log.Fatalf("Erro ao criar persistência: %v", err)
	}

	// 2. Criar modelo Ollama Cloud
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
		log.Fatalf("Erro ao criar modelo Ollama Cloud: %v", err)
	}

	// 3. Criar agente com reasoning e persistência integrada
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:              ctx,
		Model:                model,
		Name:                 "Workflow Agent",
		Reasoning:            true,
		ReasoningModel:       model,
		ReasoningMinSteps:    1,
		ReasoningMaxSteps:    3,
		ReasoningPersistence: persistence, // Integrar persistência
	})

	if err != nil {
		log.Fatalf("Erro ao criar agente: %v", err)
	}

	// 4. Executar agente múltiplas vezes
	prompts := []string{
		"Qual é a capital da Itália?",
		"Qual é a população do Brasil?",
		"Qual é o maior planeta do sistema solar?",
	}

	for i, prompt := range prompts {
		fmt.Printf("\n--- Execução %d ---\n", i+1)
		fmt.Printf("Prompt: %s\n", prompt)

		response, err := ag.Run(prompt)
		if err != nil {
			fmt.Printf("Erro: %v\n", err)
			continue
		}

		fmt.Printf("Resposta: %s\n", response.TextContent)

		// A persistência está integrada no agente
		fmt.Printf("Reasoning steps armazenados na persistência\n")
	}
}

// Exemplo de como recuperar reasoning history
func exampleRetrieveReasoningHistory() {
	ctx := context.Background()

	// Criar persistência
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeSQLite,
		Database: "/tmp/agno_reasoning.db",
	}

	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		log.Fatalf("Erro ao criar persistência: %v", err)
	}

	// Recuperar histórico de reasoning
	runID := "run-001"
	history, err := persistence.GetReasoningHistory(ctx, runID)
	if err != nil {
		fmt.Printf("Histórico não encontrado para run %s: %v\n", runID, err)
		return
	}

	fmt.Printf("Histórico de Reasoning para %s:\n", runID)
	fmt.Printf("  Status: %s\n", history.Status)
	fmt.Printf("  Total de Steps: %d\n", len(history.Steps))
	fmt.Printf("  Total de Tokens: %d\n", history.TotalTokens)
	fmt.Printf("  Reasoning Tokens: %d\n", history.ReasoningTokens)

	// Listar todos os steps
	for _, step := range history.Steps {
		fmt.Printf("\n  Step %d: %s\n", step.StepNumber, step.Title)
		fmt.Printf("    Raciocínio: %s\n", step.Reasoning)
		fmt.Printf("    Ação: %s\n", step.Action)
		fmt.Printf("    Resultado: %s\n", step.Result)
		fmt.Printf("    Confiança: %.2f\n", step.Confidence)
	}
}
