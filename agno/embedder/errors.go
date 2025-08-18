package embedder

import "errors"

// Erros comuns do sistema de embedder
var (
	ErrNotImplemented   = errors.New("method not implemented")
	ErrInvalidDimension = errors.New("invalid embedding dimension")
	ErrEmptyText        = errors.New("text cannot be empty")
	ErrAPIKeyMissing    = errors.New("API key is required")
	ErrInvalidResponse  = errors.New("invalid response from embedding service")
)
