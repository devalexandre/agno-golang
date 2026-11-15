// Package mongodb implements the vectordb.VectorDB interface for MongoDB using
// go.mongodb.org/mongo-driver. Vector search uses the Atlas `$vectorSearch`
// aggregation stage, which requires MongoDB Atlas (or a local Atlas deployment)
// with a vector search index named by Options.VectorIndex.
package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// MongoDB implements vectordb.VectorDB backed by MongoDB Atlas Vector Search.
type MongoDB struct {
	*vectordb.BaseVectorDB
	client      *mongo.Client
	coll        *mongo.Collection
	vectorIndex string
	numCandidatesMul int
}

// Options configures a MongoDB.
type Options struct {
	URI         string // mongodb+srv://... (Atlas)
	Client      *mongo.Client
	Database    string
	Collection  string
	VectorIndex string // name of the Atlas vector search index (defaults to "vector_index")
	Embedder    embedder.Embedder
}

// New creates a MongoDB. Provide either URI or an existing Client.
func New(ctx context.Context, opts Options) (*MongoDB, error) {
	if opts.Database == "" {
		opts.Database = "agno"
	}
	if opts.Collection == "" {
		opts.Collection = "documents"
	}
	if opts.VectorIndex == "" {
		opts.VectorIndex = "vector_index"
	}
	client := opts.Client
	if client == nil {
		if opts.URI == "" {
			return nil, fmt.Errorf("mongodb: URI or Client is required")
		}
		var err error
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(opts.URI))
		if err != nil {
			return nil, fmt.Errorf("mongodb: connect: %w", err)
		}
	}
	return &MongoDB{
		BaseVectorDB:     vectordb.NewBaseVectorDB(opts.Embedder, vectordb.SearchTypeVector, vectordb.DistanceCosine),
		client:           client,
		coll:             client.Database(opts.Database).Collection(opts.Collection),
		vectorIndex:      opts.VectorIndex,
		numCandidatesMul: 10,
	}, nil
}

// Create ensures the collection exists (MongoDB creates it lazily on first write).
// The Atlas vector search index must be created out-of-band (UI/API/CLI).
func (m *MongoDB) Create(ctx context.Context) error {
	return m.client.Database(m.coll.Database().Name()).CreateCollection(ctx, m.coll.Name())
}

// Exists reports whether the collection exists.
func (m *MongoDB) Exists(ctx context.Context) (bool, error) {
	names, err := m.coll.Database().ListCollectionNames(ctx, bson.M{"name": m.coll.Name()})
	if err != nil {
		return false, err
	}
	return len(names) > 0, nil
}

// Drop removes the collection.
func (m *MongoDB) Drop(ctx context.Context) error {
	return m.coll.Drop(ctx)
}

// Optimize is a no-op for MongoDB.
func (m *MongoDB) Optimize(ctx context.Context) error { return nil }

type mongoDoc struct {
	ID        string    `bson:"_id"`
	Content   string    `bson:"content"`
	Name      string    `bson:"name"`
	Source    string    `bson:"source"`
	Embedding []float64 `bson:"embedding"`
}

// Insert embeds and inserts documents.
func (m *MongoDB) Insert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := m.EmbedDocuments(docs); err != nil {
		return err
	}
	models := make([]interface{}, 0, len(docs))
	for _, d := range docs {
		models = append(models, mongoDoc{
			ID: d.ID, Content: d.Content, Name: d.Name, Source: d.Source, Embedding: d.Embeddings,
		})
	}
	_, err := m.coll.InsertMany(ctx, models)
	return err
}

// Upsert embeds and upserts documents by _id.
func (m *MongoDB) Upsert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := m.EmbedDocuments(docs); err != nil {
		return err
	}
	for _, d := range docs {
		_, err := m.coll.UpdateByID(ctx, d.ID, bson.M{"$set": mongoDoc{
			ID: d.ID, Content: d.Content, Name: d.Name, Source: d.Source, Embedding: d.Embeddings,
		}}, options.Update().SetUpsert(true))
		if err != nil {
			return err
		}
	}
	return nil
}

// Search performs a vector search.
func (m *MongoDB) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return m.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch runs an Atlas $vectorSearch aggregation.
func (m *MongoDB) VectorSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	vec, err := m.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$vectorSearch", Value: bson.M{
			"index":         m.vectorIndex,
			"path":          "embedding",
			"queryVector":   vec,
			"numCandidates": limit * m.numCandidatesMul,
			"limit":         limit,
		}}},
		bson.D{{Key: "$project", Value: bson.M{
			"content": 1, "name": 1, "source": 1,
			"score": bson.M{"$meta": "vectorSearchScore"},
		}}},
	}

	cur, err := m.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var results []*vectordb.SearchResult
	for cur.Next(ctx) {
		var row struct {
			ID      string  `bson:"_id"`
			Content string  `bson:"content"`
			Name    string  `bson:"name"`
			Source  string  `bson:"source"`
			Score   float64 `bson:"score"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}
		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{ID: row.ID, Content: row.Content, Name: row.Name, Source: row.Source},
			Score:    row.Score,
		})
	}
	return results, cur.Err()
}

// KeywordSearch runs a regex match on content (no Atlas Search dependency).
func (m *MongoDB) KeywordSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	cur, err := m.coll.Find(ctx, bson.M{"content": bson.M{"$regex": query, "$options": "i"}},
		options.Find().SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var results []*vectordb.SearchResult
	for cur.Next(ctx) {
		var d mongoDoc
		if err := cur.Decode(&d); err != nil {
			return nil, err
		}
		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{ID: d.ID, Content: d.Content, Name: d.Name, Source: d.Source},
		})
	}
	return results, cur.Err()
}

// HybridSearch is not implemented.
func (m *MongoDB) HybridSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("mongodb: hybrid search not implemented")
}

// GetCount returns the number of documents.
func (m *MongoDB) GetCount(ctx context.Context) (int64, error) {
	return m.coll.CountDocuments(ctx, bson.M{})
}

// DocExists checks existence by the document's ID.
func (m *MongoDB) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return m.IDExists(ctx, doc.ID)
}

// NameExists checks whether a document with the given name exists.
func (m *MongoDB) NameExists(ctx context.Context, name string) (bool, error) {
	n, err := m.coll.CountDocuments(ctx, bson.M{"name": name})
	return n > 0, err
}

// IDExists checks whether a document with the given _id exists.
func (m *MongoDB) IDExists(ctx context.Context, id string) (bool, error) {
	n, err := m.coll.CountDocuments(ctx, bson.M{"_id": id})
	return n > 0, err
}

// Close disconnects the client.
func (m *MongoDB) Close(ctx context.Context) error { return m.client.Disconnect(ctx) }

var _ vectordb.VectorDB = (*MongoDB)(nil)
