package exceptions

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	rl := Classify(NewModelProviderError("slow down", 429, "gpt", "gpt-4o"))
	if rl.Kind != ProviderErrorRateLimit || !rl.IsRetryable() {
		t.Fatalf("expected retryable rate limit, got %+v", rl)
	}
	cw := Classify(NewModelProviderError("This model's maximum context length is 8192", 400, "", ""))
	if cw.Kind != ProviderErrorContextWindow || cw.IsRetryable() {
		t.Fatalf("expected non-retryable context window, got %+v", cw)
	}
	bad := Classify(NewModelProviderError("bad request", 400, "", ""))
	if bad.Kind != ProviderErrorGeneric || bad.IsRetryable() {
		t.Fatalf("400 must be non retryable")
	}
	if !Classify(NewModelProviderError("upstream", 503, "", "")).IsRetryable() {
		t.Fatalf("503 must be retryable")
	}
	wrapped := fmt.Errorf("call failed: %w", NewModelProviderError("too many tokens", 400, "", ""))
	if !IsContextWindowExceeded(wrapped) || IsRateLimit(wrapped) || IsRetryable(wrapped) {
		t.Fatalf("wrapped classification failed")
	}
}

func TestSessionBusyAndStatus(t *testing.T) {
	err := fmt.Errorf("x: %w", NewSessionBusyError("", "r1", "s1"))
	if !IsSessionBusy(err) || HTTPStatusOf(err) != 409 {
		t.Fatalf("session busy not detected")
	}
	if HTTPStatusOf(ErrRunNotFound) != 404 {
		t.Fatalf("run not found status")
	}
	var sb *SessionBusyError
	if !errors.As(err, &sb) || sb.SessionID != "s1" {
		t.Fatalf("errors.As failed")
	}
}

func TestEmbeddingError(t *testing.T) {
	e := NewEmbeddingError("Invalid API key sk-abcdefghijklmnop", 0, "m", "openai")
	if e.Reason() != "authentication" || e.IsRetryable() {
		t.Fatalf("reason %s", e.Reason())
	}
	if strings.Contains(e.SafeMessage(), "abcdefghijkl") {
		t.Fatalf("secret not redacted: %s", e.SafeMessage())
	}
	if NewEmbeddingError("x", 429, "", "").Reason() != "rate_limit" {
		t.Fatalf("429 rate limit")
	}
}

func TestRedactSecrets(t *testing.T) {
	in := "postgres://user:hunter2@host/db api_key=XYZ123 Bearer abcdefghijkl"
	out := RedactSecrets(in)
	for _, leak := range []string{"hunter2", "XYZ123", "abcdefghijkl"} {
		if strings.Contains(out, leak) {
			t.Fatalf("leak %q in %q", leak, out)
		}
	}
}

func TestAgentRunControl(t *testing.T) {
	stop := fmt.Errorf("w: %w", NewStopAgentRun(errors.New("halt"), "", ""))
	if !IsStopAgentRun(stop) || IsRetryAgentRun(stop) {
		t.Fatal("stop")
	}
	if !IsRetryAgentRun(NewRetryAgentRun(errors.New("again"), "", "")) {
		t.Fatal("retry")
	}
	if IsRetryable(NewInputCheckError("no", "", nil)) {
		t.Fatal("input check must not retry")
	}
}
