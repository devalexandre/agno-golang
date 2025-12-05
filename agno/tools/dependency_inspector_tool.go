package tools

import (
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DependencyInspectorTool fornece análise de dependências do projeto
type DependencyInspectorTool struct {
	toolkit.Toolkit
	dependencies    []DependencyInfo
	reports         []DependencyReport
	vulnerabilities []Vulnerability
	maxResults      int
}

// DependencyInfo representa informações sobre uma dependência
type DependencyInfo struct {
	Module          string
	Version         string
	LatestVersion   string
	UpdateAvailable bool
	DirectDep       bool
	IndirectDep     bool
	CriticalVulns   int
	HighVulns       int
	MediumVulns     int
	LastUpdated     time.Time
	License         string
}

// DependencyReport representa um relatório de dependências
type DependencyReport struct {
	ReportID             string
	ProjectPath          string
	TotalDeps            int
	DirectDeps           int
	IndirectDeps         int
	UpdatesAvailable     int
	VulnerabilitiesFound int
	GeneratedAt          time.Time
	Dependencies         []DependencyInfo
}

// Vulnerability representa uma vulnerabilidade encontrada
type Vulnerability struct {
	VulnID      string
	Module      string
	Version     string
	Severity    string // "critical", "high", "medium", "low"
	Title       string
	Description string
	CVE         string
	FixedIn     string
	ReportedAt  time.Time
}

// AnalyzeDependenciesParams parâmetros para analisar dependências
type AnalyzeDependenciesParams struct {
	ProjectPath          string `json:"project_path" description:"Caminho do projeto Go"`
	CheckVulnerabilities bool   `json:"check_vulnerabilities" description:"Verificar vulnerabilidades"`
}

// CheckForUpdatesParams parâmetros para verificar atualizações
type CheckForUpdatesParams struct {
	ProjectPath string `json:"project_path" description:"Caminho do projeto Go"`
	CheckMinor  bool   `json:"check_minor" description:"Verificar atualizações minor"`
}

// GetVulnerabilitiesParams parâmetros para obter vulnerabilidades
type GetVulnerabilitiesParams struct {
	Module   string `json:"module" description:"Nome do módulo (opcional)"`
	Severity string `json:"severity" description:"Filtro de severidade (critical, high, medium, low)"`
}

// UpdateDependencyParams parâmetros para atualizar dependência
type UpdateDependencyParams struct {
	ProjectPath string `json:"project_path" description:"Caminho do projeto Go"`
	Module      string `json:"module" description:"Nome do módulo a atualizar"`
	Version     string `json:"version" description:"Versão alvo (opcional - usa latest)"`
}

// LicenseCheckParams parâmetros para verificação de licenças
type LicenseCheckParams struct {
	ProjectPath     string   `json:"project_path" description:"Caminho do projeto Go"`
	AllowedLicenses []string `json:"allowed_licenses" description:"Licenças permitidas"`
}

// NewDependencyInspectorTool cria uma nova instância
func NewDependencyInspectorTool() *DependencyInspectorTool {
	tool := &DependencyInspectorTool{
		dependencies:    make([]DependencyInfo, 0),
		reports:         make([]DependencyReport, 0),
		vulnerabilities: make([]Vulnerability, 0),
		maxResults:      1000,
	}
	tool.Toolkit = toolkit.NewToolkit()
	tool.Toolkit.Name = "DependencyInspectorTool"
	tool.Toolkit.Description = "Ferramenta de inspeção, análise e gerenciamento de dependências"

	tool.Register("analyze_dependencies",
		"Analisar dependências do projeto",
		tool,
		tool.AnalyzeDependencies,
		AnalyzeDependenciesParams{},
	)

	tool.Register("check_for_updates",
		"Verificar atualizações de dependências",
		tool,
		tool.CheckForUpdates,
		CheckForUpdatesParams{},
	)

	tool.Register("get_vulnerabilities",
		"Obter vulnerabilidades conhecidas",
		tool,
		tool.GetVulnerabilities,
		GetVulnerabilitiesParams{},
	)

	tool.Register("update_dependency",
		"Atualizar uma dependência",
		tool,
		tool.UpdateDependency,
		UpdateDependencyParams{},
	)

	tool.Register("check_licenses",
		"Verificar compatibilidade de licenças",
		tool,
		tool.CheckLicenses,
		LicenseCheckParams{},
	)

	tool.Register("get_dependency_tree",
		"Obter árvore de dependências",
		tool,
		tool.GetDependencyTree,
		struct{}{},
	)

	return tool
}

// AnalyzeDependencies analisa dependências do projeto
func (t *DependencyInspectorTool) AnalyzeDependencies(params AnalyzeDependenciesParams) (map[string]interface{}, error) {
	if params.ProjectPath == "" {
		return nil, fmt.Errorf("caminho do projeto não pode estar vazio")
	}

	// Simular análise de dependências do go.mod
	deps := []DependencyInfo{
		{
			Module:          "github.com/go-chi/chi/v5",
			Version:         "v5.0.10",
			LatestVersion:   "v5.1.0",
			UpdateAvailable: true,
			DirectDep:       true,
			IndirectDep:     false,
			CriticalVulns:   0,
			HighVulns:       0,
			MediumVulns:     0,
			LastUpdated:     time.Now().AddDate(0, -1, 0),
			License:         "MIT",
		},
		{
			Module:          "gorm.io/gorm",
			Version:         "v1.25.4",
			LatestVersion:   "v1.25.5",
			UpdateAvailable: true,
			DirectDep:       true,
			IndirectDep:     false,
			CriticalVulns:   0,
			HighVulns:       1,
			MediumVulns:     2,
			LastUpdated:     time.Now().AddDate(0, -2, 0),
			License:         "MIT",
		},
		{
			Module:          "github.com/golang/protobuf",
			Version:         "v1.5.3",
			LatestVersion:   "v1.5.3",
			UpdateAvailable: false,
			DirectDep:       false,
			IndirectDep:     true,
			CriticalVulns:   0,
			HighVulns:       0,
			MediumVulns:     0,
			LastUpdated:     time.Now().AddDate(0, -3, 0),
			License:         "BSD-3-Clause",
		},
	}

	t.dependencies = append(t.dependencies, deps...)

	report := DependencyReport{
		ReportID:             fmt.Sprintf("rep_%d", time.Now().UnixNano()),
		ProjectPath:          params.ProjectPath,
		TotalDeps:            len(deps),
		DirectDeps:           2,
		IndirectDeps:         1,
		UpdatesAvailable:     2,
		VulnerabilitiesFound: 3,
		GeneratedAt:          time.Now(),
		Dependencies:         deps,
	}

	t.reports = append(t.reports, report)

	// Simular vulnerabilidades se solicitado
	vulns := 0
	if params.CheckVulnerabilities {
		t.vulnerabilities = append(t.vulnerabilities, Vulnerability{
			VulnID:      "VULN-001",
			Module:      "gorm.io/gorm",
			Version:     "v1.25.4",
			Severity:    "high",
			Title:       "SQL Injection Vulnerability",
			Description: "Potencial SQL injection em querybuilder",
			CVE:         "CVE-2024-1234",
			FixedIn:     "v1.25.5",
			ReportedAt:  time.Now().AddDate(0, 0, -10),
		})
		vulns = 1
	}

	return map[string]interface{}{
		"success":               true,
		"report_id":             report.ReportID,
		"total_dependencies":    len(deps),
		"direct_dependencies":   2,
		"indirect_dependencies": 1,
		"updates_available":     2,
		"vulnerabilities_found": vulns,
		"project_path":          params.ProjectPath,
		"dependencies":          deps,
	}, nil
}

// CheckForUpdates verifica atualizações de dependências
func (t *DependencyInspectorTool) CheckForUpdates(params CheckForUpdatesParams) (map[string]interface{}, error) {
	if params.ProjectPath == "" {
		return nil, fmt.Errorf("caminho do projeto não pode estar vazio")
	}

	// Simular verificação de atualizações
	updates := []map[string]interface{}{
		{
			"module":    "github.com/go-chi/chi/v5",
			"current":   "v5.0.10",
			"latest":    "v5.1.0",
			"type":      "minor",
			"published": "2024-01-15",
			"breaking":  false,
		},
		{
			"module":       "gorm.io/gorm",
			"current":      "v1.25.4",
			"latest":       "v1.25.5",
			"type":         "patch",
			"published":    "2024-01-10",
			"breaking":     false,
			"security_fix": true,
		},
	}

	return map[string]interface{}{
		"success":      true,
		"project_path": params.ProjectPath,
		"updates":      len(updates),
		"details":      updates,
		"check_minor":  params.CheckMinor,
	}, nil
}

// GetVulnerabilities obtém vulnerabilidades
func (t *DependencyInspectorTool) GetVulnerabilities(params GetVulnerabilitiesParams) (map[string]interface{}, error) {
	vulns := make([]Vulnerability, 0)

	for _, vuln := range t.vulnerabilities {
		// Filtrar por módulo se especificado
		if params.Module != "" && vuln.Module != params.Module {
			continue
		}

		// Filtrar por severidade se especificado
		if params.Severity != "" && vuln.Severity != params.Severity {
			continue
		}

		vulns = append(vulns, vuln)
	}

	// Se não houver vulnerabilidades no histórico, simular algumas
	if len(vulns) == 0 {
		vulns = []Vulnerability{
			{
				VulnID:      "VULN-002",
				Module:      "github.com/go-chi/chi/v5",
				Version:     "v5.0.10",
				Severity:    "medium",
				Title:       "Path Traversal",
				Description: "Possível path traversal em middlewares",
				CVE:         "CVE-2024-5678",
				FixedIn:     "v5.1.0",
				ReportedAt:  time.Now().AddDate(0, 0, -5),
			},
		}
	}

	return map[string]interface{}{
		"success":         true,
		"vulnerabilities": len(vulns),
		"details":         vulns,
		"module_filter":   params.Module,
		"severity_filter": params.Severity,
	}, nil
}

// UpdateDependency atualiza uma dependência
func (t *DependencyInspectorTool) UpdateDependency(params UpdateDependencyParams) (map[string]interface{}, error) {
	if params.ProjectPath == "" {
		return nil, fmt.Errorf("caminho do projeto não pode estar vazio")
	}

	if params.Module == "" {
		return nil, fmt.Errorf("módulo não pode estar vazio")
	}

	version := params.Version
	if version == "" {
		version = "latest"
	}

	// Simular atualização
	command := fmt.Sprintf("go get -u %s@%s", params.Module, version)

	return map[string]interface{}{
		"success":        true,
		"module":         params.Module,
		"target_version": version,
		"command":        command,
		"status":         "updated",
		"updated_at":     time.Now().Format(time.RFC3339),
	}, nil
}

// CheckLicenses verifica compatibilidade de licenças
func (t *DependencyInspectorTool) CheckLicenses(params LicenseCheckParams) (map[string]interface{}, error) {
	if params.ProjectPath == "" {
		return nil, fmt.Errorf("caminho do projeto não pode estar vazio")
	}

	// Simular verificação de licenças
	licenses := []map[string]interface{}{
		{
			"module":  "github.com/go-chi/chi/v5",
			"license": "MIT",
			"allowed": true,
		},
		{
			"module":  "gorm.io/gorm",
			"license": "MIT",
			"allowed": true,
		},
		{
			"module":  "github.com/golang/protobuf",
			"license": "BSD-3-Clause",
			"allowed": true,
		},
	}

	compliant := 3
	nonCompliant := 0

	return map[string]interface{}{
		"success":          true,
		"project_path":     params.ProjectPath,
		"total":            len(licenses),
		"compliant":        compliant,
		"non_compliant":    nonCompliant,
		"allowed_licenses": params.AllowedLicenses,
		"details":          licenses,
	}, nil
}

// GetDependencyTree obtém a árvore de dependências
func (t *DependencyInspectorTool) GetDependencyTree(params struct{}) (map[string]interface{}, error) {
	// Simular árvore de dependências
	tree := map[string]interface{}{
		"root": "myproject",
		"deps": []map[string]interface{}{
			{
				"module":  "github.com/go-chi/chi/v5",
				"version": "v5.0.10",
				"subdeps": []map[string]interface{}{
					{
						"module":  "github.com/stretchr/testify",
						"version": "v1.8.4",
					},
				},
			},
			{
				"module":  "gorm.io/gorm",
				"version": "v1.25.4",
				"subdeps": []map[string]interface{}{
					{
						"module":  "gorm.io/driver/mysql",
						"version": "v1.5.2",
					},
					{
						"module":  "github.com/jinzhu/now",
						"version": "v1.1.5",
					},
				},
			},
		},
	}

	return map[string]interface{}{
		"success":   true,
		"tree":      tree,
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}
