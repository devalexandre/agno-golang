package tools

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// SelfValidationGate valida e sanitiza inputs antes de execução
type SelfValidationGate struct {
	toolkit.Toolkit
	validationRules map[string]ValidationRule
	blockList       map[string]bool
	allowList       map[string]bool
	validationLog   []ValidationEntry
	safetyThreshold float64
}

// ValidationRule define regra de validação
type ValidationRule struct {
	Name              string
	Type              string // "email", "url", "phone", "ip", "sql", "script", "number", "text"
	Pattern           string
	MinLength         int
	MaxLength         int
	AllowedCharacters string
	CustomValidator   string
	Priority          int
}

// ValidationEntry registra resultado de validação
type ValidationEntry struct {
	Timestamp      time.Time `json:"timestamp"`
	InputType      string    `json:"input_type"`
	InputValue     string    `json:"input_value"`
	IsValid        bool      `json:"is_valid"`
	Violations     []string  `json:"violations"`
	SanitizedValue string    `json:"sanitized_value"`
	RiskScore      float64   `json:"risk_score"`
}

// ValidateInputParams parâmetros para validar input
type ValidateInputParams struct {
	InputValue string `json:"input_value" description:"Valor a validar"`
	InputType  string `json:"input_type" description:"Tipo (email, url, phone, ip, sql, script, number, text)"`
	StrictMode bool   `json:"strict_mode" description:"Modo rigoroso de validação"`
}

// SanitizeInputParams parâmetros para sanitizar input
type SanitizeInputParams struct {
	InputValue string `json:"input_value" description:"Valor a sanitizar"`
	InputType  string `json:"input_type" description:"Tipo de input"`
	RemoveHTML bool   `json:"remove_html" description:"Remover HTML"`
	RemoveSQL  bool   `json:"remove_sql" description:"Remover SQL injection patterns"`
	Normalize  bool   `json:"normalize" description:"Normalizar espaços e caracteres"`
}

// ValidationResult resultado da validação
type ValidationResult struct {
	IsValid           bool      `json:"is_valid"`
	InputValue        string    `json:"input_value"`
	InputType         string    `json:"input_type"`
	Violations        []string  `json:"violations"`
	SanitizedValue    string    `json:"sanitized_value"`
	RiskScore         float64   `json:"risk_score"` // 0-1
	Confidence        float64   `json:"confidence"`
	RecommendedAction string    `json:"recommended_action"`
	Timestamp         time.Time `json:"timestamp"`
}

// NewSelfValidationGate cria novo gate
func NewSelfValidationGate() *SelfValidationGate {
	g := &SelfValidationGate{
		validationRules: make(map[string]ValidationRule),
		blockList:       make(map[string]bool),
		allowList:       make(map[string]bool),
		validationLog:   make([]ValidationEntry, 0),
		safetyThreshold: 0.7,
	}
	g.Toolkit = toolkit.NewToolkit()

	g.Toolkit.Register(
		"ValidateInput",
		"Validar input contra regras de segurança",
		g,
		g.ValidateInput,
		ValidateInputParams{},
	)

	g.Toolkit.Register(
		"SanitizeInput",
		"Sanitizar input removendo conteúdo perigoso",
		g,
		g.SanitizeInput,
		SanitizeInputParams{},
	)

	g.Toolkit.Register(
		"CheckAgainstBlocklist",
		"Verificar se input está em blocklist",
		g,
		g.CheckAgainstBlocklist,
		CheckBlockListParams{},
	)

	g.Toolkit.Register(
		"RegisterValidationRule",
		"Registrar nova regra de validação",
		g,
		g.RegisterValidationRule,
		ValidationRule{},
	)

	g.Toolkit.Register(
		"GetValidationLog",
		"Obter log de validações",
		g,
		g.GetValidationLog,
		GetLogParams{},
	)

	// Inicializar regras padrão
	g.initializeDefaultRules()

	return g
}

