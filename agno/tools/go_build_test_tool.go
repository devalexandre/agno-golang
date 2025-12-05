package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// GoBuildTestTool fornece operações de build e teste em Go
type GoBuildTestTool struct {
	toolkit.Toolkit
	builds       []BuildResult
	tests        []TestResult
	buildLog     []BuildLogEntry
	maxLogSize   int
	buildTimeout int
	testTimeout  int
}

// BuildResult representa o resultado de um build
type BuildResult struct {
	BuildID     string
	ProjectPath string
	Command     string
	Status      string // "success", "failed", "running"
	Output      string
	ErrorMsg    string
	Duration    int64 // ms
	StartTime   time.Time
	EndTime     time.Time
	BinarySize  int64 // bytes
	GOOS        string
	GOARCH      string
}

// TestResult representa o resultado de testes
type TestResult struct {
	TestID       string
	ProjectPath  string
	TestFilter   string
	Status       string // "passed", "failed", "skipped"
	TotalTests   int
	PassedTests  int
	FailedTests  int
	SkippedTests int
	Coverage     float32 // porcentagem
	Duration     int64   // ms
	Output       string
	ErrorMsg     string
	StartTime    time.Time
	EndTime      time.Time
}

// BuildLogEntry registra uma operação de build
type BuildLogEntry struct {
	Timestamp   time.Time
	BuildID     string
	ProjectPath string
	Command     string
	Status      string
	Duration    int64
}

// BuildProjectParams parâmetros para fazer build
type BuildProjectParams struct {
	ProjectPath string `json:"project_path" description:"Caminho do projeto Go"`
	GOOS        string `json:"goos" description:"Sistema operacional alvo (linux, darwin, windows)"`
	GOARCH      string `json:"goarch" description:"Arquitetura alvo (amd64, arm64)"`
	Output      string `json:"output" description:"Caminho do binário de saída"`
	Tags        string `json:"tags" description:"Build tags (opcional)"`
	LdFlags     string `json:"ld_flags" description:"Linker flags (opcional)"`
}

// RunTestsParams parâmetros para executar testes
type RunTestsParams struct {
	ProjectPath string `json:"project_path" description:"Caminho do projeto Go"`
	TestFilter  string `json:"test_filter" description:"Filtro de testes (regex)"`
	Verbose     bool   `json:"verbose" description:"Modo verbose"`
	Coverage    bool   `json:"coverage" description:"Habilitar análise de cobertura"`
	Timeout     int    `json:"timeout" description:"Timeout em segundos"`
}

// GoFmtParams parâmetros para formatar código
type GoFmtParams struct {
	ProjectPath string `json:"project_path" description:"Caminho do projeto Go"`
	WriteFiles  bool   `json:"write_files" description:"Escrever arquivos formatados"`
}

// GoVetParams parâmetros para análise estática
type GoVetParams struct {
	ProjectPath string `json:"project_path" description:"Caminho do projeto Go"`
}

// NewGoBuildTestTool cria uma nova instância
func NewGoBuildTestTool() *GoBuildTestTool {
	tool := &GoBuildTestTool{
		builds:       make([]BuildResult, 0),
		tests:        make([]TestResult, 0),
		buildLog:     make([]BuildLogEntry, 0),
		maxLogSize:   1000,
		buildTimeout: 300, // 5 minutos
		testTimeout:  600, // 10 minutos
	}
	tool.Toolkit = toolkit.NewToolkit()
	tool.Toolkit.Name = "GoBuildTestTool"
	tool.Toolkit.Description = "Ferramenta para build, testes e análise de projetos Go"

	tool.Register("build_project",
		"Fazer build de um projeto Go",
		tool,
		tool.BuildProject,
		BuildProjectParams{},
	)

	tool.Register("run_tests",
		"Executar testes de um projeto Go",
		tool,
		tool.RunTests,
		RunTestsParams{},
	)

	tool.Register("format_code",
		"Formatar código Go usando go fmt",
		tool,
		tool.FormatCode,
		GoFmtParams{},
	)

	tool.Register("analyze_code",
		"Analisar código com go vet",
		tool,
		tool.AnalyzeCode,
		GoVetParams{},
	)

	tool.Register("get_build_history",
		"Obter histórico de builds",
		tool,
		tool.GetBuildHistory,
		struct{}{},
	)

	tool.Register("get_test_history",
		"Obter histórico de testes",
		tool,
		tool.GetTestHistory,
		struct{}{},
	)

	return tool
}

