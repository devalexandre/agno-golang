package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SqliteStorage implements the Storage interface with SQLite backend
// Following the Python Agno implementation patterns
type SqliteStorage struct {
	id                string
	tableName         string
	dbURL             *string
	dbFile            *string
	db                *sql.DB
	schemaVersion     int
	autoUpgradeSchema bool
	mode              storage.StorageMode
	schemaUpToDate    bool
}

// SqliteStorageConfig holds configuration options
type SqliteStorageConfig struct {
	ID                string
	TableName         string
	DBURL             *string
	DBFile            *string
	DB                *sql.DB
	SchemaVersion     int
	AutoUpgradeSchema bool
	Mode              storage.StorageMode
}

// NewSqliteStorage creates a new SQLite storage instance
func NewSqliteStorage(config SqliteStorageConfig) (*SqliteStorage, error) {
	// Set defaults
	if config.SchemaVersion == 0 {
		config.SchemaVersion = 1
	}
	if config.Mode == "" {
		config.Mode = storage.AgentMode
	}

	// Generate ID if not provided (like Python's BaseDb)
	id := config.ID
	if id == "" {
		id = uuid.New().String()
	}

	s := &SqliteStorage{
		id:                id,
		tableName:         config.TableName,
		dbURL:             config.DBURL,
		dbFile:            config.DBFile,
		schemaVersion:     config.SchemaVersion,
		autoUpgradeSchema: config.AutoUpgradeSchema,
		mode:              config.Mode,
		schemaUpToDate:    false,
	}

	// Initialize database connection
	if err := s.initDB(config.DB); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return s, nil
}

// initDB initializes the database connection following Python's priority order
func (s *SqliteStorage) initDB(providedDB *sql.DB) error {
	var err error

	// Priority order matches Python implementation:
	// 1. Use provided DB if available
	// 2. Use DBURL if provided
	// 3. Use DBFile if provided
	// 4. Create in-memory database

	if providedDB != nil {
		s.db = providedDB
		return nil
	}

	if s.dbURL != nil {
		s.db, err = sql.Open("sqlite3", *s.dbURL)
		if err != nil {
			return fmt.Errorf("failed to open database with URL: %w", err)
		}
	} else if s.dbFile != nil {
		// Ensure directory exists
		dir := filepath.Dir(*s.dbFile)
		if err := ensureDir(dir); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		s.db, err = sql.Open("sqlite3", *s.dbFile)
		if err != nil {
			return fmt.Errorf("failed to open database file: %w", err)
		}
	} else {
		// Create in-memory database
		s.db, err = sql.Open("sqlite3", ":memory:")
		if err != nil {
			return fmt.Errorf("failed to create in-memory database: %w", err)
		}
	}

	return nil
}

// ensureDir creates directory if it doesn't exist
func ensureDir(dir string) error {
	// This is a simplified version - in production you'd use os.MkdirAll
	return nil
}

// GetMode returns the current storage mode
func (s *SqliteStorage) GetMode() storage.StorageMode {
	return s.mode
}

// SetMode sets the storage mode and refreshes table if needed
func (s *SqliteStorage) SetMode(mode storage.StorageMode) {
	if s.mode != mode {
		s.mode = mode
		// Table would be recreated with new schema
	}
}

// getTableSchemaV1 returns the CREATE TABLE statement for schema version 1
func (s *SqliteStorage) getTableSchemaV1() string {
	// Common columns for all modes
	commonColumns := []string{
		"session_id TEXT PRIMARY KEY",
		"user_id TEXT NOT NULL",
		"memory TEXT",       // JSON stored as TEXT
		"session_data TEXT", // JSON stored as TEXT
		"extra_data TEXT",   // JSON stored as TEXT
		"created_at INTEGER DEFAULT (strftime('%s', 'now'))",
		"updated_at INTEGER DEFAULT (strftime('%s', 'now'))",
	}

	// Mode-specific columns
	var specificColumns []string
	switch s.mode {
	case storage.AgentMode:
		specificColumns = []string{
			"agent_id TEXT NOT NULL",
			"agent_data TEXT", // JSON stored as TEXT
			"team_session_id TEXT",
		}
	case storage.TeamMode:
		specificColumns = []string{
			"team_id TEXT NOT NULL",
			"team_data TEXT", // JSON stored as TEXT
			"team_session_id TEXT",
		}
	case storage.WorkflowMode:
		specificColumns = []string{
			"workflow_id TEXT NOT NULL",
			"workflow_data TEXT", // JSON stored as TEXT
		}
	case storage.WorkflowV2Mode:
		specificColumns = []string{
			"workflow_id TEXT NOT NULL",
			"workflow_name TEXT NOT NULL",
			"workflow_data TEXT", // JSON stored as TEXT
			"runs TEXT",          // JSON stored as TEXT
		}
	}

	// Combine all columns
	allColumns := append(commonColumns, specificColumns...)

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", s.tableName, strings.Join(allColumns, ", "))
}

