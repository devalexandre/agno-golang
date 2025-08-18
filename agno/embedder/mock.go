package embedder

import (
	"crypto/rand"
	"fmt"
	"math"
)

// MockEmbedder embedder mock para testes
type MockEmbedder struct {
	BaseEmbedder
	FixedEmbedding []float64
	ShouldError    bool
	ErrorMessage   string
}

// NewMockEmbedder cria um novo embedder mock
func NewMockEmbedder(dimensions int) *MockEmbedder {
	return &MockEmbedder{
		BaseEmbedder: BaseEmbedder{
			ID:         "mock-embedder",
			Dimensions: dimensions,
		},
		ShouldError: false,
	}
}

// WithFixedEmbedding configura um embedding fixo
func (m *MockEmbedder) WithFixedEmbedding(embedding []float64) *MockEmbedder {
	m.FixedEmbedding = embedding
	m.Dimensions = len(embedding)
	return m
}

// WithError configura o mock para retornar erro
func (m *MockEmbedder) WithError(errorMessage string) *MockEmbedder {
	m.ShouldError = true
	m.ErrorMessage = errorMessage
	return m
}

// GetEmbedding gets mock embedding
func (m *MockEmbedder) GetEmbedding(text string) ([]float64, error) {
	if text == "" {
		return nil, ErrEmptyText
	}

	if m.ShouldError {
		if m.ErrorMessage != "" {
			return nil, fmt.Errorf("%s", m.ErrorMessage)
		}
		return nil, ErrInvalidResponse
	}

	if m.FixedEmbedding != nil {
		return m.FixedEmbedding, nil
	}

	// Gerar embedding aleatório normalizado
	embedding := make([]float64, m.Dimensions)
	bytes := make([]byte, m.Dimensions*8)
	rand.Read(bytes)

	var sumSquares float64
	for i := 0; i < m.Dimensions; i++ {
		// Converter bytes para float64 entre -1 e 1
		val := float64(int64(bytes[i*8])<<56|int64(bytes[i*8+1])<<48|int64(bytes[i*8+2])<<40|int64(bytes[i*8+3])<<32|
			int64(bytes[i*8+4])<<24|int64(bytes[i*8+5])<<16|int64(bytes[i*8+6])<<8|int64(bytes[i*8+7])) / math.MaxInt64
		embedding[i] = val
		sumSquares += val * val
	}

	// Normalizar
	norm := math.Sqrt(sumSquares)
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}

// GetEmbeddingAndUsage gets mock embedding and usage information
func (m *MockEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	embedding, err := m.GetEmbedding(text)
	if err != nil {
		return nil, nil, err
	}

	usage := map[string]interface{}{
		"model":        m.ID,
		"dimensions":   len(embedding),
		"input_tokens": len(text) / 4, // Aproximação simples
		"total_tokens": len(text) / 4,
		"mock":         true,
	}

	return embedding, usage, nil
}
