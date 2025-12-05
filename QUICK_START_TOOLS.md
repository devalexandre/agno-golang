# 🚀 Quick Start Guide - Implementando os Primeiros Tools

## Objetivo

Guia prático para implementar os primeiros 3 tools em 1-2 semanas.

---

## Phase 1: Setup (1-2 dias)

### 1.1 Criar Branch de Desenvolvimento

```bash
git checkout -b feature/new-tools-tier1
```

### 1.2 Criar Estrutura Base

```bash
# Criar diretórios para novos tools
mkdir -p agno/tools/csv
mkdir -p agno/tools/env
mkdir -p agno/tools/git

# Criar arquivos base
touch agno/tools/csv_tools.go
touch agno/tools/env_config_tools.go
touch agno/tools/git_tools.go

# Testes
touch agno/tools/csv_tools_test.go
touch agno/tools/env_config_tools_test.go
touch agno/tools/git_tools_test.go
```

### 1.3 Atualizar go.mod se necessário

```bash
# Adicionar dependências necessárias
go get github.com/go-git/go-git/v5
go get gopkg.in/yaml.v3
```

---

## Phase 2: Implementação - CSV Tools (3-4 dias)

### Passo 1: Criar Base da Ferramenta

```go
// agno/tools/csv_tools.go
package tools

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// CSVTool provides CSV file operations
type CSVTool struct {
	toolkit.Toolkit
	MaxFileSize int64
	MaxRows     int
}

// NewCSVTool creates a new CSVTool instance
func NewCSVTool(maxFileSize int64, maxRows int) *CSVTool {
	ct := &CSVTool{
		MaxFileSize: maxFileSize,
		MaxRows:     maxRows,
	}
	ct.Toolkit = toolkit.NewToolkit()
	ct.Toolkit.Name = "CSVTool"
	ct.Toolkit.Description = "CSV file operations tool"

	// Register methods
	ct.Toolkit.Register("ReadCSV", "Read CSV file", ct, ct.ReadCSV, ReadCSVParams{})
	ct.Toolkit.Register("WriteCSV", "Write CSV file", ct, ct.WriteCSV, WriteCSVParams{})

	return ct
}

// ReadCSVParams represents parameters for reading CSV files
type ReadCSVParams struct {
	FilePath  string `json:"file_path" description:"Path to CSV file" required:"true"`
	Delimiter string `json:"delimiter,omitempty" description:"CSV delimiter"`
	Limit     int    `json:"limit,omitempty" description:"Max rows to return"`
}

// WriteCSVParams represents parameters for writing CSV files
type WriteCSVParams struct {
	FilePath string     `json:"file_path" description:"Path to save CSV" required:"true"`
	Headers  []string   `json:"headers" description:"Column headers" required:"true"`
	Data     [][]string `json:"data" description:"CSV data" required:"true"`
}

// CSVReadResult represents read result
type CSVReadResult struct {
	Headers  []string   `json:"headers"`
	Rows     [][]string `json:"rows"`
	RowCount int        `json:"row_count"`
	Success  bool       `json:"success"`
	Error    string     `json:"error,omitempty"`
}

// ReadCSV reads CSV file
func (ct *CSVTool) ReadCSV(params ReadCSVParams) (interface{}, error) {
	file, err := os.Open(params.FilePath)
	if err != nil {
		return CSVReadResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to open file: %v", err),
		}, nil
	}
	defer file.Close()

	stat, _ := file.Stat()
	if stat.Size() > ct.MaxFileSize {
		return CSVReadResult{
			Success: false,
			Error:   "File size exceeds maximum",
		}, nil
	}

	delimiter := ','
	if params.Delimiter != "" {
		delimiter = rune(params.Delimiter[0])
	}

	reader := csv.NewReader(file)
	reader.Comma = delimiter

	// Read headers
	headers, err := reader.Read()
	if err != nil {
		return CSVReadResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to read headers: %v", err),
		}, nil
	}

	// Read data
	var rows [][]string
	limit := params.Limit
	if limit == 0 || limit > ct.MaxRows {
		limit = ct.MaxRows
	}

	for i := 0; i < limit; i++ {
		row, err := reader.Read()
		if err != nil {
			break
		}
		rows = append(rows, row)
	}

	return CSVReadResult{
		Headers:  headers,
		Rows:     rows,
		RowCount: len(rows),
		Success:  true,
	}, nil
}

// WriteCSV writes CSV file
func (ct *CSVTool) WriteCSV(params WriteCSVParams) (interface{}, error) {
	file, err := os.Create(params.FilePath)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to create file: %v", err),
		}, nil
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write(params.Headers)
	for _, row := range params.Data {
		writer.Write(row)
	}

	return map[string]interface{}{
		"success":       true,
		"file_path":     params.FilePath,
		"rows_written": len(params.Data),
	}, nil
}
```

### Passo 2: Criar Testes Básicos

