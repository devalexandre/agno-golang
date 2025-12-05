package tools

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// APIClientTool fornece operações de cliente HTTP/REST
type APIClientTool struct {
	toolkit.Toolkit
	requests       []APIRequest
	responses      []APIResponse
	client         *http.Client
	defaultHeaders map[string]string
	requestLog     []RequestLogEntry
	maxLogSize     int
}

// APIRequest representa uma requisição HTTP
type APIRequest struct {
	RequestID  string
	Method     string // GET, POST, PUT, DELETE, PATCH
	URL        string
	Headers    map[string]string
	Body       string
	CreatedAt  time.Time
	Status     string // "pending", "sent", "completed", "failed"
	RetryCount int
}

// APIResponse representa uma resposta HTTP
type APIResponse struct {
	RequestID    string
	StatusCode   int
	Headers      map[string]string
	Body         string
	ContentType  string
	ResponseTime int64 // ms
	Size         int   // bytes
	ErrorMsg     string
	ReceivedAt   time.Time
}

// MakeRequestParams parâmetros para fazer uma requisição
type MakeRequestParams struct {
	Method     string            `json:"method" description:"HTTP method (GET, POST, PUT, DELETE, PATCH)"`
	URL        string            `json:"url" description:"Request URL"`
	Headers    map[string]string `json:"headers" description:"Request headers"`
	Body       string            `json:"body" description:"Request body"`
	Timeout    int               `json:"timeout" description:"Timeout em segundos"`
	RetryCount int               `json:"retry_count" description:"Número de tentativas"`
}

// RequestLogEntry registra uma requisição no histórico
type RequestLogEntry struct {
	RequestID  string
	Method     string
	URL        string
	StatusCode int
	Duration   int64
	Timestamp  time.Time
	Success    bool
}

// NewAPIClientTool cria uma nova instância do APIClientTool
func NewAPIClientTool() *APIClientTool {
	tool := &APIClientTool{
		requests:       make([]APIRequest, 0),
		responses:      make([]APIResponse, 0),
		requestLog:     make([]RequestLogEntry, 0),
		defaultHeaders: make(map[string]string),
		maxLogSize:     1000,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	tool.Toolkit = toolkit.NewToolkit()
	tool.Toolkit.Name = "APIClientTool"
	tool.Toolkit.Description = "Ferramenta para requisições HTTP/REST e gerenciamento de APIs"

	tool.Register("make_request",
		"Fazer uma requisição HTTP/REST",
		tool,
		tool.MakeRequest,
		MakeRequestParams{},
	)

	tool.Register("set_default_header",
		"Definir um header padrão para todas as requisições",
		tool,
		tool.SetDefaultHeader,
		SetHeaderParams{},
	)

	tool.Register("get_request_history",
		"Obter histórico de requisições",
		tool,
		tool.GetRequestHistory,
		struct{}{},
	)

	tool.Register("validate_url",
		"Validar e testar conectividade com uma URL",
		tool,
		tool.ValidateURL,
		ValidateURLParams{},
	)

	return tool
}

// SetHeaderParams parâmetros para definir header
type SetHeaderParams struct {
	Key   string `json:"key" description:"Header key"`
	Value string `json:"value" description:"Header value"`
}

// ValidateURLParams parâmetros para validar URL
type ValidateURLParams struct {
	URL     string `json:"url" description:"URL a validar"`
	Timeout int    `json:"timeout" description:"Timeout em segundos"`
}

// MakeRequest executa uma requisição HTTP
func (t *APIClientTool) MakeRequest(params MakeRequestParams) (map[string]interface{}, error) {
	if params.URL == "" {
		return nil, fmt.Errorf("URL não pode estar vazia")
	}

	// Validar método
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true,
		"DELETE": true, "PATCH": true, "HEAD": true,
	}
	if !validMethods[params.Method] {
		return nil, fmt.Errorf("método HTTP inválido: %s", params.Method)
	}

	// Preparar headers
	headers := make(map[string]string)
	for k, v := range t.defaultHeaders {
		headers[k] = v
	}
	for k, v := range params.Headers {
		headers[k] = v
	}

	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())

	// Registrar requisição
	apiReq := APIRequest{
		RequestID: requestID,
		Method:    params.Method,
		URL:       params.URL,
		Headers:   headers,
		Body:      params.Body,
		CreatedAt: time.Now(),
		Status:    "sent",
	}
	t.requests = append(t.requests, apiReq)

	// Executar requisição com retry
	var lastErr error
	retries := params.RetryCount
	if retries < 0 {
		retries = 0
	}

	for attempt := 0; attempt <= retries; attempt++ {
		resp, err := t.executeRequest(params.Method, params.URL, headers, params.Body, params.Timeout)
		if err == nil {
			// Sucesso
			logEntry := RequestLogEntry{
				RequestID:  requestID,
				Method:     params.Method,
				URL:        params.URL,
				StatusCode: resp.StatusCode,
				Duration:   resp.ResponseTime,
				Timestamp:  time.Now(),
				Success:    true,
			}
			t.requestLog = append(t.requestLog, logEntry)

			// Limitar tamanho do log
			if len(t.requestLog) > t.maxLogSize {
				t.requestLog = t.requestLog[1:]
			}

			t.responses = append(t.responses, *resp)
			return map[string]interface{}{
				"success":          true,
				"request_id":       requestID,
				"status_code":      resp.StatusCode,
				"body":             resp.Body,
				"headers":          resp.Headers,
				"response_time_ms": resp.ResponseTime,
				"size_bytes":       resp.Size,
			}, nil
		}

		lastErr = err
		if attempt < retries {
			// Aguardar antes de tentar novamente
			time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
		}
	}

	logEntry := RequestLogEntry{
		RequestID:  requestID,
		Method:     params.Method,
		URL:        params.URL,
		StatusCode: 0,
		Timestamp:  time.Now(),
		Success:    false,
	}
	t.requestLog = append(t.requestLog, logEntry)

	return map[string]interface{}{
		"success":    false,
		"request_id": requestID,
		"error":      lastErr.Error(),
	}, lastErr
}

