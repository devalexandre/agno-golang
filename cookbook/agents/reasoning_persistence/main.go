package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func main() {
	// Criar persistência usando a factory pattern
	// Agora o usuário não precisa se preocupar com drivers específicos
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeSQLite,
		Database: ":memory:", // Banco de dados em memória
	}

	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		log.Fatalf("Failed to create persistence: %v", err)
	}

	ctx := context.Background()

	// ===== Exemplo 1: Salvar Reasoning Steps =====
	fmt.Println("=== Exemplo 1: Salvar Reasoning Steps ===")
	runID := "run-001"
	agentID := "agent-reasoning-001"

	for i := 1; i <= 5; i++ {
		step := reasoning.ReasoningStepRecord{
			RunID:           runID,
			AgentID:         agentID,
			StepNumber:      i,
			Title:           fmt.Sprintf("Analysis Step %d", i),
			Reasoning:       fmt.Sprintf("Analyzing problem from angle %d", i),
			Action:          "analyze",
			Result:          fmt.Sprintf("Found insight %d", i),
			Confidence:      0.75 + float64(i)*0.05,
			NextAction:      "continue",
			ReasoningTokens: 100 * i,
			InputTokens:     30 * i,
			OutputTokens:    70 * i,
			Duration:        int64(1000 * i),
			Metadata: map[string]interface{}{
				"model":       "o1",
				"temperature": 0.7,
				"step_type":   "analysis",
			},
		}

		err := persistence.SaveReasoningStep(ctx, step)
		if err != nil {
			log.Printf("Error saving step %d: %v", i, err)
			continue
		}

		fmt.Printf("✓ Step %d salvo com sucesso (ID: %d)\n", i, step.ID)
	}

	// ===== Exemplo 2: Listar Reasoning Steps =====
	fmt.Println("\n=== Exemplo 2: Listar Reasoning Steps ===")
	steps, err := persistence.ListReasoningSteps(ctx, runID)
	if err != nil {
		log.Fatalf("Error listing steps: %v", err)
	}

	fmt.Printf("Total de steps: %d\n", len(steps))
	for _, step := range steps {
		fmt.Printf("  Step %d: %s (Confiança: %.2f, Tokens: %d)\n",
			step.StepNumber, step.Title, step.Confidence, step.ReasoningTokens)
	}

	// ===== Exemplo 3: Atualizar Histórico =====
	fmt.Println("\n=== Exemplo 3: Atualizar Histórico ===")
	history := reasoning.ReasoningHistory{
		ID:              "history-001",
		RunID:           runID,
		AgentID:         agentID,
		TotalTokens:     1500,
		ReasoningTokens: 750,
		InputTokens:     150,
		OutputTokens:    350,
		TotalDuration:   15000,
		StartTime:       time.Now().Add(-15 * time.Second),
		EndTime:         time.Now(),
		Status:          "completed",
	}

	err = persistence.UpdateReasoningHistory(ctx, history)
	if err != nil {
		log.Fatalf("Error updating history: %v", err)
	}

	fmt.Println("✓ Histórico atualizado com sucesso")

	// ===== Exemplo 4: Recuperar Histórico Completo =====
	fmt.Println("\n=== Exemplo 4: Recuperar Histórico Completo ===")
	retrievedHistory, err := persistence.GetReasoningHistory(ctx, runID)
	if err != nil {
		log.Fatalf("Error getting history: %v", err)
	}

	fmt.Printf("Histórico ID: %s\n", retrievedHistory.ID)
	fmt.Printf("Status: %s\n", retrievedHistory.Status)
	fmt.Printf("Total de Steps: %d\n", len(retrievedHistory.Steps))
	fmt.Printf("Total de Tokens: %d\n", retrievedHistory.TotalTokens)
	fmt.Printf("Reasoning Tokens: %d\n", retrievedHistory.ReasoningTokens)
	fmt.Printf("Duração Total: %dms\n", retrievedHistory.TotalDuration)

	// ===== Exemplo 5: Obter Estatísticas =====
	fmt.Println("\n=== Exemplo 5: Obter Estatísticas ===")
	stats, err := persistence.GetReasoningStats(ctx, runID)
	if err != nil {
		log.Fatalf("Error getting stats: %v", err)
	}

	fmt.Printf("Total de Steps: %v\n", stats["total_steps"])
	fmt.Printf("Total de Reasoning Tokens: %v\n", stats["total_reasoning_tokens"])
	fmt.Printf("Total de Input Tokens: %v\n", stats["total_input_tokens"])
	fmt.Printf("Total de Output Tokens: %v\n", stats["total_output_tokens"])
	fmt.Printf("Confiança Média: %.2f\n", stats["avg_confidence"])
	fmt.Printf("Duração Total: %vms\n", stats["total_duration_ms"])

	// ===== Exemplo 6: Análise Detalhada =====
	fmt.Println("\n=== Exemplo 6: Análise Detalhada dos Steps ===")
	for _, step := range retrievedHistory.Steps {
		fmt.Printf("\nStep %d: %s\n", step.StepNumber, step.Title)
		fmt.Printf("  Raciocínio: %s\n", step.Reasoning)
		fmt.Printf("  Ação: %s\n", step.Action)
		fmt.Printf("  Resultado: %s\n", step.Result)
		fmt.Printf("  Confiança: %.2f\n", step.Confidence)
		fmt.Printf("  Tokens - Reasoning: %d, Input: %d, Output: %d\n",
			step.ReasoningTokens, step.InputTokens, step.OutputTokens)
		fmt.Printf("  Duração: %dms\n", step.Duration)
		if step.Metadata != nil {
			fmt.Printf("  Metadata: %v\n", step.Metadata)
		}
	}

	// ===== Exemplo 7: Múltiplas Execuções =====
	fmt.Println("\n=== Exemplo 7: Múltiplas Execuções ===")

	// Segunda execução
	runID2 := "run-002"
	for i := 1; i <= 3; i++ {
		step := reasoning.ReasoningStepRecord{
			RunID:           runID2,
			AgentID:         agentID,
			StepNumber:      i,
			Title:           fmt.Sprintf("Quick Analysis %d", i),
			Reasoning:       "Fast reasoning",
			Action:          "quick_check",
			Result:          "OK",
			Confidence:      0.9,
			ReasoningTokens: 50 * i,
			InputTokens:     15 * i,
			OutputTokens:    35 * i,
			Duration:        500,
		}

		err := persistence.SaveReasoningStep(ctx, step)
		if err != nil {
			log.Printf("Error saving step: %v", err)
		}
	}

	// Atualizar histórico da segunda execução
	history2 := reasoning.ReasoningHistory{
		ID:              "history-002",
		RunID:           runID2,
		AgentID:         agentID,
		TotalTokens:     300,
		ReasoningTokens: 150,
		InputTokens:     45,
		OutputTokens:    105,
		TotalDuration:   1500,
		StartTime:       time.Now().Add(-2 * time.Second),
		EndTime:         time.Now(),
		Status:          "completed",
	}

	err = persistence.UpdateReasoningHistory(ctx, history2)
	if err != nil {
		log.Fatalf("Error updating history 2: %v", err)
	}

	// Comparar execuções
	stats1, _ := persistence.GetReasoningStats(ctx, runID)
	stats2, _ := persistence.GetReasoningStats(ctx, runID2)

	fmt.Printf("Execução 1 (run-001):\n")
	fmt.Printf("  Steps: %v\n", stats1["total_steps"])
	fmt.Printf("  Tokens: %v\n", stats1["total_reasoning_tokens"])

	fmt.Printf("Execução 2 (run-002):\n")
	fmt.Printf("  Steps: %v\n", stats2["total_steps"])
	fmt.Printf("  Tokens: %v\n", stats2["total_reasoning_tokens"])

	// ===== Exemplo 8: Limpeza =====
	fmt.Println("\n=== Exemplo 8: Limpeza de Dados ===")
	err = persistence.DeleteReasoningHistory(ctx, runID)
	if err != nil {
		log.Fatalf("Error deleting history: %v", err)
	}

	fmt.Printf("✓ Histórico de %s deletado com sucesso\n", runID)

	// Verificar que foi deletado
	_, err = persistence.GetReasoningHistory(ctx, runID)
	if err != nil {
		fmt.Println("✓ Confirmado: histórico foi deletado")
	}

	fmt.Println("\n=== Exemplo Completo Finalizado ===")
}
