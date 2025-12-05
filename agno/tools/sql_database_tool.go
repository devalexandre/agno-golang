package tools

import (
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// SQLDatabaseTool fornece operações de banco de dados SQL
type SQLDatabaseTool struct {
	toolkit.Toolkit
	connections map[string]DatabaseConnection
	queryLog    []QueryExecution
	maxResults  int
}

// DatabaseConnection representa uma conexão de banco
type DatabaseConnection struct {
	ConnectionID string
	DatabaseType string // "postgres", "mysql", "sqlite"
	Host         string
	Port         int
	Database     string
	Username     string
	IsConnected  bool
	ConnectedAt  time.Time
}

// QueryExecution registra execução de query
type QueryExecution struct {
	QueryID       string
	Query         string
	Status        string // "success", "error", "running"
	ExecutionTime int64  // ms
	RowsAffected  int
	ErrorMsg      string
	ExecutedAt    time.Time
}

// ExecuteQueryParams parâmetros para executar query
type ExecuteQueryParams struct {
	ConnectionID string `json:"connection_id" description:"ID da conexão"`
	Query        string `json:"query" description:"SQL query"`
	Timeout      int    `json:"timeout" description:"Timeout em segundos"`
	MaxResults   int    `json:"max_results" description:"Máximo de linhas a retornar"`
}

// ConnectDBParams parâmetros para conectar
type ConnectDBParams struct {
	DatabaseType string `json:"database_type" description:"postgres, mysql, sqlite"`
	Host         string `json:"host" description:"Host do servidor"`
	Port         int    `json:"port" description:"Porta"`
	Database     string `json:"database" description:"Nome do banco"`
	Username     string `json:"username" description:"Usuário"`
	Password     string `json:"password" description:"Senha"`
}

// QueryResult resultado de query
type QueryResult struct {
	Success       bool                     `json:"success"`
	QueryID       string                   `json:"query_id,omitempty"`
	Rows          []map[string]interface{} `json:"rows,omitempty"`
	RowCount      int                      `json:"row_count"`
	ExecutionTime int64                    `json:"execution_time_ms"`
	Message       string                   `json:"message"`
	ErrorMsg      string                   `json:"error_msg,omitempty"`
}

// NewSQLDatabaseTool cria novo tool
func NewSQLDatabaseTool() *SQLDatabaseTool {
	t := &SQLDatabaseTool{
		connections: make(map[string]DatabaseConnection),
		queryLog:    make([]QueryExecution, 0),
		maxResults:  1000,
	}
	t.Toolkit = toolkit.NewToolkit()

	t.Toolkit.Register(
		"ConnectDatabase",
		"Conectar a um banco de dados",
		t,
		t.ConnectDatabase,
		ConnectDBParams{},
	)

	t.Toolkit.Register(
		"ExecuteQuery",
		"Executar uma query SQL",
		t,
		t.ExecuteQuery,
		ExecuteQueryParams{},
	)

	t.Toolkit.Register(
		"GetQueryHistory",
		"Obter histórico de queries",
		t,
		t.GetQueryHistory,
		GetHistoryParams{},
	)

	t.Toolkit.Register(
		"ListConnections",
		"Listar conexões ativas",
		t,
		t.ListConnections,
		ListConnectionsParams{},
	)

	return t
}

// ConnectDatabase conecta a um banco
func (t *SQLDatabaseTool) ConnectDatabase(params ConnectDBParams) (interface{}, error) {
	if params.DatabaseType == "" || params.Database == "" {
		return map[string]interface{}{
			"success": false,
			"message": "database_type e database obrigatórios",
		}, fmt.Errorf("parâmetros obrigatórios faltando")
	}

	// Validar tipo de banco
	validTypes := map[string]bool{
		"postgres": true,
		"mysql":    true,
		"sqlite":   true,
	}
	if !validTypes[params.DatabaseType] {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Tipo de banco não suportado: %s", params.DatabaseType),
		}, fmt.Errorf("tipo inválido")
	}

	// Gerar ID
	connID := fmt.Sprintf("conn_%s_%d", params.DatabaseType, time.Now().UnixNano())

	conn := DatabaseConnection{
		ConnectionID: connID,
		DatabaseType: params.DatabaseType,
		Host:         params.Host,
		Port:         params.Port,
		Database:     params.Database,
		Username:     params.Username,
		IsConnected:  true,
		ConnectedAt:  time.Now(),
	}

	t.connections[connID] = conn

	return map[string]interface{}{
		"success":       true,
		"connection_id": connID,
		"database_type": params.DatabaseType,
		"database":      params.Database,
		"message":       fmt.Sprintf("Conectado a %s/%s", params.DatabaseType, params.Database),
		"timestamp":     time.Now(),
	}, nil
}