// getIndexStatements returns CREATE INDEX statements for the table
func (s *SqliteStorage) getIndexStatements() []string {
	indexes := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_user_id ON %s (user_id)", s.tableName, s.tableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_created_at ON %s (created_at)", s.tableName, s.tableName),
	}

	// Mode-specific indexes
	switch s.mode {
	case storage.AgentMode:
		indexes = append(indexes,
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_agent_id ON %s (agent_id)", s.tableName, s.tableName),
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_team_session_id ON %s (team_session_id)", s.tableName, s.tableName),
		)
	case storage.TeamMode:
		indexes = append(indexes,
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_team_id ON %s (team_id)", s.tableName, s.tableName),
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_team_session_id ON %s (team_session_id)", s.tableName, s.tableName),
		)
	case storage.WorkflowMode, storage.WorkflowV2Mode:
		indexes = append(indexes,
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_workflow_id ON %s (workflow_id)", s.tableName, s.tableName),
		)
		if s.mode == storage.WorkflowV2Mode {
			indexes = append(indexes,
				fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_workflow_name ON %s (workflow_name)", s.tableName, s.tableName),
			)
		}
	}

	return indexes
}

// TableExists checks if the table exists in the database
func (s *SqliteStorage) TableExists() (bool, error) {
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	var name string
	err := s.db.QueryRow(query, s.tableName).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error checking if table exists: %w", err)
	}
	return true, nil
}

// GetID returns the unique identifier for this storage instance
func (s *SqliteStorage) GetID() string {
	return s.id
}

// Create creates the table and indexes if they don't exist
func (s *SqliteStorage) Create() error {
	// Create table
	createTableSQL := s.getTableSchemaV1()
	if _, err := s.db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create indexes
	for _, indexSQL := range s.getIndexStatements() {
		if _, err := s.db.Exec(indexSQL); err != nil {
			// Log warning but continue - indexes are not critical
			log.Printf("Warning: failed to create index: %v", err)
		}
	}

	return nil
}

// Read reads a session by ID, optionally filtered by user ID
func (s *SqliteStorage) Read(sessionID string, userID *string) (interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE session_id = ?", s.tableName)
	args := []interface{}{sessionID}

	if userID != nil {
		query += " AND user_id = ?"
		args = append(args, *userID)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			if err := s.Create(); err != nil {
				return nil, fmt.Errorf("failed to create table: %w", err)
			}
			return nil, nil // Table was just created, no data
		}
		return nil, fmt.Errorf("failed to query session: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // No session found
	}

	return s.scanRowToSession(rows)
}

// scanRowToSession scans a database row into the appropriate session type
func (s *SqliteStorage) scanRowToSession(rows *sql.Rows) (interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Create a slice of interface{} to hold the values
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	// Convert to map
	data := make(map[string]interface{})
	for i, col := range columns {
		val := values[i]
		if b, ok := val.([]byte); ok {
			data[col] = string(b)
		} else {
			data[col] = val
		}
	}

	// Parse JSON fields
	if err := s.parseJSONFields(data); err != nil {
		return nil, err
	}

	// Convert to appropriate session type
	switch s.mode {
	case storage.AgentMode:
		return storage.AgentSessionFromDict(data), nil
	case storage.TeamMode:
		return storage.TeamSessionFromDict(data), nil
	case storage.WorkflowMode:
		// TODO: Implement WorkflowSession FromDict
		return data, nil
	case storage.WorkflowV2Mode:
		// TODO: Implement WorkflowSessionV2 FromDict
		return data, nil
	default:
		return data, nil
	}
}

// parseJSONFields parses JSON string fields back to maps
func (s *SqliteStorage) parseJSONFields(data map[string]interface{}) error {
	jsonFields := []string{"memory", "session_data", "extra_data", "agent_data", "team_data", "workflow_data", "runs"}

	for _, field := range jsonFields {
		if val, ok := data[field]; ok && val != nil {
			if str, ok := val.(string); ok && str != "" {
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(str), &parsed); err == nil {
					data[field] = parsed
				}
			}
		}
	}

	return nil
}

