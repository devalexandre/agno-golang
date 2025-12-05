package tools

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// ContextAwareMemoryManager gerencia memória com contexto
type ContextAwareMemoryManager struct {
	toolkit.Toolkit
	memories       map[string]MemoryEntry
	contextIndex   map[string][]string // context -> memory IDs
	accessPatterns map[string]AccessPattern
	relevanceCache map[string][]string
	mu             sync.RWMutex
}

// MemoryEntry representa uma entrada de memória
type MemoryEntry struct {
	ID           string            `json:"id"`
	Content      string            `json:"content"`
	Context      map[string]string `json:"context"`
	Tags         []string          `json:"tags"`
	CreatedAt    time.Time         `json:"created_at"`
	LastAccessed time.Time         `json:"last_accessed"`
	AccessCount  int64             `json:"access_count"`
	Relevance    float64           `json:"relevance"`
	TTL          int               `json:"ttl_seconds"` // 0 = sem expiração
	Priority     int               `json:"priority"`    // 0-10
	Embeddings   []float64         `json:"embeddings"`  // Para similarity search
}

// AccessPattern rastreia padrões de acesso
type AccessPattern struct {
	MemoryID    string
	AccessTimes []time.Time
	Sources     []string
	Contexts    []string
}

// StoreMemoryParams parâmetros para armazenar memória
type StoreMemoryParams struct {
	Content  string            `json:"content" description:"Conteúdo a armazenar"`
	Context  map[string]string `json:"context" description:"Contexto (ex: user_id, domain)"`
	Tags     []string          `json:"tags" description:"Tags para categorização"`
	TTL      int               `json:"ttl" description:"Time to live em segundos (0 = sem expiração)"`
	Priority int               `json:"priority" description:"Prioridade 0-10"`
}

// RetrieveMemoryParams parâmetros para recuperar memória
type RetrieveMemoryParams struct {
	MemoryID   string   `json:"memory_id" description:"ID da memória"`
	ContextKey string   `json:"context_key" description:"Chave de contexto"`
	ContextVal string   `json:"context_value" description:"Valor de contexto"`
	Tags       []string `json:"tags" description:"Filtrar por tags"`
	Limit      int      `json:"limit" description:"Número máximo de resultados"`
	OrderBy    string   `json:"order_by" description:"Ordenar por (relevance, recency, access_count)"`
}

// RelevantMemoriesParams parâmetros para buscar memórias relevantes
type RelevantMemoriesParams struct {
	Query        string            `json:"query" description:"Query para buscar"`
	Context      map[string]string `json:"context" description:"Contexto atual"`
	TopK         int               `json:"top_k" description:"Número de resultados"`
	MinRelevance float64           `json:"min_relevance" description:"Relevância mínima (0-1)"`
}

