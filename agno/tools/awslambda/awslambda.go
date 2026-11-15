// Package awslambda ports Agno's AWSLambdaTools: listing and invoking AWS
// Lambda functions through the Lambda REST API (2015-03-31), signed with
// AWS Signature Version 4.
package awslambda

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultRegion matches the Python toolkit's region_name default.
const DefaultRegion = "us-east-1"

// AWSLambdaTools lists and invokes Lambda functions.
type AWSLambdaTools struct {
	toolkit.Toolkit
	region       string
	baseURL      string
	creds        aws.CredentialsProvider
	enableList   bool
	enableInvoke bool
	all          bool
	httpClient   *http.Client
	signer       *v4.Signer
	now          func() time.Time
}

// Option configures AWSLambdaTools.
type Option func(*AWSLambdaTools)

// WithRegion sets the AWS region (default us-east-1).
func WithRegion(r string) Option { return func(a *AWSLambdaTools) { a.region = r } }

// WithStaticCredentials uses fixed credentials instead of the default AWS chain.
func WithStaticCredentials(accessKeyID, secretAccessKey, sessionToken string) Option {
	return func(a *AWSLambdaTools) {
		a.creds = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, sessionToken)
	}
}

// WithCredentialsProvider sets a custom AWS credentials provider.
func WithCredentialsProvider(p aws.CredentialsProvider) Option {
	return func(a *AWSLambdaTools) { a.creds = p }
}

// WithBaseURL overrides the endpoint (default https://lambda.<region>.amazonaws.com).
func WithBaseURL(u string) Option { return func(a *AWSLambdaTools) { a.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(a *AWSLambdaTools) { a.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(a *AWSLambdaTools) { a.httpClient = c } }

// WithEnableListFunctions toggles ListFunctions (default true).
func WithEnableListFunctions(v bool) Option { return func(a *AWSLambdaTools) { a.enableList = v } }

// WithEnableInvokeFunction toggles InvokeFunction (default true).
func WithEnableInvokeFunction(v bool) Option { return func(a *AWSLambdaTools) { a.enableInvoke = v } }

// WithAll registers every tool regardless of the enable flags.
func WithAll() Option { return func(a *AWSLambdaTools) { a.all = true } }

// ListFunctionsParams are the (empty) parameters of ListFunctions.
type ListFunctionsParams struct{}

// InvokeFunctionParams are the parameters of InvokeFunction.
type InvokeFunctionParams struct {
	FunctionName string `json:"function_name" description:"The name of the Lambda function to invoke." required:"true"`
	Payload      string `json:"payload,omitempty" description:"The JSON payload to send to the function. Defaults to \"{}\"."`
}

// New creates the AWS Lambda toolkit.
func New(opts ...Option) *AWSLambdaTools {
	a := &AWSLambdaTools{region: DefaultRegion, enableList: true, enableInvoke: true, signer: v4.NewSigner(), now: time.Now}
	a.Toolkit = toolkit.NewToolkit()
	a.Toolkit.Name = "aws-lambda"
	a.Toolkit.Description = "List and invoke AWS Lambda functions."
	for _, o := range opts {
		o(a)
	}
	if a.baseURL == "" {
		a.baseURL = "https://lambda." + a.region + ".amazonaws.com"
	}
	if a.httpClient == nil {
		a.httpClient = a.Toolkit.HTTPClient()
	}
	if a.all || a.enableList {
		a.Toolkit.Register("ListFunctions", "List all AWS Lambda functions in the configured AWS account.", a, a.ListFunctions, ListFunctionsParams{})
	}
	if a.all || a.enableInvoke {
		a.Toolkit.Register("InvokeFunction", "Invoke a specific AWS Lambda function with an optional JSON payload.", a, a.InvokeFunction, InvokeFunctionParams{})
	}
	return a
}

func (a *AWSLambdaTools) credentials(ctx context.Context) (aws.Credentials, error) {
	if a.creds == nil {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(a.region))
		if err != nil {
			return aws.Credentials{}, fmt.Errorf("failed to load AWS config: %w", err)
		}
		if cfg.Credentials == nil {
			return aws.Credentials{}, fmt.Errorf("no AWS credentials found")
		}
		a.creds = cfg.Credentials
	}
	return a.creds.Retrieve(ctx)
}

// call sends a SigV4-signed request and returns the status and body.
func (a *AWSLambdaTools) call(method, path string, body []byte) (int, []byte, error) {
	ctx := context.Background()
	creds, err := a.credentials(ctx)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(a.baseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	sum := sha256.Sum256(body)
	if err := a.signer.SignHTTP(ctx, creds, req, hex.EncodeToString(sum[:]), "lambda", a.region, a.now()); err != nil {
		return 0, nil, err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, restkit.MaxBodyBytes))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, data, &restkit.APIError{Name: "AWS Lambda", Status: resp.StatusCode, Body: string(data)}
	}
	return resp.StatusCode, data, nil
}

// ListFunctions returns "Available Lambda functions: a, b".
func (a *AWSLambdaTools) ListFunctions(params ListFunctionsParams) (interface{}, error) {
	_, data, err := a.call(http.MethodGet, "/2015-03-31/functions/", nil)
	if err != nil {
		return "Error listing functions: " + err.Error(), nil
	}
	var resp struct {
		Functions []struct {
			FunctionName string `json:"FunctionName"`
		} `json:"Functions"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "Error listing functions: " + err.Error(), nil
	}
	names := make([]string, 0, len(resp.Functions))
	for _, f := range resp.Functions {
		names = append(names, f.FunctionName)
	}
	return "Available Lambda functions: " + strings.Join(names, ", "), nil
}

// InvokeFunction synchronously invokes a function and returns its status and payload.
func (a *AWSLambdaTools) InvokeFunction(params InvokeFunctionParams) (interface{}, error) {
	payload := params.Payload
	if payload == "" {
		payload = "{}"
	}
	status, data, err := a.call(http.MethodPost, "/2015-03-31/functions/"+url.PathEscape(params.FunctionName)+"/invocations", []byte(payload))
	if err != nil {
		return "Error invoking function: " + err.Error(), nil
	}
	return fmt.Sprintf("Function invoked successfully. Status code: %d, Payload: %s", status, string(data)), nil
}

// Execute implements toolkit.Tool.
func (a *AWSLambdaTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return a.Toolkit.Execute(methodName, input)
}
