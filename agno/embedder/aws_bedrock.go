package embedder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// Bedrock embedding model IDs.
const (
	BedrockCohereEmbedMultilingualV3 = "cohere.embed-multilingual-v3"
	BedrockCohereEmbedEnglishV3      = "cohere.embed-english-v3"
	BedrockCohereEmbedV4             = "cohere.embed-v4:0"
	BedrockTitanEmbedTextV2          = "amazon.titan-embed-text-v2:0"
	BedrockTitanEmbedTextV1          = "amazon.titan-embed-text-v1"
)

// BedrockInvoker is the subset of the bedrockruntime client used by
// AwsBedrockEmbedder (satisfied by *bedrockruntime.Client).
type BedrockInvoker interface {
	InvokeModel(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
}

// AwsBedrockOptions configures an AwsBedrockEmbedder.
type AwsBedrockOptions struct {
	// Model defaults to cohere.embed-multilingual-v3 (as in Agno Python).
	// Cohere v3/v4 and Amazon Titan text embedding models are supported.
	Model string
	// InputType for Cohere models: search_document, search_query (default),
	// classification or clustering.
	InputType string
	// Truncate for Cohere models (v3: NONE|START|END, v4: NONE|LEFT|RIGHT).
	Truncate string
	// EmbeddingTypes for Cohere models (float, int8, ...). The float
	// embedding is preferred when several are returned.
	EmbeddingTypes []string
	// OutputDimension for Cohere v4 (256, 512, 1024, 1536) and Titan v2
	// (256, 512, 1024).
	OutputDimension int
	// MaxTokens for Cohere v4.
	MaxTokens int
	// Normalize for Titan v2 (default true when nil).
	Normalize *bool
	// RequestParams are merged into the JSON request body.
	RequestParams map[string]interface{}

	// Region, AccessKeyID, SecretAccessKey and SessionToken fall back to the
	// AWS_* environment variables and then the default AWS credential chain.
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	// Config, when set, is used as-is instead of loading the default config.
	Config *aws.Config
	// Client overrides the bedrockruntime client (useful for tests).
	Client BedrockInvoker
}

// AwsBedrockEmbedder embeds text with AWS Bedrock (InvokeModel), mirroring
// Agno Python's AwsBedrockEmbedder (Cohere Embed v3/v4) and additionally
// supporting Amazon Titan text embeddings.
type AwsBedrockEmbedder struct {
	BaseEmbedder
	opts   AwsBedrockOptions
	client BedrockInvoker
}

// NewAwsBedrockEmbedder creates a Bedrock embedder. Credentials are resolved
// from opts, the environment, then the default AWS chain (SSO, profile, IAM).
func NewAwsBedrockEmbedder(opts AwsBedrockOptions) (*AwsBedrockEmbedder, error) {
	if opts.Model == "" {
		opts.Model = BedrockCohereEmbedMultilingualV3
	}
	if opts.InputType == "" {
		opts.InputType = "search_query"
	}
	e := &AwsBedrockEmbedder{opts: opts}
	e.ID = opts.Model
	e.Dimensions = e.defaultDimensions()

	if opts.Client != nil {
		e.client = opts.Client
		return e, nil
	}
	cfg, err := loadAWSConfig(opts)
	if err != nil {
		return nil, err
	}
	e.client = bedrockruntime.NewFromConfig(cfg)
	return e, nil
}

func loadAWSConfig(opts AwsBedrockOptions) (aws.Config, error) {
	if opts.Config != nil {
		return *opts.Config, nil
	}
	region := firstNonEmpty(opts.Region, os.Getenv("AWS_REGION"), os.Getenv("AWS_DEFAULT_REGION"))
	var lo []func(*awsconfig.LoadOptions) error
	if region != "" {
		lo = append(lo, awsconfig.WithRegion(region))
	}
	ak := firstNonEmpty(opts.AccessKeyID, os.Getenv("AWS_ACCESS_KEY_ID"))
	sk := firstNonEmpty(opts.SecretAccessKey, os.Getenv("AWS_SECRET_ACCESS_KEY"))
	if ak != "" && sk != "" {
		st := firstNonEmpty(opts.SessionToken, os.Getenv("AWS_SESSION_TOKEN"))
		lo = append(lo, awsconfig.WithCredentialsProvider(aws.CredentialsProviderFunc(
			func(context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: ak, SecretAccessKey: sk, SessionToken: st, Source: "agno"}, nil
			})))
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), lo...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("aws bedrock embedder: load AWS config: %w", err)
	}
	return cfg, nil
}

func (e *AwsBedrockEmbedder) isV4() bool {
	return strings.Contains(strings.ToLower(e.opts.Model), "embed-v4")
}
func (e *AwsBedrockEmbedder) isTitan() bool {
	return strings.HasPrefix(strings.ToLower(e.opts.Model), "amazon.titan-embed")
}

