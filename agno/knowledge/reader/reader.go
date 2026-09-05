// Package reader define a interface canônica para leitores de documentos
// (knowledge readers). Espelha o contrato de agno-python/knowledge/reader.
//
// Cada formato/origem (PDF, CSV, Markdown, DOCX, Website, YouTube, etc.)
// implementa a interface Reader, produzindo uma lista de *document.Document
// pronta para chunking, embedding e indexação no VectorDB.
//
// Esta é uma interface da Fase 0 do PLANO_PARIDADE_PYTHON.md — implementações
// concretas são entregues na Fase 3 (Knowledge / RAG completo).
package reader

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/document"
)

// Reader é o contrato canônico para qualquer leitor de conteúdo.
type Reader interface {
	// Read lê a origem (caminho de arquivo, URL, query, etc.) e retorna
	// os documentos extraídos. A interpretação de `source` é específica
	// de cada implementação.
	Read(ctx context.Context, source string) ([]*document.Document, error)

	// Name retorna o identificador do reader (ex.: "pdf", "csv", "website").
	Name() string

	// SupportedExtensions retorna as extensões/esquemas suportados
	// (ex.: [".pdf"], [".csv", ".tsv"], ["http", "https"]). Pode ser vazio
	// para readers baseados em query (ex.: arxiv, wikipedia).
	SupportedExtensions() []string
}

// BaseReader fornece implementação default reutilizável por readers concretos.
type BaseReader struct {
	ID         string
	Extensions []string
	// ChunkSize/Overlap são dicas opcionais para o pipeline de chunking.
	ChunkSize int
	Overlap   int
}

// Name implementa Reader.
func (b *BaseReader) Name() string { return b.ID }

// SupportedExtensions implementa Reader.
func (b *BaseReader) SupportedExtensions() []string { return b.Extensions }
