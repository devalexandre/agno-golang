package weaviate

import (
	"context"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/entities/models"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
)

type Weaviate struct {
	*vectordb.BaseVectorDB
	client    *weaviate.Client
	className string
}

type WeaviateConfig struct {
	Host      string
	Scheme    string
	APIKey    string
	ClassName string
	Embedder  embedder.Embedder
	Distance  vectordb.Distance
}

func NewWeaviate(config WeaviateConfig) (*Weaviate, error) {
	cfg := weaviate.Config{
		Host:   config.Host,
		Scheme: config.Scheme,
	}

	if config.APIKey != "" {
		cfg.AuthConfig = auth.ApiKey{Value: config.APIKey}
	}

	c, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to weaviate: %w", err)
	}

	if config.Distance == "" {
		config.Distance = vectordb.DistanceCosine
	}

	return &Weaviate{
		BaseVectorDB: vectordb.NewBaseVectorDB(config.Embedder, vectordb.SearchTypeVector, config.Distance),
		client:       c,
		className:    config.ClassName,
	}, nil
}

func (w *Weaviate) Create(ctx context.Context) error {
	dist := "cosine"
	switch w.Distance {
	case vectordb.DistanceL2:
		dist = "l2-squared"
	case vectordb.DistanceCosine:
		dist = "cosine"
	case vectordb.DistanceDot:
		dist = "dot"
	}

	classObj := &models.Class{
		Class:      w.className,
		Vectorizer: "none",
		VectorConfig: map[string]interface{}{
			"distance": dist,
		},
		Properties: []*models.Property{
			{
				Name:     "content",
				DataType: []string{"text"},
			},
			{
				Name:     "metadata",
				DataType: []string{"text"}, // Storing as JSON string for now
			},
			{
				Name:     "name",
				DataType: []string{"string"},
			},
		},
	}

	return w.client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
}

func (w *Weaviate) Exists(ctx context.Context) (bool, error) {
	return w.client.Schema().ClassExistenceChecker().WithClassName(w.className).Do(ctx)
}

func (w *Weaviate) Drop(ctx context.Context) error {
	return w.client.Schema().ClassDeleter().WithClassName(w.className).Do(ctx)
}

func (w *Weaviate) Optimize(ctx context.Context) error {
	return nil
}

func (w *Weaviate) Insert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	if err := w.EmbedDocuments(documents); err != nil {
		return err
	}

	objects := make([]*models.Object, len(documents))
	for i, doc := range documents {
		v32 := make([]float32, len(doc.Embeddings))
		for j, v := range doc.Embeddings {
			v32[j] = float32(v)
		}

		objects[i] = &models.Object{
			Class: w.className,
			ID:    strToUUID(doc.ID),
			Properties: map[string]interface{}{
				"content":  doc.Content,
				"metadata": doc.Metadata, // Weaviate client handles maps in properties
				"name":     doc.Name,
			},
			Vector: v32,
		}
	}

	_, err := w.client.Batch().ObjectsBatcher().WithObjects(objects...).Do(ctx)
	return err
}

func (w *Weaviate) Upsert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	// Simple implementation: Batch insert handles upsert if ID is provided in some cases,
	// but strictly we should check or use the specific upsert logic if available.
	return w.Insert(ctx, documents, filters)
}

func (w *Weaviate) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return w.VectorSearch(ctx, query, limit, filters)
}

func (w *Weaviate) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	vector, err := w.EmbedQuery(query)
	if err != nil {
		return nil, err
	}

	v32 := make([]float32, len(vector))
	for i, v := range vector {
		v32[i] = float32(v)
	}

	result, err := w.client.GraphQL().Get().
		WithClassName(w.className).
		WithFields(graphql.Field{Name: "content"}, graphql.Field{Name: "metadata"}, graphql.Field{Name: "name"}, graphql.Field{Name: "_additional", Fields: []graphql.Field{{Name: "distance"}, {Name: "id"}}}).
		WithNearVector(w.client.GraphQL().NearVectorArgBuilder().WithVector(v32)).
		WithLimit(limit).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	return w.parseGraphQLResponse(result)
}

func (w *Weaviate) parseGraphQLResponse(res *models.GraphQLResponse) ([]*vectordb.SearchResult, error) {
	if res.Errors != nil {
		return nil, fmt.Errorf("graphql error: %v", res.Errors)
	}

	data := res.Data["Get"].(map[string]interface{})[w.className].([]interface{})
	results := make([]*vectordb.SearchResult, 0, len(data))

	for _, item := range data {
		m := item.(map[string]interface{})
		additional := m["_additional"].(map[string]interface{})

		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{
				ID:      additional["id"].(string),
				Content: m["content"].(string),
				Name:    m["name"].(string),
			},
			Score:    1.0 - additional["distance"].(float64), // Convert distance to score
			Distance: additional["distance"].(float64),
		})
	}

	return results, nil
}

func (w *Weaviate) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("keyword search not implemented for weaviate")
}

func (w *Weaviate) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("hybrid search not implemented for weaviate")
}

func (w *Weaviate) GetCount(ctx context.Context) (int64, error) {
	result, err := w.client.GraphQL().Aggregate().
		WithClassName(w.className).
		WithFields(graphql.Field{Name: "meta", Fields: []graphql.Field{{Name: "count"}}}).
		Do(ctx)
	if err != nil {
		return 0, err
	}

	// Parsing aggregate response is nested
	return 0, fmt.Errorf("aggregate parsing not implemented")
}

func (w *Weaviate) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return w.IDExists(ctx, doc.ID)
}

func (w *Weaviate) NameExists(ctx context.Context, name string) (bool, error) {
	return false, nil
}

func (w *Weaviate) IDExists(ctx context.Context, id string) (bool, error) {
	return w.client.Data().Checker().WithClassName(w.className).WithID(strToUUID(id)).Do(ctx)
}

func strToUUID(id string) string {
	// Weaviate requires UUIDs. If the provided ID is not a UUID, we should hash it or similar.
	// For this implementation, we assume it's already a UUID or handled by the user.
	return id
}
