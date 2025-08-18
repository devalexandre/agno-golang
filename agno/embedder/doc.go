// Package embedder provides interfaces and implementations for embedding systems
package embedder

// Available embedders:
//
// - OpenAIEmbedder: Utiliza a API da OpenAI para gerar embeddings
// - OllamaEmbedder: Utiliza Ollama (local) para gerar embeddings
// - MockEmbedder: Embedder mock para testes
//
// Exemplo de uso:
//
//	// OpenAI Embedder
//	embedder := NewOpenAIEmbedder(
//		WithAPIKey("your-api-key"),
//		WithModel("text-embedding-3-small"),
//	)
//
//	embedding, err := embedder.GetEmbedding("Hello, world!")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Ollama Embedder
//	ollamaEmbedder := NewOllamaEmbedder(
//		WithOllamaHost("http://localhost:11434"),
//		WithOllamaModel("nomic-embed-text", 768),
//	)
//
//	embedding, err = ollamaEmbedder.GetEmbedding("Hello, world!")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Mock Embedder para testes
//	mockEmbedder := NewMockEmbedder(384)
//	embedding, err = mockEmbedder.GetEmbedding("Hello, world!")
//	if err != nil {
//		log.Fatal(err)
//	}
