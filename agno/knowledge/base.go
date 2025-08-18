package knowledge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// Knowledge é a interface base para bases de conhecimento
type Knowledge interface {
	// Load carrega documentos na base de conhecimento
	Load(ctx context.Context, recreate bool) error

	// LoadDocument carrega um documento específico
	LoadDocument(ctx context.Context, doc document.Document) error

	// Search busca documentos na base de conhecimento
	Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*SearchResult, error)

	// Drop remove todos os documentos da base
	Drop(ctx context.Context) error

	// Exists verifica se a base de conhecimento existe
	Exists(ctx context.Context) (bool, error)

	// GetCount retorna o número de documentos na base
	GetCount(ctx context.Context) (int64, error)

	// GetInfo retorna informações sobre a base de conhecimento
	GetInfo() KnowledgeInfo
}

// VectorDB é uma alias para vectordb.VectorDB para compatibilidade nativa como no Agno Python
type VectorDB = vectordb.VectorDB

// SearchResult é uma alias para vectordb.SearchResult para compatibilidade nativa
type SearchResult = vectordb.SearchResult

// KnowledgeInfo contém informações sobre a base de conhecimento
type KnowledgeInfo struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SearchFilters define filtros para busca
type SearchFilters struct {
	Include map[string]interface{} `json:"include,omitempty"`
	Exclude map[string]interface{} `json:"exclude,omitempty"`
}

// BaseKnowledge implementação base para bases de conhecimento
type BaseKnowledge struct {
	Name         string
	VectorDB     VectorDB
	Embedder     embedder.Embedder
	NumDocuments int
	Filters      *SearchFilters
	Recreate     bool
	Metadata     map[string]interface{}
}

// NewBaseKnowledge cria uma nova instância de BaseKnowledge
func NewBaseKnowledge(name string, vectorDB VectorDB) *BaseKnowledge {
	return &BaseKnowledge{
		Name:         name,
		VectorDB:     vectorDB,
		NumDocuments: 5,
		Metadata:     make(map[string]interface{}),
	}
}

// GetInfo retorna informações sobre a base de conhecimento
func (k *BaseKnowledge) GetInfo() KnowledgeInfo {
	return KnowledgeInfo{
		Name:        k.Name,
		Type:        "base",
		Description: fmt.Sprintf("Base knowledge: %s", k.Name),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    k.Metadata,
	}
}

// SearchDocuments busca documentos com filtros
func (k *BaseKnowledge) SearchDocuments(ctx context.Context, query string, numDocuments int, filters map[string]interface{}) ([]document.Document, error) {
	if k.VectorDB == nil {
		return nil, fmt.Errorf("vector database not configured")
	}

	results, err := k.VectorDB.Search(ctx, query, numDocuments, filters)
	if err != nil {
		return nil, err
	}

	// Convert SearchResult to Document slice
	docs := make([]document.Document, len(results))
	for i, result := range results {
		docs[i] = *result.Document
	}

	return docs, nil
}

// LoadDocuments carrega documentos na base de conhecimento
func (k *BaseKnowledge) LoadDocuments(ctx context.Context, docs []document.Document, recreate bool) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	// Verificar se deve recriar
	if recreate {
		if err := k.VectorDB.Drop(ctx); err != nil {
			// Ignorar erro se não existir
		}
	}

	// Criar tabela se não existir
	if err := k.VectorDB.Create(ctx); err != nil {
		return fmt.Errorf("failed to create vector database: %w", err)
	}

	// Verificar se já possui documentos
	if !recreate {
		if c, err := k.VectorDB.GetCount(ctx); err == nil && c > 0 {
			return nil // Já tem documentos
		}
	}

	// Inserir documentos
	if len(docs) > 0 {
		// Convert []document.Document to []*document.Document
		docPtrs := make([]*document.Document, len(docs))
		for i := range docs {
			docPtrs[i] = &docs[i]
		}
		return k.VectorDB.Insert(ctx, docPtrs, nil)
	}

	return nil
}

// Search implementa Knowledge interface
func (k *BaseKnowledge) Search(ctx context.Context, query string, numDocuments int) ([]document.Document, error) {
	if numDocuments <= 0 {
		numDocuments = k.NumDocuments
	}

	var filters map[string]interface{}
	if k.Filters != nil {
		filters = k.Filters.Include
	}

	return k.SearchDocuments(ctx, query, numDocuments, filters)
}

// Add adiciona documentos à base de conhecimento
func (k *BaseKnowledge) Add(ctx context.Context, documents []document.Document) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	// Convert []document.Document to []*document.Document
	docPtrs := make([]*document.Document, len(documents))
	for i := range documents {
		docPtrs[i] = &documents[i]
	}

	return k.VectorDB.Insert(ctx, docPtrs, nil)
}

// Exists verifica se a base de conhecimento existe
func (k *BaseKnowledge) Exists(ctx context.Context) (bool, error) {
	if k.VectorDB == nil {
		return false, fmt.Errorf("vector database not configured")
	}

	return k.VectorDB.Exists(ctx)
}

// Drop remove a base de conhecimento
func (k *BaseKnowledge) Drop(ctx context.Context) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	return k.VectorDB.Drop(ctx)
}

// Load implementa Knowledge interface
func (k *BaseKnowledge) Load(ctx context.Context, recreate bool) error {
	// Implementação padrão vazia - deve ser sobrescrita pelas subclasses
	return nil
}

// SetEmbedder configura o embedder
func (k *BaseKnowledge) SetEmbedder(emb embedder.Embedder) {
	k.Embedder = emb
	// Note: VectorDB implementation should handle embedder internally
}

// GetEmbedder retorna o embedder configurado
func (k *BaseKnowledge) GetEmbedder() embedder.Embedder {
	if k.Embedder != nil {
		return k.Embedder
	}
	if k.VectorDB != nil {
		return k.VectorDB.GetEmbedder()
	}
	return nil
}

// ValidateDocuments valida documentos antes de processar
func ValidateDocuments(docs []document.Document) error {
	if len(docs) == 0 {
		return fmt.Errorf("no documents to process")
	}

	for i, doc := range docs {
		if doc.ID == "" {
			return fmt.Errorf("document at index %d has empty ID", i)
		}
		if doc.Content == "" {
			return fmt.Errorf("document at index %d has empty content", i)
		}
	}

	return nil
}

// SanitizeFileName sanitiza nome de arquivo para usar como nome de coleção/tabela
func SanitizeFileName(filename string) string {
	// Remover extensão
	name := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	// Substituir caracteres especiais por underscore
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ".", "_")

	// Garantir que começa com letra
	if len(name) > 0 && !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) {
		name = "kb_" + name
	}

	// Converter para minúsculas
	return strings.ToLower(name)
}
