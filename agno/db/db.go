package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// DBType representa o tipo de banco de dados
type DBType string

const (
	PostgreSQL DBType = "postgres"
	MySQL      DBType = "mysql"
	SQLite     DBType = "sqlite3"
	MariaDB    DBType = "mariadb" // MariaDB (internamente usa driver MySQL)
)

// Config contém as configurações de conexão do banco de dados
type Config struct {
	Type     DBType // Tipo do banco de dados
	Host     string // Host do banco (não usado para SQLite ou quando DSN é fornecido)
	Port     int    // Porta do banco (não usado para SQLite ou quando DSN é fornecido)
	User     string // Usuário do banco (não usado para SQLite ou quando DSN é fornecido)
	Password string // Senha do banco (não usado para SQLite ou quando DSN é fornecido)
	Database string // Nome do banco de dados ou caminho do arquivo (para SQLite)
	SSLMode  string // Modo SSL (apenas PostgreSQL: disable, require, verify-ca, verify-full)

	// DSN (Data Source Name) - Connection string completa
	// Se fornecido, ignora Host, Port, User, Password, Database e SSLMode
	// Exemplo PostgreSQL: "postgres://user:pass@localhost:5432/dbname?sslmode=disable"
	// Exemplo MySQL: "user:pass@tcp(localhost:3306)/dbname?parseTime=true"
	// Exemplo SQLite: "./mydb.db" ou "file:mydb.db?cache=shared&mode=rwc"
	DSN string

	// Opções avançadas
	MaxOpenConns int // Máximo de conexões abertas (padrão: 10)
	MaxIdleConns int // Máximo de conexões ociosas (padrão: 5)
}

// DB é um wrapper que abstrai o acesso ao banco de dados
// Suporta PostgreSQL, MySQL, SQLite e MariaDB
type DB struct {
	*sql.DB
	config Config
}

// New cria uma nova conexão com o banco de dados
// Exemplo de uso:
//
//	// PostgreSQL
//	db, err := db.New(db.Config{
//		Type: db.PostgreSQL,
//		Host: "localhost",
//		Port: 5432,
//		User: "user",
//		Password: "password",
//		Database: "mydb",
//	})
//
//	// MySQL
//	db, err := db.New(db.Config{
//		Type: db.MySQL,
//		Host: "localhost",
//		Port: 3306,
//		User: "user",
//		Password: "password",
//		Database: "mydb",
//	})
//
//	// SQLite
//	db, err := db.New(db.Config{
//		Type: db.SQLite,
//		Database: "./mydb.db",
//	})
func New(config Config) (*DB, error) {
	// Validar configuração
	if config.Type == "" {
		return nil, fmt.Errorf("database type is required")
	}

	// Se DSN não foi fornecido, validar campos necessários
	if config.DSN == "" && config.Database == "" {
		return nil, fmt.Errorf("database name or DSN is required")
	}

	// Definir valores padrão
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 10
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 5
	}

	// Determinar connection string
	var connString string
	var err error

	if config.DSN != "" {
		// Usar DSN fornecido diretamente
		connString = config.DSN
	} else {
		// Construir connection string a partir dos campos
		connString, err = buildConnectionString(config)
		if err != nil {
			return nil, fmt.Errorf("failed to build connection string: %w", err)
		}
	}

	// Determinar o driver correto
	driver := string(config.Type)
	if config.Type == MariaDB {
		driver = "mysql" // MariaDB usa o driver MySQL
	}

	// Abrir conexão
	sqlDB, err := sql.Open(driver, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configurar pool de conexões
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)

	// Testar conexão
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		DB:     sqlDB,
		config: config,
	}, nil
}

// buildConnectionString constrói a string de conexão baseada no tipo de banco
func buildConnectionString(config Config) (string, error) {
	switch config.Type {
	case PostgreSQL:
		return buildPostgreSQLConnectionString(config), nil
	case MySQL, MariaDB:
		return buildMySQLConnectionString(config), nil
	case SQLite:
		return config.Database, nil
	default:
		return "", fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

// buildPostgreSQLConnectionString constrói a connection string para PostgreSQL
func buildPostgreSQLConnectionString(config Config) string {
	sslMode := config.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.Database,
		sslMode,
	)
}

// buildMySQLConnectionString constrói a connection string para MySQL/MariaDB
func buildMySQLConnectionString(config Config) string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)
}

// GetConfig retorna a configuração do banco de dados
func (db *DB) GetConfig() Config {
	return db.config
}

// GetType retorna o tipo do banco de dados
func (db *DB) GetType() DBType {
	return db.config.Type
}

// IsPostgreSQL verifica se o banco é PostgreSQL
func (db *DB) IsPostgreSQL() bool {
	return db.config.Type == PostgreSQL
}

// IsMySQL verifica se o banco é MySQL ou MariaDB
func (db *DB) IsMySQL() bool {
	return db.config.Type == MySQL || db.config.Type == MariaDB
}

// IsSQLite verifica se o banco é SQLite
func (db *DB) IsSQLite() bool {
	return db.config.Type == SQLite
}

// NewFromDSN cria uma nova conexão usando uma DSN (Data Source Name) diretamente
// Útil quando você já tem uma connection string pronta
func NewFromDSN(dbType DBType, dsn string) (*DB, error) {
	if dbType == "" {
		return nil, fmt.Errorf("database type is required")
	}
	if dsn == "" {
		return nil, fmt.Errorf("DSN is required")
	}

	sqlDB, err := sql.Open(string(dbType), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configurar pool de conexões com valores padrão
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)

	// Testar conexão
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		DB: sqlDB,
		config: Config{
			Type:         dbType,
			MaxOpenConns: 10,
			MaxIdleConns: 5,
		},
	}, nil
}