// ValidateInput valida input
func (g *SelfValidationGate) ValidateInput(params ValidateInputParams) (interface{}, error) {
	if params.InputValue == "" {
		return ValidationResult{
			IsValid:           false,
			Violations:        []string{"Input vazio"},
			RiskScore:         1.0,
			RecommendedAction: "REJECT",
			Timestamp:         time.Now(),
		}, fmt.Errorf("input vazio")
	}

	violations := make([]string, 0)
	riskScore := 0.0

	// Verificar blocklist
	if g.blockList[strings.ToLower(params.InputValue)] {
		violations = append(violations, "Valor na blocklist")
		riskScore = 1.0
	}

	// Verificar allowlist (se tem, só permite valores nela)
	if len(g.allowList) > 0 {
		if !g.allowList[strings.ToLower(params.InputValue)] {
			violations = append(violations, "Valor não em allowlist")
			riskScore = 0.9
		}
	}

	// Validação por tipo
	typeViolations, typeRisk := g.validateByType(params.InputValue, params.InputType, params.StrictMode)
	violations = append(violations, typeViolations...)
	riskScore = (riskScore + typeRisk) / 2

	// Detectar padrões perigosos
	dangerViolations, dangerRisk := g.detectDangerousPatterns(params.InputValue)
	violations = append(violations, dangerViolations...)
	riskScore = (riskScore + dangerRisk) / 2

	isValid := len(violations) == 0 && riskScore < g.safetyThreshold

	sanitized := params.InputValue
	if !isValid {
		sanitized = g.basicSanitize(params.InputValue)
	}

	result := ValidationResult{
		IsValid:        isValid,
		InputValue:     params.InputValue,
		InputType:      params.InputType,
		Violations:     violations,
		SanitizedValue: sanitized,
		RiskScore:      riskScore,
		Confidence:     1.0 - riskScore,
		Timestamp:      time.Now(),
	}

	if isValid {
		result.RecommendedAction = "ACCEPT"
	} else if riskScore < 0.9 {
		result.RecommendedAction = "REVIEW"
	} else {
		result.RecommendedAction = "REJECT"
	}

	// Registrar no log
	g.validationLog = append(g.validationLog, ValidationEntry{
		Timestamp:      time.Now(),
		InputType:      params.InputType,
		InputValue:     params.InputValue,
		IsValid:        isValid,
		Violations:     violations,
		SanitizedValue: sanitized,
		RiskScore:      riskScore,
	})

	if len(g.validationLog) > 10000 {
		g.validationLog = g.validationLog[1:]
	}

	return result, nil
}

// SanitizeInput sanitiza input
func (g *SelfValidationGate) SanitizeInput(params SanitizeInputParams) (interface{}, error) {
	if params.InputValue == "" {
		return map[string]interface{}{
			"original":     "",
			"sanitized":    "",
			"changes_made": 0,
		}, nil
	}

	sanitized := params.InputValue

	// Remover HTML tags
	if params.RemoveHTML {
		sanitized = g.removeHTML(sanitized)
	}

	// Remover SQL patterns
	if params.RemoveSQL {
		sanitized = g.removeSQLPatterns(sanitized)
	}

	// Normalizar
	if params.Normalize {
		sanitized = g.normalizeString(sanitized)
	}

	changesMade := 0
	if sanitized != params.InputValue {
		changesMade = 1
	}

	return map[string]interface{}{
		"original":     params.InputValue,
		"sanitized":    sanitized,
		"changes_made": changesMade,
		"timestamp":    time.Now(),
	}, nil
}

// CheckAgainstBlocklist verifica blocklist
func (g *SelfValidationGate) CheckAgainstBlocklist(params CheckBlockListParams) (interface{}, error) {
	isBlocked := g.blockList[strings.ToLower(params.Value)]

	return map[string]interface{}{
		"value":          params.Value,
		"is_blocked":     isBlocked,
		"blocklist_size": len(g.blockList),
		"timestamp":      time.Now(),
	}, nil
}

// RegisterValidationRule registra regra
func (g *SelfValidationGate) RegisterValidationRule(params ValidationRule) (interface{}, error) {
	if params.Name == "" {
		return nil, fmt.Errorf("nome obrigatório")
	}

	g.validationRules[params.Name] = params

	return map[string]interface{}{
		"success":   true,
		"rule_name": params.Name,
		"rule_type": params.Type,
		"message":   "Regra registrada com sucesso",
		"timestamp": time.Now(),
	}, nil
}

// GetValidationLog retorna log
func (g *SelfValidationGate) GetValidationLog(params GetLogParams) (interface{}, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	start := len(g.validationLog) - limit
	if start < 0 {
		start = 0
	}

	return map[string]interface{}{
		"total_validations":  len(g.validationLog),
		"recent_validations": g.validationLog[start:],
		"limit":              limit,
		"timestamp":          time.Now(),
	}, nil
}

// Helper functions

