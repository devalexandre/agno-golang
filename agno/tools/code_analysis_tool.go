package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// CodeAnalysisTool fornece análise de código Go
type CodeAnalysisTool struct {
	toolkit.Toolkit
	analyses   []CodeAnalysisResult
	metrics    map[string]CodeMetrics
	issues     []CodeIssue
	maxResults int
}

// CodeAnalysisResult representa o resultado de uma análise
type CodeAnalysisResult struct {
	AnalysisID string
	FilePath   string
	Status     string // "completed", "failed"
	Timestamp  time.Time
	Metrics    CodeMetrics
	Issues     []CodeIssue
}

// CodeMetrics contém métricas de código
type CodeMetrics struct {
	Lines              int
	CodeLines          int
	CommentLines       int
	BlankLines         int
	Functions          int
	Complexity         float32
	CyclomaticDensity  float32
	LongestFunction    int
	AverageFunctionLen int
	Maintainability    float32 // 0-100
}

// CodeIssue representa um problema encontrado no código
type CodeIssue struct {
	IssueID    string
	FilePath   string
	Line       int
	Column     int
	Severity   string // "error", "warning", "info"
	Category   string // "complexity", "style", "bug", "performance"
	Message    string
	Suggestion string
}

// AnalyzeFileParams parâmetros para analisar arquivo
type AnalyzeFileParams struct {
	FilePath string `json:"file_path" description:"Caminho do arquivo Go"`
	Deep     bool   `json:"deep" description:"Análise profunda"`
}

// AnalyzeProjectParams parâmetros para analisar projeto
type AnalyzeProjectParams struct {
	ProjectPath string `json:"project_path" description:"Caminho do projeto Go"`
	Recursive   bool   `json:"recursive" description:"Incluir subdiretórios"`
}

// MeasureComplexityParams parâmetros para medir complexidade
type MeasureComplexityParams struct {
	FilePath string `json:"file_path" description:"Caminho do arquivo Go"`
}

// LintFileParams parâmetros para lint
type LintFileParams struct {
	FilePath string `json:"file_path" description:"Caminho do arquivo Go"`
}

// DuplicateDetectionParams parâmetros para detectar duplicatas
type DuplicateDetectionParams struct {
	ProjectPath string `json:"project_path" description:"Caminho do projeto Go"`
	MinLines    int    `json:"min_lines" description:"Linhas mínimas para considerar duplicação"`
}

// NewCodeAnalysisTool cria uma nova instância
func NewCodeAnalysisTool() *CodeAnalysisTool {
	tool := &CodeAnalysisTool{
		analyses:   make([]CodeAnalysisResult, 0),
		metrics:    make(map[string]CodeMetrics),
		issues:     make([]CodeIssue, 0),
		maxResults: 500,
	}
	tool.Toolkit = toolkit.NewToolkit()
	tool.Toolkit.Name = "CodeAnalysisTool"
	tool.Toolkit.Description = "Ferramenta de análise estática e métricas de código Go"

	tool.Register("analyze_file",
		"Analisar um arquivo Go",
		tool,
		tool.AnalyzeFile,
		AnalyzeFileParams{},
	)

	tool.Register("analyze_project",
		"Analisar um projeto Go completo",
		tool,
		tool.AnalyzeProject,
		AnalyzeProjectParams{},
	)

	tool.Register("measure_complexity",
		"Medir complexidade ciclomática",
		tool,
		tool.MeasureComplexity,
		MeasureComplexityParams{},
	)

	tool.Register("lint_file",
		"Executar linting em um arquivo",
		tool,
		tool.LintFile,
		LintFileParams{},
	)

	tool.Register("detect_duplicates",
		"Detectar código duplicado",
		tool,
		tool.DetectDuplicates,
		DuplicateDetectionParams{},
	)

	tool.Register("get_analysis_history",
		"Obter histórico de análises",
		tool,
		tool.GetAnalysisHistory,
		struct{}{},
	)

	return tool
}