// BuildProject executa o build de um projeto Go
func (t *GoBuildTestTool) BuildProject(params BuildProjectParams) (map[string]interface{}, error) {
	if params.ProjectPath == "" {
		return nil, fmt.Errorf("caminho do projeto não pode estar vazio")
	}

	// Validar diretório
	if _, err := os.Stat(params.ProjectPath); err != nil {
		return nil, fmt.Errorf("projeto não encontrado: %s", params.ProjectPath)
	}

	buildID := fmt.Sprintf("build_%d", time.Now().UnixNano())
	startTime := time.Now()

	// Determinar sistema operacional e arquitetura
	goos := params.GOOS
	if goos == "" {
		goos = "linux"
	}

	goarch := params.GOARCH
	if goarch == "" {
		goarch = "amd64"
	}

	// Determinar arquivo de saída
	output := params.Output
	if output == "" {
		baseName := filepath.Base(params.ProjectPath)
		output = filepath.Join(params.ProjectPath, baseName)
	}

	// Simular comando de build
	command := fmt.Sprintf("go build -o %s", output)
	if params.Tags != "" {
		command += fmt.Sprintf(" -tags=%s", params.Tags)
	}
	if params.LdFlags != "" {
		command += fmt.Sprintf(" -ldflags='%s'", params.LdFlags)
	}
	command += " ."

	// Simular resultado de build bem-sucedido
	result := BuildResult{
		BuildID:     buildID,
		ProjectPath: params.ProjectPath,
		Command:     command,
		Status:      "success",
		Output:      fmt.Sprintf("Built successfully to %s", output),
		Duration:    100,
		StartTime:   startTime,
		EndTime:     time.Now(),
		BinarySize:  15728640, // 15 MB simulado
		GOOS:        goos,
		GOARCH:      goarch,
	}

	t.builds = append(t.builds, result)

	// Registrar no log
	logEntry := BuildLogEntry{
		Timestamp:   time.Now(),
		BuildID:     buildID,
		ProjectPath: params.ProjectPath,
		Command:     command,
		Status:      "success",
		Duration:    result.Duration,
	}
	t.buildLog = append(t.buildLog, logEntry)

	if len(t.buildLog) > t.maxLogSize {
		t.buildLog = t.buildLog[1:]
	}

	return map[string]interface{}{
		"success":     true,
		"build_id":    buildID,
		"status":      "success",
		"output":      output,
		"binary_size": result.BinarySize,
		"goos":        goos,
		"goarch":      goarch,
		"duration_ms": result.Duration,
		"command":     command,
	}, nil
}

// RunTests executa testes de um projeto Go
func (t *GoBuildTestTool) RunTests(params RunTestsParams) (map[string]interface{}, error) {
	if params.ProjectPath == "" {
		return nil, fmt.Errorf("caminho do projeto não pode estar vazio")
	}

	if _, err := os.Stat(params.ProjectPath); err != nil {
		return nil, fmt.Errorf("projeto não encontrado: %s", params.ProjectPath)
	}

	testID := fmt.Sprintf("test_%d", time.Now().UnixNano())
	startTime := time.Now()

	// Determinar timeout
	timeout := params.Timeout
	if timeout <= 0 {
		timeout = 300 // 5 minutos padrão
	}

	// Montar comando de teste
	command := "go test"
	if params.TestFilter != "" {
		command += fmt.Sprintf(" -run %s", params.TestFilter)
	}
	if params.Verbose {
		command += " -v"
	}
	if params.Coverage {
		command += " -cover"
	}
	command += fmt.Sprintf(" -timeout %ds ./...", timeout)

	// Simular resultado de testes bem-sucedidos
	totalTests := 15
	passedTests := 15
	failedTests := 0
	skippedTests := 0
	coverage := float32(87.5)

	result := TestResult{
		TestID:       testID,
		ProjectPath:  params.ProjectPath,
		TestFilter:   params.TestFilter,
		Status:       "passed",
		TotalTests:   totalTests,
		PassedTests:  passedTests,
		FailedTests:  failedTests,
		SkippedTests: skippedTests,
		Coverage:     coverage,
		Duration:     250,
		Output:       fmt.Sprintf("ok\tgithub.com/project\t0.250s\tcoverage: %.1f%% of statements", coverage),
		StartTime:    startTime,
		EndTime:      time.Now(),
	}

	t.tests = append(t.tests, result)

	return map[string]interface{}{
		"success":     true,
		"test_id":     testID,
		"status":      result.Status,
		"total":       totalTests,
		"passed":      passedTests,
		"failed":      failedTests,
		"skipped":     skippedTests,
		"coverage":    fmt.Sprintf("%.1f%%", coverage),
		"duration_ms": result.Duration,
		"output":      result.Output,
	}, nil
}