func (e *AwsBedrockEmbedder) defaultDimensions() int {
	switch {
	case e.isV4():
		if e.opts.OutputDimension > 0 {
			return e.opts.OutputDimension
		}
		return 1536
	case e.isTitan():
		if strings.Contains(e.opts.Model, "v2") {
			if e.opts.OutputDimension > 0 {
				return e.opts.OutputDimension
			}
			return 1024
		}
		return 1536
	default:
		return 1024
	}
}

func (e *AwsBedrockEmbedder) requestBody(text string) ([]byte, error) {
	body := map[string]interface{}{}
	if e.isTitan() {
		body["inputText"] = text
		if strings.Contains(e.opts.Model, "v2") {
			if e.opts.OutputDimension > 0 {
				body["dimensions"] = e.opts.OutputDimension
			}
			normalize := true
			if e.opts.Normalize != nil {
				normalize = *e.opts.Normalize
			}
			body["normalize"] = normalize
		}
	} else {
		body["texts"] = []string{text}
		body["input_type"] = e.opts.InputType
		if e.opts.Truncate != "" {
			body["truncate"] = e.opts.Truncate
		}
		if len(e.opts.EmbeddingTypes) > 0 {
			body["embedding_types"] = e.opts.EmbeddingTypes
		}
		if e.isV4() {
			if e.opts.OutputDimension > 0 {
				body["output_dimension"] = e.opts.OutputDimension
			}
			if e.opts.MaxTokens > 0 {
				body["max_tokens"] = e.opts.MaxTokens
			}
		}
	}
	for k, v := range e.opts.RequestParams {
		body[k] = v
	}
	return json.Marshal(body)
}

// extractBedrockEmbedding handles Titan ({"embedding": [...]}) and Cohere
// ({"embeddings": [[...]]} or {"embeddings": {"float": [[...]]}}) responses.
func extractBedrockEmbedding(raw []byte) ([]float64, map[string]interface{}, error) {
	var resp struct {
		Embedding           []float64       `json:"embedding"`
		Embeddings          json.RawMessage `json:"embeddings"`
		InputTextTokenCount int             `json:"inputTextTokenCount"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, fmt.Errorf("aws bedrock embedder: decode response: %w", err)
	}
	usage := map[string]interface{}{}
	if resp.InputTextTokenCount > 0 {
		usage["input_tokens"] = resp.InputTextTokenCount
	}
	if len(resp.Embedding) > 0 {
		return resp.Embedding, usage, nil
	}
	if len(resp.Embeddings) == 0 {
		return nil, usage, ErrInvalidResponse
	}
	var list [][]float64
	if json.Unmarshal(resp.Embeddings, &list) == nil {
		if len(list) == 0 {
			return nil, usage, ErrInvalidResponse
		}
		return list[0], usage, nil
	}
	var byType map[string][][]float64
	if err := json.Unmarshal(resp.Embeddings, &byType); err != nil {
		return nil, usage, fmt.Errorf("aws bedrock embedder: unexpected embeddings format: %w", err)
	}
	if f := byType["float"]; len(f) > 0 {
		return f[0], usage, nil
	}
	for _, v := range byType {
		if len(v) > 0 {
			return v[0], usage, nil
		}
	}
	return nil, usage, ErrInvalidResponse
}

func (e *AwsBedrockEmbedder) embed(ctx context.Context, text string) ([]float64, map[string]interface{}, error) {
	if text == "" {
		return nil, nil, ErrEmptyText
	}
	if e.client == nil {
		return nil, nil, errors.New("aws bedrock embedder: client not initialised")
	}
	body, err := e.requestBody(text)
	if err != nil {
		return nil, nil, err
	}
	out, err := e.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(e.opts.Model),
		Body:        body,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("aws bedrock embedder (%s): %w", e.opts.Model, err)
	}
	emb, usage, err := extractBedrockEmbedding(out.Body)
	if usage != nil {
		usage["model"] = e.opts.Model
	}
	return emb, usage, err
}

// GetEmbedding implements Embedder.
func (e *AwsBedrockEmbedder) GetEmbedding(text string) ([]float64, error) {
	emb, _, err := e.embed(context.Background(), text)
	return emb, err
}

// GetEmbeddingAndUsage implements Embedder.
func (e *AwsBedrockEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	return e.embed(context.Background(), text)
}

// GetEmbeddingWithContext embeds text honouring ctx.
func (e *AwsBedrockEmbedder) GetEmbeddingWithContext(ctx context.Context, text string) ([]float64, error) {
	emb, _, err := e.embed(ctx, text)
	return emb, err
}

var _ Embedder = (*AwsBedrockEmbedder)(nil)
