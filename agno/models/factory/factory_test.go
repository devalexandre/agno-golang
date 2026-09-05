package factory

import (
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestParseModelString(t *testing.T) {
	cases := []struct {
		spec         string
		wantProvider string
		wantID       string
	}{
		{"openai:gpt-4o", "openai", "gpt-4o"},
		{"anthropic:claude-sonnet-4-5", "anthropic", "claude-sonnet-4-5"},
		{"OpenAI:gpt-4o", "openai", "gpt-4o"},
		{" openai : gpt-4o ", "openai", "gpt-4o"},
		// Model IDs may contain colons: only the first one splits.
		{"ollama:llama3.2:3b", "ollama", "llama3.2:3b"},
		{"aws:anthropic.claude-3-5-sonnet-20241022-v2:0", "aws", "anthropic.claude-3-5-sonnet-20241022-v2:0"},
	}

	for _, tc := range cases {
		provider, id, err := ParseModelString(tc.spec)
		if err != nil {
			t.Fatalf("ParseModelString(%q) returned error: %v", tc.spec, err)
		}
		if provider != tc.wantProvider || id != tc.wantID {
			t.Errorf("ParseModelString(%q) = (%q, %q), want (%q, %q)", tc.spec, provider, id, tc.wantProvider, tc.wantID)
		}
	}
}

func TestParseModelStringErrors(t *testing.T) {
	for _, spec := range []string{"", "   ", "gpt-4o", ":gpt-4o", "openai:", "openai:   "} {
		if _, _, err := ParseModelString(spec); err == nil {
			t.Errorf("ParseModelString(%q) expected an error, got nil", spec)
		}
	}
}

// TestAllProvidersResolve makes sure every canonical provider name is routed to a
// constructor, and that the spec parser recognizes it.
func TestAllProvidersResolve(t *testing.T) {
	for _, name := range ListProviders() {
		provider, id, err := ParseModelString(name + ":some-model")
		if err != nil {
			t.Fatalf("ParseModelString for provider %q: %v", name, err)
		}
		if provider != name {
			t.Errorf("provider %q parsed as %q", name, provider)
		}
		if id != "some-model" {
			t.Errorf("provider %q: model ID = %q, want \"some-model\"", name, id)
		}
		if _, ok := providers[provider]; !ok {
			t.Errorf("provider %q has no constructor registered", provider)
		}
	}
}

func TestListProviders(t *testing.T) {
	want := []string{
		"anthropic", "aws", "azure", "cohere", "deepseek", "fireworks", "google",
		"groq", "huggingface", "litellm", "maritaca", "meta", "mistral", "nvidia",
		"ollama", "openai", "openrouter", "perplexity", "together", "vertexai",
		"vllm", "xai",
	}

	got := ListProviders()
	if len(got) != len(want) {
		t.Fatalf("ListProviders() returned %d providers, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ListProviders()[%d] = %q, want %q (list must be sorted)", i, got[i], want[i])
		}
	}
}

func TestAliasesResolveToKnownProviders(t *testing.T) {
	cases := map[string]string{
		"gemini":       "google",
		"claude":       "anthropic",
		"bedrock":      "aws",
		"aws-bedrock":  "aws",
		"vertex":       "vertexai",
		"azure-openai": "azure",
		"grok":         "xai",
		"hf":           "huggingface",
		"llama":        "meta",
		"mistralai":    "mistral",
		"gpt":          "openai",
	}

	for alias, canonical := range cases {
		provider, _, err := ParseModelString(alias + ":model-x")
		if err != nil {
			t.Fatalf("ParseModelString for alias %q: %v", alias, err)
		}
		if provider != canonical {
			t.Errorf("alias %q resolved to %q, want %q", alias, provider, canonical)
		}
	}

	// Every registered alias must point at a real provider.
	for alias, canonical := range Aliases() {
		if _, ok := providers[canonical]; !ok {
			t.Errorf("alias %q points at unknown provider %q", alias, canonical)
		}
	}
}

func TestGetModelUnknownProvider(t *testing.T) {
	_, err := GetModel("not-a-provider:some-model")
	if err == nil {
		t.Fatal("GetModel with an unknown provider should return an error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "not-a-provider") {
		t.Errorf("error should name the unknown provider, got: %s", msg)
	}
	// The error must be actionable: it lists what is supported.
	for _, provider := range []string{"openai", "anthropic", "ollama"} {
		if !strings.Contains(msg, provider) {
			t.Errorf("error should list supported provider %q, got: %s", provider, msg)
		}
	}
}

func TestGetModelInvalidSpec(t *testing.T) {
	if _, err := GetModel("gpt-4o"); err == nil {
		t.Fatal("GetModel without a provider prefix should return an error")
	}
}

// TestGetModelBuildsOpenAICompatibleProviders builds the providers whose
// constructors only need an API key, and checks the model ID from the spec
// reaches the model.
func TestGetModelBuildsOpenAICompatibleProviders(t *testing.T) {
	cases := []struct {
		spec string
		id   string
	}{
		{"openai:gpt-4o", "gpt-4o"},
		{"gpt:gpt-4o-mini", "gpt-4o-mini"},
		{"groq:llama-3.3-70b-versatile", "llama-3.3-70b-versatile"},
		{"deepseek:deepseek-chat", "deepseek-chat"},
		{"mistral:mistral-large-latest", "mistral-large-latest"},
		{"mistralai:mistral-small-latest", "mistral-small-latest"},
		{"xai:grok-2", "grok-2"},
		{"grok:grok-2", "grok-2"},
		{"together:meta-llama/Llama-3-70b-chat-hf", "meta-llama/Llama-3-70b-chat-hf"},
		{"fireworks:accounts/fireworks/models/llama-v3-70b", "accounts/fireworks/models/llama-v3-70b"},
		{"perplexity:sonar", "sonar"},
		{"nvidia:meta/llama-3.1-70b-instruct", "meta/llama-3.1-70b-instruct"},
		{"openrouter:openai/gpt-4o", "openai/gpt-4o"},
		{"meta:llama-3.3-70b", "llama-3.3-70b"},
		{"llama:llama-3.3-70b", "llama-3.3-70b"},
		{"huggingface:meta-llama/Llama-3-8b", "meta-llama/Llama-3-8b"},
		{"hf:meta-llama/Llama-3-8b", "meta-llama/Llama-3-8b"},
		{"maritaca:sabia-3", "sabia-3"},
		{"cohere:command-r-plus", "command-r-plus"},
		{"litellm:gpt-4o", "gpt-4o"},
		{"vllm:qwen2.5", "qwen2.5"},
		{"ollama:llama3.2", "llama3.2"},
	}

	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			model, err := GetModel(tc.spec, models.WithAPIKey("test-key"))
			if err != nil {
				t.Fatalf("GetModel(%q) returned error: %v", tc.spec, err)
			}
			if model == nil {
				t.Fatalf("GetModel(%q) returned a nil model", tc.spec)
			}
			if got := model.GetID(); got != tc.id {
				t.Errorf("GetModel(%q).GetID() = %q, want %q", tc.spec, got, tc.id)
			}
		})
	}
}

func TestGetModelOptionsAreForwarded(t *testing.T) {
	// A caller option must be able to override the ID coming from the spec.
	model, err := GetModel("openai:gpt-4o", models.WithAPIKey("test-key"), models.WithID("gpt-4o-mini"))
	if err != nil {
		t.Fatalf("GetModel returned error: %v", err)
	}
	if got := model.GetID(); got != "gpt-4o-mini" {
		t.Errorf("GetID() = %q, want %q (caller options must win over the spec)", got, "gpt-4o-mini")
	}
}
