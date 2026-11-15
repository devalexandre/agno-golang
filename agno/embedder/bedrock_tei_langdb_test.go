package embedder

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func fakeAWSConfig(url string) *aws.Config {
	return &aws.Config{
		Region:       "us-west-2",
		BaseEndpoint: aws.String(url),
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET"}, nil
		}),
	}
}

func TestAwsBedrockEmbedderCohereV4(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":{"float":[[0.1,0.2,0.3]],"int8":[[1,2,3]]}}`))
	}))
	defer srv.Close()

	e, err := NewAwsBedrockEmbedder(AwsBedrockOptions{Model: BedrockCohereEmbedV4, OutputDimension: 512, MaxTokens: 1000,
		EmbeddingTypes: []string{"float", "int8"}, Config: fakeAWSConfig(srv.URL)})
	if err != nil {
		t.Fatal(err)
	}
	if e.GetDimensions() != 512 {
		t.Errorf("dims = %d", e.GetDimensions())
	}
	emb, err := e.GetEmbedding("hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(emb) != 3 || emb[0] != 0.1 {
		t.Errorf("embedding = %v", emb)
	}
	if !strings.HasSuffix(gotPath, "/model/cohere.embed-v4:0/invoke") {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256") {
		t.Errorf("request not SigV4-signed: %q", gotAuth)
	}
	if gotBody["input_type"] != "search_query" || gotBody["output_dimension"] != float64(512) || gotBody["max_tokens"] != float64(1000) {
		t.Errorf("body = %v", gotBody)
	}
	if texts, _ := gotBody["texts"].([]interface{}); len(texts) != 1 || texts[0] != "hello" {
		t.Errorf("texts = %v", gotBody["texts"])
	}
}

func TestAwsBedrockEmbedderTitanAndV3Defaults(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = nil
		_ = json.Unmarshal(b, &gotBody)
		if strings.Contains(r.URL.Path, "titan") {
			_, _ = w.Write([]byte(`{"embedding":[1,2],"inputTextTokenCount":4}`))
			return
		}
		_, _ = w.Write([]byte(`{"embeddings":[[5,6]]}`))
	}))
	defer srv.Close()

	titan, err := NewAwsBedrockEmbedder(AwsBedrockOptions{Model: BedrockTitanEmbedTextV2, OutputDimension: 256, Config: fakeAWSConfig(srv.URL)})
	if err != nil {
		t.Fatal(err)
	}
	emb, usage, err := titan.GetEmbeddingAndUsage("hi")
	if err != nil || len(emb) != 2 || usage["input_tokens"] != 4 {
		t.Fatalf("titan: %v %v %v", emb, usage, err)
	}
	if gotBody["inputText"] != "hi" || gotBody["dimensions"] != float64(256) || gotBody["normalize"] != true {
		t.Errorf("titan body = %v", gotBody)
	}

	v3, err := NewAwsBedrockEmbedder(AwsBedrockOptions{Config: fakeAWSConfig(srv.URL), Truncate: "END"})
	if err != nil {
		t.Fatal(err)
	}
	if v3.GetDimensions() != 1024 || v3.GetID() != BedrockCohereEmbedMultilingualV3 {
		t.Errorf("v3 defaults: %d %s", v3.GetDimensions(), v3.GetID())
	}
	if emb, err := v3.GetEmbedding("x"); err != nil || emb[0] != 5 {
		t.Fatalf("v3: %v %v", emb, err)
	}
	if gotBody["truncate"] != "END" {
		t.Errorf("v3 body = %v", gotBody)
	}
	if _, ok := gotBody["output_dimension"]; ok {
		t.Errorf("v3 must not send output_dimension")
	}
}

func TestSentenceTransformerEmbedderTEI(t *testing.T) {
	var gotBody teiEmbedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`[[0.5,0.25]]`))
	}))
	defer srv.Close()
	e := NewSentenceTransformerEmbedder(srv.URL+"/", "", 0)
	e.Prompt = "query: "
	e.NormalizeEmbeddings = true
	emb, err := e.GetEmbedding("cats")
	if err != nil || len(emb) != 2 {
		t.Fatalf("%v %v", emb, err)
	}
	if gotBody.Inputs[0] != "query: cats" || !gotBody.Normalize || !gotBody.Truncate {
		t.Errorf("body = %+v", gotBody)
	}
	if e.GetDimensions() != 384 {
		t.Errorf("dims = %d", e.GetDimensions())
	}
}

func TestLangDBEmbedder(t *testing.T) {
	t.Setenv("LANGDB_PROJECT_ID", "")
	if _, err := NewLangDBEmbedder("k", "", "", 0, ""); err != ErrLangDBProjectMissing {
		t.Fatalf("expected ErrLangDBProjectMissing, got %v", err)
	}
	e, err := NewLangDBEmbedder("k", "proj1", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if e.BaseURL != "https://api.us-east-1.langdb.ai/proj1/v1" || e.GetDimensions() != 1536 || e.Model != "text-embedding-ada-002" {
		t.Errorf("unexpected config %s %d %s", e.BaseURL, e.GetDimensions(), e.Model)
	}
}
