package client

import (
	"encoding/base64"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

// buildUserMessage converts an agno message into an OpenAI user message. Plain
// text messages stay a simple string; messages carrying media become an array of
// content parts.
//
// The Chat Completions API takes images, input audio and files. Video has no
// representation, so videos are dropped here rather than failing the request.
func buildUserMessage(msg models.Message) openai.ChatCompletionMessageParamUnion {
	if !msg.HasMedia() {
		return openai.UserMessage(msg.Content)
	}

	parts := make([]openai.ChatCompletionContentPartUnionParam, 0, 1+len(msg.Images)+len(msg.Audio)+len(msg.Files))
	if msg.Content != "" {
		parts = append(parts, openai.TextContentPart(msg.Content))
	}

	for _, img := range msg.Images {
		// A remote URL passes through untouched; inline bytes become a data URL.
		url, err := img.DataURL()
		if err != nil {
			continue
		}
		parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL:    url,
			Detail: img.Detail,
		}))
	}

	for _, aud := range msg.Audio {
		// Audio must be sent inline: the API takes base64 payloads, not URLs.
		data, _, err := aud.Resolve(true)
		if err != nil || len(data) == 0 {
			continue
		}
		parts = append(parts, openai.InputAudioContentPart(openai.ChatCompletionContentPartInputAudioInputAudioParam{
			Data:   base64.StdEncoding.EncodeToString(data),
			Format: aud.AudioFormat(),
		}))
	}

	for _, f := range msg.Files {
		data, mime, err := f.Resolve(true)
		if err != nil || len(data) == 0 {
			continue
		}
		// file_data expects a data URL, not bare base64.
		parts = append(parts, openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
			FileData: param.NewOpt(models.EncodeDataURL(data, mime)),
			Filename: param.NewOpt(f.Filename()),
		}))
	}

	if len(parts) == 0 {
		return openai.UserMessage(msg.Content)
	}
	return openai.UserMessage(parts)
}