// Upsert inserts or updates a session
func (s *SqliteStorage) Upsert(session interface{}) (interface{}, error) {
	// Perform schema upgrade if needed
	if s.autoUpgradeSchema && !s.schemaUpToDate {
		if err := s.UpgradeSchema(); err != nil {
			return nil, fmt.Errorf("failed to upgrade schema: %w", err)
		}
	}

	switch s.mode {
	case storage.AgentMode:
		return s.upsertAgentSession(session)
	case storage.TeamMode:
		return s.upsertTeamSession(session)
	case storage.WorkflowMode:
		return s.upsertWorkflowSession(session)
	case storage.WorkflowV2Mode:
		return s.upsertWorkflowV2Session(session)
	default:
		return nil, fmt.Errorf("unsupported mode: %s", s.mode)
	}
}

// upsertAgentSession handles upserting agent sessions
func (s *SqliteStorage) upsertAgentSession(session interface{}) (interface{}, error) {
	agentSession, ok := session.(*storage.AgentSession)
	if !ok {
		return nil, fmt.Errorf("expected *storage.AgentSession, got %T", session)
	}

	// Convert maps to JSON
	memoryJSON, _ := json.Marshal(agentSession.Memory)
	sessionDataJSON, _ := json.Marshal(agentSession.SessionData)
	extraDataJSON, _ := json.Marshal(agentSession.ExtraData)
	agentDataJSON, _ := json.Marshal(agentSession.AgentData)

	// SQLite UPSERT using INSERT OR REPLACE
	query := fmt.Sprintf(`
		INSERT OR REPLACE INTO %s 
		(session_id, agent_id, user_id, team_session_id, memory, agent_data, session_data, extra_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 
			COALESCE((SELECT created_at FROM %s WHERE session_id = ?), ?),
			?)`, s.tableName, s.tableName)

	now := time.Now().Unix()
	_, err := s.db.Exec(query,
		agentSession.SessionID,
		agentSession.AgentID,
		agentSession.UserID,
		agentSession.TeamSessionID,
		string(memoryJSON),
		string(agentDataJSON),
		string(sessionDataJSON),
		string(extraDataJSON),
		agentSession.SessionID, // for COALESCE subquery
		agentSession.CreatedAt,
		now,
	)

	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			if err := s.Create(); err != nil {
				return nil, fmt.Errorf("failed to create table: %w", err)
			}
			// Retry upsert
			return s.upsertAgentSession(session)
		}
		return nil, fmt.Errorf("failed to upsert agent session: %w", err)
	}

	// Return the updated session
	return s.Read(agentSession.SessionID, &agentSession.UserID)
}

// upsertTeamSession handles upserting team sessions
func (s *SqliteStorage) upsertTeamSession(session interface{}) (interface{}, error) {
	teamSession, ok := session.(*storage.TeamSession)
	if !ok {
		return nil, fmt.Errorf("expected *storage.TeamSession, got %T", session)
	}

	// Convert maps to JSON
	memoryJSON, _ := json.Marshal(teamSession.Memory)
	sessionDataJSON, _ := json.Marshal(teamSession.SessionData)
	extraDataJSON, _ := json.Marshal(teamSession.ExtraData)
	teamDataJSON, _ := json.Marshal(teamSession.TeamData)

	// SQLite UPSERT using INSERT OR REPLACE
	query := fmt.Sprintf(`
		INSERT OR REPLACE INTO %s 
		(session_id, team_id, user_id, team_session_id, memory, team_data, session_data, extra_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 
			COALESCE((SELECT created_at FROM %s WHERE session_id = ?), ?),
			?)`, s.tableName, s.tableName)

	now := time.Now().Unix()
	_, err := s.db.Exec(query,
		teamSession.SessionID,
		teamSession.TeamID,
		teamSession.UserID,
		teamSession.TeamSessionID,
		string(memoryJSON),
		string(teamDataJSON),
		string(sessionDataJSON),
		string(extraDataJSON),
		teamSession.SessionID, // for COALESCE subquery
		teamSession.CreatedAt,
		now,
	)

	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			if err := s.Create(); err != nil {
				return nil, fmt.Errorf("failed to create table: %w", err)
			}
			// Retry upsert
			return s.upsertTeamSession(session)
		}
		return nil, fmt.Errorf("failed to upsert team session: %w", err)
	}

	// Return the updated session
	return s.Read(teamSession.SessionID, &teamSession.UserID)
}