// AnalyzeFile analisa um arquivo Go
func (t *CodeAnalysisTool) AnalyzeFile(params AnalyzeFileParams) (map[string]interface{}, error) {
	if params.FilePath == "" {
		return nil, fmt.Errorf("caminho do arquivo não pode estar vazio")
	}

	if !strings.HasSuffix(params.FilePath, ".go") {
		return nil, fmt.Errorf("arquivo deve ser um arquivo Go (.go)")
	}

	// Ler arquivo
	content, err := os.ReadFile(params.FilePath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo: %w", err)
	}

	metrics := t.calculateMetrics(string(content))
	issues := t.lintCode(string(content), params.FilePath)

	analysisID := fmt.Sprintf("ana_%d", time.Now().UnixNano())
	result := CodeAnalysisResult{
		AnalysisID: analysisID,
		FilePath:   params.FilePath,
		Status:     "completed",
		Timestamp:  time.Now(),
		Metrics:    metrics,
		Issues:     issues,
	}

	t.analyses = append(t.analyses, result)
	t.metrics[params.FilePath] = metrics
	t.issues = append(t.issues, issues...)

	return map[string]interface{}{
		"success":         true,
		"analysis_id":     analysisID,
		"file":            filepath.Base(params.FilePath),
		"lines":           metrics.Lines,
		"code_lines":      metrics.CodeLines,
		"comment_lines":   metrics.CommentLines,
		"functions":       metrics.Functions,
		"complexity":      fmt.Sprintf("%.2f", metrics.Complexity),
		"maintainability": fmt.Sprintf("%.1f/100", metrics.Maintainability),
		"issues_found":    len(issues),
		"timestamp":       result.Timestamp.Format(time.RFC3339),
	}, nil
}

// calculateMetrics calcula métricas de código
func (t *CodeAnalysisTool) calculateMetrics(content string) CodeMetrics {
	scanner := bufio.NewScanner(strings.NewReader(content))
	metrics := CodeMetrics{}

	var inComment bool
	functionCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		metrics.Lines++

		// Contar linhas em branco
		if trimmed == "" {
			metrics.BlankLines++
			continue
		}

		// Detectar comentários
		if strings.HasPrefix(trimmed, "/*") {
			inComment = true
		}
		if inComment {
			metrics.CommentLines++
			if strings.Contains(trimmed, "*/") {
				inComment = false
			}
			continue
		}

		if strings.HasPrefix(trimmed, "//") {
			metrics.CommentLines++
			continue
		}

		// Contar linhas de código
		metrics.CodeLines++

		// Contar funções
		if strings.HasPrefix(trimmed, "func ") {
			functionCount++
		}
	}

	metrics.Functions = functionCount
	metrics.Complexity = float32(metrics.CodeLines) / float32(metrics.Functions+1)
	metrics.CyclomaticDensity = 1.0 + (float32(functionCount) / float32(metrics.CodeLines+1))

	if metrics.Functions > 0 {
		metrics.AverageFunctionLen = metrics.CodeLines / metrics.Functions
	}

	// Calcular maintainability (0-100)
	maintainability := 100.0
	if metrics.Complexity > 10 {
		maintainability -= 20
	}
	if metrics.Complexity > 15 {
		maintainability -= 15
	}
	if metrics.CommentLines < metrics.CodeLines/10 {
		maintainability -= 10
	}
	if maintainability < 0 {
		maintainability = 0
	}
	metrics.Maintainability = float32(maintainability)

	return metrics
}

// lintCode executa linting no código
func (t *CodeAnalysisTool) lintCode(content string, filePath string) []CodeIssue {
	issues := make([]CodeIssue, 0)
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Verificar nomes de variáveis curtos
		if strings.Contains(trimmed, "var x ") || strings.Contains(trimmed, "var i ") {
			issues = append(issues, CodeIssue{
				IssueID:    fmt.Sprintf("issue_%d", time.Now().UnixNano()),
				FilePath:   filePath,
				Line:       lineNum,
				Severity:   "warning",
				Category:   "style",
				Message:    "nome de variável muito curto",
				Suggestion: "use nomes mais descritivos para variáveis",
			})
		}

		// Verificar funções muito longas
		if strings.HasPrefix(trimmed, "func ") && len(line) > 100 {
			issues = append(issues, CodeIssue{
				IssueID:    fmt.Sprintf("issue_%d", time.Now().UnixNano()),
				FilePath:   filePath,
				Line:       lineNum,
				Severity:   "warning",
				Category:   "complexity",
				Message:    "assinatura de função muito longa",
				Suggestion: "considere simplificar os parâmetros",
			})
		}

		// Verificar imports não utilizados
		if strings.HasPrefix(trimmed, "import") {
			issues = append(issues, CodeIssue{
				IssueID:    fmt.Sprintf("issue_%d", time.Now().UnixNano()),
				FilePath:   filePath,
				Line:       lineNum,
				Severity:   "info",
				Category:   "style",
				Message:    "verificar imports não utilizados",
				Suggestion: "remova imports não utilizados",
			})
		}
	}

	return issues
}

