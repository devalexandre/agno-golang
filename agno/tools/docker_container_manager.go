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

// checkDockerAvailable verifica se docker está instalado
func checkDockerAvailable() error {
	cmd := exec.Command("which", "docker")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("⚠️  AVISO: docker não está instalado. Instale a partir de: https://docs.docker.com/get-docker/")
	}
	return nil
}

// DockerContainerManager gerencia containers Docker
type DockerContainerManager struct {
	toolkit.Toolkit
	containers []DockerContainer
	images     []DockerImage
	operations []DockerOperation
	maxResults int
}

// DockerContainer representa um container Docker
type DockerContainer struct {
	ContainerID string
	Name        string
	ImageName   string
	Status      string // "running", "stopped", "exited", "paused"
	Ports       map[string]string
	Volumes     []string
	Environment map[string]string
	CreatedAt   time.Time
	StartedAt   time.Time
	Memory      int64   // bytes
	CPUUsage    float32 // percentage
}

// DockerImage representa uma imagem Docker
type DockerImage struct {
	ImageID    string
	Repository string
	Tag        string
	Size       int64 // bytes
	CreatedAt  time.Time
	PullCount  int
	Status     string // "available", "downloading", "failed"
}

// DockerOperation registra uma operação
type DockerOperation struct {
	OperationID string
	Type        string // "pull", "run", "stop", "remove", "build"
	Status      string // "success", "failed", "running"
	ContainerID string
	Output      string
	Error       string
	Duration    int64 // ms
	Timestamp   time.Time
}

// PullImageParams parâmetros para puxar imagem
type PullImageParams struct {
	ImageName string `json:"image_name" description:"Nome da imagem (ex: nginx:latest)"`
	Registry  string `json:"registry" description:"Registry (docker.io, gcr.io, etc)"`
}

// RunContainerParams parâmetros para executar container
type RunContainerParams struct {
	ImageName     string            `json:"image_name" description:"Nome da imagem"`
	ContainerName string            `json:"container_name" description:"Nome do container"`
	Ports         map[string]string `json:"ports" description:"Mapeamento de portas"`
	Volumes       []string          `json:"volumes" description:"Volumes"`
	Environment   map[string]string `json:"environment" description:"Variáveis de ambiente"`
	Detach        bool              `json:"detach" description:"Executar em background"`
}

// StopContainerParams parâmetros para parar container
type StopContainerParams struct {
	ContainerID string `json:"container_id" description:"ID ou nome do container"`
	Timeout     int    `json:"timeout" description:"Timeout em segundos"`
}

// RemoveContainerParams parâmetros para remover container
type RemoveContainerParams struct {
	ContainerID   string `json:"container_id" description:"ID ou nome do container"`
	Force         bool   `json:"force" description:"Forçar remoção"`
	RemoveVolumes bool   `json:"remove_volumes" description:"Remover volumes associados"`
}

// GetLogsParams parâmetros para obter logs
type GetLogsParams struct {
	ContainerID string `json:"container_id" description:"ID ou nome do container"`
	Tail        int    `json:"tail" description:"Últimas N linhas"`
	Follow      bool   `json:"follow" description:"Continuar seguindo logs"`
}

// GetContainerStatsParams parâmetros para obter estatísticas de container
type GetContainerStatsParams struct {
	ContainerID string `json:"container_id" description:"ID ou nome do container"`
}

// NewDockerContainerManager cria nova instância
func NewDockerContainerManager() *DockerContainerManager {
	// Verificar se docker está disponível
	if err := checkDockerAvailable(); err != nil {
		fmt.Printf("%v\n", err)
	}

	tool := &DockerContainerManager{
		containers: make([]DockerContainer, 0),
		images:     make([]DockerImage, 0),
		operations: make([]DockerOperation, 0),
		maxResults: 1000,
	}
	tool.Toolkit = toolkit.NewToolkit()
	tool.Toolkit.Name = "DockerContainerManager"
	tool.Toolkit.Description = "Gerenciador de containers e imagens Docker"

	tool.Register("pull_image",
		"Puxar imagem do registry",
		tool,
		tool.PullImage,
		PullImageParams{},
	)

	tool.Register("run_container",
		"Executar um novo container",
		tool,
		tool.RunContainer,
		RunContainerParams{},
	)

	tool.Register("stop_container",
		"Parar um container em execução",
		tool,
		tool.StopContainer,
		StopContainerParams{},
	)

	tool.Register("remove_container",
		"Remover um container",
		tool,
		tool.RemoveContainer,
		RemoveContainerParams{},
	)

	tool.Register("get_container_logs",
		"Obter logs de um container",
		tool,
		tool.GetContainerLogs,
		GetLogsParams{},
	)

	tool.Register("list_containers",
		"Listar containers",
		tool,
		tool.ListContainers,
		struct{}{},
	)

	tool.Register("list_images",
		"Listar imagens disponíveis",
		tool,
		tool.ListImages,
		struct{}{},
	)

	tool.Register("get_container_stats",
		"Obter estatísticas de um container",
		tool,
		tool.GetContainerStats,
		GetContainerStatsParams{},
	)

	return tool
}