// ExecuteQuery executa uma query
func (t *SQLDatabaseTool) ExecuteQuery(params ExecuteQueryParams) (interface{}, error) {
	if params.ConnectionID == "" || params.Query == "" {
		return QueryResult{Success: false}, fmt.Errorf("connection_id e query obrigatórios")
	}

	conn, exists := t.connections[params.ConnectionID]
	if !exists {
		return QueryResult{Success: false}, fmt.Errorf("conexão não encontrada")
	}

	// Sanitizar query (detectar padrões perigosos)
	if !t.isSafeQuery(params.Query) {
		return QueryResult{
			Success: false,
			Message: "Query contém padrões perigosos",
		}, fmt.Errorf("query não segura")
	}

	// Simular execução
	queryID := fmt.Sprintf("query_%d", time.Now().UnixNano())
	startTime := time.Now()

	// Simular diferentes tipos de queries
	var rows []map[string]interface{}
	rowCount := 0
	queryType := t.detectQueryType(params.Query)

	switch queryType {
	case "SELECT":
		// Simular seleção
		rowCount = 5
		for i := 1; i <= rowCount && i <= params.MaxResults; i++ {
			rows = append(rows, map[string]interface{}{
				"id":    i,
				"name":  fmt.Sprintf("Row %d", i),
				"value": i * 100,
			})
		}
	case "INSERT", "UPDATE", "DELETE":
		rowCount = 1 // Típico de modificação
	case "CREATE", "DROP", "ALTER":
		rowCount = 0 // Operações DDL
	}

	executionTime := time.Since(startTime).Milliseconds()

	// Registrar execução
	exec := QueryExecution{
		QueryID:       queryID,
		Query:         params.Query,
		Status:        "success",
		ExecutionTime: executionTime,
		RowsAffected:  rowCount,
		ExecutedAt:    time.Now(),
	}
	t.queryLog = append(t.queryLog, exec)

	return QueryResult{
		Success:       true,
		QueryID:       queryID,
		Rows:          rows,
		RowCount:      rowCount,
		ExecutionTime: executionTime,
		Message:       fmt.Sprintf("Query executada com sucesso em %s", conn.Database),
	}, nil
}

// GetQueryHistory retorna histórico
func (t *SQLDatabaseTool) GetQueryHistory(params GetHistoryParams) (interface{}, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	start := len(t.queryLog) - limit
	if start < 0 {
		start = 0
	}

	return map[string]interface{}{
		"total_queries":  len(t.queryLog),
		"recent_queries": t.queryLog[start:],
		"limit":          limit,
		"timestamp":      time.Now(),
	}, nil
}

// ListConnections lista conexões ativas
func (t *SQLDatabaseTool) ListConnections(params ListConnectionsParams) (interface{}, error) {
	connections := make([]DatabaseConnection, 0)

	for _, conn := range t.connections {
		if conn.IsConnected {
			connections = append(connections, conn)
		}
	}

	return map[string]interface{}{
		"total_connections": len(connections),
		"connections":       connections,
		"timestamp":         time.Now(),
	}, nil
}

// Helper functions

func (t *SQLDatabaseTool) isSafeQuery(query string) bool {
	queryUpper := strings.ToUpper(query)

	// Detectar padrões perigosos
	dangerPatterns := []string{
		"DROP DATABASE",
		"TRUNCATE",
		"DELETE FROM",
		"ALTER DATABASE",
	}

	for _, pattern := range dangerPatterns {
		if strings.Contains(queryUpper, pattern) {
			// Permitir se estiver em comentário
			if !strings.Contains(queryUpper, "--") {
				return false
			}
		}
	}

	return true
}

func (t *SQLDatabaseTool) detectQueryType(query string) string {
	queryUpper := strings.TrimSpace(strings.ToUpper(query))

	types := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "TRUNCATE"}
	for _, qtype := range types {
		if strings.HasPrefix(queryUpper, qtype) {
			return qtype
		}
	}

	return "UNKNOWN"
}

// GetHistoryParams parâmetros
type GetHistoryParams struct {
	Limit int `json:"limit" description:"Número máximo de registros"`
}

// ListConnectionsParams parâmetros
type ListConnectionsParams struct {
	OnlyActive bool `json:"only_active" description:"Filtrar apenas ativas"`
}
