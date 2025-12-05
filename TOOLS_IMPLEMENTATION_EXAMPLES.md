# Agno Go Tools - Exemplos de Implementação

## Implementações Base para Tier 1

### 1. CSV Tools - Implementação Base

```go
// agno/tools/csv_tools.go
package tools

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// CSVTool provides CSV file operations
type CSVTool struct {
	toolkit.Toolkit
	MaxFileSize int64 // Maximum file size in bytes (default: 10MB)
	MaxRows     int   // Maximum rows to process
}

// ReadCSVParams represents parameters for reading CSV files
type ReadCSVParams struct {
	FilePath  string `json:"file_path" description:"Path to the CSV file" required:"true"`
	Delimiter string `json:"delimiter,omitempty" description:"CSV delimiter (default: comma)"`
	Limit     int    `json:"limit,omitempty" description:"Maximum rows to return"`
	Skip      int    `json:"skip,omitempty" description:"Number of rows to skip from the start"`
}

// WriteCSVParams represents parameters for writing CSV files
type WriteCSVParams struct {
	FilePath string        `json:"file_path" description:"Path to save the CSV file" required:"true"`
	Headers  []string      `json:"headers" description:"CSV headers" required:"true"`
	Data     [][]string    `json:"data" description:"CSV data rows" required:"true"`
	Append   bool          `json:"append,omitempty" description:"Append to existing file instead of overwriting"`
}

// FilterCSVParams represents parameters for filtering CSV data
type FilterCSVParams struct {
	FilePath  string `json:"file_path" description:"Path to the CSV file" required:"true"`
	Column    string `json:"column" description:"Column name to filter on" required:"true"`
	Operator  string `json:"operator" description:"Operator: equals, contains, gt, lt, between" required:"true"`
	Value     string `json:"value" description:"Value to filter" required:"true"`
	Value2    string `json:"value2,omitempty" description:"Second value for 'between' operator"`
	Limit     int    `json:"limit,omitempty" description:"Maximum rows to return"`
}

// AggregateCSVParams represents parameters for aggregating CSV data
type AggregateCSVParams struct {
	FilePath    string `json:"file_path" description:"Path to the CSV file" required:"true"`
	GroupByCol  string `json:"group_by_col" description:"Column to group by" required:"true"`
	AggregateCol string `json:"aggregate_col" description:"Column to aggregate" required:"true"`
	Operation   string `json:"operation" description:"Operation: sum, avg, count, min, max" required:"true"`
}

// CSVReadResult represents the result of CSV read operation
type CSVReadResult struct {
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
	RowCount int        `json:"row_count"`
	Success  bool       `json:"success"`
	Error    string     `json:"error,omitempty"`
}

// NewCSVTool creates a new CSVTool instance
func NewCSVTool(maxFileSize int64, maxRows int) *CSVTool {
	ct := &CSVTool{
		MaxFileSize: maxFileSize,
		MaxRows:     maxRows,
	}
	ct.Toolkit = toolkit.NewToolkit()
	ct.Toolkit.Name = "CSVTool"
	ct.Toolkit.Description = "A comprehensive CSV tool for reading, writing, filtering, aggregating, and transforming CSV files with support for large datasets"

	// Register methods
	ct.Toolkit.Register("ReadCSV", "Read and parse CSV file", ct, ct.ReadCSV, ReadCSVParams{})
	ct.Toolkit.Register("WriteCSV", "Write data to CSV file", ct, ct.WriteCSV, WriteCSVParams{})
	ct.Toolkit.Register("FilterCSV", "Filter CSV data based on conditions", ct, ct.FilterCSV, FilterCSVParams{})
	ct.Toolkit.Register("AggregateCSV", "Aggregate CSV data with grouping", ct, ct.AggregateCSV, AggregateCSVParams{})

	return ct
}

// ReadCSV reads and parses a CSV file
func (ct *CSVTool) ReadCSV(params ReadCSVParams) (interface{}, error) {
	file, err := os.Open(params.FilePath)
	if err != nil {
		return CSVReadResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to open file: %v", err),
		}, nil
	}
	defer file.Close()

	// Check file size
	stat, _ := file.Stat()
	if stat.Size() > ct.MaxFileSize {
		return CSVReadResult{
			Success: false,
			Error:   fmt.Sprintf("File size exceeds maximum allowed: %d > %d", stat.Size(), ct.MaxFileSize),
		}, nil
	}

	delimiter := ','
	if params.Delimiter != "" {
		delim := rune(params.Delimiter[0])
		delimiter = delim
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

	// Skip rows if specified
	for i := 0; i < params.Skip; i++ {
		_, err := reader.Read()
		if err != nil {
			break
		}
	}

	// Read data rows
	var rows [][]string
	rowCount := 0
	limit := params.Limit
	if limit == 0 || limit > ct.MaxRows {
		limit = ct.MaxRows
	}

	for rowCount < limit {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		rows = append(rows, row)
		rowCount++
	}

	return CSVReadResult{
		Headers:  headers,
		Rows:     rows,
		RowCount: rowCount,
		Success:  true,
	}, nil
}

// WriteCSV writes data to a CSV file
func (ct *CSVTool) WriteCSV(params WriteCSVParams) (interface{}, error) {
	var file *os.File
	var err error

	if params.Append {
		file, err = os.OpenFile(params.FilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	} else {
		file, err = os.Create(params.FilePath)
	}

	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to open/create file: %v", err),
		}, nil
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write headers if not appending
	if !params.Append {
		writer.Write(params.Headers)
	}

	// Write data
	for _, row := range params.Data {
		writer.Write(row)
	}

	return map[string]interface{}{
		"success":    true,
		"file_path": params.FilePath,
		"rows_written": len(params.Data),
	}, nil
}

// FilterCSV filters CSV data based on conditions
func (ct *CSVTool) FilterCSV(params FilterCSVParams) (interface{}, error) {
	// First read the entire file
	readResult, _ := ct.ReadCSV(ReadCSVParams{
		FilePath: params.FilePath,
		Limit:    ct.MaxRows,
	})

	result := readResult.(CSVReadResult)
	if !result.Success {
		return result, nil
	}

	// Find the column index
	columnIndex := -1
	for i, header := range result.Headers {
		if header == params.Column {
			columnIndex = i
			break
		}
	}

	if columnIndex == -1 {
		return CSVReadResult{
			Success: false,
			Error:   fmt.Sprintf("Column '%s' not found", params.Column),
		}, nil
	}

	// Filter rows
	var filteredRows [][]string
	count := 0
	for _, row := range result.Rows {
		if columnIndex >= len(row) {
			continue
		}

		cellValue := row[columnIndex]
		if ct.matchesCondition(cellValue, params.Operator, params.Value, params.Value2) {
			filteredRows = append(filteredRows, row)
			count++
			if params.Limit > 0 && count >= params.Limit {
				break
			}
		}
	}

	return CSVReadResult{
		Headers:  result.Headers,
		Rows:     filteredRows,
		RowCount: count,
		Success:  true,
	}, nil
}

// AggregateCSV aggregates CSV data
func (ct *CSVTool) AggregateCSV(params AggregateCSVParams) (interface{}, error) {
	readResult, _ := ct.ReadCSV(ReadCSVParams{
		FilePath: params.FilePath,
		Limit:    ct.MaxRows,
	})

	result := readResult.(CSVReadResult)
	if !result.Success {
		return result, nil
	}

	// Find column indices
	groupIdx := -1
	aggIdx := -1
	for i, header := range result.Headers {
		if header == params.GroupByCol {
			groupIdx = i
		}
		if header == params.AggregateCol {
			aggIdx = i
		}
	}

	if groupIdx == -1 || aggIdx == -1 {
		return map[string]interface{}{
			"success": false,
			"error":   "Column not found",
		}, nil
	}

	// Aggregate data
	groups := make(map[string][]float64)
	for _, row := range result.Rows {
		if groupIdx >= len(row) || aggIdx >= len(row) {
			continue
		}
		groupKey := row[groupIdx]
		value, _ := strconv.ParseFloat(row[aggIdx], 64)
		groups[groupKey] = append(groups[groupKey], value)
	}

	// Calculate aggregation
	aggregatedData := [][]string{
		{params.GroupByCol, params.Operation},
	}

	for groupKey := range groups {
		aggregatedValue := ct.calculateAggregation(groups[groupKey], params.Operation)
		aggregatedData = append(aggregatedData, []string{groupKey, fmt.Sprintf("%.2f", aggregatedValue)})
	}

	return map[string]interface{}{
		"success": true,
		"data":    aggregatedData,
	}, nil
}

// Helper functions
func (ct *CSVTool) matchesCondition(cellValue, operator, value, value2 string) bool {
	switch operator {
	case "equals":
		return cellValue == value
	case "contains":
		return strings.Contains(cellValue, value)
	case "gt":
		cellNum, _ := strconv.ParseFloat(cellValue, 64)
		valueNum, _ := strconv.ParseFloat(value, 64)
		return cellNum > valueNum
	case "lt":
		cellNum, _ := strconv.ParseFloat(cellValue, 64)
		valueNum, _ := strconv.ParseFloat(value, 64)
		return cellNum < valueNum
	case "between":
		cellNum, _ := strconv.ParseFloat(cellValue, 64)
		val1, _ := strconv.ParseFloat(value, 64)
		val2, _ := strconv.ParseFloat(value2, 64)
		return cellNum >= val1 && cellNum <= val2
	default:
		return false
	}
}

func (ct *CSVTool) calculateAggregation(values []float64, operation string) float64 {
	if len(values) == 0 {
		return 0
	}

	switch operation {
	case "sum":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum
	case "avg":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values))
	case "count":
		return float64(len(values))
	case "min":
		min := values[0]
		for _, v := range values {
			if v < min {
				min = v
			}
		}
		return min
	case "max":
		max := values[0]
		for _, v := range values {
			if v > max {
				max = v
			}
		}
		return max
	default:
		return 0
	}
}
```

