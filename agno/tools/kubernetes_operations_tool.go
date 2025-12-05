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

// checkKubectlAvailable verifica se kubectl está instalado
func checkKubectlAvailable() error {
	cmd := exec.Command("which", "kubectl")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("⚠️  AVISO: kubectl não está instalado. Instale a partir de: https://kubernetes.io/docs/tasks/tools/")
	}
	return nil
}

// KubernetesOperationsTool executa operações kubectl REAIS
type KubernetesOperationsTool struct {
	toolkit.Toolkit
}

// KubeResult resultado de operação kubectl
type KubeResult struct {
	Success    bool      `json:"success"`
	Output     string    `json:"output"`
	Error      string    `json:"error,omitempty"`
	Command    string    `json:"command"`
	ExitCode   int       `json:"exit_code"`
	Timestamp  time.Time `json:"timestamp"`
	ExecutedAt string    `json:"executed_at"`
}

// VersionParams parâmetros para version
type VersionParams struct {
	Short bool `json:"short" description:"Formato curto"`
}

// GetNamespacesParams parâmetros para listar namespaces
type GetNamespacesParams struct {
	Format string `json:"format" description:"Formato (json, yaml, wide)"`
}

// GetNodesParams parâmetros para listar nodes
type GetNodesParams struct {
	Format string `json:"format" description:"Formato (json, yaml, wide)"`
	Labels string `json:"labels" description:"Filtro de labels (opcional)"`
}

// GetPodsParams parâmetros para listar pods
type GetPodsParams struct {
	Namespace string `json:"namespace" description:"Namespace (padrão: default)"`
	Format    string `json:"format" description:"Formato (json, yaml, wide)"`
	AllNS     bool   `json:"all_ns" description:"Listar de todos os namespaces"`
}

// GetServicesParams parâmetros para listar services
type GetServicesParams struct {
	Namespace string `json:"namespace" description:"Namespace (padrão: default)"`
	Format    string `json:"format" description:"Formato (json, yaml, wide)"`
}

// GetLogsParams parâmetros para obter logs
type GetLogsKubeParams struct {
	PodName   string `json:"pod_name" description:"Nome do pod"`
	Namespace string `json:"namespace" description:"Namespace (padrão: default)"`
	Container string `json:"container" description:"Nome do container (opcional)"`
	Lines     int    `json:"lines" description:"Últimas N linhas (padrão: 50)"`
}

// DescribeResourceParams parâmetros para descrever recurso
type DescribeResourceParams struct {
	ResourceType string `json:"resource_type" description:"Tipo (pod, service, deployment, etc)"`
	Name         string `json:"name" description:"Nome do recurso"`
	Namespace    string `json:"namespace" description:"Namespace (padrão: default)"`
}

// ApplyParams parâmetros para aplicar manifesto
type ApplyParams struct {
	FilePath string `json:"file_path" description:"Caminho do arquivo YAML"`
}

// DeleteParams parâmetros para deletar recurso
type DeleteKubeParams struct {
	ResourceType string `json:"resource_type" description:"Tipo (pod, service, deployment, etc)"`
	Name         string `json:"name" description:"Nome do recurso"`
	Namespace    string `json:"namespace" description:"Namespace (padrão: default)"`
}

// NewKubernetesOperationsTool cria nova instância com operações REAIS
func NewKubernetesOperationsTool() *KubernetesOperationsTool {
	// Verificar se kubectl está disponível
	if err := checkKubectlAvailable(); err != nil {
		fmt.Printf("%v\n", err)
	}

	t := &KubernetesOperationsTool{
		Toolkit: toolkit.NewToolkit(),
	}

	t.Toolkit.Register(
		"Version",
		"Obter versão do kubectl e informações do cluster",
		t,
		t.Version,
		VersionParams{},
	)

	t.Toolkit.Register(
		"GetNamespaces",
		"Listar todos os namespaces",
		t,
		t.GetNamespaces,
		GetNamespacesParams{},
	)

	t.Toolkit.Register(
		"GetNodes",
		"Listar nodes do cluster",
		t,
		t.GetNodes,
		GetNodesParams{},
	)

	t.Toolkit.Register(
		"GetPods",
		"Listar pods do namespace",
		t,
		t.GetPods,
		GetPodsParams{},
	)

	t.Toolkit.Register(
		"GetServices",
		"Listar services do namespace",
		t,
		t.GetServices,
		GetServicesParams{},
	)

	t.Toolkit.Register(
		"GetLogs",
		"Obter logs de um pod",
		t,
		t.GetLogs,
		GetLogsKubeParams{},
	)

	t.Toolkit.Register(
		"DescribeResource",
		"Descrever recurso Kubernetes",
		t,
		t.DescribeResource,
		DescribeResourceParams{},
	)

	t.Toolkit.Register(
		"Apply",
		"Aplicar manifesto YAML",
		t,
		t.Apply,
		ApplyParams{},
	)

	t.Toolkit.Register(
		"Delete",
		"Deletar recurso Kubernetes",
		t,
		t.Delete,
		DeleteKubeParams{},
	)

	return t
}

