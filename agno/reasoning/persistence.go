package reasoning

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ReasoningStepRecord representa um reasoning step armazenado no banco de dados
type ReasoningStepRecord struct {
	ID              int64
	RunID           string
	AgentID         string
	StepNumber      int
	Title           string
	Reasoning       string
	Action          string
	Result          string
	Confidence      float64
	NextAction      string
	ReasoningTokens int
	InputTokens     int
	OutputTokens    int
	Duration        int64 // em millisegundos
	Timestamp       time.Time
	Metadata        map[string]interface{}
}

// ReasoningHistory representa o histórico completo de reasoning de uma execução
type ReasoningHistory struct {
	ID              string
	RunID           string
	AgentID         string
	Steps           []ReasoningStepRecord
	TotalTokens     int
	ReasoningTokens int
	InputTokens     int
	OutputTokens    int
	TotalDuration   int64 // em millisegundos
	StartTime       time.Time
	EndTime         time.Time
	Status          string // "running", "completed", "failed"
	Error           string
}

// ReasoningPersistence interface para persistência de reasoning steps
type ReasoningPersistence interface {
	// SaveReasoningStep salva um reasoning step
	SaveReasoningStep(ctx context.Context, step ReasoningStepRecord) error

	// GetReasoningHistory obtém o histórico de reasoning de uma execução
	GetReasoningHistory(ctx context.Context, runID string) (*ReasoningHistory, error)

	// GetReasoningStep obtém um reasoning step específico
	GetReasoningStep(ctx context.Context, id int64) (*ReasoningStepRecord, error)

	// ListReasoningSteps lista todos os reasoning steps de uma execução
	ListReasoningSteps(ctx context.Context, runID string) ([]ReasoningStepRecord, error)

	// UpdateReasoningHistory atualiza o histórico de reasoning
	UpdateReasoningHistory(ctx context.Context, history ReasoningHistory) error

	// DeleteReasoningHistory deleta o histórico de reasoning
	DeleteReasoningHistory(ctx context.Context, runID string) error

	// GetReasoningStats obtém estatísticas de reasoning
	GetReasoningStats(ctx context.Context, runID string) (map[string]interface{}, error)
}

// SQLiteReasoningPersistence implementação SQLite de ReasoningPersistence
type SQLiteReasoningPersistence struct {
	db *sql.DB
}

// NewSQLiteReasoningPersistence cria uma nova instância de SQLiteReasoningPersistence
func NewSQLiteReasoningPersistence(db *sql.DB) (*SQLiteReasoningPersistence, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	srp := &SQLiteReasoningPersistence{db: db}

	// Criar tabelas se não existirem
	if err := srp.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return srp, nil
}

