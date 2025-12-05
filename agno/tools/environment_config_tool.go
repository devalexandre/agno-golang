package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// EnvironmentConfigTool fornece gerenciamento de configurações e variáveis de ambiente
type EnvironmentConfigTool struct {
	toolkit.Toolkit
	configs    map[string]ConfigProfile
	envVars    map[string]string
	history    []ConfigChange
	currentEnv string
	maxHistory int
}

// ConfigProfile representa um perfil de configuração
type ConfigProfile struct {
	Name        string
	Description string
	Variables   map[string]string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	IsActive    bool
}

// ConfigChange registra uma mudança de configuração
type ConfigChange struct {
	ChangeID    string
	ProfileName string
	VarName     string
	OldValue    string
	NewValue    string
	Action      string // "set", "delete", "profile_create", "profile_delete"
	Timestamp   time.Time
	Username    string
}

// SetEnvVarParams parâmetros para definir variável de ambiente
type SetEnvVarParams struct {
	VarName   string `json:"var_name" description:"Nome da variável de ambiente"`
	Value     string `json:"value" description:"Valor da variável"`
	Profile   string `json:"profile" description:"Perfil (opcional)"`
	Encrypted bool   `json:"encrypted" description:"Se é valor sensível"`
}

// CreateConfigProfileParams parâmetros para criar perfil
type CreateConfigProfileParams struct {
	Name        string            `json:"name" description:"Nome do perfil"`
	Description string            `json:"description" description:"Descrição do perfil"`
	Variables   map[string]string `json:"variables" description:"Variáveis do perfil"`
}

// LoadConfigFileParams parâmetros para carregar arquivo de config
type LoadConfigFileParams struct {
	FilePath string `json:"file_path" description:"Caminho do arquivo (.env ou .yml)"`
	Profile  string `json:"profile" description:"Perfil para aplicar"`
}

// NewEnvironmentConfigTool cria uma nova instância
func NewEnvironmentConfigTool() *EnvironmentConfigTool {
	tool := &EnvironmentConfigTool{
		configs:    make(map[string]ConfigProfile),
		envVars:    make(map[string]string),
		history:    make([]ConfigChange, 0),
		currentEnv: "default",
		maxHistory: 500,
	}
	tool.Toolkit = toolkit.NewToolkit()
	tool.Toolkit.Name = "EnvironmentConfigTool"
	tool.Toolkit.Description = "Gerenciador de configurações e variáveis de ambiente"

	tool.Register("set_env_var",
		"Definir uma variável de ambiente",
		tool,
		tool.SetEnvVar,
		SetEnvVarParams{},
	)

	tool.Register("get_env_var",
		"Obter valor de uma variável de ambiente",
		tool,
		tool.GetEnvVar,
		GetEnvVarParams{},
	)

	tool.Register("create_config_profile",
		"Criar um novo perfil de configuração",
		tool,
		tool.CreateConfigProfile,
		CreateConfigProfileParams{},
	)

	tool.Register("load_config_file",
		"Carregar configurações de um arquivo",
		tool,
		tool.LoadConfigFile,
		LoadConfigFileParams{},
	)

	tool.Register("get_config_profile",
		"Obter um perfil de configuração",
		tool,
		tool.GetConfigProfile,
		GetConfigProfileParams{},
	)

	tool.Register("activate_profile",
		"Ativar um perfil de configuração",
		tool,
		tool.ActivateProfile,
		ActivateProfileParams{},
	)

	tool.Register("validate_config",
		"Validar configuração contra requisitos",
		tool,
		tool.ValidateConfig,
		ValidateConfigParams{},
	)

	tool.Register("get_config_history",
		"Obter histórico de alterações de configuração",
		tool,
		tool.GetConfigHistory,
		GetConfigHistoryParams{},
	)

	// Carregar variáveis de ambiente do sistema
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			tool.envVars[parts[0]] = parts[1]
		}
	}

	return tool
}

// GetEnvVarParams parâmetros para obter variável
type GetEnvVarParams struct {
	VarName string `json:"var_name" description:"Nome da variável"`
}

// GetConfigProfileParams parâmetros para obter perfil
type GetConfigProfileParams struct {
	ProfileName string `json:"profile_name" description:"Nome do perfil"`
}

// ActivateProfileParams parâmetros para ativar perfil
type ActivateProfileParams struct {
	ProfileName string `json:"profile_name" description:"Nome do perfil a ativar"`
}

