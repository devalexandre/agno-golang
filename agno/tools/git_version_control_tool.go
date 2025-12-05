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

// checkGitAvailable verifica se git está instalado
func checkGitAvailable() error {
	cmd := exec.Command("which", "git")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("⚠️  AVISO: git não está instalado. Instale com: sudo apt-get install git (Ubuntu/Debian) ou brew install git (macOS)")
	}
	return nil
}

// GitTool executa operações Git REAIS
type GitTool struct {
	toolkit.Toolkit
}

// InitRepoParams parâmetros para inicializar repositório
type InitRepoParams struct {
	Path      string `json:"path" description:"Caminho do repositório a ser criado"`
	RemoteURL string `json:"remote_url" description:"URL remota (opcional) ex: https://github.com/user/repo.git"`
}

// CommitParams parâmetros para criar commit
type CommitParams struct {
	Path    string   `json:"path" description:"Caminho do repositório"`
	Files   []string `json:"files" description:"Arquivos para adicionar (usar '.' para todos)"`
	Message string   `json:"message" description:"Mensagem do commit"`
	Author  string   `json:"author" description:"Nome do autor (opcional)"`
	Email   string   `json:"email" description:"Email do autor (opcional)"`
}

// BranchParams parâmetros para operações de branch
type BranchParams struct {
	Path       string `json:"path" description:"Caminho do repositório"`
	BranchName string `json:"branch_name" description:"Nome da nova branch"`
	BaseBranch string `json:"base_branch" description:"Branch base (padrão: main/master)"`
	Checkout   bool   `json:"checkout" description:"Fazer checkout da branch após criar"`
}

// LogParams parâmetros para obter histórico
type LogParams struct {
	Path   string `json:"path" description:"Caminho do repositório"`
	Limit  int    `json:"limit" description:"Número de commits (padrão: 10)"`
	Format string `json:"format" description:"Formato do log (oneline, full, etc)"`
}

// PullParams parâmetros para pull
type PullParams struct {
	Path   string `json:"path" description:"Caminho do repositório"`
	Remote string `json:"remote" description:"Nome do remote (padrão: origin)"`
	Branch string `json:"branch" description:"Branch a puxar (padrão: HEAD)"`
}

// PushParams parâmetros para push
type PushParams struct {
	Path   string `json:"path" description:"Caminho do repositório"`
	Remote string `json:"remote" description:"Nome do remote (padrão: origin)"`
	Branch string `json:"branch" description:"Branch a enviar (padrão: HEAD)"`
}

// StatusParams parâmetros para status
type StatusParams struct {
	Path   string `json:"path" description:"Caminho do repositório"`
	Format string `json:"format" description:"Formato (porcelain para parsing)"`
}

// GitResult resultado de operação
type GitResult struct {
	Success    bool      `json:"success"`
	Output     string    `json:"output"`
	Error      string    `json:"error,omitempty"`
	Command    string    `json:"command"`
	ExitCode   int       `json:"exit_code"`
	Timestamp  time.Time `json:"timestamp"`
	ExecutedAt string    `json:"executed_at"`
}

// NewGitTool cria novo Git tool com operações REAIS
func NewGitTool() *GitTool {
	// Verificar se git está disponível
	if err := checkGitAvailable(); err != nil {
		fmt.Printf("%v\n", err)
	}

	t := &GitTool{
		Toolkit: toolkit.NewToolkit(),
	}

	// Registrar operações Git reais
	t.Toolkit.Register(
		"InitRepository",
		"Inicializar novo repositório Git",
		t,
		t.InitRepository,
		InitRepoParams{},
	)

	t.Toolkit.Register(
		"GetStatus",
		"Obter status atual do repositório",
		t,
		t.GetStatus,
		StatusParams{},
	)

	t.Toolkit.Register(
		"GetLog",
		"Obter histórico de commits",
		t,
		t.GetLog,
		LogParams{},
	)

	t.Toolkit.Register(
		"CreateCommit",
		"Criar novo commit com mudanças",
		t,
		t.CreateCommit,
		CommitParams{},
	)

	t.Toolkit.Register(
		"CreateBranch",
		"Criar nova branch",
		t,
		t.CreateBranch,
		BranchParams{},
	)

	t.Toolkit.Register(
		"PullChanges",
		"Puxar mudanças do repositório remoto",
		t,
		t.PullChanges,
		PullParams{},
	)

	t.Toolkit.Register(
		"PushChanges",
		"Enviar commits para repositório remoto",
		t,
		t.PushChanges,
		PushParams{},
	)

	return t
}