---

### 2. Environment/Config Tools - Implementação Base

```go
// agno/tools/env_config_tools.go
package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"gopkg.in/yaml.v3"
)

// EnvConfigTool provides environment and configuration management
type EnvConfigTool struct {
	toolkit.Toolkit
	EnvVars map[string]string
}

// LoadEnvFileParams represents parameters for loading env files
type LoadEnvFileParams struct {
	FilePath string `json:"file_path" description:"Path to .env file" required:"true"`
	Override bool   `json:"override,omitempty" description:"Override existing environment variables"`
}

// GetEnvVarParams represents parameters for getting env variables
type GetEnvVarParams struct {
	Key          string `json:"key" description:"Environment variable key" required:"true"`
	DefaultValue string `json:"default_value,omitempty" description:"Default value if key not found"`
}

// SetEnvVarParams represents parameters for setting env variables
type SetEnvVarParams struct {
	Key   string `json:"key" description:"Environment variable key" required:"true"`
	Value string `json:"value" description:"Environment variable value" required:"true"`
}

// LoadConfigParams represents parameters for loading configuration files
type LoadConfigParams struct {
	FilePath string `json:"file_path" description:"Path to config file (json, yaml, toml)" required:"true"`
}

// ValidateEnvParams represents parameters for validating environment
type ValidateEnvParams struct {
	RequiredVars []string `json:"required_vars" description:"List of required environment variables" required:"true"`
}

// NewEnvConfigTool creates a new EnvConfigTool instance
func NewEnvConfigTool() *EnvConfigTool {
	ect := &EnvConfigTool{
		EnvVars: make(map[string]string),
	}
	ect.Toolkit = toolkit.NewToolkit()
	ect.Toolkit.Name = "EnvConfigTool"
	ect.Toolkit.Description = "Manages environment variables and configuration files with support for .env, JSON, and YAML formats"

	ect.Toolkit.Register("LoadEnvFile", "Load environment variables from .env file", ect, ect.LoadEnvFile, LoadEnvFileParams{})
	ect.Toolkit.Register("GetEnvVar", "Get environment variable value", ect, ect.GetEnvVar, GetEnvVarParams{})
	ect.Toolkit.Register("SetEnvVar", "Set environment variable", ect, ect.SetEnvVar, SetEnvVarParams{})
	ect.Toolkit.Register("LoadConfig", "Load configuration file", ect, ect.LoadConfig, LoadConfigParams{})
	ect.Toolkit.Register("ValidateEnv", "Validate required environment variables", ect, ect.ValidateEnv, ValidateEnvParams{})

	return ect
}

// LoadEnvFile loads environment variables from a .env file
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
	loadedVars := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		ect.EnvVars[key] = value
		if params.Override {
			os.Setenv(key, value)
		}
		loadedVars++
	}

	return map[string]interface{}{
		"success":       true,
		"vars_loaded":   loadedVars,
		"file_path":     params.FilePath,
	}, nil
}

// GetEnvVar gets an environment variable value
func (ect *EnvConfigTool) GetEnvVar(params GetEnvVarParams) (interface{}, error) {
	// Check internal storage first
	if value, exists := ect.EnvVars[params.Key]; exists {
		return map[string]interface{}{
			"success": true,
			"key":     params.Key,
			"value":   value,
		}, nil
	}

	// Check system environment
	if value, exists := os.LookupEnv(params.Key); exists {
		return map[string]interface{}{
			"success": true,
			"key":     params.Key,
			"value":   value,
		}, nil
	}

	// Return default or error
	if params.DefaultValue != "" {
		return map[string]interface{}{
			"success":      true,
			"key":          params.Key,
			"value":        params.DefaultValue,
			"is_default":   true,
		}, nil
	}

	return map[string]interface{}{
		"success": false,
		"error":   fmt.Sprintf("Environment variable '%s' not found", params.Key),
	}, nil
}

// SetEnvVar sets an environment variable
func (ect *EnvConfigTool) SetEnvVar(params SetEnvVarParams) (interface{}, error) {
	err := os.Setenv(params.Key, params.Value)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to set environment variable: %v", err),
		}, nil
	}

	ect.EnvVars[params.Key] = params.Value

	return map[string]interface{}{
		"success": true,
		"key":     params.Key,
		"value":   params.Value,
	}, nil
}

// LoadConfig loads a configuration file (JSON, YAML, etc.)
func (ect *EnvConfigTool) LoadConfig(params LoadConfigParams) (interface{}, error) {
	data, err := os.ReadFile(params.FilePath)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to read file: %v", err),
		}, nil
	}

	ext := filepath.Ext(params.FilePath)
	var config interface{}

	switch ext {
	case ".json":
		err = json.Unmarshal(data, &config)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &config)
	default:
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Unsupported file format: %s", ext),
		}, nil
	}

	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to parse config: %v", err),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"config":  config,
		"format":  ext,
	}, nil
}

// ValidateEnv validates that required environment variables are set
func (ect *EnvConfigTool) ValidateEnv(params ValidateEnvParams) (interface{}, error) {
	missing := []string{}
	found := []string{}

	for _, varName := range params.RequiredVars {
		if _, exists := ect.EnvVars[varName]; exists {
			found = append(found, varName)
		} else if _, exists := os.LookupEnv(varName); exists {
			found = append(found, varName)
		} else {
			missing = append(missing, varName)
		}
	}

	return map[string]interface{}{
		"success":     len(missing) == 0,
		"found":       found,
		"missing":     missing,
		"total_found": len(found),
		"total_missing": len(missing),
	}, nil
}
```