```go
// agno/tools/csv_tools_test.go
package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCSV(t *testing.T) {
	// Create temporary CSV file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.csv")

	// Write test data
	file, _ := os.Create(testFile)
	file.WriteString("name,age,email\n")
	file.WriteString("John,30,john@example.com\n")
	file.WriteString("Jane,28,jane@example.com\n")
	file.Close()

	// Test read
	ct := NewCSVTool(1024*1024, 1000)
	result, _ := ct.ReadCSV(ReadCSVParams{
		FilePath: testFile,
		Limit:    100,
	})

	csvResult := result.(CSVReadResult)
	if !csvResult.Success {
		t.Errorf("Read failed: %v", csvResult.Error)
	}

	if len(csvResult.Headers) != 3 {
		t.Errorf("Expected 3 headers, got %d", len(csvResult.Headers))
	}

	if csvResult.RowCount != 2 {
		t.Errorf("Expected 2 rows, got %d", csvResult.RowCount)
	}
}

func TestWriteCSV(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "output.csv")

	ct := NewCSVTool(1024*1024, 1000)
	result, _ := ct.WriteCSV(WriteCSVParams{
		FilePath: testFile,
		Headers:  []string{"name", "age"},
		Data: [][]string{
			{"John", "30"},
			{"Jane", "28"},
		},
	})

	writeResult := result.(map[string]interface{})
	if !writeResult["success"].(bool) {
		t.Error("Write failed")
	}

	// Verify file exists
	if _, err := os.Stat(testFile); err != nil {
		t.Errorf("Output file not created: %v", err)
	}
}
```

### Passo 3: Rodar Testes

```bash
cd agno/tools
go test -v -run TestCSV
go test -cover
```

---

## Phase 3: Implementação - Env/Config Tools (2-3 dias)

### Passo 1: Criar Arquivo Base

```go
// agno/tools/env_config_tools.go
package tools

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

type EnvConfigTool struct {
	toolkit.Toolkit
	EnvVars map[string]string
}

func NewEnvConfigTool() *EnvConfigTool {
	ect := &EnvConfigTool{
		EnvVars: make(map[string]string),
	}
	ect.Toolkit = toolkit.NewToolkit()
	ect.Toolkit.Name = "EnvConfigTool"
	ect.Toolkit.Description = "Environment and configuration management"

	ect.Toolkit.Register("LoadEnvFile", "Load .env file", ect, ect.LoadEnvFile, LoadEnvFileParams{})
	ect.Toolkit.Register("GetEnvVar", "Get env variable", ect, ect.GetEnvVar, GetEnvVarParams{})
	ect.Toolkit.Register("SetEnvVar", "Set env variable", ect, ect.SetEnvVar, SetEnvVarParams{})

	return ect
}

type LoadEnvFileParams struct {
	FilePath string `json:"file_path" description:"Path to .env file" required:"true"`
	Override bool   `json:"override,omitempty" description:"Override existing vars"`
}

type GetEnvVarParams struct {
	Key          string `json:"key" description:"Environment variable key" required:"true"`
	DefaultValue string `json:"default_value,omitempty" description:"Default value"`
}

type SetEnvVarParams struct {
	Key   string `json:"key" description:"Variable key" required:"true"`
	Value string `json:"value" description:"Variable value" required:"true"`
}

func (ect *EnvConfigTool) LoadEnvFile(params LoadEnvFileParams) (interface{}, error) {
	file, err := os.Open(params.FilePath)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to open file: %v", err),
		}, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	loaded := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		ect.EnvVars[key] = value
		if params.Override {
			os.Setenv(key, value)
		}
		loaded++
	}

	return map[string]interface{}{
		"success":      true,
		"vars_loaded":  loaded,
		"file_path":    params.FilePath,
	}, nil
}

func (ect *EnvConfigTool) GetEnvVar(params GetEnvVarParams) (interface{}, error) {
	if value, exists := ect.EnvVars[params.Key]; exists {
		return map[string]interface{}{
			"success": true,
			"value":   value,
		}, nil
	}

	if value, exists := os.LookupEnv(params.Key); exists {
		return map[string]interface{}{
			"success": true,
			"value":   value,
		}, nil
	}

	if params.DefaultValue != "" {
		return map[string]interface{}{
			"success":     true,
			"value":       params.DefaultValue,
			"is_default":  true,
		}, nil
	}

	return map[string]interface{}{
		"success": false,
		"error":   fmt.Sprintf("Variable not found: %s", params.Key),
	}, nil
}

func (ect *EnvConfigTool) SetEnvVar(params SetEnvVarParams) (interface{}, error) {
	err := os.Setenv(params.Key, params.Value)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to set: %v", err),
		}, nil
	}

	ect.EnvVars[params.Key] = params.Value

	return map[string]interface{}{
		"success": true,
		"key":     params.Key,
	}, nil
}
```

### Passo 2: Testes