// InitRepository inicializa repositório Git REAL
func (t *GitTool) InitRepository(params InitRepoParams) (interface{}, error) {
	if params.Path == "" {
		return GitResult{Success: false, Error: "path é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// git init
	cmd := exec.CommandContext(ctx, "git", "init", params.Path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	errMsg := stderr.String()
	if err != nil || strings.Contains(errMsg, "fatal") {
		return GitResult{
			Success:    false,
			Error:      fmt.Sprintf("❌ Falha ao inicializar repositório em %s - %s", params.Path, errMsg),
			Command:    "git init " + params.Path,
			ExitCode:   cmd.ProcessState.ExitCode(),
			Timestamp:  time.Now(),
			ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	result := GitResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      errMsg,
		Command:    "git init " + params.Path,
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Se remoteURL fornecida, adicionar
	if params.RemoteURL != "" {
		cmdRemote := exec.CommandContext(ctx, "git", "-C", params.Path, "remote", "add", "origin", params.RemoteURL)
		cmdRemote.Stdout = &stdout
		cmdRemote.Stderr = &stderr
		cmdRemote.Run() // Ignorar erro se remote já existe
	}

	return result, nil
}

// GetStatus retorna status do repositório
func (t *GitTool) GetStatus(params StatusParams) (interface{}, error) {
	if params.Path == "" {
		return GitResult{Success: false, Error: "path é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"-C", params.Path, "status"}
	if params.Format == "porcelain" {
		args = append(args, "--porcelain")
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	errMsg := stderr.String()
	if err != nil || strings.Contains(errMsg, "fatal: not a git repository") {
		return GitResult{
			Success:    false,
			Error:      fmt.Sprintf("❌ %s não é um repositório Git válido", params.Path),
			Command:    "git status",
			ExitCode:   cmd.ProcessState.ExitCode(),
			Timestamp:  time.Now(),
			ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	result := GitResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      errMsg,
		Command:    "git status",
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// GetLog retorna histórico de commits
func (t *GitTool) GetLog(params LogParams) (interface{}, error) {
	if params.Path == "" {
		return GitResult{Success: false, Error: "path é obrigatório"}, nil
	}

	if params.Limit <= 0 {
		params.Limit = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	format := params.Format
	if format == "" {
		format = "oneline"
	}

	limitStr := fmt.Sprintf("-%d", params.Limit)
	cmd := exec.CommandContext(ctx, "git", "-C", params.Path, "log", "--pretty="+format, limitStr)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := GitResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("git log --pretty=%s -%d", format, params.Limit),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// CreateCommit cria commit REAL
func (t *GitTool) CreateCommit(params CommitParams) (interface{}, error) {
	if params.Path == "" {
		return GitResult{Success: false, Error: "path é obrigatório"}, nil
	}
	if params.Message == "" {
		return GitResult{Success: false, Error: "message é obrigatória"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Adicionar autor se fornecido
	if params.Author != "" && params.Email != "" {
		cmdConfig := exec.CommandContext(ctx, "git", "-C", params.Path, "config", "user.name", params.Author)
		cmdConfig.Run() // Ignorar erro
		cmdConfig = exec.CommandContext(ctx, "git", "-C", params.Path, "config", "user.email", params.Email)
		cmdConfig.Run() // Ignorar erro
	}

	// Adicionar arquivos
	files := params.Files
	if len(files) == 0 {
		files = []string{"."}
	}

	cmdAdd := exec.CommandContext(ctx, "git", append([]string{"-C", params.Path, "add"}, files...)...)
	var stderr bytes.Buffer
	cmdAdd.Stderr = &stderr
	errAdd := cmdAdd.Run()

	if errAdd != nil && strings.Contains(stderr.String(), "not a git repository") {
		return GitResult{
			Success:  false,
			Error:    "Não é um repositório Git",
			Command:  "git add",
			ExitCode: cmdAdd.ProcessState.ExitCode(),
		}, nil
	}

	// Criar commit
	cmdCommit := exec.CommandContext(ctx, "git", "-C", params.Path, "commit", "-m", params.Message)
	var stdout bytes.Buffer
	cmdCommit.Stdout = &stdout
	cmdCommit.Stderr = &stderr

	errCommit := cmdCommit.Run()

	result := GitResult{
		Success:    errCommit == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("git add & git commit -m \"%s\"", params.Message),
		ExitCode:   cmdCommit.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// CreateBranch cria branch REAL
func (t *GitTool) CreateBranch(params BranchParams) (interface{}, error) {
	if params.Path == "" {
		return GitResult{Success: false, Error: "path é obrigatório"}, nil
	}
	if params.BranchName == "" {
		return GitResult{Success: false, Error: "branch_name é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Criar branch
	args := []string{"-C", params.Path, "branch", params.BranchName}
	if params.BaseBranch != "" {
		args = append(args, params.BaseBranch)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := GitResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("git branch %s", params.BranchName),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Checkout se solicitado
	if err == nil && params.Checkout {
		cmdCheckout := exec.CommandContext(ctx, "git", "-C", params.Path, "checkout", params.BranchName)
		cmdCheckout.Stdout = &stdout
		cmdCheckout.Stderr = &stderr
		cmdCheckout.Run()
	}

	return result, nil
}

// PullChanges puxa mudanças REAIS
func (t *GitTool) PullChanges(params PullParams) (interface{}, error) {
	if params.Path == "" {
		return GitResult{Success: false, Error: "path é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	remote := params.Remote
	if remote == "" {
		remote = "origin"
	}

	args := []string{"-C", params.Path, "pull", remote}
	if params.Branch != "" {
		args = append(args, params.Branch)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := GitResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    "git pull " + remote,
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// PushChanges envia mudanças REAIS
func (t *GitTool) PushChanges(params PushParams) (interface{}, error) {
	if params.Path == "" {
		return GitResult{Success: false, Error: "path é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	remote := params.Remote
	if remote == "" {
		remote = "origin"
	}

	args := []string{"-C", params.Path, "push", remote}
	if params.Branch != "" {
		args = append(args, params.Branch)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := GitResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    "git push " + remote,
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}