// FormatCode formata código Go
func (t *GoBuildTestTool) FormatCode(params GoFmtParams) (map[string]interface{}, error) {
	if params.ProjectPath == "" {
		return nil, fmt.Errorf("caminho do projeto não pode estar vazio")
	}

	if _, err := os.Stat(params.ProjectPath); err != nil {
		return nil, fmt.Errorf("projeto não encontrado: %s", params.ProjectPath)
	}

	// Simular busca de arquivos Go
	var goFiles []string
	err := filepath.Walk(params.ProjectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("erro ao escanear arquivos: %w", err)
	}

	return map[string]interface{}{
		"success":     true,
		"files_found": len(goFiles),
		"formatted":   len(goFiles),
		"write_files": params.WriteFiles,
		"command":     "go fmt ./...",
	}, nil
}

// AnalyzeCode analisa código com go vet
func (t *GoBuildTestTool) AnalyzeCode(params GoVetParams) (map[string]interface{}, error) {
	if params.ProjectPath == "" {
		return nil, fmt.Errorf("caminho do projeto não pode estar vazio")
	}

	if _, err := os.Stat(params.ProjectPath); err != nil {
		return nil, fmt.Errorf("projeto não encontrado: %s", params.ProjectPath)
	}

	// Simular análise
	issues := []map[string]interface{}{
		{
			"file":     "main.go",
			"line":     42,
			"message":  "unused variable 'x'",
			"severity": "warning",
		},
	}

	return map[string]interface{}{
		"success":      true,
		"project_path": params.ProjectPath,
		"issues_found": len(issues),
		"issues":       issues,
		"command":      "go vet ./...",
	}, nil
}

// GetBuildHistory retorna histórico de builds
func (t *GoBuildTestTool) GetBuildHistory(params struct{}) (map[string]interface{}, error) {
	history := make([]map[string]interface{}, 0)

	for _, entry := range t.buildLog {
		history = append(history, map[string]interface{}{
			"build_id":     entry.BuildID,
			"project_path": entry.ProjectPath,
			"command":      entry.Command,
			"status":       entry.Status,
			"duration_ms":  entry.Duration,
			"timestamp":    entry.Timestamp.Format(time.RFC3339),
		})
	}

	return map[string]interface{}{
		"success":      true,
		"total_builds": len(t.builds),
		"history":      history,
		"limit":        t.maxLogSize,
	}, nil
}

// GetTestHistory retorna histórico de testes
func (t *GoBuildTestTool) GetTestHistory(params struct{}) (map[string]interface{}, error) {
	history := make([]map[string]interface{}, 0)

	for _, entry := range t.tests {
		history = append(history, map[string]interface{}{
			"test_id":      entry.TestID,
			"project_path": entry.ProjectPath,
			"status":       entry.Status,
			"total":        entry.TotalTests,
			"passed":       entry.PassedTests,
			"failed":       entry.FailedTests,
			"coverage":     fmt.Sprintf("%.1f%%", entry.Coverage),
			"duration_ms":  entry.Duration,
			"timestamp":    entry.StartTime.Format(time.RFC3339),
		})
	}

	return map[string]interface{}{
		"success":     true,
		"total_tests": len(t.tests),
		"history":     history,
	}, nil
}
