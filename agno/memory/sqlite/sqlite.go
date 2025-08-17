package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/memory"
	"github.com/google/uuid"
	"github.com/vingarcia/ksql"
	ksqlite "github.com/vingarcia/ksql/adapters/modernc-ksqlite"
)

// UserMemoryRecord represents a user memory record for KSQL
type UserMemoryRecord struct {
	ID        string    `ksql:"id"`
	UserID    string    `ksql:"user_id"`
	Memory    string    `ksql:"memory"`
	Input     string    `ksql:"input"`
	Summary   string    `ksql:"summary"`
	CreatedAt time.Time `ksql:"created_at"`
	UpdatedAt time.Time `ksql:"updated_at"`
}

// SessionSummaryRecord represents a session summary record for KSQL
type SessionSummaryRecord struct {
	ID        string    `ksql:"id"`
	UserID    string    `ksql:"user_id"`
	SessionID string    `ksql:"session_id"`
	Summary   string    `ksql:"summary"`
	CreatedAt time.Time `ksql:"created_at"`
	UpdatedAt time.Time `ksql:"updated_at"`
}

// SqliteMemoryDb implements MemoryDatabase interface for SQLite using KSQL
type SqliteMemoryDb struct {
	db        ksql.Provider
	tableName string
	dbFile    string
}

// NewSqliteMemoryDb creates a new SQLite memory database instance
func NewSqliteMemoryDb(tableName, dbFile string) (*SqliteMemoryDb, error) {
	db, err := ksqlite.New(context.Background(), dbFile, ksql.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	memDb := &SqliteMemoryDb{
		db:        db,
		tableName: tableName,
		dbFile:    dbFile,
	}

	// Create tables if they don't exist
	if err := memDb.CreateTables(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return memDb, nil
}

// CreateTables creates the necessary tables
func (db *SqliteMemoryDb) CreateTables(ctx context.Context) error {
	// User memories table
	memoriesTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			memory TEXT NOT NULL,
			input TEXT,
			summary TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`, db.tableName)

	// Session summaries table
	summariesTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s_summaries (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			summary TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, session_id)
		)
	`, db.tableName)

	// Create indexes
	memoryIndexes := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_%s_user_id ON %s(user_id);
	`, db.tableName, db.tableName)

	summaryIndexes := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_%s_summaries_user_id ON %s_summaries(user_id);
		CREATE INDEX IF NOT EXISTS idx_%s_summaries_session_id ON %s_summaries(session_id);
	`, db.tableName, db.tableName, db.tableName, db.tableName)

	if _, err := db.db.Exec(ctx, memoriesTable); err != nil {
		return fmt.Errorf("failed to create memories table: %w", err)
	}

	if _, err := db.db.Exec(ctx, summariesTable); err != nil {
		return fmt.Errorf("failed to create summaries table: %w", err)
	}

	if _, err := db.db.Exec(ctx, memoryIndexes); err != nil {
		return fmt.Errorf("failed to create memory indexes: %w", err)
	}

	if _, err := db.db.Exec(ctx, summaryIndexes); err != nil {
		return fmt.Errorf("failed to create summary indexes: %w", err)
	}

	return nil
}

// Helper method to convert memory.UserMemory to record
func (db *SqliteMemoryDb) memoryToRecord(mem *memory.UserMemory) *UserMemoryRecord {
	return &UserMemoryRecord{
		ID:        mem.ID,
		UserID:    mem.UserID,
		Memory:    mem.Memory,
		Input:     mem.Input,
		Summary:   mem.Summary,
		CreatedAt: mem.CreatedAt,
		UpdatedAt: mem.UpdatedAt,
	}
}

