// Package presettest holds shared, network-free tests for the OpenAI-compatible
// provider wrappers (cerebras, deepinfra, moonshot, ...). Requests are captured
// by replacing http.DefaultTransport, so no live calls are made.
package presettest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Captured is a request seen by the fake transport.
type Captured struct {
	URL    string
	Path   string
	Header http.Header
	Body   map[string]interface{}
}

// Recorder is an http.RoundTripper that records requests and answers with a
// canned OpenAI chat completion.
type Recorder struct {
	mu       sync.Mutex
	Requests []Captured
	Reply    string
}

const cannedReply = `{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"pong"}}]}`

// RoundTrip implements http.RoundTripper.
func (r *Recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	var body map[string]interface{}
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(raw, &body)
	}
	r.mu.Lock()
	r.Requests = append(r.Requests, Captured{URL: req.URL.String(), Path: req.URL.Path, Header: req.Header.Clone(), Body: body})
	r.mu.Unlock()
	reply := r.Reply
	if reply == "" {
		reply = cannedReply
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(reply)),
		Request:    req,
	}, nil
}

// Last returns the last captured request.
func (r *Recorder) Last(t *testing.T) Captured {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.Requests) == 0 {
		t.Fatalf("no request captured")
	}
	return r.Requests[len(r.Requests)-1]
}

// Install swaps http.DefaultTransport for a Recorder for the duration of the test.
func Install(t *testing.T) *Recorder {
	t.Helper()
	rec := &Recorder{}
	prev := http.DefaultTransport
	http.DefaultTransport = rec
	t.Cleanup(func() { http.DefaultTransport = prev })
	return rec
}

// Case describes a provider preset under test.
type Case struct {
	New            func(...models.OptionClient) (models.AgnoModelInterface, error)
	EnvAPIKey      string
	DefaultModel   string
	DefaultBaseURL string
	// Extra options applied in every sub-test (e.g. required account IDs).
	Options []models.OptionClient
}

// Invoke runs one non-streaming call and returns the captured request.
func Invoke(t *testing.T, rec *Recorder, m models.AgnoModelInterface) Captured {
	t.Helper()
	resp, err := m.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "ping"}})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if resp.Content != "pong" {
		t.Fatalf("unexpected content %q", resp.Content)
	}
	return rec.Last(t)
}

// Run executes the standard preset assertions: default base URL path, auth
// header from the env key, default model, model override and extra headers.
func Run(t *testing.T, c Case) {
	t.Run("defaults", func(t *testing.T) {
		rec := Install(t)
		if c.EnvAPIKey != "" {
			t.Setenv(c.EnvAPIKey, "env-secret")
		}
		m, err := c.New(c.Options...)
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		if got := m.GetID(); got != c.DefaultModel {
			t.Fatalf("GetID = %q, want %q", got, c.DefaultModel)
		}
		req := Invoke(t, rec, m)
		want := strings.TrimRight(c.DefaultBaseURL, "/") + "/chat/completions"
		if req.URL != want {
			t.Fatalf("URL = %q, want %q", req.URL, want)
		}
		if c.EnvAPIKey != "" {
			if got := req.Header.Get("Authorization"); got != "Bearer env-secret" {
				t.Fatalf("Authorization = %q, want bearer from %s", got, c.EnvAPIKey)
			}
		}
		if got := req.Body["model"]; got != c.DefaultModel {
			t.Fatalf("body model = %v, want %q", got, c.DefaultModel)
		}
	})

	t.Run("overrides", func(t *testing.T) {
		rec := Install(t)
		headers := http.Header{}
		headers.Set("X-Extra", "1")
		opts := append([]models.OptionClient{}, c.Options...)
		opts = append(opts,
			models.WithAPIKey("explicit"),
			models.WithID("custom-model"),
			models.WithBaseURL("http://example.invalid/v9"),
			models.WithDefaultHeaders(headers),
		)
		m, err := c.New(opts...)
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		req := Invoke(t, rec, m)
		if req.URL != "http://example.invalid/v9/chat/completions" {
			t.Fatalf("URL = %q", req.URL)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer explicit" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := req.Header.Get("X-Extra"); got != "1" {
			t.Fatalf("X-Extra = %q", got)
		}
		if got := req.Body["model"]; got != "custom-model" {
			t.Fatalf("body model = %v", got)
		}
	})
}