func (g *SelfValidationGate) initializeDefaultRules() {
	// Email
	g.validationRules["email"] = ValidationRule{
		Name:    "email",
		Type:    "email",
		Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
	}

	// URL
	g.validationRules["url"] = ValidationRule{
		Name:    "url",
		Type:    "url",
		Pattern: `^https?://[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}.*$`,
	}

	// Phone (formato simples)
	g.validationRules["phone"] = ValidationRule{
		Name:    "phone",
		Type:    "phone",
		Pattern: `^[0-9+\-\s\(\)]{7,20}$`,
	}

	// IP
	g.validationRules["ip"] = ValidationRule{
		Name:    "ip",
		Type:    "ip",
		Pattern: `^(\d{1,3}\.){3}\d{1,3}$`,
	}
}

func (g *SelfValidationGate) validateByType(value string, inputType string, strictMode bool) ([]string, float64) {
	violations := make([]string, 0)
	risk := 0.0

	rule, exists := g.validationRules[inputType]
	if !exists {
		return violations, 0.0
	}

	// Verificar comprimento
	if rule.MinLength > 0 && len(value) < rule.MinLength {
		violations = append(violations, fmt.Sprintf("Mínimo %d caracteres", rule.MinLength))
		risk = 0.3
	}

	if rule.MaxLength > 0 && len(value) > rule.MaxLength {
		violations = append(violations, fmt.Sprintf("Máximo %d caracteres", rule.MaxLength))
		risk = 0.5
	}

	// Validar padrão
	if rule.Pattern != "" {
		re, err := regexp.Compile(rule.Pattern)
		if err == nil {
			if !re.MatchString(value) {
				violations = append(violations, fmt.Sprintf("Não corresponde a formato %s", inputType))
				risk = 0.4
			}
		}
	}

	return violations, risk
}

func (g *SelfValidationGate) detectDangerousPatterns(value string) ([]string, float64) {
	violations := make([]string, 0)
	risk := 0.0

	valueLower := strings.ToLower(value)

	// SQL Injection patterns
	sqlPatterns := []string{"drop", "delete", "insert", "update", "exec", "script", "eval", "system"}
	for _, pattern := range sqlPatterns {
		if strings.Contains(valueLower, pattern) {
			violations = append(violations, fmt.Sprintf("Padrão perigoso detectado: %s", pattern))
			risk += 0.2
		}
	}

	// XSS patterns
	xssPatterns := []string{"<script", "javascript:", "onerror=", "onclick=", "onload="}
	for _, pattern := range xssPatterns {
		if strings.Contains(valueLower, pattern) {
			violations = append(violations, fmt.Sprintf("Possível XSS: %s", pattern))
			risk += 0.25
		}
	}

	// Path traversal
	if strings.Contains(value, "../") || strings.Contains(value, "..\\") {
		violations = append(violations, "Path traversal detectado")
		risk += 0.15
	}

	if risk > 1.0 {
		risk = 1.0
	}

	return violations, risk
}

func (g *SelfValidationGate) basicSanitize(value string) string {
	// Remove caracteres perigosos básicos
	dangerous := []string{"<", ">", "\"", "'", "&", ";", "|", "`"}
	sanitized := value
	for _, char := range dangerous {
		sanitized = strings.ReplaceAll(sanitized, char, "")
	}
	return sanitized
}

func (g *SelfValidationGate) removeHTML(value string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(value, "")
}

func (g *SelfValidationGate) removeSQLPatterns(value string) string {
	sqlKeywords := []string{"DROP", "DELETE", "INSERT", "UPDATE", "EXEC", "EXECUTE"}
	result := value
	for _, keyword := range sqlKeywords {
		result = strings.ReplaceAll(result, keyword, "")
	}
	return result
}

func (g *SelfValidationGate) normalizeString(value string) string {
	// Remover espaços múltiplos
	re := regexp.MustCompile(`\s+`)
	normalized := re.ReplaceAllString(strings.TrimSpace(value), " ")
	return normalized
}

// AddToBlocklist adiciona valor a blocklist
func (g *SelfValidationGate) AddToBlocklist(value string) {
	g.blockList[strings.ToLower(value)] = true
}

// AddToAllowlist adiciona valor a allowlist
func (g *SelfValidationGate) AddToAllowlist(value string) {
	g.allowList[strings.ToLower(value)] = true
}

// SetSafetyThreshold define threshold de risco
func (g *SelfValidationGate) SetSafetyThreshold(threshold float64) {
	if threshold > 1.0 {
		threshold = 1.0
	}
	if threshold < 0.0 {
		threshold = 0.0
	}
	g.safetyThreshold = threshold
}

// CheckBlockListParams parâmetros para verificar blocklist
type CheckBlockListParams struct {
	Value string `json:"value" description:"Valor a verificar"`
}

// GetLogParams parâmetros para obter log
type GetLogParams struct {
	Limit int `json:"limit" description:"Número máximo de registros"`
}