// Helper method to convert record to memory.UserMemory
func (db *SqliteMemoryDb) recordToMemory(record *UserMemoryRecord) *memory.UserMemory {
	return &memory.UserMemory{
		ID:        record.ID,
		UserID:    record.UserID,
		Memory:    record.Memory,
		Input:     record.Input,
		Summary:   record.Summary,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

// CreateUserMemory creates a new user memory
func (db *SqliteMemoryDb) CreateUserMemory(ctx context.Context, mem *memory.UserMemory) error {
	if mem.ID == "" {
		mem.ID = uuid.New().String()
	}
	mem.CreatedAt = time.Now()
	mem.UpdatedAt = time.Now()

	record := db.memoryToRecord(mem)
	table := ksql.NewTable(db.tableName, "id")

	return db.db.Insert(ctx, table, record)
}

// GetUserMemories retrieves all memories for a user
func (db *SqliteMemoryDb) GetUserMemories(ctx context.Context, userID string) ([]*memory.UserMemory, error) {
	var records []UserMemoryRecord

	query := fmt.Sprintf("FROM %s WHERE user_id = ? ORDER BY created_at DESC", db.tableName)
	err := db.db.Query(ctx, &records, query, userID)
	if err != nil {
		return nil, err
	}

	memories := make([]*memory.UserMemory, len(records))
	for i, record := range records {
		memories[i] = db.recordToMemory(&record)
	}

	return memories, nil
}

// UpdateUserMemory updates an existing user memory
func (db *SqliteMemoryDb) UpdateUserMemory(ctx context.Context, mem *memory.UserMemory) error {
	mem.UpdatedAt = time.Now()

	record := db.memoryToRecord(mem)
	table := ksql.NewTable(db.tableName, "id")

	return db.db.Patch(ctx, table, record)
}

// DeleteUserMemory deletes a specific user memory
func (db *SqliteMemoryDb) DeleteUserMemory(ctx context.Context, memoryID string) error {
	table := ksql.NewTable(db.tableName, "id")
	return db.db.Delete(ctx, table, memoryID)
}

// ClearUserMemories deletes all memories for a user
func (db *SqliteMemoryDb) ClearUserMemories(ctx context.Context, userID string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", db.tableName)
	_, err := db.db.Exec(ctx, query, userID)
	return err
}

// Helper method to convert memory.SessionSummary to record
func (db *SqliteMemoryDb) summaryToRecord(summary *memory.SessionSummary) *SessionSummaryRecord {
	return &SessionSummaryRecord{
		ID:        summary.ID,
		UserID:    summary.UserID,
		SessionID: summary.SessionID,
		Summary:   summary.Summary,
		CreatedAt: summary.CreatedAt,
		UpdatedAt: summary.UpdatedAt,
	}
}

// Helper method to convert record to memory.SessionSummary
func (db *SqliteMemoryDb) recordToSummary(record *SessionSummaryRecord) *memory.SessionSummary {
	return &memory.SessionSummary{
		ID:        record.ID,
		UserID:    record.UserID,
		SessionID: record.SessionID,
		Summary:   record.Summary,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

// CreateSessionSummary creates a new session summary
func (db *SqliteMemoryDb) CreateSessionSummary(ctx context.Context, summary *memory.SessionSummary) error {
	if summary.ID == "" {
		summary.ID = uuid.New().String()
	}
	summary.CreatedAt = time.Now()
	summary.UpdatedAt = time.Now()

	record := db.summaryToRecord(summary)

	// Use INSERT OR REPLACE to handle unique constraint
	query := fmt.Sprintf(`
		INSERT OR REPLACE INTO %s_summaries (id, user_id, session_id, summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, db.tableName)

	_, err := db.db.Exec(ctx, query,
		record.ID, record.UserID, record.SessionID, record.Summary,
		record.CreatedAt, record.UpdatedAt)

	return err
}

// GetSessionSummary retrieves a session summary
func (db *SqliteMemoryDb) GetSessionSummary(ctx context.Context, userID, sessionID string) (*memory.SessionSummary, error) {
	var record SessionSummaryRecord

	query := fmt.Sprintf("FROM %s_summaries WHERE user_id = ? AND session_id = ?", db.tableName)
	err := db.db.QueryOne(ctx, &record, query, userID, sessionID)
	if err != nil {
		if err == ksql.ErrRecordNotFound {
			return nil, fmt.Errorf("session summary not found for user %s, session %s", userID, sessionID)
		}
		return nil, err
	}

	return db.recordToSummary(&record), nil
}

// UpdateSessionSummary updates an existing session summary
func (db *SqliteMemoryDb) UpdateSessionSummary(ctx context.Context, summary *memory.SessionSummary) error {
	summary.UpdatedAt = time.Now()

	query := fmt.Sprintf(`
		UPDATE %s_summaries 
		SET summary = ?, updated_at = ?
		WHERE user_id = ? AND session_id = ?
	`, db.tableName)

	_, err := db.db.Exec(ctx, query,
		summary.Summary, summary.UpdatedAt, summary.UserID, summary.SessionID)

	return err
}

// DeleteSessionSummary deletes a session summary
func (db *SqliteMemoryDb) DeleteSessionSummary(ctx context.Context, userID, sessionID string) error {
	query := fmt.Sprintf("DELETE FROM %s_summaries WHERE user_id = ? AND session_id = ?", db.tableName)
	_, err := db.db.Exec(ctx, query, userID, sessionID)
	return err
}

// UpgradeSchema upgrades the database schema
func (db *SqliteMemoryDb) UpgradeSchema(ctx context.Context) error {
	// For now, just recreate tables - in production you'd want proper migrations
	return db.CreateTables(ctx)
}

// DropTables drops all tables
func (db *SqliteMemoryDb) DropTables(ctx context.Context) error {
	queries := []string{
		fmt.Sprintf("DROP TABLE IF EXISTS %s", db.tableName),
		fmt.Sprintf("DROP TABLE IF EXISTS %s_summaries", db.tableName),
	}

	for _, query := range queries {
		if _, err := db.db.Exec(ctx, query); err != nil {
			return fmt.Errorf("failed to drop table: %w", err)
		}
	}

	return nil
}

// Close closes the database connection (if applicable)
func (db *SqliteMemoryDb) Close() error {
	// KSQL providers may not expose Close method directly
	// This would be implementation specific
	return nil
}
