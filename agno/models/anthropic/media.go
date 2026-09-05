package anthropic

import (
	"encoding/base64"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/devalexandre/agno-golang/agno/models"
)

// userBlocks converts an agno message into Anthropic content blocks: text first,
// then media.
//
// The Messages API takes images and PDF documents. Audio and video have no
// representation, so they are dropped here rather than failing the request.
func userBlocks(m models.Message) []sdk.ContentBlockParamUnion {
	blocks := make([]sdk.ContentBlockParamUnion, 0, 1+len(m.Images)+len(m.Files))
	if m.Content != "" {
		blocks = append(blocks, sdk.NewTextBlock(m.Content))
	}

	for _, img := range m.Images {
		// A remote URL is passed by reference; inline bytes are sent as base64.
		if len(img.Data) == 0 && img.URL != "" && !isDataURL(img.URL) {
			blocks = append(blocks, sdk.NewImageBlock(sdk.URLImageSourceParam{URL: img.URL}))
			continue
		}
		data, mime, err := img.Resolve(false)
		if err != nil || len(data) == 0 {
			continue
		}
		blocks = append(blocks, sdk.NewImageBlockBase64(mime, base64.StdEncoding.EncodeToString(data)))
	}

	for _, f := range m.Files {
		if len(f.Data) == 0 && f.URL != "" && !isDataURL(f.URL) {
			blocks = append(blocks, sdk.NewDocumentBlock(sdk.URLPDFSourceParam{URL: f.URL}))
			continue
		}
		data, _, err := f.Resolve(false)
		if err != nil || len(data) == 0 {
			continue
		}
		// Base64 documents are PDF-only on this API; other types are dropped.
		blocks = append(blocks, sdk.NewDocumentBlock(sdk.Base64PDFSourceParam{
			Data: base64.StdEncoding.EncodeToString(data),
		}))
	}

	return blocks
}

func isDataURL(s string) bool {
	return len(s) >= 5 && s[:5] == "data:"
}
