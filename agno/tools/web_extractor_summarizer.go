package tools

import (
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// WebExtractorSummarizer extrai e resume conteúdo web
type WebExtractorSummarizer struct {
	toolkit.Toolkit
	extractionCache map[string]CachedExtraction
	summaryHistory  []SummaryRecord
}

// CachedExtraction armazena extração em cache
type CachedExtraction struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	ExtractedAt time.Time         `json:"extracted_at"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Length      int               `json:"length"`
	Language    string            `json:"language"`
	MainImages  []string          `json:"main_images"`
	Links       []LinkInfo        `json:"links"`
	Metadata    map[string]string `json:"metadata"`
}

// LinkInfo informações de link extraído
type LinkInfo struct {
	URL  string `json:"url"`
	Text string `json:"text"`
	Type string `json:"type"` // internal, external, mailto, phone
}

// SummaryRecord registro de resumo gerado
type SummaryRecord struct {
	SummaryID        string    `json:"summary_id"`
	URL              string    `json:"url"`
	OriginalLen      int       `json:"original_length"`
	SummaryLen       int       `json:"summary_length"`
	CompressionRatio float64   `json:"compression_ratio"`
	KeyPoints        []string  `json:"key_points"`
	Style            string    `json:"style"` // bullet, paragraph, abstract
	CreatedAt        time.Time `json:"created_at"`
	Language         string    `json:"language"`
}

// ExtractWebPageParams parâmetros para extrair página
type ExtractWebPageParams struct {
	URL             string `json:"url" description:"URL a extrair"`
	IncludeImages   bool   `json:"include_images" description:"Incluir imagens"`
	IncludeLinks    bool   `json:"include_links" description:"Incluir links"`
	ExtractMetadata bool   `json:"extract_metadata" description:"Extrair metadados"`
	UseCache        bool   `json:"use_cache" description:"Usar cache se disponível"`
}

// SummarizeContentParams parâmetros para resumir
type SummarizeContentParams struct {
	URL            string `json:"url" description:"URL a resumir"`
	Content        string `json:"content" description:"Conteúdo a resumir (alternativa a URL)"`
	Style          string `json:"style" description:"bullet, paragraph, abstract"`
	MaxLength      int    `json:"max_length" description:"Comprimento máximo do resumo"`
	KeyPointsCount int    `json:"key_points_count" description:"Número de pontos-chave"`
	Language       string `json:"language" description:"Idioma do conteúdo"`
}

// ExtractionResult resultado de extração
type ExtractionResult struct {
	Success     bool              `json:"success"`
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Length      int               `json:"length"`
	Images      []string          `json:"images,omitempty"`
	Links       []LinkInfo        `json:"links,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Language    string            `json:"language"`
	ExtractedAt time.Time         `json:"extracted_at"`
	Message     string            `json:"message"`
}

// SummaryResult resultado de resumo
type SummaryResult struct {
	Success          bool      `json:"success"`
	URL              string    `json:"url"`
	OriginalLength   int       `json:"original_length"`
	Summary          string    `json:"summary"`
	SummaryLength    int       `json:"summary_length"`
	CompressionRatio float64   `json:"compression_ratio"`
	KeyPoints        []string  `json:"key_points"`
	Style            string    `json:"style"`
	Language         string    `json:"language"`
	CreatedAt        time.Time `json:"created_at"`
	Message          string    `json:"message"`
}

// NewWebExtractorSummarizer cria novo extrator
func NewWebExtractorSummarizer() *WebExtractorSummarizer {
	w := &WebExtractorSummarizer{
		extractionCache: make(map[string]CachedExtraction),
		summaryHistory:  make([]SummaryRecord, 0),
	}
	w.Toolkit = toolkit.NewToolkit()

	w.Toolkit.Register(
		"ExtractWebPage",
		"Extrair conteúdo de página web",
		w,
		w.ExtractWebPage,
		ExtractWebPageParams{},
	)

	w.Toolkit.Register(
		"SummarizeContent",
		"Gerar resumo de conteúdo web",
		w,
		w.SummarizeContent,
		SummarizeContentParams{},
	)

	w.Toolkit.Register(
		"ExtractKeyInsights",
		"Extrair insights principais de conteúdo",
		w,
		w.ExtractKeyInsights,
		ExtractInsightsParams{},
	)

	w.Toolkit.Register(
		"GetExtractedContent",
		"Obter conteúdo já extraído",
		w,
		w.GetExtractedContent,
		GetCachedParams{},
	)

	w.Toolkit.Register(
		"ClearCache",
		"Limpar cache de extrações",
		w,
		w.ClearCache,
		ClearCacheParams{},
	)

	return w
}

