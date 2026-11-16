package reranker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// Bedrock reranking model IDs.
const (
	AmazonRerankV1  = "amazon.rerank-v1:0"
	CohereRerankV35 = "cohere.rerank-v3-5:0"
)

// bedrockMaxSources is the Bedrock Rerank API limit per request.
const bedrockMaxSources = 1000

// AwsBedrockReranker reorders results with the AWS Bedrock Rerank API
// (bedrock-agent-runtime), supporting Amazon Rerank 1.0 and Cohere Rerank 3.5,
// mirroring Agno Python's AwsBedrockReranker.
//
// It calls the REST endpoint POST https://bedrock-agent-runtime.{region}.amazonaws.com/rerank
// signed with SigV4 from the AWS SDK, so no extra SDK module is needed.
// Credentials come from the default AWS chain (env vars, profile, SSO, IAM).
type AwsBedrockReranker struct {
	BaseReranker
	Region string
	// AdditionalModelRequestFields are passed as
	// modelConfiguration.additionalModelRequestFields.
	AdditionalModelRequestFields map[string]interface{}
	// Endpoint overrides the regional endpoint (tests, VPC endpoints).
	Endpoint   string
	HTTPClient *http.Client
	// Credentials override the default AWS credential chain.
	Credentials aws.CredentialsProvider
}

// NewAwsBedrockReranker creates a Bedrock reranker. model defaults to
// cohere.rerank-v3-5:0; region falls back to AWS_REGION, then us-west-2.
// Note: Amazon Rerank 1.0 is not available in us-east-1.
func NewAwsBedrockReranker(model, region string, topN int) *AwsBedrockReranker {
	if model == "" {
		model = CohereRerankV35
	}
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-west-2"
	}
	return &AwsBedrockReranker{
		BaseReranker: BaseReranker{ID: model, Model: model, TopN: topN},
		Region:       region,
		HTTPClient:   &http.Client{Timeout: 60 * time.Second},
	}
}

// NewCohereBedrockReranker is a Bedrock reranker preconfigured with Cohere Rerank 3.5.
func NewCohereBedrockReranker(region string, topN int) *AwsBedrockReranker {
	return NewAwsBedrockReranker(CohereRerankV35, region, topN)
}

// NewAmazonReranker is a Bedrock reranker preconfigured with Amazon Rerank 1.0.
func NewAmazonReranker(region string, topN int) *AwsBedrockReranker {
	return NewAwsBedrockReranker(AmazonRerankV1, region, topN)
}

// ModelARN returns the foundation-model ARN used in the request.
func (r *AwsBedrockReranker) ModelARN() string {
	if strings.HasPrefix(r.Model, "arn:") {
		return r.Model
	}
	return fmt.Sprintf("arn:aws:bedrock:%s::foundation-model/%s", r.Region, r.Model)
}

func (r *AwsBedrockReranker) buildRequest(query string, docs []string, topN int) map[string]interface{} {
	sources := make([]map[string]interface{}, len(docs))
	for i, d := range docs {
		sources[i] = map[string]interface{}{
			"type": "INLINE",
			"inlineDocumentSource": map[string]interface{}{
				"type":         "TEXT",
				"textDocument": map[string]interface{}{"text": d},
			},
		}
	}
	modelCfg := map[string]interface{}{"modelArn": r.ModelARN()}
	if len(r.AdditionalModelRequestFields) > 0 {
		modelCfg["additionalModelRequestFields"] = r.AdditionalModelRequestFields
	}
	brCfg := map[string]interface{}{"modelConfiguration": modelCfg}
	if topN > 0 {
		brCfg["numberOfResults"] = topN
	}
	return map[string]interface{}{
		"queries": []map[string]interface{}{{"type": "TEXT", "textQuery": map[string]interface{}{"text": query}}},
		"sources": sources,
		"rerankingConfiguration": map[string]interface{}{
			"type":                          "BEDROCK_RERANKING_MODEL",
			"bedrockRerankingConfiguration": brCfg,
		},
	}
}

func (r *AwsBedrockReranker) credentials(ctx context.Context) (aws.Credentials, error) {
	prov := r.Credentials
	if prov == nil {
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(r.Region))
		if err != nil {
			return aws.Credentials{}, fmt.Errorf("aws bedrock reranker: load AWS config: %w", err)
		}
		if cfg.Credentials == nil {
			return aws.Credentials{}, fmt.Errorf("aws bedrock reranker: no AWS credentials found")
		}
		r.Credentials = cfg.Credentials
		prov = cfg.Credentials
	}
	return prov.Retrieve(ctx)
}

type bedrockRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevanceScore"`
	} `json:"results"`
}

// Rerank implements Reranker.
func (r *AwsBedrockReranker) Rerank(ctx context.Context, query string, results []*vectordb.SearchResult, limit int) ([]*vectordb.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}
	if len(results) > bedrockMaxSources {
		return nil, fmt.Errorf("%w: aws bedrock reranker accepts at most %d documents, got %d", ErrInvalidInput, bedrockMaxSources, len(results))
	}
	topN := effectiveLimit(limit, r.TopN)
	if topN > len(results) {
		topN = len(results)
	}
	docs := make([]string, len(results))
	for i, res := range results {
		if res != nil && res.Document != nil {
			docs[i] = res.Document.Content
		}
	}
	payload, err := json.Marshal(r.buildRequest(query, docs, topN))
	if err != nil {
		return nil, err
	}
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://bedrock-agent-runtime.%s.amazonaws.com", r.Region)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/rerank", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	creds, err := r.credentials(ctx)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(payload)
	if err := v4.NewSigner().SignHTTP(ctx, creds, req, hex.EncodeToString(sum[:]), "bedrock", r.Region, time.Now()); err != nil {
		return nil, fmt.Errorf("aws bedrock reranker: sign request: %w", err)
	}

	client := r.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aws bedrock reranker: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out bedrockRerankResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	reranked := make([]*vectordb.SearchResult, 0, len(out.Results))
	for _, item := range out.Results {
		if item.Index < 0 || item.Index >= len(results) || results[item.Index] == nil {
			continue
		}
		c := copyResult(results[item.Index])
		c.Score = item.RelevanceScore
		reranked = append(reranked, c)
	}
	sort.SliceStable(reranked, func(i, j int) bool { return reranked[i].Score > reranked[j].Score })
	if topN > 0 && len(reranked) > topN {
		reranked = reranked[:topN]
	}
	return reranked, nil
}

var _ Reranker = (*AwsBedrockReranker)(nil)