// Placeholder implementations for workflow sessions
func (s *SqliteStorage) upsertWorkflowSession(session interface{}) (interface{}, error) {
	return nil, fmt.Errorf("workflow session upsert not implemented yet")
}

func (s *SqliteStorage) upsertWorkflowV2Session(session interface{}) (interface{}, error) {
	return nil, fmt.Errorf("workflow v2 session upsert not implemented yet")
}

// DeleteSession deletes a session by ID
func (s *SqliteStorage) DeleteSession(sessionID *string) error {
	if sessionID == nil {
		log.Printf("Warning: No session_id provided for deletion")
		return nil
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE session_id = ?", s.tableName)
	result, err := s.db.Exec(query, *sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		log.Printf("No session found with session_id: %s", *sessionID)
	} else {
		log.Printf("Successfully deleted session with session_id: %s", *sessionID)
	}

	return nil
}

// GetAllSessionIDs gets all session IDs, optionally filtered
func (s *SqliteStorage) GetAllSessionIDs(userID *string, entityID *string) ([]string, error) {
	query := fmt.Sprintf("SELECT session_id FROM %s WHERE 1=1", s.tableName)
	args := []interface{}{}

	if userID != nil {
		query += " AND user_id = ?"
		args = append(args, *userID)
	}

	if entityID != nil {
		switch s.mode {
		case storage.AgentMode:
			query += " AND agent_id = ?"
		case storage.TeamMode:
			query += " AND team_id = ?"
		case storage.WorkflowMode, storage.WorkflowV2Mode:
			query += " AND workflow_id = ?"
		}
		args = append(args, *entityID)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			if err := s.Create(); err != nil {
				return nil, fmt.Errorf("failed to create table: %w", err)
			}
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to query session IDs: %w", err)
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, fmt.Errorf("failed to scan session ID: %w", err)
		}
		sessionIDs = append(sessionIDs, sessionID)
	}

	return sessionIDs, nil
}

