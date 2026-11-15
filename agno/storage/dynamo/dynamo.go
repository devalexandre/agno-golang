// Package dynamo provides an Amazon DynamoDB-backed implementation of the
// storage contracts (storage.DB, storage.Storage, storage.KnowledgeStorage,
// session search and metrics), a port of Python agno/db/dynamo.
//
// Each kvstore collection maps to the table "{prefix}_{collection}" with the
// string hash key "id". Items hold the JSON document in "data" and indexed
// fields as "idx_<field>" attributes used in Scan filter expressions. Tables
// are created on demand (on-demand billing) by CreateTables.
package dynamo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/devalexandre/agno-golang/agno/storage/kvstore"
)

// API is the subset of the DynamoDB client used by the backend.
type API interface {
	DescribeTable(ctx context.Context, in *dynamodb.DescribeTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	CreateTable(ctx context.Context, in *dynamodb.CreateTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error)
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	DeleteItem(ctx context.Context, in *dynamodb.DeleteItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Scan(ctx context.Context, in *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	BatchWriteItem(ctx context.Context, in *dynamodb.BatchWriteItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
}

// Config configures the DynamoDB backend.
type Config struct {
	// Client is an existing client; when nil one is built from the default
	// AWS config (AWS_REGION, credentials chain). DYNAMODB_ENDPOINT
	// overrides the endpoint (e.g. http://localhost:8000 for DynamoDB Local).
	Client API
	// Region overrides the AWS region.
	Region string
	// Endpoint overrides the service endpoint.
	Endpoint string
	// Prefix is the table name prefix (default "agno").
	Prefix string
}

// Backend is a kvstore.Backend over DynamoDB.
type Backend struct {
	client API
	prefix string
}

// NewBackend creates a DynamoDB backend.
func NewBackend(ctx context.Context, cfg Config) (*Backend, error) {
	client := cfg.Client
	if client == nil {
		var loadOpts []func(*config.LoadOptions) error
		if cfg.Region != "" {
			loadOpts = append(loadOpts, config.WithRegion(cfg.Region))
		}
		awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
		if err != nil {
			return nil, fmt.Errorf("dynamo: load aws config: %w", err)
		}
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = os.Getenv("DYNAMODB_ENDPOINT")
		}
		client = dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
			if endpoint != "" {
				o.BaseEndpoint = aws.String(endpoint)
			}
		})
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "agno"
	}
	return &Backend{client: client, prefix: prefix}, nil
}

// New creates a DynamoDB-backed store implementing every storage contract.
func New(ctx context.Context, cfg Config, opts ...kvstore.Option) (*kvstore.Store, error) {
	b, err := NewBackend(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return kvstore.New(b, append([]kvstore.Option{kvstore.WithID("dynamo")}, opts...)...), nil
}

func (b *Backend) table(coll string) *string { return aws.String(b.prefix + "_" + coll) }

func key(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}}
}

// Init creates every missing table and waits until they are active.
func (b *Backend) Init(ctx context.Context) error {
	for _, coll := range kvstore.AllCollections {
		_, err := b.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: b.table(coll)})
		if err == nil {
			continue
		}
		var nf *types.ResourceNotFoundException
		if !errors.As(err, &nf) {
			return fmt.Errorf("dynamo: describe %s: %w", coll, err)
		}
		_, err = b.client.CreateTable(ctx, &dynamodb.CreateTableInput{
			TableName:            b.table(coll),
			BillingMode:          types.BillingModePayPerRequest,
			AttributeDefinitions: []types.AttributeDefinition{{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS}},
			KeySchema:            []types.KeySchemaElement{{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash}},
		})
		var inUse *types.ResourceInUseException
		if err != nil && !errors.As(err, &inUse) {
			return fmt.Errorf("dynamo: create %s: %w", coll, err)
		}
	}
	if c, ok := b.client.(dynamodb.DescribeTableAPIClient); ok {
		w := dynamodb.NewTableExistsWaiter(c)
		for _, coll := range kvstore.AllCollections {
			if err := w.Wait(ctx, &dynamodb.DescribeTableInput{TableName: b.table(coll)}, 2*time.Minute); err != nil {
				return fmt.Errorf("dynamo: wait %s: %w", coll, err)
			}
		}
	}
	return nil
}