// PullImage puxa uma imagem do registry - EXECUTA COMANDO DOCKER REAL
func (t *DockerContainerManager) PullImage(params PullImageParams) (map[string]interface{}, error) {
	if params.ImageName == "" {
		return nil, fmt.Errorf("nome da imagem não pode estar vazio")
	}

	operationID := fmt.Sprintf("op_%d", time.Now().UnixNano())
	startTime := time.Now()

	// Executar comando docker pull real
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "pull", params.ImageName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	status := "success"
	output := stdout.String()
	errMsg := ""

	if err != nil {
		status = "failed"
		errMsg = stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
	}

	operation := DockerOperation{
		OperationID: operationID,
		Type:        "pull",
		Status:      status,
		Output:      output,
		Error:       errMsg,
		Duration:    duration,
		Timestamp:   time.Now(),
	}
	t.operations = append(t.operations, operation)

	result := map[string]interface{}{
		"success":      status == "success",
		"operation_id": operationID,
		"image_name":   params.ImageName,
		"status":       status,
		"duration_ms":  duration,
		"output":       output,
	}

	if errMsg != "" {
		result["error"] = errMsg
	}

	return result, nil
}

// RunContainer executa um novo container - EXECUTA DOCKER RUN real
func (t *DockerContainerManager) RunContainer(params RunContainerParams) (map[string]interface{}, error) {
	if params.ImageName == "" {
		return nil, fmt.Errorf("nome da imagem não pode estar vazio")
	}

	operationID := fmt.Sprintf("op_%d", time.Now().UnixNano())
	startTime := time.Now()

	args := []string{"run", "-d"}

	// Adicionar nome do container
	if params.ContainerName != "" {
		args = append(args, "--name", params.ContainerName)
	}

	// Adicionar mapeamento de portas
	for hostPort, containerPort := range params.Ports {
		args = append(args, "-p", fmt.Sprintf("%s:%s", hostPort, containerPort))
	}

	// Adicionar volumes
	for _, volume := range params.Volumes {
		args = append(args, "-v", volume)
	}

	// Adicionar variáveis de ambiente
	for key, value := range params.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Adicionar nome da imagem
	args = append(args, params.ImageName)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	status := "success"
	output := stdout.String()
	errMsg := ""
	containerID := strings.TrimSpace(output)

	if err != nil {
		status = "failed"
		errMsg = stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		containerID = ""
	}

	operation := DockerOperation{
		OperationID: operationID,
		Type:        "run",
		Status:      status,
		ContainerID: containerID,
		Output:      output,
		Error:       errMsg,
		Duration:    duration,
		Timestamp:   time.Now(),
	}
	t.operations = append(t.operations, operation)

	result := map[string]interface{}{
		"success":        status == "success",
		"operation_id":   operationID,
		"container_id":   containerID,
		"container_name": params.ContainerName,
		"image":          params.ImageName,
		"status":         status,
		"duration_ms":    duration,
	}

	if errMsg != "" {
		result["error"] = errMsg
	}

	return result, nil
}