// MemoryResult resultado de operação de memória
type MemoryResult struct {
	Success   bool          `json:"success"`
	MemoryID  string        `json:"memory_id,omitempty"`
	Memories  []MemoryEntry `json:"memories,omitempty"`
	Count     int           `json:"count"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
}

// PruneParams parâmetros para limpar memórias expiradas
type PruneParams struct {
	RemoveExpired      bool    `json:"remove_expired" description:"Remover entradas expiradas"`
	RemoveLowRelevance bool    `json:"remove_low_relevance" description:"Remover baixa relevância"`
	MinRelevance       float64 `json:"min_relevance" description:"Relevância mínima para manter"`
}

// NewContextAwareMemoryManager cria novo manager
func NewContextAwareMemoryManager() *ContextAwareMemoryManager {
	m := &ContextAwareMemoryManager{
		memories:       make(map[string]MemoryEntry),
		contextIndex:   make(map[string][]string),
		accessPatterns: make(map[string]AccessPattern),
		relevanceCache: make(map[string][]string),
	}
	m.Toolkit = toolkit.NewToolkit()

	m.Toolkit.Register(
		"StoreMemory",
		"Armazenar nova memória com contexto",
		m,
		m.StoreMemory,
		StoreMemoryParams{},
	)

	m.Toolkit.Register(
		"RetrieveMemory",
		"Recuperar memória por ID ou contexto",
		m,
		m.RetrieveMemory,
		RetrieveMemoryParams{},
	)

	m.Toolkit.Register(
		"FindRelevantMemories",
		"Buscar memórias relevantes para um query",
		m,
		m.FindRelevantMemories,
		RelevantMemoriesParams{},
	)

	m.Toolkit.Register(
		"UpdateMemoryRelevance",
		"Atualizar relevância de uma memória",
		m,
		m.UpdateMemoryRelevance,
		UpdateRelevanceParams{},
	)

	m.Toolkit.Register(
		"PruneMemories",
		"Limpar memórias expiradas ou irrelevantes",
		m,
		m.PruneMemories,
		PruneParams{},
	)

	return m
}

// StoreMemory armazena nova memória
func (m *ContextAwareMemoryManager) StoreMemory(params StoreMemoryParams) (interface{}, error) {
	if params.Content == "" {
		return MemoryResult{Success: false}, fmt.Errorf("content obrigatório")
	}

	// Gerar ID único
	hash := md5.Sum([]byte(fmt.Sprintf("%s-%d", params.Content, time.Now().UnixNano())))
	memoryID := hex.EncodeToString(hash[:])[:12]

	entry := MemoryEntry{
		ID:           memoryID,
		Content:      params.Content,
		Context:      params.Context,
		Tags:         params.Tags,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		AccessCount:  1,
		Relevance:    1.0,
		TTL:          params.TTL,
		Priority:     params.Priority,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.memories[memoryID] = entry

	// Indexar por contexto
	for key, value := range params.Context {
		contextKey := fmt.Sprintf("%s:%s", key, value)
		m.contextIndex[contextKey] = append(m.contextIndex[contextKey], memoryID)
	}

	return MemoryResult{
		Success:   true,
		MemoryID:  memoryID,
		Message:   fmt.Sprintf("Memória %s armazenada com sucesso", memoryID),
		Timestamp: time.Now(),
	}, nil
}

// RetrieveMemory recupera memória por ID ou contexto
func (m *ContextAwareMemoryManager) RetrieveMemory(params RetrieveMemoryParams) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := MemoryResult{
		Success:   true,
		Memories:  make([]MemoryEntry, 0),
		Timestamp: time.Now(),
	}

	// Se ID específico
	if params.MemoryID != "" {
		if entry, exists := m.memories[params.MemoryID]; exists {
			m.recordAccess(params.MemoryID)
			result.Memories = append(result.Memories, entry)
			result.Count = 1
			return result, nil
		}
		return result, fmt.Errorf("memória não encontrada")
	}

	// Buscar por contexto
	if params.ContextKey != "" && params.ContextVal != "" {
		contextKey := fmt.Sprintf("%s:%s", params.ContextKey, params.ContextVal)
		if memoryIDs, exists := m.contextIndex[contextKey]; exists {
			for _, id := range memoryIDs {
				if entry, ok := m.memories[id]; ok {
					// Filtrar por tags se especificado
					if len(params.Tags) > 0 && !m.hasAllTags(entry, params.Tags) {
						continue
					}
					result.Memories = append(result.Memories, entry)
				}
			}
		}
	}

	// Limitar resultados
	if params.Limit > 0 && len(result.Memories) > params.Limit {
		result.Memories = result.Memories[:params.Limit]
	}

	result.Count = len(result.Memories)
	return result, nil
}

// FindRelevantMemories busca memórias relevantes para um query
func (m *ContextAwareMemoryManager) FindRelevantMemories(params RelevantMemoriesParams) (interface{}, error) {
	if params.Query == "" {
		return MemoryResult{Success: false}, fmt.Errorf("query obrigatório")
	}

	if params.TopK <= 0 {
		params.TopK = 10
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Buscar memórias relevantes (similarity search simples)
	candidates := make([]struct {
		entry MemoryEntry
		score float64
	}, 0)

	queryWords := strings.Fields(strings.ToLower(params.Query))

	for _, entry := range m.memories {
		// Pular se expirada
		if entry.TTL > 0 && time.Since(entry.CreatedAt).Seconds() > float64(entry.TTL) {
			continue
		}

		// Calcular score
		score := m.calculateSimilarity(queryWords, entry)

		// Considerar contexto
		for key, val := range params.Context {
			if ctxVal, exists := entry.Context[key]; exists && ctxVal == val {
				score += 0.2
			}
		}

		// Aplicar relevância armazenada
		score *= entry.Relevance

		if score >= params.MinRelevance {
			candidates = append(candidates, struct {
				entry MemoryEntry
				score float64
			}{entry, score})
		}
	}

	// Ordenar por score (bubble sort)
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Retornar top K
	result := MemoryResult{
		Success:   true,
		Memories:  make([]MemoryEntry, 0),
		Timestamp: time.Now(),
	}

	for i := 0; i < len(candidates) && i < params.TopK; i++ {
		result.Memories = append(result.Memories, candidates[i].entry)
	}

	result.Count = len(result.Memories)
	return result, nil
}

// UpdateMemoryRelevance atualiza relevância de uma memória
func (m *ContextAwareMemoryManager) UpdateMemoryRelevance(params UpdateRelevanceParams) (interface{}, error) {
	if params.MemoryID == "" {
		return MemoryResult{Success: false}, fmt.Errorf("memory_id obrigatório")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.memories[params.MemoryID]; exists {
		entry.Relevance = params.Relevance
		if entry.Relevance > 1.0 {
			entry.Relevance = 1.0
		}
		if entry.Relevance < 0.0 {
			entry.Relevance = 0.0
		}

		m.memories[params.MemoryID] = entry

		return MemoryResult{
			Success:   true,
			MemoryID:  params.MemoryID,
			Message:   fmt.Sprintf("Relevância atualizada para %.2f", params.Relevance),
			Timestamp: time.Now(),
		}, nil
	}

	return MemoryResult{Success: false}, fmt.Errorf("memória não encontrada")
}

// PruneMemories limpa memórias expiradas ou irrelevantes
func (m *ContextAwareMemoryManager) PruneMemories(params PruneParams) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	removed := 0
	now := time.Now()

	idsToRemove := make([]string, 0)

	for id, entry := range m.memories {
		// Remover expiradas
		if params.RemoveExpired && entry.TTL > 0 {
			if now.Sub(entry.CreatedAt).Seconds() > float64(entry.TTL) {
				idsToRemove = append(idsToRemove, id)
				removed++
				continue
			}
		}

		// Remover baixa relevância
		if params.RemoveLowRelevance && entry.Relevance < params.MinRelevance {
			idsToRemove = append(idsToRemove, id)
			removed++
		}
	}

	// Remover entradas
	for _, id := range idsToRemove {
		delete(m.memories, id)
	}

	return map[string]interface{}{
		"success":            true,
		"removed_count":      removed,
		"remaining_memories": len(m.memories),
		"timestamp":          now,
	}, nil
}

// Helper functions

func (m *ContextAwareMemoryManager) recordAccess(memoryID string) {
	if entry, exists := m.memories[memoryID]; exists {
		entry.LastAccessed = time.Now()
		entry.AccessCount++
		m.memories[memoryID] = entry
	}
}

func (m *ContextAwareMemoryManager) hasAllTags(entry MemoryEntry, tags []string) bool {
	for _, tag := range tags {
		found := false
		for _, t := range entry.Tags {
			if t == tag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (m *ContextAwareMemoryManager) calculateSimilarity(queryWords []string, entry MemoryEntry) float64 {
	contentWords := strings.Fields(strings.ToLower(entry.Content))
	matches := 0

	for _, qword := range queryWords {
		for _, cword := range contentWords {
			if strings.Contains(cword, qword) {
				matches++
			}
		}
	}

	if len(queryWords) == 0 {
		return 0.0
	}

	// Pontuação com bônus por prioridade
	baseScore := float64(matches) / float64(len(queryWords))
	priorityBonus := float64(entry.Priority) * 0.01
	recencyBonus := 0.0

	daysSinceAccess := time.Since(entry.LastAccessed).Hours() / 24
	if daysSinceAccess < 1 {
		recencyBonus = 0.2
	} else if daysSinceAccess < 7 {
		recencyBonus = 0.1
	}

	return baseScore + priorityBonus + recencyBonus
}

// UpdateRelevanceParams parâmetros para atualizar relevância
type UpdateRelevanceParams struct {
	MemoryID  string  `json:"memory_id" description:"ID da memória"`
	Relevance float64 `json:"relevance" description:"Nova relevância (0-1)"`
}
