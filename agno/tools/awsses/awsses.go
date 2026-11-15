// Package awsses ports Agno's AWSSESTool: sending plain-text email through
// Amazon SES (SESv2 REST API, POST /v2/email/outbound-emails) signed with
// AWS Signature Version 4.
package awsses

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// AWSSESTool sends email with Amazon SES.
type AWSSESTool struct {
	toolkit.Toolkit
	senderEmail     string
	senderName      string
	region          string
	baseURL         string
	creds           aws.CredentialsProvider
	enableSendEmail bool
	all             bool
	httpClient      *http.Client
	signer          *v4.Signer
	now             func() time.Time
}

// Option configures AWSSESTool.
type Option func(*AWSSESTool)

// WithSenderEmail sets the sender address.
func WithSenderEmail(s string) Option { return func(a *AWSSESTool) { a.senderEmail = s } }

// WithSenderName sets the sender display name.
func WithSenderName(s string) Option { return func(a *AWSSESTool) { a.senderName = s } }

// WithRegion sets the AWS region (default us-east-1).
func WithRegion(r string) Option { return func(a *AWSSESTool) { a.region = r } }

// WithStaticCredentials uses fixed credentials instead of the default AWS chain.
func WithStaticCredentials(accessKeyID, secretAccessKey, sessionToken string) Option {
	return func(a *AWSSESTool) {
		a.creds = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, sessionToken)
	}
}

// WithCredentialsProvider sets a custom AWS credentials provider.
func WithCredentialsProvider(p aws.CredentialsProvider) Option {
	return func(a *AWSSESTool) { a.creds = p }
}

// WithBaseURL overrides the endpoint (default https://email.<region>.amazonaws.com).
func WithBaseURL(u string) Option { return func(a *AWSSESTool) { a.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(a *AWSSESTool) { a.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(a *AWSSESTool) { a.httpClient = c } }

// WithEnableSendEmail toggles SendEmail (default true).
func WithEnableSendEmail(v bool) Option { return func(a *AWSSESTool) { a.enableSendEmail = v } }

// WithAll registers every tool regardless of the enable flags.
func WithAll() Option { return func(a *AWSSESTool) { a.all = true } }

// SendEmailParams are the parameters of SendEmail.
type SendEmailParams struct {
	Subject       string `json:"subject" description:"The subject of the email." required:"true"`
	Body          string `json:"body" description:"The body of the email." required:"true"`
	ReceiverEmail string `json:"receiver_email" description:"The email address of the receiver." required:"true"`
}

// New creates the AWS SES toolkit.
func New(opts ...Option) *AWSSESTool {
	a := &AWSSESTool{region: DefaultRegion, enableSendEmail: true, signer: v4.NewSigner(), now: time.Now}
	a.Toolkit = toolkit.NewToolkit()
	a.Toolkit.Name = "aws_ses_tool"
	a.Toolkit.Description = "Send emails using Amazon SES."
	for _, o := range opts {
		o(a)
	}
	if a.baseURL == "" {
		a.baseURL = "https://email." + a.region + ".amazonaws.com"
	}
	if a.httpClient == nil {
		a.httpClient = a.Toolkit.HTTPClient()
	}
	if a.all || a.enableSendEmail {
		a.Toolkit.Register("SendEmail", "Use this tool to send an email using AWS SES.", a, a.SendEmail, SendEmailParams{})
	}
	return a
}

func (a *AWSSESTool) credentials(ctx context.Context) (aws.Credentials, error) {
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

// SendEmail sends a plain-text email and returns "Email sent successfully!".
func (a *AWSSESTool) SendEmail(params SendEmailParams) (interface{}, error) {
	if params.Subject == "" {
		return "Email subject cannot be empty.", nil
	}
	if params.Body == "" {
		return "Email body cannot be empty.", nil
	}
	content := func(s string) map[string]string { return map[string]string{"Charset": "UTF-8", "Data": s} }
	payload, err := json.Marshal(map[string]interface{}{
		"FromEmailAddress": fmt.Sprintf("%s <%s>", a.senderName, a.senderEmail),
		"Destination":      map[string][]string{"ToAddresses": {params.ReceiverEmail}},
		"Content": map[string]interface{}{
			"Simple": map[string]interface{}{
				"Subject": content(params.Subject),
				"Body":    map[string]interface{}{"Text": content(params.Body)},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	creds, err := a.credentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.baseURL, "/")+"/v2/email/outbound-emails", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	sum := sha256.Sum256(payload)
	if err := a.signer.SignHTTP(ctx, creds, req, hex.EncodeToString(sum[:]), "ses", a.region, a.now()); err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, restkit.MaxBodyBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to send email: %w", &restkit.APIError{Name: "AWS SES", Status: resp.StatusCode, Body: string(data)})
	}
	return "Email sent successfully!", nil
}

// Execute implements toolkit.Tool.
func (a *AWSSESTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return a.Toolkit.Execute(methodName, input)
}
