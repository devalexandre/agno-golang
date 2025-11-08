package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/google/uuid"
)

// DB Interface Implementation
// These methods implement storage.DB interface to maintain Python compatibility

// CreateSession creates a new agent session
func (s *SqliteStorage) CreateSession(ctx context.Context, session *storage.AgentSession) error {
	now := time.Now().Unix()
	session.CreatedAt = now
	session.UpdatedAt = now

	// Marshal JSON fields
	memoryJSON, _ := json.Marshal(session.Memory)
	sessionDataJSON, _ := json.Marshal(session.SessionData)
	agentDataJSON, _ := json.Marshal(session.AgentData)
	extraDataJSON, _ := json.Marshal(session.ExtraData)

	query := `
		INSERT INTO ` + s.tableName + ` (
			session_id, user_id, memory, session_data, agent_id, agent_data, 
			team_session_id, extra_data, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		session.SessionID,
		session.UserID,
		string(memoryJSON),
		string(sessionDataJSON),
		session.AgentID,
		string(agentDataJSON),
		session.TeamSessionID,
		string(extraDataJSON),
		session.CreatedAt,
		session.UpdatedAt,
	)

	return err
}

// ReadSession reads an agent session by ID
func (s *SqliteStorage) ReadSession(ctx context.Context, sessionID string) (*storage.AgentSession, error) {
	query := `SELECT * FROM ` + s.tableName + ` WHERE session_id = ?`

	row := s.db.QueryRowContext(ctx, query, sessionID)

	var session storage.AgentSession
	var memoryJSON, sessionDataJSON, agentDataJSON, extraDataJSON string
	var teamSessionID sql.NullString

	err := row.Scan(
		&session.SessionID,
		&session.UserID,
		&memoryJSON,
		&sessionDataJSON,
		&session.AgentID,
		&agentDataJSON,
		&teamSessionID,
		&extraDataJSON,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	json.Unmarshal([]byte(memoryJSON), &session.Memory)
	json.Unmarshal([]byte(sessionDataJSON), &session.SessionData)
	json.Unmarshal([]byte(agentDataJSON), &session.AgentData)
	json.Unmarshal([]byte(extraDataJSON), &session.ExtraData)

	if teamSessionID.Valid {
		session.TeamSessionID = &teamSessionID.String
	}

	return &session, nil
}

// UpdateSession updates an existing agent session
func (s *SqliteStorage) UpdateSession(ctx context.Context, session *storage.AgentSession) error {
	session.UpdatedAt = time.Now().Unix()

	// Marshal JSON fields
	memoryJSON, _ := json.Marshal(session.Memory)
	sessionDataJSON, _ := json.Marshal(session.SessionData)
	agentDataJSON, _ := json.Marshal(session.AgentData)
	extraDataJSON, _ := json.Marshal(session.ExtraData)

	query := `
		UPDATE ` + s.tableName + ` 
		SET user_id = ?, memory = ?, session_data = ?, agent_id = ?, agent_data = ?,
			team_session_id = ?, extra_data = ?, updated_at = ?
		WHERE session_id = ?
	`

	_, err := s.db.ExecContext(ctx, query,
		session.UserID,
		string(memoryJSON),
		string(sessionDataJSON),
		session.AgentID,
		string(agentDataJSON),
		session.TeamSessionID,
		string(extraDataJSON),
		session.UpdatedAt,
		session.SessionID,
	)

	return err
}

// CreateRun creates a new agent run
func (s *SqliteStorage) CreateRun(ctx context.Context, run *storage.AgentRun) error {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}

	now := time.Now()
	run.CreatedAt = now
	run.UpdatedAt = now

	// Create runs table if it doesn't exist
	if err := s.createRunsTableIfNotExists(); err != nil {
		return err
	}

	// Marshal JSON fields
	messagesJSON, _ := json.Marshal(run.Messages)
	runDataJSON, _ := json.Marshal(run.RunData)
	metricsJSON, _ := json.Marshal(run.Metrics)

	query := `
INSERT INTO agent_runs (
id, session_id, user_id, run_name, run_data, user_message, 
agent_message, messages, metrics, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

	_, err := s.db.ExecContext(ctx, query,
		run.ID,
		run.SessionID,
		run.UserID,
		run.RunName,
		string(runDataJSON),
		run.UserMessage,
		run.AgentMessage,
		string(messagesJSON),
		string(metricsJSON),
		run.CreatedAt.Unix(),
		run.UpdatedAt.Unix(),
	)

	return err
}

// ReadRun reads an agent run by ID
func (s *SqliteStorage) ReadRun(ctx context.Context, runID string) (*storage.AgentRun, error) {
	query := `SELECT * FROM agent_runs WHERE id = ?`

	row := s.db.QueryRowContext(ctx, query, runID)

	var run storage.AgentRun
	var messagesJSON, runDataJSON, metricsJSON string
	var createdAt, updatedAt int64

	err := row.Scan(
		&run.ID,
		&run.SessionID,
		&run.UserID,
		&run.RunName,
		&runDataJSON,
		&run.UserMessage,
		&run.AgentMessage,
		&messagesJSON,
		&metricsJSON,
		&createdAt,
		&updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("run not found: %s", runID)
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	json.Unmarshal([]byte(messagesJSON), &run.Messages)
	json.Unmarshal([]byte(runDataJSON), &run.RunData)
	json.Unmarshal([]byte(metricsJSON), &run.Metrics)

	run.CreatedAt = time.Unix(createdAt, 0)
	run.UpdatedAt = time.Unix(updatedAt, 0)

	return &run, nil
}

// UpdateRun updates an existing agent run
func (s *SqliteStorage) UpdateRun(ctx context.Context, run *storage.AgentRun) error {
	run.UpdatedAt = time.Now()

	// Marshal JSON fields
	messagesJSON, _ := json.Marshal(run.Messages)
	runDataJSON, _ := json.Marshal(run.RunData)
	metricsJSON, _ := json.Marshal(run.Metrics)

	query := `
UPDATE agent_runs 
SET session_id = ?, user_id = ?, run_name = ?, run_data = ?, 
user_message = ?, agent_message = ?, messages = ?, metrics = ?, updated_at = ?
WHERE id = ?
`

	_, err := s.db.ExecContext(ctx, query,
		run.SessionID,
		run.UserID,
		run.RunName,
		string(runDataJSON),
		run.UserMessage,
		run.AgentMessage,
		string(messagesJSON),
		string(metricsJSON),
		run.UpdatedAt.Unix(),
		run.ID,
	)

	return err
}

// DeleteRun deletes an agent run
func (s *SqliteStorage) DeleteRun(ctx context.Context, runID string) error {
	query := `DELETE FROM agent_runs WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, runID)
	return err
}

// CreateTables creates all necessary tables
func (s *SqliteStorage) CreateTables(ctx context.Context) error {
	// Create sessions table
	if err := s.Create(); err != nil {
		return err
	}

	// Create runs table
	return s.createRunsTableIfNotExists()
}

// DropTables drops all tables
func (s *SqliteStorage) DropTables(ctx context.Context) error {
	// Drop runs table
	if _, err := s.db.Exec(`DROP TABLE IF EXISTS agent_runs`); err != nil {
		return err
	}

	// Drop sessions table
	return s.Drop()
}

// createRunsTableIfNotExists creates the agent_runs table if it doesn't exist
func (s *SqliteStorage) createRunsTableIfNotExists() error {
	query := `
		CREATE TABLE IF NOT EXISTS agent_runs (
id TEXT PRIMARY KEY,
session_id TEXT NOT NULL,
user_id TEXT,
run_name TEXT,
run_data TEXT,
user_message TEXT,
agent_message TEXT,
messages TEXT,
metrics TEXT,
created_at INTEGER,
updated_at INTEGER,
FOREIGN KEY (session_id) REFERENCES ` + s.tableName + `(session_id)
		)
	`

	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create agent_runs table: %w", err)
	}

	// Create indexes
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_runs_session_id ON agent_runs(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_user_id ON agent_runs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_created_at ON agent_runs(created_at)`,
	}

	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// GetRunsForSession gets all runs for a session
func (s *SqliteStorage) GetRunsForSession(ctx context.Context, sessionID string) ([]*storage.AgentRun, error) {
	query := `SELECT * FROM agent_runs WHERE session_id = ? ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*storage.AgentRun
	for rows.Next() {
		var run storage.AgentRun
		var messagesJSON, runDataJSON, metricsJSON string
		var createdAt, updatedAt int64

		err := rows.Scan(
			&run.ID,
			&run.SessionID,
			&run.UserID,
			&run.RunName,
			&runDataJSON,
			&run.UserMessage,
			&run.AgentMessage,
			&messagesJSON,
			&metricsJSON,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON fields
		json.Unmarshal([]byte(messagesJSON), &run.Messages)
		json.Unmarshal([]byte(runDataJSON), &run.RunData)
		json.Unmarshal([]byte(metricsJSON), &run.Metrics)

		run.CreatedAt = time.Unix(createdAt, 0)
		run.UpdatedAt = time.Unix(updatedAt, 0)

		runs = append(runs, &run)
	}

	return runs, rows.Err()
}

// UpgradeSchema upgrades the database schema (DB interface)
func (s *SqliteStorage) UpgradeSchema(ctx context.Context) error {
	// For now, just ensure tables exist
	return s.CreateTables(ctx)
}

// DB Interface methods that don't conflict with Storage interface

// DeleteSession deletes an agent session (DB interface compatible)
func (s *SqliteStorage) DeleteSession(ctx context.Context, sessionID string) error {
	return s.deleteSessionByPtr(&sessionID)
}

// GetAllSessionIDs gets all session IDs for a user (DB interface compatible)
func (s *SqliteStorage) GetAllSessionIDs(ctx context.Context, userID string) ([]string, error) {
	return s.getAllSessionIDsByPtr(&userID, nil)
}

// GetAllSessions gets all sessions for a user (DB interface compatible)
func (s *SqliteStorage) GetAllSessions(ctx context.Context, userID string) ([]*storage.AgentSession, error) {
	sessions, err := s.getAllSessionsByPtr(&userID, nil)
	if err != nil {
		return nil, err
	}

	// Convert []interface{} to []*AgentSession
	result := make([]*storage.AgentSession, 0, len(sessions))
	for _, sess := range sessions {
		if agentSession, ok := sess.(*storage.AgentSession); ok {
			result = append(result, agentSession)
		}
	}

	return result, nil
}