// ExtractWebPage extrai conteúdo de página
func (w *WebExtractorSummarizer) ExtractWebPage(params ExtractWebPageParams) (interface{}, error) {
	if params.URL == "" {
		return ExtractionResult{Success: false}, fmt.Errorf("URL obrigatória")
	}

	// Verificar cache
	if params.UseCache {
		if cached, exists := w.extractionCache[params.URL]; exists {
			if time.Now().Before(cached.ExpiresAt) {
				return ExtractionResult{
					Success:     true,
					URL:         cached.URL,
					Title:       cached.Title,
					Content:     cached.Content,
					Length:      cached.Length,
					Images:      cached.MainImages,
					Links:       cached.Links,
					Metadata:    cached.Metadata,
					Language:    cached.Language,
					ExtractedAt: cached.ExtractedAt,
					Message:     "Conteúdo recuperado do cache",
				}, nil
			}
		}
	}

	// Simular extração (em produção seria HTTP request)
	content := w.simulateWebExtraction(params.URL)
	title := w.extractTitle(params.URL)
	language := "pt-BR"

	// Extrair imagens
	var images []string
	if params.IncludeImages {
		images = w.extractImages(content)
	}

	// Extrair links
	var links []LinkInfo
	if params.IncludeLinks {
		links = w.extractLinks(content)
	}

	// Extrair metadados
	metadata := make(map[string]string)
	if params.ExtractMetadata {
		metadata = w.extractMetadata(content)
	}

	// Armazenar em cache
	cached := CachedExtraction{
		URL:         params.URL,
		Title:       title,
		Content:     content,
		ExtractedAt: time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Length:      len(content),
		Language:    language,
		MainImages:  images,
		Links:       links,
		Metadata:    metadata,
	}

	w.extractionCache[params.URL] = cached

	return ExtractionResult{
		Success:     true,
		URL:         params.URL,
		Title:       title,
		Content:     content,
		Length:      len(content),
		Images:      images,
		Links:       links,
		Metadata:    metadata,
		Language:    language,
		ExtractedAt: time.Now(),
		Message:     fmt.Sprintf("Página extraída com sucesso (%d caracteres)", len(content)),
	}, nil
}

// SummarizeContent gera resumo de conteúdo
func (w *WebExtractorSummarizer) SummarizeContent(params SummarizeContentParams) (interface{}, error) {
	content := params.Content

	// Se URL fornecida, extrair conteúdo
	if params.URL != "" && params.Content == "" {
		extraction, _ := w.ExtractWebPage(ExtractWebPageParams{
			URL:      params.URL,
			UseCache: true,
		})

		if result, ok := extraction.(ExtractionResult); ok {
			content = result.Content
		}
	}

	if content == "" {
		return SummaryResult{Success: false}, fmt.Errorf("conteúdo obrigatório")
	}

	if params.Style == "" {
		params.Style = "paragraph"
	}

	if params.KeyPointsCount <= 0 {
		params.KeyPointsCount = 5
	}

	if params.MaxLength <= 0 {
		params.MaxLength = 200
	}

	// Gerar resumo
	summary := w.generateSummary(content, params.Style, params.MaxLength)
	keyPoints := w.extractKeyPoints(content, params.KeyPointsCount)

	compressionRatio := float64(len(summary)) / float64(len(content))

	record := SummaryRecord{
		SummaryID:        fmt.Sprintf("summary_%d", time.Now().UnixNano()),
		URL:              params.URL,
		OriginalLen:      len(content),
		SummaryLen:       len(summary),
		CompressionRatio: compressionRatio,
		KeyPoints:        keyPoints,
		Style:            params.Style,
		CreatedAt:        time.Now(),
		Language:         params.Language,
	}

	w.summaryHistory = append(w.summaryHistory, record)

	return SummaryResult{
		Success:          true,
		URL:              params.URL,
		OriginalLength:   len(content),
		Summary:          summary,
		SummaryLength:    len(summary),
		CompressionRatio: compressionRatio,
		KeyPoints:        keyPoints,
		Style:            params.Style,
		Language:         params.Language,
		CreatedAt:        time.Now(),
		Message:          fmt.Sprintf("Resumo gerado (%d%% do original)", int(compressionRatio*100)),
	}, nil
}

// ExtractKeyInsights extrai insights principais
func (w *WebExtractorSummarizer) ExtractKeyInsights(params ExtractInsightsParams) (interface{}, error) {
	content := params.Content

	if params.URL != "" && params.Content == "" {
		extraction, _ := w.ExtractWebPage(ExtractWebPageParams{
			URL:      params.URL,
			UseCache: true,
		})

		if result, ok := extraction.(ExtractionResult); ok {
			content = result.Content
		}
	}

	if content == "" {
		return nil, fmt.Errorf("conteúdo obrigatório")
	}

	insights := w.extractInsights(content, params.Category)

	return map[string]interface{}{
		"success":   true,
		"url":       params.URL,
		"category":  params.Category,
		"insights":  insights,
		"count":     len(insights),
		"timestamp": time.Now(),
	}, nil
}