// Version retorna versão do kubectl REAL
func (t *KubernetesOperationsTool) Version(params VersionParams) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"version"}
	if params.Short {
		args = append(args, "--short")
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	errMsg := stderr.String()
	if err != nil || strings.Contains(errMsg, "Unable to connect") || strings.Contains(errMsg, "connection refused") {
		return KubeResult{
			Success:    false,
			Error:      fmt.Sprintf("❌ Falha ao conectar ao cluster Kubernetes - Verifique se kubectl está configurado e o cluster está acessível"),
			Command:    "kubectl version",
			ExitCode:   cmd.ProcessState.ExitCode(),
			Timestamp:  time.Now(),
			ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	result := KubeResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      errMsg,
		Command:    "kubectl version",
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// GetNamespaces lista namespaces REAIS
func (t *KubernetesOperationsTool) GetNamespaces(params GetNamespacesParams) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"get", "namespaces"}
	if params.Format != "" {
		args = append(args, "-o", params.Format)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := KubeResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    "kubectl get namespaces",
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// GetNodes lista nodes REAIS
func (t *KubernetesOperationsTool) GetNodes(params GetNodesParams) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"get", "nodes"}
	if params.Format != "" {
		args = append(args, "-o", params.Format)
	}
	if params.Labels != "" {
		args = append(args, "-l", params.Labels)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := KubeResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    "kubectl get nodes",
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// GetPods lista pods REAIS
func (t *KubernetesOperationsTool) GetPods(params GetPodsParams) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := params.Namespace
	if namespace == "" {
		namespace = "default"
	}

	args := []string{"get", "pods", "-n", namespace}
	if params.AllNS {
		args = []string{"get", "pods", "-A"}
	}
	if params.Format != "" {
		args = append(args, "-o", params.Format)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := KubeResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    "kubectl get pods",
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// GetServices lista services REAIS
func (t *KubernetesOperationsTool) GetServices(params GetServicesParams) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := params.Namespace
	if namespace == "" {
		namespace = "default"
	}

	args := []string{"get", "services", "-n", namespace}
	if params.Format != "" {
		args = append(args, "-o", params.Format)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := KubeResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    "kubectl get services",
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// GetLogs obtém logs de pod REAIS
func (t *KubernetesOperationsTool) GetLogs(params GetLogsKubeParams) (interface{}, error) {
	if params.PodName == "" {
		return KubeResult{Success: false, Error: "pod_name é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	namespace := params.Namespace
	if namespace == "" {
		namespace = "default"
	}

	lines := params.Lines
	if lines <= 0 {
		lines = 50
	}

	args := []string{"logs", params.PodName, "-n", namespace, "--tail", fmt.Sprintf("%d", lines)}
	if params.Container != "" {
		args = append(args, "-c", params.Container)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := KubeResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("kubectl logs %s", params.PodName),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// DescribeResource descreve recurso REAL
func (t *KubernetesOperationsTool) DescribeResource(params DescribeResourceParams) (interface{}, error) {
	if params.ResourceType == "" || params.Name == "" {
		return KubeResult{Success: false, Error: "resource_type e name são obrigatórios"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := params.Namespace
	if namespace == "" {
		namespace = "default"
	}

	resource := fmt.Sprintf("%s/%s", params.ResourceType, params.Name)
	args := []string{"describe", resource, "-n", namespace}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := KubeResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("kubectl describe %s", resource),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// Apply aplica manifesto REAL
func (t *KubernetesOperationsTool) Apply(params ApplyParams) (interface{}, error) {
	if params.FilePath == "" {
		return KubeResult{Success: false, Error: "file_path é obrigatório"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", params.FilePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := KubeResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("kubectl apply -f %s", params.FilePath),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}

// Delete deleta recurso REAL
func (t *KubernetesOperationsTool) Delete(params DeleteKubeParams) (interface{}, error) {
	if params.ResourceType == "" || params.Name == "" {
		return KubeResult{Success: false, Error: "resource_type e name são obrigatórios"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	namespace := params.Namespace
	if namespace == "" {
		namespace = "default"
	}

	resource := fmt.Sprintf("%s/%s", params.ResourceType, params.Name)
	cmd := exec.CommandContext(ctx, "kubectl", "delete", resource, "-n", namespace)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := KubeResult{
		Success:    err == nil,
		Output:     stdout.String(),
		Error:      stderr.String(),
		Command:    fmt.Sprintf("kubectl delete %s", resource),
		ExitCode:   cmd.ProcessState.ExitCode(),
		Timestamp:  time.Now(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return result, nil
}
