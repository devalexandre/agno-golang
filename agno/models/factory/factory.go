// Package factory resolves a model from a "provider:model-id" string, so callers
// can pick a model without importing every provider package themselves.
//
//	model, err := factory.GetModel("openai:gpt-4o")
//	model, err := factory.GetModel("claude:claude-sonnet-4-5", models.WithAPIKey(key))
//
// It lives in its own package (instead of agno/models) to avoid an import cycle:
// every provider package imports agno/models.
package factory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/aimlapi"
	"github.com/devalexandre/agno-golang/agno/models/anthropic"
	"github.com/devalexandre/agno-golang/agno/models/aws/bedrock"
	"github.com/devalexandre/agno-golang/agno/models/azure"
	"github.com/devalexandre/agno-golang/agno/models/cohere"
	"github.com/devalexandre/agno-golang/agno/models/deepseek"
	"github.com/devalexandre/agno-golang/agno/models/fireworks"
	"github.com/devalexandre/agno-golang/agno/models/flexai"
	"github.com/devalexandre/agno-golang/agno/models/google/gemini"
	"github.com/devalexandre/agno-golang/agno/models/groq"
	"github.com/devalexandre/agno-golang/agno/models/heabsy"
	"github.com/devalexandre/agno-golang/agno/models/huggingface"
	"github.com/devalexandre/agno-golang/agno/models/litellm"
	"github.com/devalexandre/agno-golang/agno/models/maritaca"
	"github.com/devalexandre/agno-golang/agno/models/meta"
	"github.com/devalexandre/agno-golang/agno/models/mistral"
	"github.com/devalexandre/agno-golang/agno/models/nvidia"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	openaichat "github.com/devalexandre/agno-golang/agno/models/openai/chat"
	"github.com/devalexandre/agno-golang/agno/models/openrouter"
	"github.com/devalexandre/agno-golang/agno/models/perplexity"
	"github.com/devalexandre/agno-golang/agno/models/together"
	"github.com/devalexandre/agno-golang/agno/models/vertexai"
	"github.com/devalexandre/agno-golang/agno/models/vllm"
	"github.com/devalexandre/agno-golang/agno/models/xai"
	"github.com/devalexandre/agno-golang/agno/models/yapi"
)

// constructor builds a model for a given model ID and client options.
type constructor func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error)

// providers maps the canonical provider name to its constructor.
var providers = map[string]constructor{
	"aimlapi": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return aimlapi.NewAIMLAPIChat(withID(id, opts)...)
	},
	"anthropic": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return anthropic.NewAnthropic(withID(id, opts)...)
	},
	"aws": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		co := clientOptions(opts)
		region, _ := co.ClientParams["region"].(string)
		maxTokens := 0
		if co.MaxTokens != nil {
			maxTokens = *co.MaxTokens
		}
		return bedrock.NewBedrock(context.Background(), bedrock.Config{
			Region:    region,
			Model:     id,
			MaxTokens: maxTokens,
		})
	},
	"azure": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		co := clientOptions(opts)
		apiVersion, _ := co.ClientParams["api_version"].(string)
		// On Azure the model ID is the deployment name.
		return azure.NewAzureChat(azure.Config{
			APIKey:     co.APIKey,
			Endpoint:   co.BaseURL,
			Deployment: id,
			APIVersion: apiVersion,
		}, opts...)
	},
	"cohere": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return cohere.NewCohereChat(withID(id, opts)...)
	},
	"deepseek": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return deepseek.NewDeepSeekChat(withID(id, opts)...)
	},
	"fireworks": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return fireworks.NewFireworksChat(withID(id, opts)...)
	},
	"flexai": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return flexai.NewFlexAIChat(withID(id, opts)...)
	},
	"google": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return gemini.NewGemini(withID(id, opts)...)
	},
	"groq": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return groq.NewGroqChat(withID(id, opts)...)
	},
	"heabsy": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return heabsy.NewHeabsyChat(withID(id, opts)...)
	},
	"huggingface": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return huggingface.NewHuggingFaceChat(withID(id, opts)...)
	},
	"litellm": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return litellm.NewLiteLLMChat(withID(id, opts)...)
	},
	"maritaca": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return maritaca.NewMaritacaChat(withID(id, opts)...)
	},
	"meta": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return meta.NewMetaChat(withID(id, opts)...)
	},
	"mistral": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return mistral.NewMistralChat(withID(id, opts)...)
	},
	"nvidia": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return nvidia.NewNvidiaChat(withID(id, opts)...)
	},
	"ollama": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return ollama.NewOllamaChat(withID(id, opts)...)
	},
	"openai": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return openaichat.NewOpenAIChat(withID(id, opts)...)
	},
	"openrouter": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return openrouter.NewOpenRouterChat(withID(id, opts)...)
	},
	"perplexity": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return perplexity.NewPerplexityChat(withID(id, opts)...)
	},
	"together": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return together.NewTogetherChat(withID(id, opts)...)
	},
	"vertexai": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		co := clientOptions(opts)
		project, _ := co.ClientParams["project"].(string)
		location, _ := co.ClientParams["location"].(string)
		return vertexai.NewVertexAI(vertexai.Config{
			Project:  project,
			Location: location,
			Model:    id,
		}, opts...)
	},
	"vllm": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return vllm.NewVLLMProvider(withID(id, opts)...)
	},
	"xai": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return xai.NewXaiChat(withID(id, opts)...)
	},
	"yapi": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return yapi.NewYAPIChat(withID(id, opts)...)
	},
}

