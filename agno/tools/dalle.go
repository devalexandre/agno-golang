package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const dalleBaseURL = "https://api.openai.com/v1/images/generations"

// DalleTool provides AI image generation using OpenAI's DALL-E API.
type DalleTool struct {
	toolkit.Toolkit
	apiKey     string
	model      string
	httpClient *http.Client
}

// DalleGenerateParams defines the parameters for the GenerateImage method.
type DalleGenerateParams struct {
	Prompt  string `json:"prompt" description:"A text description of the image to generate." required:"true"`
	Size    string `json:"size,omitempty" description:"Image size: 1024x1024, 1024x1792, 1792x1024. Default: 1024x1024."`
	Quality string `json:"quality,omitempty" description:"Image quality: standard, hd. Default: standard."`
	Style   string `json:"style,omitempty" description:"Image style: vivid, natural. Default: vivid."`
	N       int    `json:"n,omitempty" description:"Number of images to generate (1-10). Default: 1."`
}

// NewDalleTool creates a new DALL-E tool.
// If apiKey is empty, it reads from the OPENAI_API_KEY environment variable.
// model defaults to "dall-e-3".
func NewDalleTool(apiKey, model string) *DalleTool {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if model == "" {
		model = "dall-e-3"
	}

	t := &DalleTool{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "DalleTool"
	tk.Description = "Generate images from text prompts using OpenAI DALL-E."

	t.Toolkit = tk
	t.Toolkit.Register("GenerateImage", "Generate an image from a text prompt.", t, t.GenerateImage, DalleGenerateParams{})

	return t
}

// GenerateImage generates an image from a text prompt using DALL-E.
func (t *DalleTool) GenerateImage(params DalleGenerateParams) (interface{}, error) {
	if params.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if t.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	size := params.Size
	if size == "" {
		size = "1024x1024"
	}
	quality := params.Quality
	if quality == "" {
		quality = "standard"
	}
	style := params.Style
	if style == "" {
		style = "vivid"
	}
	n := params.N
	if n <= 0 {
		n = 1
	}

	reqBody := map[string]interface{}{
		"model":   t.model,
		"prompt":  params.Prompt,
		"size":    size,
		"quality": quality,
		"style":   style,
		"n":       n,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", dalleBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dall-e request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dall-e API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	return result, nil
}

// Execute implements the toolkit.Tool interface.
func (t *DalleTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}
