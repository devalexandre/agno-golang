package models

import "strings"

// Capabilities describes which media a provider can accept as input. Callers use
// it to warn or degrade instead of sending media a provider will reject.
type Capabilities struct {
	Vision bool // image input
	Audio  bool // audio input
	Video  bool // video input
	Files  bool // document input (PDF and similar)
}

// providerCapabilities maps a provider key to the media it accepts. Keys match
// the prefixes understood by the model factory.
var providerCapabilities = map[string]Capabilities{
	"openai":     {Vision: true, Audio: true, Files: true},
	"azure":      {Vision: true, Audio: true, Files: true},
	"anthropic":  {Vision: true, Files: true},
	"google":     {Vision: true, Audio: true, Video: true, Files: true},
	"vertexai":   {Vision: true, Audio: true, Video: true, Files: true},
	"aws":        {Vision: true, Files: true},
	"groq":       {Vision: true},
	"mistral":    {Vision: true},
	"xai":        {Vision: true},
	"ollama":     {Vision: true},
	"openrouter": {Vision: true},
	"together":   {Vision: true},
	"fireworks":  {Vision: true},
	"nvidia":     {Vision: true},
	"meta":       {Vision: true},
	"litellm":    {Vision: true},
	"vllm":       {Vision: true},

	// Text-only providers.
	"cohere":      {},
	"deepseek":    {},
	"perplexity":  {},
	"huggingface": {},
	"maritaca":    {},
}

// CapabilitiesFor returns the media a provider accepts. Unknown providers are
// reported as text-only, which is the safe default.
func CapabilitiesFor(provider string) Capabilities {
	return providerCapabilities[strings.ToLower(provider)]
}

// UnsupportedMedia lists the kinds of media in msg that provider cannot accept.
// An empty result means the message is safe to send as-is.
func UnsupportedMedia(provider string, msg Message) []string {
	caps := CapabilitiesFor(provider)
	var unsupported []string
	if len(msg.Images) > 0 && !caps.Vision {
		unsupported = append(unsupported, "images")
	}
	if len(msg.Audio) > 0 && !caps.Audio {
		unsupported = append(unsupported, "audio")
	}
	if len(msg.Videos) > 0 && !caps.Video {
		unsupported = append(unsupported, "video")
	}
	if len(msg.Files) > 0 && !caps.Files {
		unsupported = append(unsupported, "files")
	}
	return unsupported
}
