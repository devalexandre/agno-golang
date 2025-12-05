package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// checkRedisCliQueueAvailable verifica se redis-cli está instalado para fila
func checkRedisCliQueueAvailable() error {
	cmd := exec.Command("which", "redis-cli")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("⚠️  AVISO: redis-cli não está instalado. Instale com: sudo apt-get install redis-tools (Ubuntu/Debian) ou brew install redis (macOS)")
	}
	return nil
}

// MessageQueueManagerTool executa operações Redis (FIFO/PubSub) REAIS
type MessageQueueManagerTool struct {
	toolkit.Toolkit
}

// QueueResult resultado de operação de fila
type QueueResult struct {
	Success    bool      `json:"success"`
	Output     string    `json:"output"`
	Error      string    `json:"error,omitempty"`
	Command    string    `json:"command"`
	ExitCode   int       `json:"exit_code"`
	Timestamp  time.Time `json:"timestamp"`
	ExecutedAt string    `json:"executed_at"`
}

// PushParams parâmetros para enviar mensagem
type QueuePushParams struct {
	Queue   string `json:"queue" description:"Nome da fila"`
	Message string `json:"message" description:"Mensagem a enviar"`
	Host    string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port    int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// PopParams parâmetros para receber mensagem
type PopParams struct {
	Queue   string `json:"queue" description:"Nome da fila"`
	Timeout int    `json:"timeout" description:"Timeout em segundos"`
	Host    string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port    int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// PublishParams parâmetros para publicar em canal
type PublishParams struct {
	Channel string `json:"channel" description:"Nome do canal"`
	Message string `json:"message" description:"Mensagem a publicar"`
	Host    string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port    int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// GetQueueLenParams parâmetros para tamanho da fila
type GetQueueLenParams struct {
	Queue string `json:"queue" description:"Nome da fila"`
	Host  string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port  int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// PingParams parâmetros para ping
type PingParams struct {
	Host string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// NewMessageQueueManagerTool cria novo message queue tool
func NewMessageQueueManagerTool() *MessageQueueManagerTool {
	// Verificar se redis-cli está disponível
	if err := checkRedisCliQueueAvailable(); err != nil {
		fmt.Printf("%v\n", err)
	}

	t := &MessageQueueManagerTool{
		Toolkit: toolkit.NewToolkit(),
	}

	t.Toolkit.Register(
		"Push",
		"Enviar mensagem para fila FIFO (RPUSH)",
		t,
		t.Push,
		QueuePushParams{},
	)

	t.Toolkit.Register(
		"Pop",
		"Receber mensagem da fila FIFO (LPOP)",
		t,
		t.Pop,
		PopParams{},
	)

	t.Toolkit.Register(
		"Publish",
		"Publicar mensagem em canal PubSub",
		t,
		t.Publish,
		PublishParams{},
	)

	t.Toolkit.Register(
		"GetQueueLength",
		"Obter tamanho da fila",
		t,
		t.GetQueueLength,
		GetQueueLenParams{},
	)

	t.Toolkit.Register(
		"Ping",
		"Testar conexão com Redis",
		t,
		t.Ping,
		PingParams{},
	)

	return t
}

// buildRedisAddr constrói endereço Redis
func buildRedisQueueAddr(host string, port int) string {
	if host == "" {
		host = "localhost"
	}
	if port <= 0 {
		port = 6379
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// Push envia mensagem REAL para fila
func (t *MessageQueueManagerTool) Push(params QueuePushParams) (interface{}, error) {
	if params.Queue == "" || params.Message == "" {
		return QueueResult{Success: false, Error: "queue e message são obrigatórios"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := buildRedisQueueAddr(params.Host, params.Port)
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return QueueResult{Success: false, Error: fmt.Sprintf("Endereço Redis inválido: %s", addr)}, nil
	}

	args := []string{"-h", parts[0], "-p", parts[1], "RPUSH", params.Queue, params.Message}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	errMsg := stderr.String()
	if err != nil || strings.Contains(errMsg, "Could not connect") || strings.Contains(errMsg, "Connection refused") {
		return QueueResult{
			Success:    false,
			Error:      fmt.Sprintf("❌ Falha ao conectar Redis em %s - Verifique se Redis está rodando e acessível", addr),
			Command:    fmt.Sprintf("redis-cli -h %s -p %s RPUSH %s", parts[0], parts[1], params.Queue),
			ExitCode:   cmd.ProcessState.ExitCode(),
			Timestamp:  time.Now(),
			ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	result := QueueResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      errMsg,
		Command:    fmt.Sprintf("redis-cli RPUSH %s (host: %s:%s)", params.Queue, parts[0], parts[1]),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// Pop recebe mensagem REAL da fila
func (t *MessageQueueManagerTool) Pop(params PopParams) (interface{}, error) {
	if params.Queue == "" {
		return QueueResult{Success: false, Error: "queue é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	addr := buildRedisQueueAddr(params.Host, params.Port)
	parts := strings.Split(addr, ":")

	timeout := params.Timeout
	if timeout <= 0 {
		timeout = 1
	}

	args := []string{"-h", parts[0], "-p", parts[1], "BLPOP", params.Queue, fmt.Sprintf("%d", timeout)}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := QueueResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("redis-cli BLPOP %s", params.Queue),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// Publish publica mensagem REAL
func (t *MessageQueueManagerTool) Publish(params PublishParams) (interface{}, error) {
	if params.Channel == "" || params.Message == "" {
		return QueueResult{Success: false, Error: "channel e message são obrigatórios"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := buildRedisQueueAddr(params.Host, params.Port)
	parts := strings.Split(addr, ":")
	args := []string{"-h", parts[0], "-p", parts[1], "PUBLISH", params.Channel, params.Message}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := QueueResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("redis-cli PUBLISH %s", params.Channel),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// GetQueueLength obtém tamanho REAL da fila
func (t *MessageQueueManagerTool) GetQueueLength(params GetQueueLenParams) (interface{}, error) {
	if params.Queue == "" {
		return QueueResult{Success: false, Error: "queue é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := buildRedisQueueAddr(params.Host, params.Port)
	parts := strings.Split(addr, ":")
	args := []string{"-h", parts[0], "-p", parts[1], "LLEN", params.Queue}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := QueueResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("redis-cli LLEN %s", params.Queue),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// Ping testa conexão REAL
func (t *MessageQueueManagerTool) Ping(params PingParams) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := buildRedisQueueAddr(params.Host, params.Port)
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return QueueResult{Success: false, Error: fmt.Sprintf("Endereço Redis inválido: %s", addr)}, nil
	}

	args := []string{"-h", parts[0], "-p", parts[1], "PING"}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	errMsg := stderr.String()
	success := err == nil && strings.Contains(stdout.String(), "PONG")

	if !success {
		return QueueResult{
			Success:    false,
			Error:      fmt.Sprintf("❌ Redis não respondeu em %s - Verifique se Redis está rodando e acessível", addr),
			Output:     stdout.String(),
			Command:    fmt.Sprintf("redis-cli -h %s -p %s PING", parts[0], parts[1]),
			ExitCode:   cmd.ProcessState.ExitCode(),
			Timestamp:  time.Now(),
			ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	result := QueueResult{
		Success:    true,
		Output:     stdout.String(),
		Error:      errMsg,
		Command:    fmt.Sprintf("redis-cli PING (host: %s:%s)", parts[0], parts[1]),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}
