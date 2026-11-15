package reranker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

func sampleResults() []*vectordb.SearchResult {
	return []*vectordb.SearchResult{
		{Document: &document.Document{ID: "a", Content: "apples"}, Score: 0.9},
		{Document: &document.Document{ID: "b", Content: "bananas"}, Score: 0.8},
		{Document: &document.Document{ID: "c", Content: "cherries"}, Score: 0.7},
	}
}

func TestAwsBedrockReranker(t *testing.T) {
	var body map[string]interface{}
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rerank" {
			t.Errorf("path = %s", r.URL.Path)
		}
		auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"results":[{"index":2,"relevanceScore":0.95},{"index":0,"relevanceScore":0.4}]}`))
	}))
	defer srv.Close()

	r := NewAmazonReranker("eu-central-1", 5)
	r.Endpoint = srv.URL
	r.AdditionalModelRequestFields = map[string]interface{}{"max_tokens_per_doc": 512}
	r.Credentials = aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "S"}, nil
	})
	in := sampleResults()
	out, err := r.Rerank(context.Background(), "red fruit", in, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Document.ID != "c" || out[0].Score != 0.95 || out[1].Document.ID != "a" {
		t.Fatalf("unexpected order %+v", out)
	}
	if in[2].Score != 0.7 {
		t.Errorf("input result mutated")
	}
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256") || !strings.Contains(auth, "/eu-central-1/bedrock/aws4_request") {
		t.Errorf("auth = %q", auth)
	}
	cfg := body["rerankingConfiguration"].(map[string]interface{})
	br := cfg["bedrockRerankingConfiguration"].(map[string]interface{})
	mc := br["modelConfiguration"].(map[string]interface{})
	if cfg["type"] != "BEDROCK_RERANKING_MODEL" || br["numberOfResults"] != float64(2) ||
		mc["modelArn"] != "arn:aws:bedrock:eu-central-1::foundation-model/amazon.rerank-v1:0" || mc["additionalModelRequestFields"] == nil {
		t.Errorf("config = %v", cfg)
	}
	q := body["queries"].([]interface{})[0].(map[string]interface{})
	if q["textQuery"].(map[string]interface{})["text"] != "red fruit" {
		t.Errorf("query = %v", q)
	}
	src := body["sources"].([]interface{})
	if len(src) != 3 || src[1].(map[string]interface{})["inlineDocumentSource"].(map[string]interface{})["textDocument"].(map[string]interface{})["text"] != "bananas" {
		t.Errorf("sources = %v", src)
	}
}

func TestAwsBedrockRerankerTooMany(t *testing.T) {
	r := NewCohereBedrockReranker("us-east-1", 0)
	many := make([]*vectordb.SearchResult, bedrockMaxSources+1)
	if _, err := r.Rerank(context.Background(), "q", many, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestSentenceTransformerReranker(t *testing.T) {
	var req teiRerankRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rerank" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_, _ = w.Write([]byte(`[{"index":1,"score":0.99},{"index":0,"score":0.5},{"index":2,"score":0.1}]`))
	}))
	defer srv.Close()
	r := NewSentenceTransformerReranker(srv.URL, "", 2)
	out, err := r.Rerank(context.Background(), "yellow fruit", sampleResults(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if req.Query != "yellow fruit" || len(req.Texts) != 3 || req.Texts[1] != "bananas" || !req.Truncate {
		t.Errorf("request = %+v", req)
	}
	if len(out) != 2 || out[0].Document.ID != "b" || out[1].Document.ID != "a" {
		t.Fatalf("unexpected %+v", out)
	}
	if r.GetID() != "BAAI/bge-reranker-v2-m3" {
		t.Errorf("id = %s", r.GetID())
	}
}
