package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/storage"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStorage struct {
	db        *sql.DB
	tableName string
	schema    string
	mode      storage.StorageMode
}

type PostgresStorageConfig struct {
	DSN       string
	TableName string
	Schema    string
	Mode      storage.StorageMode
	DB        *sql.DB
}

func NewPostgresStorage(config PostgresStorageConfig) (*PostgresStorage, error) {
	if config.TableName == "" {
		config.TableName = "agno_sessions"
	}
	if config.Schema == "" {
		config.Schema = "public"
	}

	var db *sql.DB
	var err error
	if config.DB != nil {
		db = config.DB
	} else {
		db, err = sql.Open("pgx", config.DSN)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres connection: %w", err)
		}
	}

	s := &PostgresStorage{
		db:        db,
		tableName: config.TableName,
		schema:    config.Schema,
		mode:      config.Mode,
	}

	if err := s.initDB(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *PostgresStorage) initDB() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.%s (
			session_id TEXT PRIMARY KEY,
			user_id TEXT,
			entity_id TEXT,
			session_data JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`, s.schema, s.tableName)

	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create indices
	indices := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_user_id ON %s.%s(user_id)", s.tableName, s.schema, s.tableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_entity_id ON %s.%s(entity_id)", s.tableName, s.schema, s.tableName),
	}

	for _, idxQuery := range indices {
		if _, err := s.db.Exec(idxQuery); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

func (s *PostgresStorage) GetID() string {
	return "postgres"
}

func (s *PostgresStorage) Create() error {
	return s.initDB()
}

func (s *PostgresStorage) Read(sessionID string, userID *string) (interface{}, error) {
	query := fmt.Sprintf("SELECT session_data FROM %s.%s WHERE session_id = $1", s.schema, s.tableName)
	args := []interface{}{sessionID}
	if userID != nil {
		query += " AND user_id = $2"
		args = append(args, *userID)
	}

	var data []byte
	err := s.db.QueryRow(query, args...).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var session map[string]interface{}
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return session, nil
}

func (s *PostgresStorage) Upsert(session interface{}) (interface{}, error) {
	var sessionID, userID, entityID string
	var sessionData []byte

	// Simplify for now, assuming it follows storage.Session structure
	// In a real implementation, we should use reflection or type assertions
	data, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	sessionData = data

	// Extract IDs from map (assuming it was converted from struct or is a map)
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if id, ok := m["session_id"].(string); ok {
		sessionID = id
	}
	if uid, ok := m["user_id"].(string); ok {
		userID = uid
	}
	// entity_id depends on the session type (agent_id, team_id, etc.)
	if eid, ok := m["agent_id"].(string); ok {
		entityID = eid
	} else if eid, ok := m["team_id"].(string); ok {
		entityID = eid
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.%s (session_id, user_id, entity_id, session_data, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (session_id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			entity_id = EXCLUDED.entity_id,
			session_data = EXCLUDED.session_data,
			updated_at = CURRENT_TIMESTAMP
	`, s.schema, s.tableName)

	_, err = s.db.Exec(query, sessionID, userID, entityID, sessionData)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (s *PostgresStorage) Drop() error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", s.schema, s.tableName)
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}
