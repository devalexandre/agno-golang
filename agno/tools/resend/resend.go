// Package resend ports Agno's ResendTools: sending email through the Resend
// API (https://resend.com/docs/api-reference).
package resend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Resend API base URL.
const DefaultBaseURL = "https://api.resend.com"

// ResendTools sends email with Resend.
type ResendTools struct {
	toolkit.Toolkit
	apiKey          string
	fromEmail       string
	baseURL         string
	enableSendEmail bool
	all             bool
	httpClient      *http.Client
	client          *restkit.Client
}

// Option configures ResendTools.
type Option func(*ResendTools)

// WithAPIKey sets the API key (default: RESEND_API_KEY).
func WithAPIKey(key string) Option { return func(r *ResendTools) { r.apiKey = key } }

// WithFromEmail sets the sender address.
func WithFromEmail(from string) Option { return func(r *ResendTools) { r.fromEmail = from } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(r *ResendTools) { r.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(r *ResendTools) { r.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(r *ResendTools) { r.httpClient = c } }

// WithEnableSendEmail toggles the SendEmail tool (default true).
func WithEnableSendEmail(v bool) Option { return func(r *ResendTools) { r.enableSendEmail = v } }

// WithAll registers every tool regardless of the enable flags.
func WithAll() Option { return func(r *ResendTools) { r.all = true } }

// SendEmailParams are the parameters of SendEmail.
type SendEmailParams struct {
	ToEmail string `json:"to_email" description:"The email address to send the email to." required:"true"`
	Subject string `json:"subject" description:"The subject of the email." required:"true"`
	Body    string `json:"body" description:"The body of the email (HTML)." required:"true"`
}

// New creates the Resend toolkit.
func New(opts ...Option) *ResendTools {
	r := &ResendTools{baseURL: DefaultBaseURL, enableSendEmail: true}
	r.Toolkit = toolkit.NewToolkit()
	r.Toolkit.Name = "resend_tools"
	r.Toolkit.Description = "Send emails using the Resend API."
	for _, o := range opts {
		o(r)
	}
	r.apiKey = restkit.Env(r.apiKey, "RESEND_API_KEY")
	if r.httpClient == nil {
		r.httpClient = r.Toolkit.HTTPClient()
	}
	r.client = &restkit.Client{
		HTTP:    r.httpClient,
		BaseURL: r.baseURL,
		Name:    "Resend API",
		Headers: map[string]string{"Authorization": "Bearer " + r.apiKey},
	}
	if r.all || r.enableSendEmail {
		r.Toolkit.Register("SendEmail", "Send an email using the Resend API. Returns if the email was sent successfully or an error message.", r, r.SendEmail, SendEmailParams{})
	}
	return r
}

// SendEmail sends an HTML email and returns a status message.
func (r *ResendTools) SendEmail(params SendEmailParams) (interface{}, error) {
	if r.apiKey == "" {
		return "Please provide an API key", nil
	}
	if params.ToEmail == "" {
		return "Please provide an email address to send the email to", nil
	}
	body := map[string]interface{}{
		"from":    r.fromEmail,
		"to":      params.ToEmail,
		"subject": params.Subject,
		"html":    params.Body,
	}
	if _, _, err := r.client.Raw(http.MethodPost, "/emails", nil, body); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return fmt.Sprintf("Email sent to %s successfully.", params.ToEmail), nil
}

// Execute implements toolkit.Tool.
func (r *ResendTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return r.Toolkit.Execute(methodName, input)
}