// GetExtractedContent obtém conteúdo extraído
func (w *WebExtractorSummarizer) GetExtractedContent(params GetCachedParams) (interface{}, error) {
	if cached, exists := w.extractionCache[params.URL]; exists {
		return cached, nil
	}

	return nil, fmt.Errorf("conteúdo não encontrado no cache")
}

// ClearCache limpa cache
func (w *WebExtractorSummarizer) ClearCache(params ClearCacheParams) (interface{}, error) {
	count := len(w.extractionCache)

	if params.ClearAll {
		w.extractionCache = make(map[string]CachedExtraction)
	} else {
		// Limpar entradas expiradas
		now := time.Now()
		for url, cached := range w.extractionCache {
			if now.After(cached.ExpiresAt) {
				delete(w.extractionCache, url)
			}
		}
	}

	return map[string]interface{}{
		"success":           true,
		"removed_count":     count,
		"remaining_entries": len(w.extractionCache),
		"timestamp":         time.Now(),
	}, nil
}

// Helper functions

func (w *WebExtractorSummarizer) simulateWebExtraction(url string) string {
	// Simular conteúdo extraído
	return fmt.Sprintf(`
	Título da Página: %s
	
	Este é um conteúdo de exemplo extraído da URL %s.
	Contém informações relevantes sobre o tópico principal.
	
	Seção 1: Introdução
	A página introduz conceitos fundamentais.
	
	Seção 2: Desenvolvimento
	Explora aspectos mais técnicos e detalhados.
	
	Seção 3: Conclusão
	Apresenta resumo e recomendações.
	
	Palavras-chave: extração, web, conteúdo, análise
	`, w.extractTitle(url), url)
}

func (w *WebExtractorSummarizer) extractTitle(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return "Página: " + parts[len(parts)-1]
	}
	return "Página Web"
}

func (w *WebExtractorSummarizer) extractImages(content string) []string {
	// Simular extração de imagens
	return []string{
		"https://example.com/image1.jpg",
		"https://example.com/image2.png",
	}
}

func (w *WebExtractorSummarizer) extractLinks(content string) []LinkInfo {
	// Simular extração de links
	return []LinkInfo{
		{URL: "https://example.com", Text: "Home", Type: "external"},
		{URL: "https://example.com/about", Text: "Sobre", Type: "internal"},
		{URL: "mailto:contato@example.com", Text: "Contato", Type: "mailto"},
	}
}

func (w *WebExtractorSummarizer) extractMetadata(content string) map[string]string {
	return map[string]string{
		"description": "Descrição da página web",
		"keywords":    "web, extração, análise",
		"author":      "Autor desconhecido",
		"date":        time.Now().Format("2006-01-02"),
	}
}

func (w *WebExtractorSummarizer) generateSummary(content string, style string, maxLen int) string {
	lines := strings.Split(content, "\n")
	summary := ""

	if style == "bullet" {
		summary = "• Informação principal extraída\n• Ponto secundário importante\n• Detalhe relevante"
	} else if style == "abstract" {
		summary = "Resumo acadêmico: O conteúdo apresenta informações essenciais sobre o tema, com análise detalhada e conclusões claras."
	} else {
		// paragraph
		summary = strings.Join(lines[:len(lines)/2], " ")
	}

	if len(summary) > maxLen {
		summary = summary[:maxLen] + "..."
	}

	return summary
}

func (w *WebExtractorSummarizer) extractKeyPoints(content string, count int) []string {
	points := []string{
		"Ponto-chave principal",
		"Insight importante",
		"Detalhe relevante",
		"Fato significativo",
		"Conclusão essencial",
	}

	if count > len(points) {
		count = len(points)
	}

	return points[:count]
}

func (w *WebExtractorSummarizer) extractInsights(content string, category string) []string {
	insights := map[string][]string{
		"technical": {
			"Implementação segue padrões modernos",
			"Arquitetura escalável",
			"Performance otimizada",
		},
		"business": {
			"Proposição de valor clara",
			"Mercado em crescimento",
			"Modelo de negócio sustentável",
		},
		"general": {
			"Informação relevante identificada",
			"Contexto importante",
			"Implicações significativas",
		},
	}

	if vals, ok := insights[category]; ok {
		return vals
	}

	return insights["general"]
}

// ExtractInsightsParams parâmetros para extrair insights
type ExtractInsightsParams struct {
	URL      string `json:"url" description:"URL do conteúdo"`
	Content  string `json:"content" description:"Conteúdo (alternativa a URL)"`
	Category string `json:"category" description:"Categoria (technical, business, general)"`
}

// GetCachedParams parâmetros para obter cache
type GetCachedParams struct {
	URL string `json:"url" description:"URL a buscar"`
}

// ClearCacheParams parâmetros para limpar cache
type ClearCacheParams struct {
	ClearAll bool `json:"clear_all" description:"Limpar tudo ou apenas expirados"`
}
