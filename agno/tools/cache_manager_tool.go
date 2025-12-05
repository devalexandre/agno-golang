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

// checkRedisCliAvailable verifica se redis-cli está instalado
func checkRedisCliAvailable() error {
	cmd := exec.Command("which", "redis-cli")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("⚠️  AVISO: redis-cli não está instalado. Instale com: sudo apt-get install redis-tools (Ubuntu/Debian) ou brew install redis (macOS)")
	}
	return nil
}

// CacheManagerTool executa operações Redis REAIS
type CacheManagerTool struct {
	toolkit.Toolkit
}

// RedisResult resultado de operação Redis
type RedisResult struct {
	Success    bool      `json:"success"`
	Output     string    `json:"output"`
	Error      string    `json:"error,omitempty"`
	Command    string    `json:"command"`
	ExitCode   int       `json:"exit_code"`
	Timestamp  time.Time `json:"timestamp"`
	ExecutedAt string    `json:"executed_at"`
}

// SetParams parâmetros para SET
type SetParams struct {
	Key     string `json:"key" description:"Chave do cache"`
	Value   string `json:"value" description:"Valor a armazenar"`
	Expires int    `json:"expires" description:"TTL em segundos (0 = infinito)"`
	Host    string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port    int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// GetParams parâmetros para GET
type GetParams struct {
	Key  string `json:"key" description:"Chave do cache"`
	Host string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// DeleteParams parâmetros para DELETE
type DeleteCacheParams struct {
	Key  string `json:"key" description:"Chave do cache"`
	Host string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// GetAllParams parâmetros para GET ALL
type GetAllParams struct {
	Pattern string `json:"pattern" description:"Padrão (ex: mykey:*)"`
	Host    string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port    int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// InfoParams parâmetros para INFO
type InfoParams struct {
	Section string `json:"section" description:"Seção (stats, server, memory, etc)"`
	Host    string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port    int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// FlushParams parâmetros para FLUSH
type FlushParams struct {
	Host string `json:"host" description:"Host Redis (padrão: localhost)"`
	Port int    `json:"port" description:"Porta Redis (padrão: 6379)"`
}

// NewCacheManagerTool cria novo cache tool com operações REAIS
func NewCacheManagerTool() *CacheManagerTool {
	// Verificar se redis-cli está disponível
	if err := checkRedisCliAvailable(); err != nil {
		fmt.Printf("%v\n", err)
	}

	t := &CacheManagerTool{
		Toolkit: toolkit.NewToolkit(),
	}

	t.Toolkit.Register(
		"Set",
		"Armazenar valor no cache Redis",
		t,
		t.Set,
		SetParams{},
	)

	t.Toolkit.Register(
		"Get",
		"Obter valor do cache Redis",
		t,
		t.Get,
		GetParams{},
	)

	t.Toolkit.Register(
		"Delete",
		"Deletar valor do cache",
		t,
		t.Delete,
		DeleteCacheParams{},
	)

	t.Toolkit.Register(
		"GetAll",
		"Obter todas as chaves que correspondem ao padrão",
		t,
		t.GetAll,
		GetAllParams{},
	)

	t.Toolkit.Register(
		"Info",
		"Obter informações do Redis",
		t,
		t.Info,
		InfoParams{},
	)

	t.Toolkit.Register(
		"Flush",
		"Limpar todo o cache",
		t,
		t.Flush,
		FlushParams{},
	)

	return t
}

// buildRedisAddr constrói endereço Redis
func buildRedisAddr(host string, port int) string {
	if host == "" {
		host = "localhost"
	}
	if port <= 0 {
		port = 6379
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// Set armazena valor REAL no Redis
func (t *CacheManagerTool) Set(params SetParams) (interface{}, error) {
	if params.Key == "" || params.Value == "" {
		return RedisResult{Success: false, Error: "key e value são obrigatórios"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := buildRedisAddr(params.Host, params.Port)
	hostPort := strings.Split(addr, ":")
	if len(hostPort) != 2 {
		return RedisResult{Success: false, Error: fmt.Sprintf("Endereço Redis inválido: %s", addr)}, nil
	}

	args := []string{"-h", hostPort[0], "-p", hostPort[1], "SET", params.Key, params.Value}

	if params.Expires > 0 {
		args = append(args, "EX", fmt.Sprintf("%d", params.Expires))
	}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Verificar se é erro de conexão
	errMsg := stderr.String()
	if err != nil || strings.Contains(errMsg, "Could not connect") || strings.Contains(errMsg, "Connection refused") {
		return RedisResult{
			Success:    false,
			Error:      fmt.Sprintf("❌ Falha ao conectar Redis em %s - Verifique se Redis está rodando e acessível", addr),
			Command:    fmt.Sprintf("redis-cli -h %s -p %s SET %s", hostPort[0], hostPort[1], params.Key),
			ExitCode:   cmd.ProcessState.ExitCode(),
			Timestamp:  time.Now(),
			ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	result := RedisResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      errMsg,
		Command:    fmt.Sprintf("redis-cli SET %s (host: %s:%s)", params.Key, hostPort[0], hostPort[1]),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// Get obtém valor REAL do Redis
func (t *CacheManagerTool) Get(params GetParams) (interface{}, error) {
	if params.Key == "" {
		return RedisResult{Success: false, Error: "key é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := buildRedisAddr(params.Host, params.Port)
	hostPort := strings.Split(addr, ":")
	if len(hostPort) != 2 {
		return RedisResult{Success: false, Error: fmt.Sprintf("Endereço Redis inválido: %s", addr)}, nil
	}

	args := []string{"-h", hostPort[0], "-p", hostPort[1], "GET", params.Key}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	errMsg := stderr.String()
	if err != nil || strings.Contains(errMsg, "Could not connect") || strings.Contains(errMsg, "Connection refused") {
		return RedisResult{
			Success:    false,
			Error:      fmt.Sprintf("❌ Falha ao conectar Redis em %s - Verifique se Redis está rodando e acessível", addr),
			Command:    fmt.Sprintf("redis-cli -h %s -p %s GET %s", hostPort[0], hostPort[1], params.Key),
			ExitCode:   cmd.ProcessState.ExitCode(),
			Timestamp:  time.Now(),
			ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	result := RedisResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      errMsg,
		Command:    fmt.Sprintf("redis-cli GET %s (host: %s:%s)", params.Key, hostPort[0], hostPort[1]),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// Delete deleta valor REAL do Redis
func (t *CacheManagerTool) Delete(params DeleteCacheParams) (interface{}, error) {
	if params.Key == "" {
		return RedisResult{Success: false, Error: "key é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := buildRedisAddr(params.Host, params.Port)
	args := []string{"-h", strings.Split(addr, ":")[0], "-p", strings.Split(addr, ":")[1], "DEL", params.Key}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := RedisResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("redis-cli DEL %s", params.Key),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// GetAll obtém todas chaves REAIS
func (t *CacheManagerTool) GetAll(params GetAllParams) (interface{}, error) {
	pattern := params.Pattern
	if pattern == "" {
		pattern = "*"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := buildRedisAddr(params.Host, params.Port)
	args := []string{"-h", strings.Split(addr, ":")[0], "-p", strings.Split(addr, ":")[1], "KEYS", pattern}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := RedisResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("redis-cli KEYS %s", pattern),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// Info obtém informações REAIS do Redis
func (t *CacheManagerTool) Info(params InfoParams) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := buildRedisAddr(params.Host, params.Port)
	args := []string{"-h", strings.Split(addr, ":")[0], "-p", strings.Split(addr, ":")[1], "INFO"}

	if params.Section != "" {
		args = append(args, params.Section)
	}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := RedisResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    "redis-cli INFO",
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// Flush limpa cache REAL
func (t *CacheManagerTool) Flush(params FlushParams) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := buildRedisAddr(params.Host, params.Port)
	args := []string{"-h", strings.Split(addr, ":")[0], "-p", strings.Split(addr, ":")[1], "FLUSHDB"}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := RedisResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    "redis-cli FLUSHDB",
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}