---

### 3. Go Dev Tools - Exemplo Base (NOVO)

```go
// agno/tools/go_dev_tools.go
package tools

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// GoDevTool provides Go-specific development operations
type GoDevTool struct {
	toolkit.Toolkit
	ProjectRoot string
}

// RunGoTestParams represents parameters for running tests
type RunGoTestParams struct {
	Packages string `json:"packages,omitempty" description:"Packages to test (e.g., './...' for all)"`
	Coverage bool   `json:"coverage,omitempty" description:"Generate coverage report"`
	Verbose  bool   `json:"verbose,omitempty" description:"Verbose output"`
	Race     bool   `json:"race,omitempty" description:"Enable race detector"`
}

// BuildGoParams represents parameters for building
type BuildGoParams struct {
	Main      string `json:"main,omitempty" description:"Main package path (default: current directory)"`
	Output    string `json:"output,omitempty" description:"Output binary name"`
	OS        string `json:"os,omitempty" description:"Target OS (linux, darwin, windows)"`
	Arch      string `json:"arch,omitempty" description:"Target architecture (amd64, arm64, etc.)"`
	CGOEnabled bool  `json:"cgo_enabled,omitempty" description:"Enable CGO"`
}

// RunGoTestResult represents test result
type RunGoTestResult struct {
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Coverage string `json:"coverage,omitempty"`
}

// NewGoDevTool creates a new GoDevTool instance
func NewGoDevTool(projectRoot string) *GoDevTool {
	gdt := &GoDevTool{
		ProjectRoot: projectRoot,
	}
	gdt.Toolkit = toolkit.NewToolkit()
	gdt.Toolkit.Name = "GoDevTool"
	gdt.Toolkit.Description = "Go development tools for building, testing, linting, and profiling Go projects"

	gdt.Toolkit.Register("RunGoTest", "Run Go tests with optional coverage", gdt, gdt.RunGoTest, RunGoTestParams{})
	gdt.Toolkit.Register("BuildGoBinary", "Build Go binary with cross-compilation support", gdt, gdt.BuildGoBinary, BuildGoParams{})

	return gdt
}

// RunGoTest runs Go tests
func (gdt *GoDevTool) RunGoTest(params RunGoTestParams) (interface{}, error) {
	packages := params.Packages
	if packages == "" {
		packages = "./..."
	}

	args := []string{"test", packages}

	if params.Verbose {
		args = append(args, "-v")
	}

	if params.Coverage {
		args = append(args, "-cover", "-coverprofile=coverage.out")
	}

	if params.Race {
		args = append(args, "-race")
	}

	cmd := exec.Command("go", args...)
	output, err := cmd.CombinedOutput()

	return RunGoTestResult{
		Success: err == nil,
		Output:  string(output),
		Error:   fmt.Sprintf("%v", err),
	}, nil
}

// BuildGoBinary builds a Go binary
func (gdt *GoDevTool) BuildGoBinary(params BuildGoParams) (interface{}, error) {
	main := params.Main
	if main == "" {
		main = "."
	}

	output := params.Output
	if output == "" {
		output = filepath.Base(main)
		if runtime.GOOS == "windows" {
			output += ".exe"
		}
	}

	osTarget := params.OS
	if osTarget == "" {
		osTarget = runtime.GOOS
	}

	arch := params.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}

	// Windows compatibility for output name
	if osTarget == "windows" && !strings.HasSuffix(output, ".exe") {
		output += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", output, main)
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("GOOS=%s", osTarget))
	cmd.Env = append(cmd.Env, fmt.Sprintf("GOARCH=%s", arch))

	if !params.CGOEnabled {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}

	output_data, err := cmd.CombinedOutput()

	return map[string]interface{}{
		"success":  err == nil,
		"output":   string(output_data),
		"binary":   output,
		"os":       osTarget,
		"arch":     arch,
		"error":    fmt.Sprintf("%v", err),
	}, nil
}
```

---

## Exemplos de Uso

### CSV Tools
```python
# Uso em agente
agent = Agent(tools=[CSVTool(10*1024*1024, 100000)])

# Ler CSV
read_result = agent.call("Read the file data.csv")

# Filtrar dados
filter_result = agent.call("Filter data.csv where status equals 'active'")

# Agregar dados
agg_result = agent.call("Sum sales by region from sales.csv")
```

### Environment Config Tools
```go
// Uso em agente
tool := NewEnvConfigTool()
agent := Agent(tools=[tool])

// Carregar .env
agent.call("Load environment variables from .env.local")

// Validar
agent.call("Validate that DATABASE_URL and API_KEY are set")
```

### Go Dev Tools
```go
// Uso em agente
tool := NewGoDevTool("/home/user/myproject")
agent := Agent(tools=[tool])

// Testar
agent.call("Run all tests with coverage and race detector")

// Compilar
agent.call("Build cross-platform binaries for linux/amd64 and darwin/arm64")
```

---

## Próximos Passos

1. Implementar testes unitários para cada tool
2. Adicionar integração com o Agent
3. Criar exemplos completos
4. Adicionar documentação
5. Publicar para uso

---

**Nota**: Cada tool deve ser testado com dados reais antes de ser usado em produção.
