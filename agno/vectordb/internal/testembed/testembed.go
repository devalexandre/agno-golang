// Package testembed provides a deterministic embedder for vector DB tests:
// identical text always yields the same normalized vector, so search ranking is
// reproducible (unlike the crypto/rand-based MockEmbedder).
package testembed

import (
	"hash/fnv"
	"math"
)

// Deterministic is a hash-seeded embedder. Same text -> same vector.
type Deterministic struct {
	Dim int
}

// New returns a deterministic embedder with the given dimensionality.
func New(dim int) *Deterministic {
	if dim <= 0 {
		dim = 16
	}
	return &Deterministic{Dim: dim}
}

// GetEmbedding implements embedder.Embedder.
func (d *Deterministic) GetEmbedding(text string) ([]float64, error) {
	vec := make([]float64, d.Dim)
	// Seed a simple LCG per dimension from the FNV hash of the text.
	h := fnv.New64a()
	_, _ = h.Write([]byte(text))
	state := h.Sum64() | 1
	var norm float64
	for i := 0; i < d.Dim; i++ {
		state = state*6364136223846793005 + 1442695040888963407
		v := float64(int64(state>>11))/float64(1<<53)*2 - 1 // in [-1, 1]
		vec[i] = v
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec, nil
}

// GetEmbeddingAndUsage implements embedder.Embedder.
func (d *Deterministic) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	v, err := d.GetEmbedding(text)
	return v, nil, err
}

// GetDimensions implements embedder.Embedder.
func (d *Deterministic) GetDimensions() int { return d.Dim }

// GetID implements embedder.Embedder.
func (d *Deterministic) GetID() string { return "deterministic-test" }