// createTables cria as tabelas necessárias
func (srp *SQLiteReasoningPersistence) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS reasoning_steps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		step_number INTEGER NOT NULL,
		title TEXT,
		reasoning TEXT,
		action TEXT,
		result TEXT,
		confidence REAL,
		next_action TEXT,
		reasoning_tokens INTEGER,
		input_tokens INTEGER,
		output_tokens INTEGER,
		duration INTEGER,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata TEXT,
		UNIQUE(run_id, step_number)
	);

	CREATE TABLE IF NOT EXISTS reasoning_history (
		id TEXT PRIMARY KEY,
		run_id TEXT NOT NULL UNIQUE,
		agent_id TEXT NOT NULL,
		total_tokens INTEGER,
		reasoning_tokens INTEGER,
		input_tokens INTEGER,
		output_tokens INTEGER,
		total_duration INTEGER,
		start_time DATETIME,
		end_time DATETIME,
		status TEXT,
		error TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_reasoning_steps_run_id ON reasoning_steps(run_id);
	CREATE INDEX IF NOT EXISTS idx_reasoning_steps_agent_id ON reasoning_steps(agent_id);
	CREATE INDEX IF NOT EXISTS idx_reasoning_history_run_id ON reasoning_history(run_id);
	CREATE INDEX IF NOT EXISTS idx_reasoning_history_agent_id ON reasoning_history(agent_id);
	`

	_, err := srp.db.Exec(schema)
	return err
}

// SaveReasoningStep salva um reasoning step
func (srp *SQLiteReasoningPersistence) SaveReasoningStep(ctx context.Context, step ReasoningStepRecord) error {
	metadataJSON, err := json.Marshal(step.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
	INSERT INTO reasoning_steps (
		run_id, agent_id, step_number, title, reasoning, action, result,
		confidence, next_action, reasoning_tokens, input_tokens, output_tokens,
		duration, metadata
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(run_id, step_number) DO UPDATE SET
		title = excluded.title,
		reasoning = excluded.reasoning,
		action = excluded.action,
		result = excluded.result,
		confidence = excluded.confidence,
		next_action = excluded.next_action,
		reasoning_tokens = excluded.reasoning_tokens,
		input_tokens = excluded.input_tokens,
		output_tokens = excluded.output_tokens,
		duration = excluded.duration,
		metadata = excluded.metadata
	`

	result, err := srp.db.ExecContext(ctx, query,
		step.RunID, step.AgentID, step.StepNumber, step.Title, step.Reasoning,
		step.Action, step.Result, step.Confidence, step.NextAction,
		step.ReasoningTokens, step.InputTokens, step.OutputTokens,
		step.Duration, string(metadataJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to save reasoning step: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil && id > 0 {
		step.ID = id
	}

	return nil
}

// GetReasoningHistory obtém o histórico de reasoning de uma execução
func (srp *SQLiteReasoningPersistence) GetReasoningHistory(ctx context.Context, runID string) (*ReasoningHistory, error) {
	query := `
	SELECT id, run_id, agent_id, total_tokens, reasoning_tokens, input_tokens,
	       output_tokens, total_duration, start_time, end_time, status, error
	FROM reasoning_history
	WHERE run_id = ?
	`

	history := &ReasoningHistory{}
	err := srp.db.QueryRowContext(ctx, query, runID).Scan(
		&history.ID, &history.RunID, &history.AgentID, &history.TotalTokens,
		&history.ReasoningTokens, &history.InputTokens, &history.OutputTokens,
		&history.TotalDuration, &history.StartTime, &history.EndTime,
		&history.Status, &history.Error,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("reasoning history not found for run %s", runID)
		}
		return nil, fmt.Errorf("failed to get reasoning history: %w", err)
	}

	// Obter os steps
	steps, err := srp.ListReasoningSteps(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reasoning steps: %w", err)
	}

	history.Steps = steps
	return history, nil
}

// GetReasoningStep obtém um reasoning step específico
func (srp *SQLiteReasoningPersistence) GetReasoningStep(ctx context.Context, id int64) (*ReasoningStepRecord, error) {
	query := `
	SELECT id, run_id, agent_id, step_number, title, reasoning, action, result,
	       confidence, next_action, reasoning_tokens, input_tokens, output_tokens,
	       duration, timestamp, metadata
	FROM reasoning_steps
	WHERE id = ?
	`

	step := &ReasoningStepRecord{}
	var metadataJSON string

	err := srp.db.QueryRowContext(ctx, query, id).Scan(
		&step.ID, &step.RunID, &step.AgentID, &step.StepNumber, &step.Title,
		&step.Reasoning, &step.Action, &step.Result, &step.Confidence,
		&step.NextAction, &step.ReasoningTokens, &step.InputTokens,
		&step.OutputTokens, &step.Duration, &step.Timestamp, &metadataJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("reasoning step not found")
		}
		return nil, fmt.Errorf("failed to get reasoning step: %w", err)
	}

	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &step.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return step, nil
}