// executeRequest executa uma requisição HTTP individual
func (t *APIClientTool) executeRequest(method, url string, headers map[string]string, body string, timeoutSecs int) (*APIResponse, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	// Adicionar headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Definir timeout
	if timeoutSecs <= 0 {
		timeoutSecs = 30
	}
	client := &http.Client{
		Timeout: time.Duration(timeoutSecs) * time.Second,
	}

	// Executar requisição
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao executar requisição: %w", err)
	}
	defer resp.Body.Close()

	// Ler resposta
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	duration := time.Since(start).Milliseconds()

	// Converter headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	apiResp := &APIResponse{
		RequestID:    fmt.Sprintf("resp_%d", time.Now().UnixNano()),
		StatusCode:   resp.StatusCode,
		Headers:      respHeaders,
		Body:         string(respBody),
		ContentType:  resp.Header.Get("Content-Type"),
		ResponseTime: duration,
		Size:         len(respBody),
		ReceivedAt:   time.Now(),
	}

	return apiResp, nil
}

// SetDefaultHeader define um header padrão
func (t *APIClientTool) SetDefaultHeader(params SetHeaderParams) (map[string]interface{}, error) {
	if params.Key == "" {
		return nil, fmt.Errorf("header key não pode estar vazia")
	}

	t.defaultHeaders[params.Key] = params.Value

	return map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("header '%s' definido com sucesso", params.Key),
	}, nil
}

// GetRequestHistory retorna o histórico de requisições
func (t *APIClientTool) GetRequestHistory(params struct{}) (map[string]interface{}, error) {
	history := make([]map[string]interface{}, 0)

	for _, entry := range t.requestLog {
		history = append(history, map[string]interface{}{
			"request_id":  entry.RequestID,
			"method":      entry.Method,
			"url":         entry.URL,
			"status_code": entry.StatusCode,
			"duration_ms": entry.Duration,
			"timestamp":   entry.Timestamp.Format(time.RFC3339),
			"success":     entry.Success,
		})
	}

	return map[string]interface{}{
		"success":      true,
		"total":        len(history),
		"request_log":  history,
		"max_log_size": t.maxLogSize,
	}, nil
}

// ValidateURL valida uma URL
func (t *APIClientTool) ValidateURL(params ValidateURLParams) (map[string]interface{}, error) {
	if params.URL == "" {
		return nil, fmt.Errorf("URL não pode estar vazia")
	}

	if params.Timeout <= 0 {
		params.Timeout = 10
	}

	// Fazer requisição HEAD
	client := &http.Client{
		Timeout: time.Duration(params.Timeout) * time.Second,
	}

	req, err := http.NewRequest("HEAD", params.URL, nil)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"url":     params.URL,
			"valid":   false,
			"error":   fmt.Sprintf("URL inválida: %v", err),
		}, nil
	}

	// Adicionar User-Agent
	req.Header.Set("User-Agent", "Agno-APIClient/1.0")

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return map[string]interface{}{
			"success":     false,
			"url":         params.URL,
			"valid":       false,
			"error":       fmt.Sprintf("falha na conexão: %v", err),
			"duration_ms": duration,
		}, nil
	}
	defer resp.Body.Close()

	isValid := resp.StatusCode >= 200 && resp.StatusCode < 400

	return map[string]interface{}{
		"success":      true,
		"url":          params.URL,
		"valid":        isValid,
		"status_code":  resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
		"duration_ms":  duration,
	}, nil
}
