// Package mongo provides a MongoDB-backed implementation of the storage
// contracts (storage.DB, storage.Storage, storage.KnowledgeStorage, session
// search and metrics), a port of Python agno/db/mongo.
//
// Each kvstore collection maps to the MongoDB collection
// "{prefix}_{collection}". Documents are stored natively under "data" (so
// they can be queried with regular MongoDB tools) with "_id" set to the
// record id and indexed fields copied to top-level "idx_<field>" keys.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	mongodrv "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/devalexandre/agno-golang/agno/storage/kvstore"
)

// Config configures the MongoDB backend.
type Config struct {
	// URI is a mongodb:// URI. Defaults to MONGODB_URI, then mongodb://localhost:27017.
	URI string
	// Client is an existing client; when set URI is ignored.
	Client *mongodrv.Client
	// Database is the database name (default "agno").
	Database string
	// Prefix is the collection name prefix (default "agno").
	Prefix string
}

// Backend is a kvstore.Backend over MongoDB.
type Backend struct {
	db     *mongodrv.Database
	prefix string
}

// NewBackend connects to MongoDB and creates a backend.
func NewBackend(ctx context.Context, cfg Config) (*Backend, error) {
	client := cfg.Client
	if client == nil {
		uri := cfg.URI
		if uri == "" {
			uri = os.Getenv("MONGODB_URI")
		}
		if uri == "" {
			uri = "mongodb://localhost:27017"
		}
		c, err := mongodrv.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			return nil, fmt.Errorf("mongo: connect: %w", err)
		}
		client = c
	}
	dbName := cfg.Database
	if dbName == "" {
		dbName = "agno"
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "agno"
	}
	return &Backend{db: client.Database(dbName), prefix: prefix}, nil
}

// New creates a MongoDB-backed store implementing every storage contract.
func New(ctx context.Context, cfg Config, opts ...kvstore.Option) (*kvstore.Store, error) {
	b, err := NewBackend(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return kvstore.New(b, append([]kvstore.Option{kvstore.WithID("mongo")}, opts...)...), nil
}

// Database returns the underlying database handle.
func (b *Backend) Database() *mongodrv.Database { return b.db }

func (b *Backend) coll(name string) *mongodrv.Collection {
	return b.db.Collection(b.prefix + "_" + name)
}

// Init pings the server and creates the index-field indexes.
func (b *Backend) Init(ctx context.Context) error {
	if err := b.db.Client().Ping(ctx, nil); err != nil {
		return fmt.Errorf("mongo: ping: %w", err)
	}
	indexes := map[string][]string{
		kvstore.CollSessions:         {"idx_user_id", "idx_agent_id"},
		kvstore.CollRuns:             {"idx_session_id", "idx_user_id"},
		kvstore.CollComponentMetrics: {"idx_date"},
	}
	for coll, fields := range indexes {
		models := make([]mongodrv.IndexModel, 0, len(fields))
		for _, f := range fields {
			models = append(models, mongodrv.IndexModel{Keys: bson.D{{Key: f, Value: 1}}})
		}
		if _, err := b.coll(coll).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("mongo: create indexes: %w", err)
		}
	}
	return nil
}

func toDocument(id string, doc []byte, index map[string]string) (bson.D, error) {
	var data bson.D
	if err := bson.UnmarshalExtJSON(doc, false, &data); err != nil {
		return nil, fmt.Errorf("mongo: convert document: %w", err)
	}
	out := bson.D{{Key: "_id", Value: id}, {Key: "data", Value: data}}
	for k, v := range index {
		out = append(out, bson.E{Key: "idx_" + k, Value: v})
	}
	return out, nil
}

func fromRaw(raw bson.Raw) ([]byte, error) {
	val, err := raw.LookupErr("data")
	if err != nil {
		return nil, fmt.Errorf("mongo: document without data: %w", err)
	}
	return bson.MarshalExtJSON(val.Document(), false, false)
}

// Put upserts a document.
func (b *Backend) Put(ctx context.Context, coll, id string, doc []byte, index map[string]string) error {
	d, err := toDocument(id, doc, index)
	if err != nil {
		return err
	}
	_, err = b.coll(coll).ReplaceOne(ctx, bson.M{"_id": id}, d, options.Replace().SetUpsert(true))
	return err
}

// Insert writes a document only if absent.
func (b *Backend) Insert(ctx context.Context, coll, id string, doc []byte, index map[string]string) (bool, error) {
	d, err := toDocument(id, doc, index)
	if err != nil {
		return false, err
	}
	_, err = b.coll(coll).InsertOne(ctx, d)
	if mongodrv.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

// Get reads a document.
func (b *Backend) Get(ctx context.Context, coll, id string) ([]byte, error) {
	raw, err := b.coll(coll).FindOne(ctx, bson.M{"_id": id}).Raw()
	if errors.Is(err, mongodrv.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return fromRaw(raw)
}

// Delete removes a document.
func (b *Backend) Delete(ctx context.Context, coll, id string) error {
	_, err := b.coll(coll).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// List returns documents matching the equality filters.
func (b *Backend) List(ctx context.Context, coll string, where map[string]string) ([][]byte, error) {
	filter := bson.M{}
	for k, v := range where {
		filter["idx_"+k] = v
	}
	cur, err := b.coll(coll).Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out [][]byte
	for cur.Next(ctx) {
		doc, err := fromRaw(cur.Current)
		if err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	return out, cur.Err()
}

// Drop drops a collection.
func (b *Backend) Drop(ctx context.Context, coll string) error {
	return b.coll(coll).Drop(ctx)
}

var (
	_ kvstore.Backend  = (*Backend)(nil)
	_ kvstore.Inserter = (*Backend)(nil)
)