// ListReasoningSteps lista todos os reasoning steps de uma execução
func (srp *SQLiteReasoningPersistence) ListReasoningSteps(ctx context.Context, runID string) ([]ReasoningStepRecord, error) {
	query := `
	SELECT id, run_id, agent_id, step_number, title, reasoning, action, result,
	       confidence, next_action, reasoning_tokens, input_tokens, output_tokens,
	       duration, timestamp, metadata
	FROM reasoning_steps
	WHERE run_id = ?
	ORDER BY step_number ASC
	`

	rows, err := srp.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query reasoning steps: %w", err)
	}
	defer rows.Close()

	var steps []ReasoningStepRecord

	for rows.Next() {
		step := ReasoningStepRecord{}
		var metadataJSON string

		err := rows.Scan(
			&step.ID, &step.RunID, &step.AgentID, &step.StepNumber, &step.Title,
			&step.Reasoning, &step.Action, &step.Result, &step.Confidence,
			&step.NextAction, &step.ReasoningTokens, &step.InputTokens,
			&step.OutputTokens, &step.Duration, &step.Timestamp, &metadataJSON,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan reasoning step: %w", err)
		}

		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &step.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		steps = append(steps, step)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reasoning steps: %w", err)
	}

	return steps, nil
}

// UpdateReasoningHistory atualiza o histórico de reasoning
func (srp *SQLiteReasoningPersistence) UpdateReasoningHistory(ctx context.Context, history ReasoningHistory) error {
	query := `
	INSERT INTO reasoning_history (
		id, run_id, agent_id, total_tokens, reasoning_tokens, input_tokens,
		output_tokens, total_duration, start_time, end_time, status, error
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		total_tokens = excluded.total_tokens,
		reasoning_tokens = excluded.reasoning_tokens,
		input_tokens = excluded.input_tokens,
		output_tokens = excluded.output_tokens,
		total_duration = excluded.total_duration,
		end_time = excluded.end_time,
		status = excluded.status,
		error = excluded.error,
		updated_at = CURRENT_TIMESTAMP
	`

	_, err := srp.db.ExecContext(ctx, query,
		history.ID, history.RunID, history.AgentID, history.TotalTokens,
		history.ReasoningTokens, history.InputTokens, history.OutputTokens,
		history.TotalDuration, history.StartTime, history.EndTime,
		history.Status, history.Error,
	)

	if err != nil {
		return fmt.Errorf("failed to update reasoning history: %w", err)
	}

	return nil
}

// DeleteReasoningHistory deleta o histórico de reasoning
func (srp *SQLiteReasoningPersistence) DeleteReasoningHistory(ctx context.Context, runID string) error {
	// Deletar steps primeiro
	_, err := srp.db.ExecContext(ctx, "DELETE FROM reasoning_steps WHERE run_id = ?", runID)
	if err != nil {
		return fmt.Errorf("failed to delete reasoning steps: %w", err)
	}

	// Deletar history
	_, err = srp.db.ExecContext(ctx, "DELETE FROM reasoning_history WHERE run_id = ?", runID)
	if err != nil {
		return fmt.Errorf("failed to delete reasoning history: %w", err)
	}

	return nil
}

// GetReasoningStats obtém estatísticas de reasoning
func (srp *SQLiteReasoningPersistence) GetReasoningStats(ctx context.Context, runID string) (map[string]interface{}, error) {
	query := `
	SELECT
		COUNT(*) as total_steps,
		SUM(reasoning_tokens) as total_reasoning_tokens,
		SUM(input_tokens) as total_input_tokens,
		SUM(output_tokens) as total_output_tokens,
		SUM(duration) as total_duration,
		AVG(confidence) as avg_confidence
	FROM reasoning_steps
	WHERE run_id = ?
	`

	stats := make(map[string]interface{})
	var totalSteps int
	var reasoningTokens, inputTokens, outputTokens, totalDuration sql.NullInt64
	var avgConfidence sql.NullFloat64

	err := srp.db.QueryRowContext(ctx, query, runID).Scan(
		&totalSteps, &reasoningTokens, &inputTokens, &outputTokens,
		&totalDuration, &avgConfidence,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get reasoning stats: %w", err)
	}

	stats["total_steps"] = totalSteps
	if reasoningTokens.Valid {
		stats["total_reasoning_tokens"] = reasoningTokens.Int64
	}
	if inputTokens.Valid {
		stats["total_input_tokens"] = inputTokens.Int64
	}
	if outputTokens.Valid {
		stats["total_output_tokens"] = outputTokens.Int64
	}
	if totalDuration.Valid {
		stats["total_duration_ms"] = totalDuration.Int64
	}
	if avgConfidence.Valid {
		stats["avg_confidence"] = avgConfidence.Float64
	}

	return stats, nil
}