// ValidateConfigParams parâmetros para validar config
type ValidateConfigParams struct {
	RequiredVars []string `json:"required_vars" description:"Variáveis obrigatórias"`
	Profile      string   `json:"profile" description:"Perfil a validar"`
}

// GetConfigHistoryParams parâmetros para obter histórico
type GetConfigHistoryParams struct {
	Limit   int    `json:"limit" description:"Número de entradas a retornar"`
	Profile string `json:"profile" description:"Filtrar por perfil (opcional)"`
}

// SetEnvVar define uma variável de ambiente
func (t *EnvironmentConfigTool) SetEnvVar(params SetEnvVarParams) (map[string]interface{}, error) {
	if params.VarName == "" {
		return nil, fmt.Errorf("nome da variável não pode estar vazio")
	}

	oldValue := t.envVars[params.VarName]
	t.envVars[params.VarName] = params.Value

	// Aplicar ao SO se não houver perfil específico
	if params.Profile == "" || params.Profile == "default" {
		os.Setenv(params.VarName, params.Value)
	}

	// Registrar mudança
	change := ConfigChange{
		ChangeID:    fmt.Sprintf("chg_%d", time.Now().UnixNano()),
		ProfileName: params.Profile,
		VarName:     params.VarName,
		OldValue:    oldValue,
		NewValue:    params.Value,
		Action:      "set",
		Timestamp:   time.Now(),
		Username:    os.Getenv("USER"),
	}
	t.history = append(t.history, change)

	// Limitar histórico
	if len(t.history) > t.maxHistory {
		t.history = t.history[1:]
	}

	return map[string]interface{}{
		"success":   true,
		"var_name":  params.VarName,
		"new_value": params.Value,
		"old_value": oldValue,
		"change_id": change.ChangeID,
		"timestamp": change.Timestamp.Format(time.RFC3339),
	}, nil
}

// GetEnvVar obtém uma variável de ambiente
func (t *EnvironmentConfigTool) GetEnvVar(params GetEnvVarParams) (map[string]interface{}, error) {
	if params.VarName == "" {
		return nil, fmt.Errorf("nome da variável não pode estar vazio")
	}

	value, exists := t.envVars[params.VarName]
	if !exists {
		value = os.Getenv(params.VarName)
	}

	return map[string]interface{}{
		"success":  true,
		"var_name": params.VarName,
		"value":    value,
		"exists":   exists || value != "",
	}, nil
}

// CreateConfigProfile cria um novo perfil de configuração
func (t *EnvironmentConfigTool) CreateConfigProfile(params CreateConfigProfileParams) (map[string]interface{}, error) {
	if params.Name == "" {
		return nil, fmt.Errorf("nome do perfil não pode estar vazio")
	}

	if _, exists := t.configs[params.Name]; exists {
		return nil, fmt.Errorf("perfil '%s' já existe", params.Name)
	}

	profile := ConfigProfile{
		Name:        params.Name,
		Description: params.Description,
		Variables:   make(map[string]string),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		IsActive:    false,
	}

	// Copiar variáveis
	if params.Variables != nil {
		for k, v := range params.Variables {
			profile.Variables[k] = v
		}
	}

	t.configs[params.Name] = profile

	// Registrar mudança
	change := ConfigChange{
		ChangeID:    fmt.Sprintf("chg_%d", time.Now().UnixNano()),
		ProfileName: params.Name,
		Action:      "profile_create",
		Timestamp:   time.Now(),
		Username:    os.Getenv("USER"),
	}
	t.history = append(t.history, change)

	return map[string]interface{}{
		"success":    true,
		"profile":    params.Name,
		"variables":  len(profile.Variables),
		"created_at": profile.CreatedAt.Format(time.RFC3339),
	}, nil
}

