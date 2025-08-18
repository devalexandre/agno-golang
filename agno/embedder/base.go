package embedder

// Embedder interface para gerenciar embedders
type Embedder interface {
	// GetEmbedding gets embedding for a text
	GetEmbedding(text string) ([]float64, error)

	// GetEmbeddingAndUsage gets embedding and usage information
	GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error)

	// GetDimensions returns the number of embedding dimensions
	GetDimensions() int

	// GetID retorna o ID/nome do modelo
	GetID() string
}

// BaseEmbedder base implementation for embedders
type BaseEmbedder struct {
	ID         string
	Dimensions int
}

// GetDimensions implementa Embedder
func (b *BaseEmbedder) GetDimensions() int {
	if b.Dimensions <= 0 {
		return 1536 // default dimension
	}
	return b.Dimensions
}

// GetID implementa Embedder
func (b *BaseEmbedder) GetID() string {
	return b.ID
}

// GetEmbedding implementa Embedder (deve ser sobrescrito)
func (b *BaseEmbedder) GetEmbedding(text string) ([]float64, error) {
	return nil, ErrNotImplemented
}

// GetEmbeddingAndUsage implements Embedder (should be overridden)
func (b *BaseEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	embedding, err := b.GetEmbedding(text)
	return embedding, nil, err
}