// StopContainer para um container - EXECUTA DOCKER STOP real
func (t *DockerContainerManager) StopContainer(params StopContainerParams) (map[string]interface{}, error) {
	if params.ContainerID == "" {
		return nil, fmt.Errorf("container_id não pode estar vazio")
	}

	operationID := fmt.Sprintf("op_%d", time.Now().UnixNano())
	startTime := time.Now()

	args := []string{"stop"}

	if params.Timeout > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", params.Timeout))
	}

	args = append(args, params.ContainerID)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	status := "success"
	output := stdout.String()
	errMsg := ""

	if err != nil {
		status = "failed"
		errMsg = stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
	}

	operation := DockerOperation{
		OperationID: operationID,
		Type:        "stop",
		Status:      status,
		ContainerID: params.ContainerID,
		Output:      output,
		Error:       errMsg,
		Duration:    duration,
		Timestamp:   time.Now(),
	}
	t.operations = append(t.operations, operation)

	result := map[string]interface{}{
		"success":      status == "success",
		"operation_id": operationID,
		"container_id": params.ContainerID,
		"status":       status,
		"duration_ms":  duration,
	}

	if errMsg != "" {
		result["error"] = errMsg
	}

	return result, nil
}

// RemoveContainer remove um container - EXECUTA DOCKER RM real
func (t *DockerContainerManager) RemoveContainer(params RemoveContainerParams) (map[string]interface{}, error) {
	if params.ContainerID == "" {
		return nil, fmt.Errorf("container_id não pode estar vazio")
	}

	operationID := fmt.Sprintf("op_%d", time.Now().UnixNano())
	startTime := time.Now()

	args := []string{"rm"}

	if params.Force {
		args = append(args, "-f")
	}

	if params.RemoveVolumes {
		args = append(args, "-v")
	}

	args = append(args, params.ContainerID)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	status := "success"
	output := stdout.String()
	errMsg := ""

	if err != nil {
		status = "failed"
		errMsg = stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
	}

	operation := DockerOperation{
		OperationID: operationID,
		Type:        "remove",
		Status:      status,
		ContainerID: params.ContainerID,
		Output:      output,
		Error:       errMsg,
		Duration:    duration,
		Timestamp:   time.Now(),
	}
	t.operations = append(t.operations, operation)

	result := map[string]interface{}{
		"success":      status == "success",
		"operation_id": operationID,
		"container_id": params.ContainerID,
		"status":       status,
		"duration_ms":  duration,
	}

	if errMsg != "" {
		result["error"] = errMsg
	}

	return result, nil
}

// GetContainerLogs obtém logs de um container - EXECUTA DOCKER LOGS real
func (t *DockerContainerManager) GetContainerLogs(params GetLogsParams) (map[string]interface{}, error) {
	if params.ContainerID == "" {
		return nil, fmt.Errorf("container_id não pode estar vazio")
	}

	args := []string{"logs"}

	if params.Tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", params.Tail))
	}

	args = append(args, params.ContainerID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	status := "success"
	output := stdout.String()
	errMsg := ""

	if err != nil {
		status = "failed"
		errMsg = stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
	}

	result := map[string]interface{}{
		"success":      status == "success",
		"container_id": params.ContainerID,
		"status":       status,
		"logs":         output,
	}

	if errMsg != "" {
		result["error"] = errMsg
	}

	return result, nil
}

// ListContainers lista containers executando DOCKER PS real
func (t *DockerContainerManager) ListContainers(params struct{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	status := "success"
	output := stdout.String()
	errMsg := ""

	if err != nil {
		status = "failed"
		errMsg = stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
	}

	result := map[string]interface{}{
		"success": status == "success",
		"status":  status,
		"output":  output,
	}

	if errMsg != "" {
		result["error"] = errMsg
	}

	return result, nil
}

// ListImages lista imagens executando DOCKER IMAGES real
func (t *DockerContainerManager) ListImages(params struct{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "images", "--format", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	status := "success"
	output := stdout.String()
	errMsg := ""

	if err != nil {
		status = "failed"
		errMsg = stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
	}

	result := map[string]interface{}{
		"success": status == "success",
		"status":  status,
		"output":  output,
	}

	if errMsg != "" {
		result["error"] = errMsg
	}

	return result, nil
}

// GetContainerStats obtém estatísticas de um container - EXECUTA DOCKER STATS real
func (t *DockerContainerManager) GetContainerStats(params GetContainerStatsParams) (map[string]interface{}, error) {
	if params.ContainerID == "" {
		return nil, fmt.Errorf("container_id não pode estar vazio")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "json", params.ContainerID)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	status := "success"
	output := stdout.String()
	errMsg := ""

	if err != nil {
		status = "failed"
		errMsg = stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
	}

	result := map[string]interface{}{
		"success":      status == "success",
		"container_id": params.ContainerID,
		"status":       status,
		"stats":        output,
	}

	if errMsg != "" {
		result["error"] = errMsg
	}

	return result, nil
}