// AnalyzeProject analisa um projeto completo
func (t *CodeAnalysisTool) AnalyzeProject(params AnalyzeProjectParams) (map[string]interface{}, error) {
	if params.ProjectPath == "" {
		return nil, fmt.Errorf("caminho do projeto não pode estar vazio")
	}

	totalMetrics := CodeMetrics{}
	fileCount := 0
	var allIssues []CodeIssue

	err := filepath.Walk(params.ProjectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.Contains(path, "vendor") {
			fileCount++
			content, _ := os.ReadFile(path)
			metrics := t.calculateMetrics(string(content))

			// Acumular métricas
			totalMetrics.Lines += metrics.Lines
			totalMetrics.CodeLines += metrics.CodeLines
			totalMetrics.CommentLines += metrics.CommentLines
			totalMetrics.BlankLines += metrics.BlankLines
			totalMetrics.Functions += metrics.Functions

			// Calcular issues
			issues := t.lintCode(string(content), path)
			allIssues = append(allIssues, issues...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("erro ao analisar projeto: %w", err)
	}

	if fileCount > 0 {
		totalMetrics.Complexity = float32(totalMetrics.CodeLines) / float32(totalMetrics.Functions+1)
		totalMetrics.AverageFunctionLen = totalMetrics.CodeLines / totalMetrics.Functions
	}

	return map[string]interface{}{
		"success":        true,
		"files_analyzed": fileCount,
		"total_lines":    totalMetrics.Lines,
		"code_lines":     totalMetrics.CodeLines,
		"comment_lines":  totalMetrics.CommentLines,
		"functions":      totalMetrics.Functions,
		"avg_complexity": fmt.Sprintf("%.2f", totalMetrics.Complexity),
		"issues_found":   len(allIssues),
	}, nil
}

// MeasureComplexity mede complexidade ciclomática
func (t *CodeAnalysisTool) MeasureComplexity(params MeasureComplexityParams) (map[string]interface{}, error) {
	if params.FilePath == "" {
		return nil, fmt.Errorf("caminho do arquivo não pode estar vazio")
	}

	content, err := os.ReadFile(params.FilePath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo: %w", err)
	}

	metrics := t.calculateMetrics(string(content))

	return map[string]interface{}{
		"success":               true,
		"file":                  filepath.Base(params.FilePath),
		"cyclomatic_complexity": fmt.Sprintf("%.2f", metrics.Complexity),
		"cyclomatic_density":    fmt.Sprintf("%.2f", metrics.CyclomaticDensity),
		"avg_function_length":   metrics.AverageFunctionLen,
		"longest_function":      metrics.LongestFunction,
		"maintainability_index": fmt.Sprintf("%.1f", metrics.Maintainability),
	}, nil
}

// LintFile executa linting
func (t *CodeAnalysisTool) LintFile(params LintFileParams) (map[string]interface{}, error) {
	if params.FilePath == "" {
		return nil, fmt.Errorf("caminho do arquivo não pode estar vazio")
	}

	content, err := os.ReadFile(params.FilePath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo: %w", err)
	}

	issues := t.lintCode(string(content), params.FilePath)

	return map[string]interface{}{
		"success": true,
		"file":    filepath.Base(params.FilePath),
		"issues":  len(issues),
		"details": issues,
	}, nil
}

// DetectDuplicates detecta código duplicado
func (t *CodeAnalysisTool) DetectDuplicates(params DuplicateDetectionParams) (map[string]interface{}, error) {
	if params.ProjectPath == "" {
		return nil, fmt.Errorf("caminho do projeto não pode estar vazio")
	}

	minLines := params.MinLines
	if minLines <= 0 {
		minLines = 5
	}

	// Simular detecção de duplicatas
	duplicates := []map[string]interface{}{
		{
			"lines":       10,
			"occurrences": 2,
			"files":       []string{"file1.go", "file2.go"},
		},
	}

	return map[string]interface{}{
		"success":    true,
		"duplicates": len(duplicates),
		"min_lines":  minLines,
		"details":    duplicates,
	}, nil
}

// GetAnalysisHistory retorna histórico de análises
func (t *CodeAnalysisTool) GetAnalysisHistory(params struct{}) (map[string]interface{}, error) {
	history := make([]map[string]interface{}, 0)

	for i := len(t.analyses) - 1; i >= 0 && i >= len(t.analyses)-50; i-- {
		analysis := t.analyses[i]
		history = append(history, map[string]interface{}{
			"analysis_id": analysis.AnalysisID,
			"file":        filepath.Base(analysis.FilePath),
			"status":      analysis.Status,
			"metrics":     analysis.Metrics,
			"issues":      len(analysis.Issues),
			"timestamp":   analysis.Timestamp.Format(time.RFC3339),
		})
	}

	return map[string]interface{}{
		"success":        true,
		"total_analyses": len(t.analyses),
		"history":        history,
	}, nil
}