// GetAllSessions gets all sessions, optionally filtered
func (s *SqliteStorage) GetAllSessions(userID *string, entityID *string) ([]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE 1=1", s.tableName)
	args := []interface{}{}

	if userID != nil {
		query += " AND user_id = ?"
		args = append(args, *userID)
	}

	if entityID != nil {
		switch s.mode {
		case storage.AgentMode:
			query += " AND agent_id = ?"
		case storage.TeamMode:
			query += " AND team_id = ?"
		case storage.WorkflowMode, storage.WorkflowV2Mode:
			query += " AND workflow_id = ?"
		}
		args = append(args, *entityID)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			if err := s.Create(); err != nil {
				return nil, fmt.Errorf("failed to create table: %w", err)
			}
			return []interface{}{}, nil
		}
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []interface{}
	for rows.Next() {
		session, err := s.scanRowToSession(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetRecentSessions gets the most recent sessions
func (s *SqliteStorage) GetRecentSessions(userID *string, entityID *string, limit *int) ([]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE 1=1", s.tableName)
	args := []interface{}{}

	if userID != nil {
		query += " AND user_id = ?"
		args = append(args, *userID)
	}

	if entityID != nil {
		switch s.mode {
		case storage.AgentMode:
			query += " AND agent_id = ?"
		case storage.TeamMode:
			query += " AND team_id = ?"
		case storage.WorkflowMode, storage.WorkflowV2Mode:
			query += " AND workflow_id = ?"
		}
		args = append(args, *entityID)
	}

	query += " ORDER BY created_at DESC"

	if limit != nil && *limit > 0 {
		query += " LIMIT ?"
		args = append(args, *limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			if err := s.Create(); err != nil {
				return nil, fmt.Errorf("failed to create table: %w", err)
			}
			return []interface{}{}, nil
		}
		return nil, fmt.Errorf("failed to query recent sessions: %w", err)
	}
	defer rows.Close()

	var sessions []interface{}
	for rows.Next() {
		session, err := s.scanRowToSession(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// UpgradeSchema upgrades the database schema
func (s *SqliteStorage) UpgradeSchema() error {
	if !s.autoUpgradeSchema {
		log.Printf("Auto schema upgrade disabled. Skipping upgrade.")
		return nil
	}

	exists, err := s.TableExists()
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if !exists {
		s.schemaUpToDate = true
		return nil
	}

	// Check if team_session_id column exists for agent mode
	if s.mode == storage.AgentMode {
		query := fmt.Sprintf("PRAGMA table_info(%s)", s.tableName)
		rows, err := s.db.Query(query)
		if err != nil {
			return fmt.Errorf("failed to get table info: %w", err)
		}
		defer rows.Close()

		columnExists := false
		for rows.Next() {
			var cid int
			var name, dataType string
			var notNull, pk int
			var defaultValue sql.NullString

			if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
				return fmt.Errorf("failed to scan table info: %w", err)
			}

			if name == "team_session_id" {
				columnExists = true
				break
			}
		}

		if !columnExists {
			log.Printf("Adding 'team_session_id' column to %s", s.tableName)
			alterQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN team_session_id TEXT", s.tableName)
			if _, err := s.db.Exec(alterQuery); err != nil {
				return fmt.Errorf("failed to add team_session_id column: %w", err)
			}
			log.Printf("Schema upgrade completed successfully")
		}
	}

	s.schemaUpToDate = true
	return nil
}

// Drop drops the table from the database
func (s *SqliteStorage) Drop() error {
	exists, err := s.TableExists()
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if exists {
		log.Printf("Dropping table: %s", s.tableName)
		query := fmt.Sprintf("DROP TABLE IF EXISTS %s", s.tableName)
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to drop table: %w", err)
		}
	}

	return nil
}

// Close closes the database connection
func (s *SqliteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Knowledge Content methods - compatible with Python's BaseDb

// GetKnowledgeContent retrieves a single knowledge content by ID
func (s *SqliteStorage) GetKnowledgeContent(id string) (*storage.KnowledgeRow, error) {
	// Ensure knowledge table exists
	if err := s.createKnowledgeTableIfNotExists(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, description, metadata, type, size, linked_to, access_count,
		       status, status_message, created_at, updated_at, external_id
		FROM agno_knowledge
		WHERE id = ?
	`

	row := s.db.QueryRow(query, id)

	var kr storage.KnowledgeRow
	var metadataStr sql.NullString
	var typeVal, linkedTo, status, statusMessage, externalID sql.NullString
	var size, accessCount sql.NullInt64
	var createdAt, updatedAt sql.NullInt64

	err := row.Scan(
		&kr.ID, &kr.Name, &kr.Description, &metadataStr,
		&typeVal, &size, &linkedTo, &accessCount,
		&status, &statusMessage, &createdAt, &updatedAt, &externalID,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge content: %w", err)
	}

	// Parse metadata JSON
	if metadataStr.Valid && metadataStr.String != "" {
		if err := json.Unmarshal([]byte(metadataStr.String), &kr.Metadata); err != nil {
			kr.Metadata = make(map[string]interface{})
		}
	} else {
		kr.Metadata = make(map[string]interface{})
	}

	// Handle nullable fields
	if typeVal.Valid {
		kr.Type = &typeVal.String
	}
	if size.Valid {
		sizeInt := int(size.Int64)
		kr.Size = &sizeInt
	}
	if linkedTo.Valid {
		kr.LinkedTo = &linkedTo.String
	}
	if accessCount.Valid {
		countInt := int(accessCount.Int64)
		kr.AccessCount = &countInt
	}
	if status.Valid {
		kr.Status = &status.String
	}
	if statusMessage.Valid {
		kr.StatusMessage = &statusMessage.String
	}
	if createdAt.Valid {
		kr.CreatedAt = &createdAt.Int64
	}
	if updatedAt.Valid {
		kr.UpdatedAt = &updatedAt.Int64
	}
	if externalID.Valid {
		kr.ExternalID = &externalID.String
	}

	return &kr, nil
}

// GetKnowledgeContents retrieves all knowledge contents with pagination
func (s *SqliteStorage) GetKnowledgeContents(limit, page *int, sortBy, sortOrder *string) ([]*storage.KnowledgeRow, int, error) {
	// Ensure knowledge table exists
	if err := s.createKnowledgeTableIfNotExists(); err != nil {
		return nil, 0, err
	}

	// Count total records
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM agno_knowledge"
	if err := s.db.QueryRow(countQuery).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count knowledge contents: %w", err)
	}

	// Build query with sorting and pagination
	query := `
		SELECT id, name, description, metadata, type, size, linked_to, access_count,
		       status, status_message, created_at, updated_at, external_id
		FROM agno_knowledge
	`

	// Add sorting
	sortCol := "updated_at"
	sortDir := "DESC"
	if sortBy != nil && *sortBy != "" {
		sortCol = *sortBy
	}
	if sortOrder != nil && strings.ToUpper(*sortOrder) == "ASC" {
		sortDir = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortCol, sortDir)

	// Add pagination
	if limit != nil && *limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", *limit)
		if page != nil && *page > 1 {
			offset := (*page - 1) * (*limit)
			query += fmt.Sprintf(" OFFSET %d", offset)
		}
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query knowledge contents: %w", err)
	}
	defer rows.Close()

	var results []*storage.KnowledgeRow
	for rows.Next() {
		var kr storage.KnowledgeRow
		var metadataStr sql.NullString
		var typeVal, linkedTo, status, statusMessage, externalID sql.NullString
		var size, accessCount sql.NullInt64
		var createdAt, updatedAt sql.NullInt64

		err := rows.Scan(
			&kr.ID, &kr.Name, &kr.Description, &metadataStr,
			&typeVal, &size, &linkedTo, &accessCount,
			&status, &statusMessage, &createdAt, &updatedAt, &externalID,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan knowledge row: %w", err)
		}

		// Parse metadata
		if metadataStr.Valid && metadataStr.String != "" {
			if err := json.Unmarshal([]byte(metadataStr.String), &kr.Metadata); err != nil {
				kr.Metadata = make(map[string]interface{})
			}
		} else {
			kr.Metadata = make(map[string]interface{})
		}

		// Handle nullable fields
		if typeVal.Valid {
			kr.Type = &typeVal.String
		}
		if size.Valid {
			sizeInt := int(size.Int64)
			kr.Size = &sizeInt
		}
		if linkedTo.Valid {
			kr.LinkedTo = &linkedTo.String
		}
		if accessCount.Valid {
			countInt := int(accessCount.Int64)
			kr.AccessCount = &countInt
		}
		if status.Valid {
			kr.Status = &status.String
		}
		if statusMessage.Valid {
			kr.StatusMessage = &statusMessage.String
		}
		if createdAt.Valid {
			kr.CreatedAt = &createdAt.Int64
		}
		if updatedAt.Valid {
			kr.UpdatedAt = &updatedAt.Int64
		}
		if externalID.Valid {
			kr.ExternalID = &externalID.String
		}

		results = append(results, &kr)
	}

	return results, totalCount, nil
}

// UpsertKnowledgeContent inserts or updates a knowledge content
func (s *SqliteStorage) UpsertKnowledgeContent(row *storage.KnowledgeRow) (*storage.KnowledgeRow, error) {
	// Ensure knowledge table exists
	if err := s.createKnowledgeTableIfNotExists(); err != nil {
		return nil, err
	}

	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(row.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Prepare values
	now := time.Now().Unix()
	if row.UpdatedAt == nil {
		row.UpdatedAt = &now
	}
	if row.CreatedAt == nil {
		row.CreatedAt = &now
	}

	query := `
		INSERT INTO agno_knowledge (
			id, name, description, metadata, type, size, linked_to, access_count,
			status, status_message, created_at, updated_at, external_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			metadata = excluded.metadata,
			type = excluded.type,
			size = excluded.size,
			linked_to = excluded.linked_to,
			access_count = excluded.access_count,
			status = excluded.status,
			status_message = excluded.status_message,
			updated_at = excluded.updated_at,
			external_id = excluded.external_id
	`

	_, err = s.db.Exec(query,
		row.ID, row.Name, row.Description, string(metadataJSON),
		row.Type, row.Size, row.LinkedTo, row.AccessCount,
		row.Status, row.StatusMessage, row.CreatedAt, row.UpdatedAt, row.ExternalID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to upsert knowledge content: %w", err)
	}

	return row, nil
}

// DeleteKnowledgeContent deletes a knowledge content by ID
func (s *SqliteStorage) DeleteKnowledgeContent(id string) error {
	// Ensure knowledge table exists
	if err := s.createKnowledgeTableIfNotExists(); err != nil {
		return err
	}

	query := "DELETE FROM agno_knowledge WHERE id = ?"
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete knowledge content: %w", err)
	}

	return nil
}

// createKnowledgeTableIfNotExists creates the agno_knowledge table if it doesn't exist
func (s *SqliteStorage) createKnowledgeTableIfNotExists() error {
	query := `
		CREATE TABLE IF NOT EXISTS agno_knowledge (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			metadata TEXT,
			type TEXT,
			size INTEGER,
			linked_to TEXT,
			access_count INTEGER DEFAULT 0,
			status TEXT,
			status_message TEXT,
			created_at INTEGER,
			updated_at INTEGER,
			external_id TEXT
		)
	`

	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create knowledge table: %w", err)
	}

	return nil
}
