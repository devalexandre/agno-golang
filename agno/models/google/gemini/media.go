package gemini

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"google.golang.org/genai"
)

// mediaParts converts the media carried by a message into genai parts.
//
// Gemini is the most capable of the supported providers here: it takes images,
// audio, video and documents, either inline (bytes) or by URI. Inline bytes and
// data URLs become InlineData blobs; remote URLs become FileData references.
func mediaParts(msg models.Message) []*genai.Part {
	parts := make([]*genai.Part, 0, len(msg.Images)+len(msg.Audio)+len(msg.Videos)+len(msg.Files))

	for _, img := range msg.Images {
		data, mime, err := img.Resolve(false)
		if p := blobOrURI(img.URL, data, mime, "image/png", err); p != nil {
			parts = append(parts, p)
		}
	}
	for _, aud := range msg.Audio {
		data, mime, err := aud.Resolve(false)
		if p := blobOrURI(aud.URL, data, mime, "audio/wav", err); p != nil {
			parts = append(parts, p)
		}
	}
	for _, vid := range msg.Videos {
		data, mime, err := vid.Resolve(false)
		if p := blobOrURI(vid.URL, data, mime, "video/mp4", err); p != nil {
			parts = append(parts, p)
		}
	}
	for _, f := range msg.Files {
		data, mime, err := f.Resolve(false)
		if p := blobOrURI(f.URL, data, mime, "application/pdf", err); p != nil {
			parts = append(parts, p)
		}
	}

	return parts
}

// blobOrURI builds an inline blob when bytes are available (Resolve already
// decoded data URLs), and a URI reference for remote media. Media that resolved
// to nothing usable is skipped rather than failing the request.
func blobOrURI(url string, data []byte, mimeType, defaultMime string, err error) *genai.Part {
	if err != nil {
		return nil
	}
	if len(data) > 0 {
		return &genai.Part{InlineData: &genai.Blob{
			Data:     data,
			MIMEType: mimeOrDefault(mimeType, defaultMime),
		}}
	}
	if url == "" {
		return nil
	}
	return &genai.Part{FileData: &genai.FileData{
		FileURI:  url,
		MIMEType: mimeOrDefault(mimeType, defaultMime),
	}}
}

func mimeOrDefault(mimeType, def string) string {
	if mimeType != "" {
		return mimeType
	}
	return def
}
