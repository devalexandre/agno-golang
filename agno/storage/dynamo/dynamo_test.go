package dynamo

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/devalexandre/agno-golang/agno/storage/kvstore"
	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

// fakeDynamo is an in-process fake of the DynamoDB operations the backend uses.
type fakeDynamo struct {
	mu     sync.Mutex
	tables map[string]map[string]map[string]types.AttributeValue
}

func newFake() *fakeDynamo {
	return &fakeDynamo{tables: map[string]map[string]map[string]types.AttributeValue{}}
}

func idOf(it map[string]types.AttributeValue) string {
	return it["id"].(*types.AttributeValueMemberS).Value
}

func (f *fakeDynamo) tbl(name *string) (map[string]map[string]types.AttributeValue, error) {
	t, ok := f.tables[*name]
	if !ok {
		return nil, &types.ResourceNotFoundException{Message: aws.String("no table")}
	}
	return t, nil
}

func (f *fakeDynamo) DescribeTable(_ context.Context, in *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, err := f.tbl(in.TableName); err != nil {
		return nil, err
	}
	return &dynamodb.DescribeTableOutput{Table: &types.TableDescription{TableName: in.TableName, TableStatus: types.TableStatusActive}}, nil
}

func (f *fakeDynamo) CreateTable(_ context.Context, in *dynamodb.CreateTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tables[*in.TableName] = map[string]map[string]types.AttributeValue{}
	return &dynamodb.CreateTableOutput{}, nil
}

func (f *fakeDynamo) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, err := f.tbl(in.TableName)
	if err != nil {
		return nil, err
	}
	id := idOf(in.Item)
	if in.ConditionExpression != nil {
		if _, exists := t[id]; exists {
			return nil, &types.ConditionalCheckFailedException{Message: aws.String("exists")}
		}
	}
	t[id] = in.Item
	return &dynamodb.PutItemOutput{}, nil
}

func (f *fakeDynamo) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, err := f.tbl(in.TableName)
	if err != nil {
		return nil, err
	}
	return &dynamodb.GetItemOutput{Item: t[idOf(in.Key)]}, nil
}

func (f *fakeDynamo) DeleteItem(_ context.Context, in *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, err := f.tbl(in.TableName)
	if err != nil {
		return nil, err
	}
	delete(t, idOf(in.Key))
	return &dynamodb.DeleteItemOutput{}, nil
}

// Scan supports "#a = :b AND ..." filters and pages of 2 items to exercise pagination.
func (f *fakeDynamo) Scan(_ context.Context, in *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, err := f.tbl(in.TableName)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(t))
	for id := range t {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	start := ""
	if in.ExclusiveStartKey != nil {
		start = idOf(in.ExclusiveStartKey)
	}
	out := &dynamodb.ScanOutput{}
	scanned := 0
	for _, id := range ids {
		if id <= start {
			continue
		}
		if scanned == 2 {
			out.LastEvaluatedKey = map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: start}}
			return out, nil
		}
		scanned++
		start = id
		it := t[id]
		match := true
		if in.FilterExpression != nil {
			for _, cond := range strings.Split(*in.FilterExpression, " AND ") {
				parts := strings.Split(cond, " = ")
				attr := in.ExpressionAttributeNames[parts[0]]
				want := in.ExpressionAttributeValues[parts[1]].(*types.AttributeValueMemberS).Value
				got, ok := it[attr].(*types.AttributeValueMemberS)
				if !ok || got.Value != want {
					match = false
				}
			}
		}
		if match {
			out.Items = append(out.Items, it)
		}
	}
	return out, nil
}

func (f *fakeDynamo) BatchWriteItem(_ context.Context, in *dynamodb.BatchWriteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for name, reqs := range in.RequestItems {
		t := f.tables[name]
		for _, r := range reqs {
			if r.DeleteRequest != nil {
				delete(t, idOf(r.DeleteRequest.Key))
			}
		}
	}
	return &dynamodb.BatchWriteItemOutput{}, nil
}

func newFakeStore(t *testing.T) *kvstore.Store {
	t.Helper()
	s, err := New(context.Background(), Config{Client: newFake()})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDynamoConformanceFake(t *testing.T) {
	storagetest.RunDBConformance(t, newFakeStore(t))
	storagetest.RunSessionSearchConformance(t, newFakeStore(t))
	storagetest.RunMetricsConformance(t, newFakeStore(t))
	s := newFakeStore(t)
	_ = s.CreateTables(context.Background())
	storagetest.RunKnowledgeConformance(t, s)
	storagetest.RunStorageConformance(t, newFakeStore(t))
	if err := s.DropTables(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestDynamoRealServer runs against DynamoDB Local when DYNAMODB_ENDPOINT is set:
// docker run -p 8000:8000 amazon/dynamodb-local
// DYNAMODB_ENDPOINT=http://localhost:8000 AWS_REGION=us-east-1 AWS_ACCESS_KEY_ID=x AWS_SECRET_ACCESS_KEY=x
func TestDynamoRealServer(t *testing.T) {
	if os.Getenv("DYNAMODB_ENDPOINT") == "" {
		t.Skip("DYNAMODB_ENDPOINT not set")
	}
	ctx := context.Background()
	s, err := New(ctx, Config{Prefix: "agno_conformance"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTables(ctx); err != nil {
		t.Fatal(err)
	}
	_ = s.DropTables(ctx)
	defer s.DropTables(ctx)
	storagetest.RunDBConformance(t, s)
}
