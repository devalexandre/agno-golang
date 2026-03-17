package milvus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type Milvus struct {
	*vectordb.BaseVectorDB
	client         client.Client
	collectionName string
	dbName         string
}

type MilvusConfig struct {
	Address        string
	Username       string
	Password       string
	CollectionName string
	DBName         string
	Embedder       embedder.Embedder
	Distance       vectordb.Distance
}

func NewMilvus(config MilvusConfig) (*Milvus, error) {
	ctx := context.Background()
	c, err := client.NewClient(ctx, client.Config{
		Address:  config.Address,
		Username: config.Username,
		Password: config.Password,
		DBName:   config.DBName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to milvus: %w", err)
	}

	if config.Distance == "" {
		config.Distance = vectordb.DistanceL2
	}

	return &Milvus{
		BaseVectorDB:   vectordb.NewBaseVectorDB(config.Embedder, vectordb.SearchTypeVector, config.Distance),
		client:         c,
		collectionName: config.CollectionName,
		dbName:         config.DBName,
	}, nil
}

func (m *Milvus) Create(ctx context.Context) error {
	schema := &entity.Schema{
		CollectionName: m.collectionName,
		AutoID:         false,
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				AutoID:     false,
				TypeParams: map[string]string{"max_length": "36"},
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", m.Dimensions),
				},
			},
			{
				Name:     "content",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "65535",
				},
			},
			{
				Name:     "metadata",
				DataType: entity.FieldTypeJSON,
			},
		},
	}

	var metricType entity.MetricType
	switch m.Distance {
	case vectordb.DistanceL2:
		metricType = entity.L2
	case vectordb.DistanceCosine:
		metricType = entity.COSINE
	case vectordb.DistanceIP:
		metricType = entity.IP
	default:
		metricType = entity.L2
	}

	err := m.client.CreateCollection(ctx, schema, 1)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	idx, err := entity.NewIndexIvfFlat(metricType, 1024)
	if err != nil {
		return fmt.Errorf("failed to create index description: %w", err)
	}

	err = m.client.CreateIndex(ctx, m.collectionName, "vector", idx, false)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return m.client.LoadCollection(ctx, m.collectionName, false)
}

func (m *Milvus) Exists(ctx context.Context) (bool, error) {
	return m.client.HasCollection(ctx, m.collectionName)
}

func (m *Milvus) Drop(ctx context.Context) error {
	return m.client.DropCollection(ctx, m.collectionName)
}

func (m *Milvus) Optimize(ctx context.Context) error {
	return m.client.Flush(ctx, m.collectionName, false)
}

func (m *Milvus) Insert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	if err := m.EmbedDocuments(documents); err != nil {
		return err
	}

	ids := make([]string, 0, len(documents))
	vectors := make([][]float32, 0, len(documents))
	contents := make([]string, 0, len(documents))
	metadatas := make([]string, 0, len(documents))

	for _, doc := range documents {
		ids = append(ids, doc.ID)
		v32 := make([]float32, len(doc.Embeddings))
		for i, v := range doc.Embeddings {
			v32[i] = float32(v)
		}
		vectors = append(vectors, v32)
		contents = append(contents, doc.Content)
		metaJSON, _ := json.Marshal(doc.Metadata)
		metadatas = append(metadatas, string(metaJSON))
	}

	idCol := entity.NewColumnVarChar("id", ids)
	vectorCol := entity.NewColumnFloatVector("vector", m.Dimensions, vectors)
	contentCol := entity.NewColumnVarChar("content", contents)
	metadataCol := entity.NewColumnVarChar("metadata", metadatas)

	_, err := m.client.Insert(ctx, m.collectionName, "", idCol, vectorCol, contentCol, metadataCol)
	return err
}

func (m *Milvus) Upsert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	// Milvus upsert requires a primary key, but since we handle IDs manually, we can use it.
	if err := m.EmbedDocuments(documents); err != nil {
		return err
	}

	ids := make([]string, 0, len(documents))
	vectors := make([][]float32, 0, len(documents))
	contents := make([]string, 0, len(documents))
	metadatas := make([]string, 0, len(documents))

	for _, doc := range documents {
		ids = append(ids, doc.ID)
		v32 := make([]float32, len(doc.Embeddings))
		for i, v := range doc.Embeddings {
			v32[i] = float32(v)
		}
		vectors = append(vectors, v32)
		contents = append(contents, doc.Content)
		metaJSON, _ := json.Marshal(doc.Metadata)
		metadatas = append(metadatas, string(metaJSON))
	}

	idCol := entity.NewColumnVarChar("id", ids)
	vectorCol := entity.NewColumnFloatVector("vector", m.Dimensions, vectors)
	contentCol := entity.NewColumnVarChar("content", contents)
	metadataCol := entity.NewColumnVarChar("metadata", metadatas)

	_, err := m.client.Upsert(ctx, m.collectionName, "", idCol, vectorCol, contentCol, metadataCol)
	return err
}

func (m *Milvus) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return m.VectorSearch(ctx, query, limit, filters)
}

func (m *Milvus) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	vector, err := m.EmbedQuery(query)
	if err != nil {
		return nil, err
	}

	v32 := make([]float32, len(vector))
	for i, v := range vector {
		v32[i] = float32(v)
	}

	var metricType entity.MetricType
	switch m.Distance {
	case vectordb.DistanceL2:
		metricType = entity.L2
	case vectordb.DistanceCosine:
		metricType = entity.COSINE
	default:
		metricType = entity.L2
	}

	searchParam, _ := entity.NewIndexIvfFlatSearchParam(10)
	res, err := m.client.Search(ctx, m.collectionName, nil, "", []string{"content", "metadata"}, []entity.Vector{entity.FloatVector(v32)}, "vector", metricType, limit, searchParam)
	if err != nil {
		return nil, err
	}

	results := make([]*vectordb.SearchResult, 0)
	for _, sr := range res {
		for i := 0; i < sr.ResultCount; i++ {
			id, _ := sr.IDs.GetAsString(i)
			content, _ := sr.Fields.GetColumn("content").GetAsString(i)
			// metadata handling would be more complex depending on how it's stored

			results = append(results, &vectordb.SearchResult{
				Document: &document.Document{
					ID:      id,
					Content: content,
				},
				Score: float64(sr.Scores[i]),
			})
		}
	}

	return results, nil
}

func (m *Milvus) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("keyword search not implemented for milvus")
}

func (m *Milvus) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("hybrid search not implemented for milvus")
}

func (m *Milvus) GetCount(ctx context.Context) (int64, error) {
	// Milvus doesn't have a direct "count" in describe, usually requires query or statistics
	return 0, fmt.Errorf("get count not directly supported in milvus without query")
}

func (m *Milvus) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return m.IDExists(ctx, doc.ID)
}

func (m *Milvus) NameExists(ctx context.Context, name string) (bool, error) {
	// In Milvus we'd query by a 'name' metadata field if it exists
	return false, nil
}

func (m *Milvus) IDExists(ctx context.Context, id string) (bool, error) {
	res, err := m.client.Query(ctx, m.collectionName, nil, fmt.Sprintf("id == '%s'", id), []string{"id"})
	if err != nil {
		return false, err
	}
	return res.GetColumn("id").Len() > 0, nil
}

func (m *Milvus) Close() error {
	return m.client.Close()
}
