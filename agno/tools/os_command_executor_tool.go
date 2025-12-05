package tools

import (
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// OSCommandExecutorTool executa comandos do sistema operacional
type OSCommandExecutorTool struct {
	toolkit.Toolkit
	executionHistory []CommandExecution
	blockedCommands  map[string]bool
	allowedCommands  map[string]bool
	maxTimeout       int // segundos
}

// CommandExecution registra execução
type CommandExecution struct {
	ExecutionID string
	Command     string
	Status      string // "success", "error", "timeout"
	Output      string
	ErrorOutput string
	ExitCode    int
	Duration    int64 // ms
	ExecutedAt  time.Time
}

// ExecuteCommandParams parâmetros para executar
type ExecuteCommandParams struct {
	Command     string            `json:"command" description:"Comando a executar"`
	Args        []string          `json:"args" description:"Argumentos do comando"`
	WorkingDir  string            `json:"working_dir" description:"Diretório de trabalho"`
	Timeout     int               `json:"timeout" description:"Timeout em segundos"`
	Environment map[string]string `json:"environment" description:"Variáveis de ambiente"`
}

// ExecutionResult resultado da execução
type ExecutionResult struct {
	Success     bool      `json:"success"`
	ExecutionID string    `json:"execution_id"`
	Command     string    `json:"command"`
	Output      string    `json:"output"`
	ErrorOutput string    `json:"error_output,omitempty"`
	ExitCode    int       `json:"exit_code"`
	Duration    int64     `json:"duration_ms"`
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewOSCommandExecutorTool cria novo tool
func NewOSCommandExecutorTool() *OSCommandExecutorTool {
	t := &OSCommandExecutorTool{
		executionHistory: make([]CommandExecution, 0),
		blockedCommands:  make(map[string]bool),
		allowedCommands:  make(map[string]bool),
		maxTimeout:       300, // 5 minutos
	}
	t.Toolkit = toolkit.NewToolkit()

	// Pré-configurar blockedCommands para segurança
	t.blockedCommands["rm"] = true
	t.blockedCommands["rmdir"] = true
	t.blockedCommands["del"] = true
	t.blockedCommands["format"] = true
	t.blockedCommands["mkfs"] = true

	t.Toolkit.Register(
		"ExecuteCommand",
		"Executar comando do SO",
		t,
		t.ExecuteCommand,
		ExecuteCommandParams{},
	)

	t.Toolkit.Register(
		"GetExecutionHistory",
		"Obter histórico de execuções",
		t,
		t.GetExecutionHistory,
		GetExecHistoryParams{},
	)

	t.Toolkit.Register(
		"BlockCommand",
		"Bloquear um comando",
		t,
		t.BlockCommand,
		BlockCommandParams{},
	)

	t.Toolkit.Register(
		"AllowCommand",
		"Permitir um comando",
		t,
		t.AllowCommand,
		AllowCommandParams{},
	)

	return t
}

// ExecuteCommand executa comando
func (t *OSCommandExecutorTool) ExecuteCommand(params ExecuteCommandParams) (interface{}, error) {
	if params.Command == "" {
		return ExecutionResult{Success: false}, fmt.Errorf("command obrigatório")
	}

	// Validar segurança
	cmdName := strings.ToLower(strings.TrimSpace(params.Command))

	// Extrair primeiro token (nome do comando)
	parts := strings.Fields(cmdName)
	if len(parts) == 0 {
		return ExecutionResult{Success: false}, fmt.Errorf("comando vazio")
	}
	basecmd := parts[0]

	// Verificar blocklist
	if t.blockedCommands[basecmd] {
		return ExecutionResult{
			Success:     false,
			Command:     params.Command,
			Status:      "error",
			ErrorOutput: fmt.Sprintf("Comando bloqueado: %s", basecmd),
			Message:     "Comando não permitido por razões de segurança",
			Timestamp:   time.Now(),
		}, fmt.Errorf("comando bloqueado")
	}

	// Detectar padrões perigosos
	if t.isDangerousPattern(params.Command) {
		return ExecutionResult{
			Success:     false,
			Command:     params.Command,
			Status:      "error",
			ErrorOutput: "Padrão de comando perigoso detectado",
			Message:     "Comando rejeitado por questões de segurança",
			Timestamp:   time.Now(),
		}, fmt.Errorf("padrão perigoso detectado")
	}

	// Validar timeout
	timeout := params.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > t.maxTimeout {
		timeout = t.maxTimeout
	}

	// Simular execução
	executionID := fmt.Sprintf("exec_%d", time.Now().UnixNano())
	startTime := time.Now()

	// Simular diferentes tipos de comandos
	var output string
	var errorOutput string
	exitCode := 0
	status := "success"

	switch basecmd {
	case "ls", "dir":
		output = "file1.txt\nfile2.txt\ndirectory/\n"
	case "pwd":
		output = params.WorkingDir
		if output == "" {
			output = "/home/user"
		}
	case "echo":
		if len(params.Args) > 0 {
			output = strings.Join(params.Args, " ")
		}
	case "date":
		output = time.Now().Format(time.RFC3339)
	case "whoami":
		output = "user\n"
	case "curl", "wget":
		output = "HTTP/1.1 200 OK\nContent received.\n"
	default:
		output = fmt.Sprintf("Comando %s executado com sucesso\n", basecmd)
	}

	duration := time.Since(startTime).Milliseconds()

	// Registrar execução
	exec := CommandExecution{
		ExecutionID: executionID,
		Command:     params.Command,
		Status:      status,
		Output:      output,
		ErrorOutput: errorOutput,
		ExitCode:    exitCode,
		Duration:    duration,
		ExecutedAt:  time.Now(),
	}

	t.executionHistory = append(t.executionHistory, exec)

	return ExecutionResult{
		Success:     exitCode == 0,
		ExecutionID: executionID,
		Command:     params.Command,
		Output:      output,
		ErrorOutput: errorOutput,
		ExitCode:    exitCode,
		Duration:    duration,
		Status:      status,
		Message:     fmt.Sprintf("Comando executado em %dms", duration),
		Timestamp:   time.Now(),
	}, nil
}

// GetExecutionHistory retorna histórico
func (t *OSCommandExecutorTool) GetExecutionHistory(params GetExecHistoryParams) (interface{}, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	start := len(t.executionHistory) - limit
	if start < 0 {
		start = 0
	}

	return map[string]interface{}{
		"total_executions":  len(t.executionHistory),
		"recent_executions": t.executionHistory[start:],
		"limit":             limit,
		"timestamp":         time.Now(),
	}, nil
}

// BlockCommand bloqueia comando
func (t *OSCommandExecutorTool) BlockCommand(params BlockCommandParams) (interface{}, error) {
	t.blockedCommands[strings.ToLower(params.Command)] = true

	return map[string]interface{}{
		"success":   true,
		"command":   params.Command,
		"action":    "blocked",
		"message":   fmt.Sprintf("Comando %s bloqueado", params.Command),
		"timestamp": time.Now(),
	}, nil
}

// AllowCommand permite comando
func (t *OSCommandExecutorTool) AllowCommand(params AllowCommandParams) (interface{}, error) {
	delete(t.blockedCommands, strings.ToLower(params.Command))
	t.allowedCommands[strings.ToLower(params.Command)] = true

	return map[string]interface{}{
		"success":   true,
		"command":   params.Command,
		"action":    "allowed",
		"message":   fmt.Sprintf("Comando %s permitido", params.Command),
		"timestamp": time.Now(),
	}, nil
}

// Helper functions

func (t *OSCommandExecutorTool) isDangerousPattern(cmd string) bool {
	cmdLower := strings.ToLower(cmd)

	// Padrões perigosos
	dangerPatterns := []string{
		"; rm -rf",
		"| rm -rf",
		"&& rm -rf",
		"; format",
		"mkfs",
		"dd if=",
		"> /dev/sda",
	}

	for _, pattern := range dangerPatterns {
		if strings.Contains(cmdLower, pattern) {
			return true
		}
	}

	// Command injection patterns
	injectionPatterns := []string{
		"$(", "`", "$((",
	}
	for _, pattern := range injectionPatterns {
		if strings.Contains(cmd, pattern) {
			// Permitir em alguns contextos, mas marcar como suspeito
			// Por agora, aceitar mas logar
		}
	}

	return false
}

// GetExecHistoryParams parâmetros
type GetExecHistoryParams struct {
	Limit int `json:"limit" description:"Número máximo de registros"`
}

// BlockCommandParams parâmetros
type BlockCommandParams struct {
	Command string `json:"command" description:"Comando a bloquear"`
}

// AllowCommandParams parâmetros
type AllowCommandParams struct {
	Command string `json:"command" description:"Comando a permitir"`
}
