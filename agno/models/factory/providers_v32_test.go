package factory

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestV32ProvidersBuild(t *testing.T) {
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "acct")
	t.Setenv("LANGDB_PROJECT_ID", "proj")
	t.Setenv("IBM_WATSONX_PROJECT_ID", "proj")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://example.openai.azure.com")

	cases := []struct{ spec, id string }{
		{"cerebras:gpt-oss-120b", "gpt-oss-120b"},
		{"cerebras-openai:llama3.1-8b", "llama3.1-8b"},
		{"cloudflare:@cf/meta/llama-3.1-8b-instruct", "workers-ai/@cf/meta/llama-3.1-8b-instruct"},
		{"cometapi:gpt-5-mini", "gpt-5-mini"},
		{"dashscope:qwen-plus", "qwen-plus"},
		{"deepinfra:meta-llama/Llama-3.3-70B-Instruct", "meta-llama/Llama-3.3-70B-Instruct"},
		{"ibm:ibm/granite-3-8b-instruct", "ibm/granite-3-8b-instruct"},
		{"watsonx:ibm/granite-3-8b-instruct", "ibm/granite-3-8b-instruct"},
		{"inception:mercury-2", "mercury-2"},
		{"internlm:internlm2.5-latest", "internlm2.5-latest"},
		{"langdb:gpt-4o", "gpt-4o"},
		{"llama-cpp:local", "local"},
		{"llama_cpp:local", "local"},
		{"llmman:qwen3", "qwen3"},
		{"lmstudio:qwen2.5", "qwen2.5"},
		{"minimax:MiniMax-M3", "MiniMax-M3"},
		{"moonshot:kimi-k3", "kimi-k3"},
		{"kimi:kimi-k2", "kimi-k2"},
		{"n1n:gpt-4o", "gpt-4o"},
		{"nebius:openai/gpt-oss-20b", "openai/gpt-oss-20b"},
		{"neosantara:grok-4.1-fast-non-reasoning", "grok-4.1-fast-non-reasoning"},
		{"nexus:openai/gpt-4", "openai/gpt-4"},
		{"openai-responses:gpt-5-mini", "gpt-5-mini"},
		{"open-responses:some-model", "some-model"},
		{"azure-openai-responses:my-deployment", "my-deployment"},
		{"portkey:gpt-4o-mini", "gpt-4o-mini"},
		{"ramp:gpt-5.6-luna", "gpt-5.6-luna"},
		{"requesty:openai/gpt-4.1", "openai/gpt-4.1"},
		{"sambanova:Meta-Llama-3.1-8B-Instruct", "Meta-Llama-3.1-8B-Instruct"},
		{"siliconflow:Qwen/QwQ-32B", "Qwen/QwQ-32B"},
		{"synthorai:claude-opus-5", "claude-opus-5"},
		{"tokenlab:gpt-5.4-mini", "gpt-5.4-mini"},
		{"trustedrouter:trustedrouter/zdr", "trustedrouter/zdr"},
		{"tuning-engines:gpt-4o", "gpt-4o"},
		{"vercel:v0-1.0-md", "v0-1.0-md"},
		{"xiaomi:mimo-v2.5-pro", "mimo-v2.5-pro"},
	}
	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			m, err := GetModel(tc.spec, models.WithAPIKey("test-key"))
			if err != nil {
				t.Fatalf("GetModel(%q): %v", tc.spec, err)
			}
			if got := m.GetID(); got != tc.id {
				t.Errorf("GetModel(%q).GetID() = %q, want %q", tc.spec, got, tc.id)
			}
		})
	}
}
