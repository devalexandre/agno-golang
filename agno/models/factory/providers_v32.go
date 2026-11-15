package factory

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/azure"
	"github.com/devalexandre/agno-golang/agno/models/cerebras"
	"github.com/devalexandre/agno-golang/agno/models/cloudflare"
	"github.com/devalexandre/agno-golang/agno/models/cometapi"
	"github.com/devalexandre/agno-golang/agno/models/dashscope"
	"github.com/devalexandre/agno-golang/agno/models/deepinfra"
	"github.com/devalexandre/agno-golang/agno/models/inception"
	"github.com/devalexandre/agno-golang/agno/models/internlm"
	"github.com/devalexandre/agno-golang/agno/models/langdb"
	llamacpp "github.com/devalexandre/agno-golang/agno/models/llama_cpp"
	"github.com/devalexandre/agno-golang/agno/models/llmman"
	"github.com/devalexandre/agno-golang/agno/models/lmstudio"
	"github.com/devalexandre/agno-golang/agno/models/minimax"
	"github.com/devalexandre/agno-golang/agno/models/moonshot"
	"github.com/devalexandre/agno-golang/agno/models/n1n"
	"github.com/devalexandre/agno-golang/agno/models/nebius"
	"github.com/devalexandre/agno-golang/agno/models/neosantara"
	"github.com/devalexandre/agno-golang/agno/models/nexus"
	"github.com/devalexandre/agno-golang/agno/models/openai/responses"
	"github.com/devalexandre/agno-golang/agno/models/portkey"
	"github.com/devalexandre/agno-golang/agno/models/ramp"
	"github.com/devalexandre/agno-golang/agno/models/requesty"
	"github.com/devalexandre/agno-golang/agno/models/sambanova"
	"github.com/devalexandre/agno-golang/agno/models/siliconflow"
	"github.com/devalexandre/agno-golang/agno/models/synthorai"
	"github.com/devalexandre/agno-golang/agno/models/tokenlab"
	"github.com/devalexandre/agno-golang/agno/models/trustedrouter"
	tuningengines "github.com/devalexandre/agno-golang/agno/models/tuning_engines"
	"github.com/devalexandre/agno-golang/agno/models/vercel"
	"github.com/devalexandre/agno-golang/agno/models/xiaomi"
)

// optionsCtor adapts a func(...OptionClient) constructor to the factory signature.
func optionsCtor(build func(...models.OptionClient) (models.AgnoModelInterface, error)) constructor {
	return func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
		return build(withID(id, opts)...)
	}
}

// Providers added for parity with Agno v3.2 (agno/models/utils.py keys). They
// live in a separate file so the core table in factory.go stays small.
func init() {
	extra := map[string]constructor{
		"azure-openai-responses": func(id string, opts []models.OptionClient) (models.AgnoModelInterface, error) {
			return azure.NewAzureResponsesFromOptions(withID(id, opts)...)
		},
		"cerebras":         optionsCtor(cerebras.NewCerebrasChat),
		"cloudflare":       optionsCtor(cloudflare.NewCloudflareChat),
		"cometapi":         optionsCtor(cometapi.NewCometAPIChat),
		"dashscope":        optionsCtor(dashscope.NewDashScopeChat),
		"deepinfra":        optionsCtor(deepinfra.NewDeepInfraChat),
		"inception":        optionsCtor(inception.NewInceptionChat),
		"internlm":         optionsCtor(internlm.NewInternLMChat),
		"langdb":           optionsCtor(langdb.NewLangDBChat),
		"llama-cpp":        optionsCtor(llamacpp.NewLlamaCppChat),
		"llmman":           optionsCtor(llmman.NewLlmmanChat),
		"lmstudio":         optionsCtor(lmstudio.NewLMStudioChat),
		"minimax":          optionsCtor(minimax.NewMiniMaxChat),
		"moonshot":         optionsCtor(moonshot.NewMoonShotChat),
		"n1n":              optionsCtor(n1n.NewN1NChat),
		"nebius":           optionsCtor(nebius.NewNebiusChat),
		"neosantara":       optionsCtor(neosantara.NewNeosantaraChat),
		"nexus":            optionsCtor(nexus.NewNexusChat),
		"open-responses":   optionsCtor(responses.NewOpenResponses),
		"openai-responses": optionsCtor(responses.NewOpenAIResponses),
		"portkey":          optionsCtor(portkey.NewPortkeyChat),
		"ramp":             optionsCtor(ramp.NewRampRouter),
		"requesty":         optionsCtor(requesty.NewRequestyChat),
		"sambanova":        optionsCtor(sambanova.NewSambanovaChat),
		"siliconflow":      optionsCtor(siliconflow.NewSiliconflowChat),
		"synthorai":        optionsCtor(synthorai.NewSynthoraiChat),
		"tokenlab":         optionsCtor(tokenlab.NewTokenLabChat),
		"trustedrouter":    optionsCtor(trustedrouter.NewTrustedRouterChat),
		"tuning-engines":   optionsCtor(tuningengines.NewTuningEnginesChat),
		"vercel":           optionsCtor(vercel.NewV0Chat),
		"xiaomi":           optionsCtor(xiaomi.NewMiMoChat),
	}
	for name, ctor := range extra {
		if _, exists := providers[name]; !exists {
			providers[name] = ctor
		}
	}

	extraAliases := map[string]string{
		"cerebras-openai":        "cerebras",
		"cerebras_openai":        "cerebras",
		"workers-ai":             "cloudflare",
		"qwen":                   "dashscope",
		"inceptionlabs":          "inception",
		"llama_cpp":              "llama-cpp",
		"llamacpp":               "llama-cpp",
		"lm-studio":              "lmstudio",
		"kimi":                   "moonshot",
		"openai-chat":            "openai",
		"openai_responses":       "openai-responses",
		"openresponses":          "open-responses",
		"azure_openai_responses": "azure-openai-responses",
		"ramp-router":            "ramp",
		"ramprouter":             "ramp",
		"samba":                  "sambanova",
		"tuning_engines":         "tuning-engines",
		"tuningengines":          "tuning-engines",
		"v0":                     "vercel",
		"mimo":                   "xiaomi",
	}
	for alias, canonical := range extraAliases {
		if _, exists := aliases[alias]; !exists {
			aliases[alias] = canonical
		}
	}
}
