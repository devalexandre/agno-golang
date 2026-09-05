// Package reranker define a interface canônica para rerankers de resultados
// de busca. Espelha o contrato de agno-python/knowledge/reranker.
//
// Um Reranker recebe os resultados de uma busca no VectorDB e os reordena
// por relevância usando um modelo dedicado (ex.: Cohere Rerank, AWS Bedrock,
// cross-encoder/Infinity), melhorando a precisão do top-N do RAG.
//
// Esta é uma interface da Fase 0 do PLANO_PARIDADE_PYTHON.md — implementações
// concretas (cohere, aws_bedrock, infinity) são entregues na Fase 3.
package reranker

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// Reranker é o contrato canônico para reordenação de resultados de busca.
type Reranker interface {
	// Rerank reordena os resultados pela relevância em relação à query e
	// retorna no máximo topN itens. Se topN <= 0, retorna todos reordenados.
	Rerank(ctx context.Context, query string, results []*vectordb.SearchResult, topN int) ([]*vectordb.SearchResult, error)

	// GetID retorna o identificador do reranker (ex.: "cohere-rerank-v3").
	GetID() string
}

// BaseReranker fornece implementação default reutilizável.
type BaseReranker struct {
	ID    string
	TopN  int
	Model string
}

// GetID implementa Reranker.
func (b *BaseReranker) GetID() string {
	if b.ID != "" {
		return b.ID
	}
	return b.Model
}
