package models

import (
	"context"
	"errors"
	"testing"
)

// mockModel is a minimal AgnoModelInterface for testing the fallback logic.
type mockModel struct {
	id  string
	err error // if non-nil, Invoke/InvokeStream fail with this error
}

func (m *mockModel) GetID() string { return m.id }

func (m *mockModel) Invoke(_ context.Context, _ []Message, _ ...Option) (*MessageResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &MessageResponse{Model: m.id, Content: "ok from " + m.id}, nil
}

func (m *mockModel) AInvoke(ctx context.Context, msgs []Message, opts ...Option) (<-chan *MessageResponse, <-chan error) {
	ch := make(chan *MessageResponse, 1)
	errCh := make(chan error, 1)
	resp, err := m.Invoke(ctx, msgs, opts...)
	if err != nil {
		errCh <- err
	} else {
		ch <- resp
	}
	close(ch)
	close(errCh)
	return ch, errCh
}

func (m *mockModel) InvokeStream(_ context.Context, _ []Message, _ ...Option) error {
	return m.err
}

func (m *mockModel) AInvokeStream(_ context.Context, _ []Message, _ ...Option) (<-chan *MessageResponse, <-chan error) {
	ch := make(chan *MessageResponse)
	errCh := make(chan error, 1)
	if m.err != nil {
		errCh <- m.err
	}
	close(ch)
	close(errCh)
	return ch, errCh
}

func TestClassifyError(t *testing.T) {
	cases := []struct {
		msg  string
		want errorClass
	}{
		{"Error 429: too many requests", classRateLimit},
		{"rate limit exceeded", classRateLimit},
		{"context window exceeded for this model", classContextOverflow},
		{"context_length_exceeded", classContextOverflow},
		{"Error 401 unauthorized", classNonRetryable},
		{"permission denied", classNonRetryable},
		{"some 500 server error", classGeneric},
	}
	for _, c := range cases {
		if got := classifyError(errors.New(c.msg)); got != c.want {
			t.Errorf("classifyError(%q) = %d, want %d", c.msg, got, c.want)
		}
	}
}

func TestFallbackPrimarySucceeds(t *testing.T) {
	primary := &mockModel{id: "primary"}
	fb := NewFallbackModel(primary, FallbackConfig{
		OnError: []AgnoModelInterface{&mockModel{id: "backup"}},
	})

	resp, err := fb.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Model != "primary" {
		t.Fatalf("expected primary to answer, got %q", resp.Model)
	}
	m := fb.Metrics()
	if m.Requests != 1 || m.Successes != 1 || m.FallbackActivations != 0 {
		t.Fatalf("unexpected metrics: %+v", m)
	}
}

func TestFallbackOnGenericError(t *testing.T) {
	primary := &mockModel{id: "primary", err: errors.New("500 server error")}
	backup := &mockModel{id: "backup"}
	fb := NewFallbackModel(primary, FallbackConfig{OnError: []AgnoModelInterface{backup}})

	resp, err := fb.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Model != "backup" {
		t.Fatalf("expected backup to answer, got %q", resp.Model)
	}
	m := fb.Metrics()
	if m.FallbackActivations != 1 || m.Successes != 1 {
		t.Fatalf("unexpected metrics: %+v", m)
	}
}

func TestFallbackRateLimitRouting(t *testing.T) {
	primary := &mockModel{id: "primary", err: errors.New("Error 429 rate limit")}
	rlBackup := &mockModel{id: "ratelimit-backup"}
	errBackup := &mockModel{id: "error-backup"}
	fb := NewFallbackModel(primary, FallbackConfig{
		OnError:     []AgnoModelInterface{errBackup},
		OnRateLimit: []AgnoModelInterface{rlBackup},
	})

	resp, err := fb.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Model != "ratelimit-backup" {
		t.Fatalf("expected rate-limit backup, got %q", resp.Model)
	}
}

func TestFallbackNonRetryableNotMasked(t *testing.T) {
	primary := &mockModel{id: "primary", err: errors.New("Error 401 unauthorized")}
	backup := &mockModel{id: "backup"}
	fb := NewFallbackModel(primary, FallbackConfig{OnError: []AgnoModelInterface{backup}})

	_, err := fb.Invoke(context.Background(), nil)
	if err == nil {
		t.Fatal("expected non-retryable auth error to surface, not be masked by fallback")
	}
	m := fb.Metrics()
	if m.FallbackActivations != 0 || m.Failures != 1 {
		t.Fatalf("unexpected metrics: %+v", m)
	}
}

func TestFallbackAllFail(t *testing.T) {
	primary := &mockModel{id: "primary", err: errors.New("500 down")}
	backup := &mockModel{id: "backup", err: errors.New("500 also down")}
	fb := NewFallbackModel(primary, FallbackConfig{OnError: []AgnoModelInterface{backup}})

	_, err := fb.Invoke(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when all models fail")
	}
	m := fb.Metrics()
	if m.Failures != 1 || m.FallbackActivations != 1 {
		t.Fatalf("unexpected metrics: %+v", m)
	}
}

func TestFallbackCallbackInvoked(t *testing.T) {
	var calls []string
	primary := &mockModel{id: "primary", err: errors.New("500")}
	backup := &mockModel{id: "backup"}
	fb := NewFallbackModel(primary, FallbackConfig{
		OnError: []AgnoModelInterface{backup},
		Callback: func(primaryID, fallbackID string, _ error) {
			calls = append(calls, primaryID+"->"+fallbackID)
		},
	})

	if _, err := fb.Invoke(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 || calls[0] != "primary->backup" {
		t.Fatalf("unexpected callback calls: %v", calls)
	}
}

func TestFallbackImplementsInterface(t *testing.T) {
	var _ AgnoModelInterface = NewFallbackModel(&mockModel{id: "x"}, FallbackConfig{})
}