func item(id string, doc []byte, index map[string]string) map[string]types.AttributeValue {
	it := key(id)
	it["data"] = &types.AttributeValueMemberS{Value: string(doc)}
	for k, v := range index {
		if v != "" { // DynamoDB rejects empty strings in some contexts; absent == ""
			it["idx_"+k] = &types.AttributeValueMemberS{Value: v}
		}
	}
	return it
}

// Put writes a document.
func (b *Backend) Put(ctx context.Context, coll, id string, doc []byte, index map[string]string) error {
	_, err := b.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: b.table(coll), Item: item(id, doc, index)})
	return err
}

// Insert writes a document only if absent.
func (b *Backend) Insert(ctx context.Context, coll, id string, doc []byte, index map[string]string) (bool, error) {
	_, err := b.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           b.table(coll),
		Item:                item(id, doc, index),
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	})
	var ccf *types.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return false, nil
	}
	return err == nil, err
}

func dataOf(it map[string]types.AttributeValue) []byte {
	if s, ok := it["data"].(*types.AttributeValueMemberS); ok {
		return []byte(s.Value)
	}
	return nil
}

// Get reads a document.
func (b *Backend) Get(ctx context.Context, coll, id string) ([]byte, error) {
	out, err := b.client.GetItem(ctx, &dynamodb.GetItemInput{TableName: b.table(coll), Key: key(id), ConsistentRead: aws.Bool(true)})
	if err != nil {
		var nf *types.ResourceNotFoundException
		if errors.As(err, &nf) {
			return nil, nil
		}
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	return dataOf(out.Item), nil
}

// Delete removes a document.
func (b *Backend) Delete(ctx context.Context, coll, id string) error {
	_, err := b.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: b.table(coll), Key: key(id)})
	return err
}

// ScanInput builds the Scan request for where.
func (b *Backend) ScanInput(coll string, where map[string]string) *dynamodb.ScanInput {
	in := &dynamodb.ScanInput{TableName: b.table(coll), ConsistentRead: aws.Bool(true)}
	if len(where) == 0 {
		return in
	}
	names := map[string]string{}
	values := map[string]types.AttributeValue{}
	var conds []string
	i := 0
	for k, v := range where {
		n, p := fmt.Sprintf("#f%d", i), fmt.Sprintf(":v%d", i)
		names[n] = "idx_" + k
		values[p] = &types.AttributeValueMemberS{Value: v}
		conds = append(conds, n+" = "+p)
		i++
	}
	in.FilterExpression = aws.String(strings.Join(conds, " AND "))
	in.ExpressionAttributeNames = names
	in.ExpressionAttributeValues = values
	return in
}

func (b *Backend) scan(ctx context.Context, in *dynamodb.ScanInput, fn func(map[string]types.AttributeValue) error) error {
	for {
		out, err := b.client.Scan(ctx, in)
		if err != nil {
			var nf *types.ResourceNotFoundException
			if errors.As(err, &nf) {
				return nil
			}
			return err
		}
		for _, it := range out.Items {
			if err := fn(it); err != nil {
				return err
			}
		}
		if len(out.LastEvaluatedKey) == 0 {
			return nil
		}
		in.ExclusiveStartKey = out.LastEvaluatedKey
	}
}

// List scans a table with equality filters.
func (b *Backend) List(ctx context.Context, coll string, where map[string]string) ([][]byte, error) {
	var out [][]byte
	err := b.scan(ctx, b.ScanInput(coll, where), func(it map[string]types.AttributeValue) error {
		out = append(out, dataOf(it))
		return nil
	})
	return out, err
}

// Drop deletes every item of a table (the table itself is kept).
func (b *Backend) Drop(ctx context.Context, coll string) error {
	var batch []types.WriteRequest
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		req := map[string][]types.WriteRequest{*b.table(coll): batch}
		for len(req) > 0 {
			out, err := b.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{RequestItems: req})
			if err != nil {
				return err
			}
			req = out.UnprocessedItems
		}
		batch = nil
		return nil
	}
	in := &dynamodb.ScanInput{TableName: b.table(coll), ProjectionExpression: aws.String("id")}
	err := b.scan(ctx, in, func(it map[string]types.AttributeValue) error {
		batch = append(batch, types.WriteRequest{DeleteRequest: &types.DeleteRequest{Key: map[string]types.AttributeValue{"id": it["id"]}}})
		if len(batch) == 25 {
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	return flush()
}

var (
	_ kvstore.Backend  = (*Backend)(nil)
	_ kvstore.Inserter = (*Backend)(nil)
	_ API              = (*dynamodb.Client)(nil)
)