// LoadConfigFile carrega configurações de um arquivo
func (t *EnvironmentConfigTool) LoadConfigFile(params LoadConfigFileParams) (map[string]interface{}, error) {
	if params.FilePath == "" {
		return nil, fmt.Errorf("caminho do arquivo não pode estar vazio")
	}

	// Verificar se arquivo existe
	if _, err := os.Stat(params.FilePath); err != nil {
		return nil, fmt.Errorf("arquivo não encontrado: %s", params.FilePath)
	}

	// Simular leitura de arquivo .env
	content, err := os.ReadFile(params.FilePath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo: %w", err)
	}

	loadedVars := make(map[string]string)
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			loadedVars[key] = value

			// Aplicar ao perfil ou variáveis globais
			if params.Profile != "" {
				t.SetEnvVar(SetEnvVarParams{
					VarName: key,
					Value:   value,
					Profile: params.Profile,
				})
			} else {
				t.envVars[key] = value
				os.Setenv(key, value)
			}
		}
	}

	return map[string]interface{}{
		"success":     true,
		"file":        filepath.Base(params.FilePath),
		"variables":   len(loadedVars),
		"profile":     params.Profile,
		"loaded_vars": loadedVars,
	}, nil
}

// GetConfigProfile obtém um perfil de configuração
func (t *EnvironmentConfigTool) GetConfigProfile(params GetConfigProfileParams) (map[string]interface{}, error) {
	if params.ProfileName == "" {
		return nil, fmt.Errorf("nome do perfil não pode estar vazio")
	}

	profile, exists := t.configs[params.ProfileName]
	if !exists {
		return nil, fmt.Errorf("perfil '%s' não encontrado", params.ProfileName)
	}

	return map[string]interface{}{
		"success":     true,
		"profile":     profile.Name,
		"description": profile.Description,
		"variables":   profile.Variables,
		"is_active":   profile.IsActive,
		"created_at":  profile.CreatedAt.Format(time.RFC3339),
		"updated_at":  profile.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// ActivateProfile ativa um perfil de configuração
func (t *EnvironmentConfigTool) ActivateProfile(params ActivateProfileParams) (map[string]interface{}, error) {
	if params.ProfileName == "" {
		return nil, fmt.Errorf("nome do perfil não pode estar vazio")
	}

	profile, exists := t.configs[params.ProfileName]
	if !exists {
		return nil, fmt.Errorf("perfil '%s' não encontrado", params.ProfileName)
	}

	// Desativar perfil anterior
	for name, prof := range t.configs {
		if prof.IsActive {
			prof.IsActive = false
			t.configs[name] = prof
		}
	}

	// Ativar novo perfil
	profile.IsActive = true
	profile.UpdatedAt = time.Now()
	t.configs[params.ProfileName] = profile
	t.currentEnv = params.ProfileName

	// Aplicar variáveis
	for k, v := range profile.Variables {
		os.Setenv(k, v)
		t.envVars[k] = v
	}

	return map[string]interface{}{
		"success": true,
		"profile": params.ProfileName,
		"active":  true,
		"vars":    len(profile.Variables),
	}, nil
}

// ValidateConfig valida a configuração
func (t *EnvironmentConfigTool) ValidateConfig(params ValidateConfigParams) (map[string]interface{}, error) {
	missing := make([]string, 0)
	found := make(map[string]string)

	for _, varName := range params.RequiredVars {
		value := os.Getenv(varName)
		if value == "" {
			value = t.envVars[varName]
		}

		if value == "" {
			missing = append(missing, varName)
		} else {
			found[varName] = "***" // Ocultar valor para segurança
		}
	}

	valid := len(missing) == 0

	return map[string]interface{}{
		"success":  true,
		"valid":    valid,
		"required": len(params.RequiredVars),
		"found":    len(found),
		"missing":  missing,
		"profile":  params.Profile,
	}, nil
}

// GetConfigHistory retorna o histórico de configurações
func (t *EnvironmentConfigTool) GetConfigHistory(params GetConfigHistoryParams) (map[string]interface{}, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	history := make([]map[string]interface{}, 0)

	// Filtrar e limitar histórico
	count := 0
	for i := len(t.history) - 1; i >= 0 && count < limit; i-- {
		entry := t.history[i]

		if params.Profile != "" && entry.ProfileName != params.Profile {
			continue
		}

		history = append(history, map[string]interface{}{
			"change_id": entry.ChangeID,
			"profile":   entry.ProfileName,
			"var_name":  entry.VarName,
			"action":    entry.Action,
			"old_value": entry.OldValue,
			"new_value": entry.NewValue,
			"timestamp": entry.Timestamp.Format(time.RFC3339),
			"username":  entry.Username,
		})
		count++
	}

	return map[string]interface{}{
		"success": true,
		"entries": history,
		"limit":   limit,
		"total":   len(t.history),
	}, nil
}