```go
// agno/tools/env_config_tools_test.go
package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	file, _ := os.Create(envFile)
	file.WriteString("DATABASE_URL=postgres://localhost\n")
	file.WriteString("API_KEY=secret123\n")
	file.WriteString("# Comment line\n")
	file.Close()

	ect := NewEnvConfigTool()
	result, _ := ect.LoadEnvFile(LoadEnvFileParams{
		FilePath: envFile,
		Override: false,
	})

	envResult := result.(map[string]interface{})
	if !envResult["success"].(bool) {
		t.Error("Load failed")
	}

	if envResult["vars_loaded"].(int) != 2 {
		t.Errorf("Expected 2 vars loaded, got %d", envResult["vars_loaded"])
	}
}

func TestGetSetEnvVar(t *testing.T) {
	ect := NewEnvConfigTool()

	// Set
	_, _ = ect.SetEnvVar(SetEnvVarParams{
		Key:   "TEST_VAR",
		Value: "test_value",
	})

	// Get
	result, _ := ect.GetEnvVar(GetEnvVarParams{
		Key: "TEST_VAR",
	})

	getResult := result.(map[string]interface{})
	if getResult["value"].(string) != "test_value" {
		t.Error("Value mismatch")
	}
}
```

---

## Phase 4: Go Dev Tools (3-4 dias)

Usar o exemplo já fornecido em `TOOLS_IMPLEMENTATION_EXAMPLES.md` como referência.

---

## Checklist de Implementação

### CSV Tools ✅
- [ ] Implementar ReadCSV
- [ ] Implementar WriteCSV
- [ ] Adicionar FilterCSV
- [ ] Adicionar AggregateCSV
- [ ] Criar testes unitários
- [ ] Criar testes de integração
- [ ] Documentar com exemplos
- [ ] Code review
- [ ] Merge para main

### Env/Config Tools ✅
- [ ] Implementar LoadEnvFile
- [ ] Implementar GetEnvVar/SetEnvVar
- [ ] Implementar LoadConfig (JSON/YAML)
- [ ] Implementar ValidateEnv
- [ ] Criar testes
- [ ] Documentar
- [ ] Code review
- [ ] Merge para main

### Go Dev Tools ✅
- [ ] Implementar RunGoTest
- [ ] Implementar BuildGoBinary
- [ ] Adicionar GoLint
- [ ] Adicionar Benchmark
- [ ] Criar testes
- [ ] Documentar
- [ ] Code review
- [ ] Merge para main

---

## Comandos Úteis

```bash
# Run specific test
go test -v -run TestReadCSV agno/tools

# Run all tool tests
go test -v ./agno/tools/...

# Check coverage
go test -cover ./agno/tools

# Generate coverage report
go test -coverprofile=coverage.out ./agno/tools
go tool cover -html=coverage.out

# Format code
go fmt ./agno/tools/...

# Run linter
golangci-lint run ./agno/tools/

# Build locally
go build ./agno/tools
```

---

## Integração com Agent

Após implementação:

```go
// Adicionar tools ao agent
agent := agno.NewAgent(
	agno.WithTools(
		NewCSVTool(10*1024*1024, 100000),
		NewEnvConfigTool(),
		NewGoDevTool("/path/to/project"),
	),
)

// Usar no agent
response := agent.Call("Read data.csv and show me the first 10 rows")
```

---

## Exemplo de PR Description

```markdown
# Add CSV, Environment, and Go Dev Tools

## Description
Implement Tier 1 core tools for Agno Go:
- CSV Tools: Read, write, filter, aggregate CSV files
- Environment/Config Tools: Load .env files, manage environment variables
- Go Dev Tools: Run tests, build binaries, cross-compilation support

## Changes
- Added csv_tools.go with 4 main functions
- Added env_config_tools.go with 5 main functions
- Added go_dev_tools.go with 2 main functions
- Added comprehensive unit tests
- Added integration tests
- Added documentation with examples

## Testing
- Unit tests: 30+ test cases, 85% coverage
- Integration tests: 8 test cases
- Manual testing: Verified all operations

## Checklist
- [x] Tests passing
- [x] Coverage >80%
- [x] Code formatted
- [x] Linting passed
- [x] Documentation complete
- [x] Examples provided
```

---

## Timeline Realista

```
Day 1: Setup e planning
Day 2-4: CSV Tools
Day 5-6: Env/Config Tools
Day 7: Go Dev Tools
Day 8: Testing e refinement
Day 9-10: Documentation e PR
```

---

## Próximas Tools (Após aprovação)

1. **SQL/Database Tools** (Tier 1)
2. **Git Tools** (Tier 1)
3. **Advanced Debugging** (Tier 3 - Inovação)
4. **Code Analysis** (Tier 3 - Inovação)

---

## Suporte

Dúvidas? Verifique:
1. `TOOLS_IMPLEMENTATION_ROADMAP.md` - Contexto geral
2. `TOOLS_IMPLEMENTATION_EXAMPLES.md` - Exemplos de código
3. `INNOVATIVE_TOOLS_PROPOSALS.md` - Ideias futuras

---

**Vamos começar! 🚀**