// aliases maps convenient spellings to the canonical provider name.
var aliases = map[string]string{
	"azure-openai": "azure",
	"azure_openai": "azure",
	"bedrock":      "aws",
	"aws-bedrock":  "aws",
	"claude":       "anthropic",
	"gemini":       "google",
	"googleai":     "google",
	"grok":         "xai",
	"hf":           "huggingface",
	"llama":        "meta",
	"mistralai":    "mistral",
	"oai":          "openai",
	"gpt":          "openai",
	"vertex":       "vertexai",
	"vertex-ai":    "vertexai",
	"vertex_ai":    "vertexai",
	"y-api":        "yapi",
	"aiml":         "aimlapi",
}

// GetModel resolves a model from a "provider:model-id" spec and returns the
// provider-backed model. The provider name is case-insensitive and accepts the
// aliases listed by Aliases (e.g. "claude:..." for Anthropic).
//
//	model, err := factory.GetModel("openai:gpt-4o")
//	model, err := factory.GetModel("ollama:llama3.2", models.WithBaseURL("http://localhost:11434"))
//
// Options are forwarded to the provider constructor, so they win over the model
// ID taken from the spec only if they set the same field explicitly.
func GetModel(spec string, opts ...models.OptionClient) (models.AgnoModelInterface, error) {
	provider, id, err := ParseModelString(spec)
	if err != nil {
		return nil, err
	}

	build, ok := providers[provider]
	if !ok {
		return nil, fmt.Errorf(
			"models/factory: unknown provider %q in spec %q. Supported providers: %s. Aliases: %s",
			provider, spec, strings.Join(ListProviders(), ", "), strings.Join(ListAliases(), ", "),
		)
	}

	return build(id, opts)
}

// ParseModelString splits a "provider:model-id" spec into its canonical provider
// name and the model ID. The model ID may itself contain colons (Bedrock and
// Ollama model IDs do), so only the first colon is significant.
func ParseModelString(spec string) (provider string, id string, err error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return "", "", fmt.Errorf("models/factory: empty model spec, expected \"provider:model-id\" (e.g. \"openai:gpt-4o\")")
	}

	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf(
			"models/factory: invalid model spec %q, expected \"provider:model-id\" (e.g. \"openai:gpt-4o\"). Supported providers: %s",
			spec, strings.Join(ListProviders(), ", "),
		)
	}

	provider = strings.ToLower(strings.TrimSpace(parts[0]))
	id = strings.TrimSpace(parts[1])

	if provider == "" {
		return "", "", fmt.Errorf("models/factory: missing provider in spec %q, expected \"provider:model-id\"", spec)
	}
	if id == "" {
		return "", "", fmt.Errorf("models/factory: missing model ID in spec %q, expected \"provider:model-id\"", spec)
	}

	if canonical, ok := aliases[provider]; ok {
		provider = canonical
	}

	return provider, id, nil
}

// ListProviders returns the canonical provider names accepted by GetModel, sorted.
func ListProviders() []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListAliases returns the accepted provider aliases, sorted.
func ListAliases() []string {
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Aliases returns a copy of the alias -> canonical provider mapping.
func Aliases() map[string]string {
	out := make(map[string]string, len(aliases))
	for alias, canonical := range aliases {
		out[alias] = canonical
	}
	return out
}

// withID prepends the model ID so caller options can still override it.
func withID(id string, opts []models.OptionClient) []models.OptionClient {
	final := make([]models.OptionClient, 0, len(opts)+1)
	final = append(final, models.WithID(id))
	final = append(final, opts...)
	return final
}

// clientOptions materializes the options into a ClientOptions value, used by the
// providers whose constructors take a Config struct instead of options.
func clientOptions(opts []models.OptionClient) models.ClientOptions {
	co := models.ClientOptions{}
	for _, opt := range opts {
		opt(&co)
	}
	return co
}
