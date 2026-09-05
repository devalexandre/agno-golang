package guardrails

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeResponse struct {
	Content string
}

type stringerValue struct{}

func (stringerValue) String() string { return "from stringer" }

func TestExtractText(t *testing.T) {
	text := "hello"
	tests := []struct {
		name     string
		data     interface{}
		expected string
		ok       bool
	}{
		{"string", "hello", "hello", true},
		{"pointer to string", &text, "hello", true},
		{"bytes", []byte("hello"), "hello", true},
		{"input", NewInput("hello"), "hello", true},
		{"output", NewOutput("hello"), "hello", true},
		{"stringer", stringerValue{}, "from stringer", true},
		{"error", errors.New("boom"), "boom", true},
		{"struct with content", fakeResponse{Content: "hello"}, "hello", true},
		{"pointer to struct", &fakeResponse{Content: "hello"}, "hello", true},
		{"map", map[string]interface{}{"content": "hello"}, "hello", true},
		{"map of strings", map[string]string{"message": "hello"}, "hello", true},
		{"slice", []string{"a", "b"}, "a\nb", true},
		{"nil", nil, "", false},
		{"int", 42, "", false},
		{"empty struct", struct{ Other int }{Other: 1}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractText(tt.data)
			if ok != tt.ok || got != tt.expected {
				t.Fatalf("ExtractText(%v) = (%q, %v), want (%q, %v)", tt.data, got, ok, tt.expected, tt.ok)
			}
		})
	}
}

func TestFuncGuardrail(t *testing.T) {
	called := false
	g := NewFunc("custom", "a custom rule", func(ctx context.Context, data interface{}) error {
		called = true
		if text, _ := ExtractText(data); strings.Contains(text, "bad") {
			return errors.New("rejected")
		}
		return nil
	})

	if g.GetName() != "custom" || g.GetDescription() != "a custom rule" {
		t.Errorf("unexpected metadata: %q / %q", g.GetName(), g.GetDescription())
	}
	if err := g.Check(context.Background(), "good"); err != nil {
		t.Errorf("expected the check to pass, got %v", err)
	}
	if err := g.Check(context.Background(), "bad"); err == nil {
		t.Error("expected the check to fail")
	}
	if !called {
		t.Error("expected the check function to be called")
	}

	empty := &Func{Name: "empty"}
	if err := empty.Check(context.Background(), "x"); err != nil {
		t.Errorf("a guardrail without a function must pass, got %v", err)
	}
}

func TestChain(t *testing.T) {
	order := []string{}
	first := NewFunc("first", "", func(ctx context.Context, data interface{}) error {
		order = append(order, "first")
		return nil
	})
	second := NewFunc("second", "", func(ctx context.Context, data interface{}) error {
		order = append(order, "second")
		return errors.New("rejected")
	})
	third := NewFunc("third", "", func(ctx context.Context, data interface{}) error {
		order = append(order, "third")
		return nil
	})

	chain := NewChain("input", first, second).Add(third)
	err := chain.Check(context.Background(), "data")
	if err == nil {
		t.Fatal("expected the chain to fail")
	}
	if !strings.Contains(err.Error(), "guardrail 'second' failed") {
		t.Errorf("expected the failing guardrail to be named, got %v", err)
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("the chain must stop at the first failure, ran %v", order)
	}
	if chain.GetName() != "input" || !strings.Contains(chain.GetDescription(), "3") {
		t.Errorf("unexpected metadata: %q / %q", chain.GetName(), chain.GetDescription())
	}
}

func TestRunAndRunInput(t *testing.T) {
	ctx := context.Background()

	if err := Run(ctx, nil, "data"); err != nil {
		t.Errorf("an empty guardrail list must pass, got %v", err)
	}

	failing := NewFunc("failing", "", func(ctx context.Context, data interface{}) error {
		return errors.New("rejected")
	})
	err := Run(ctx, []Guardrail{failing}, "data")
	if err == nil || !strings.Contains(err.Error(), "guardrail 'failing' failed") {
		t.Fatalf("expected a wrapped error, got %v", err)
	}

	// RunInput must give an InputChecker the mutable input.
	pii := NewPIIDetectionGuardrail(WithPIIMode(PIIModeRedact))
	input := NewInput("mail me at a@b.co")
	if err := RunInput(ctx, []Guardrail{pii}, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Content != "mail me at [REDACTED_EMAIL]" {
		t.Errorf("expected the input to be redacted, got %q", input.Content)
	}

	output := NewOutput("mail me at a@b.co")
	if err := RunOutput(ctx, []Guardrail{pii}, output); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Content != "mail me at [REDACTED_EMAIL]" {
		t.Errorf("expected the output to be redacted, got %q", output.Content)
	}

	if err := RunInput(ctx, []Guardrail{pii}, nil); err != nil {
		t.Errorf("a nil input must pass, got %v", err)
	}
	if err := RunOutput(ctx, []Guardrail{pii}, nil); err != nil {
		t.Errorf("a nil output must pass, got %v", err)
	}
}

func TestCheckErrors(t *testing.T) {
	inputErr := NewInputCheckError("blocked", "", nil)
	if inputErr.Stage != StageInput || inputErr.Trigger != TriggerInputNotAllowed {
		t.Errorf("unexpected defaults: %+v", inputErr)
	}
	if inputErr.ErrorID() != string(TriggerInputNotAllowed) {
		t.Errorf("unexpected error id: %q", inputErr.ErrorID())
	}
	if inputErr.Error() != "blocked" {
		t.Errorf("unexpected message: %q", inputErr.Error())
	}

	outputErr := NewOutputCheckError("blocked", TriggerOffTopic, map[string]interface{}{"k": "v"})
	if outputErr.Stage != StageOutput || outputErr.Trigger != TriggerOffTopic {
		t.Errorf("unexpected defaults: %+v", outputErr)
	}

	named := outputErr.WithGuardrail("Custom")
	if named.Error() != "Custom: blocked" {
		t.Errorf("unexpected message: %q", named.Error())
	}
	if outputErr.Guardrail != "" {
		t.Error("WithGuardrail must not mutate the original error")
	}

	// Check errors must survive wrapping.
	chain := NewChain("chain", NewFunc("failing", "", func(ctx context.Context, data interface{}) error {
		return NewInputCheckError("blocked", TriggerPromptInjection, nil)
	}))
	err := chain.Check(context.Background(), "data")
	if !IsTrigger(err, TriggerPromptInjection) {
		t.Errorf("expected the trigger to survive wrapping, got %v", err)
	}
	if _, ok := AsCheckError(errors.New("other")); ok {
		t.Error("a plain error must not be reported as a check error")
	}
	if IsTrigger(errors.New("other"), TriggerPIIDetected) {
		t.Error("a plain error has no trigger")
	}
}
