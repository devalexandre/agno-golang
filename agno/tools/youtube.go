package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// YouTubeTool provides access to YouTube Data API
type YouTubeTool struct {
	toolkit.Toolkit
	APIKey string
}

// NewYouTubeTool creates a new YouTube tool
func NewYouTubeTool(apiKey string) *YouTubeTool {
	t := &YouTubeTool{
		APIKey: apiKey,
	}

	tk := toolkit.NewToolkit()
	tk.Name = "YouTube"
	tk.Description = "Search and retrieve video information from YouTube"

	t.Toolkit = tk
	t.Toolkit.Register("SearchVideos", "Search for videos on YouTube", t, t.SearchVideos, YouTubeSearchParams{})

	return t
}

type YouTubeSearchParams struct {
	Query string `json:"query" jsonschema:"description=The search query,required=true"`
	Count int    `json:"count" jsonschema:"description=Number of videos to return (max 10),default=3"`
}

type YouTubeSearchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title        string `json:"title"`
			Description  string `json:"description"`
			ChannelTitle string `json:"channelTitle"`
			PublishedAt  string `json:"publishedAt"`
		} `json:"snippet"`
	} `json:"items"`
}

// SearchVideos searches YouTube for videos
func (t *YouTubeTool) SearchVideos(params YouTubeSearchParams) (string, error) {
	if t.APIKey == "" {
		return "", fmt.Errorf("YouTube API Key must be set")
	}

	if params.Count <= 0 {
		params.Count = 3
	}
	if params.Count > 10 {
		params.Count = 10
	}

	apiURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/search?part=snippet&type=video&q=%s&maxResults=%d&key=%s",
		url.QueryEscape(params.Query), params.Count, t.APIKey)

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to search youtube: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("youtube api error: %s", string(body))
	}

	var result YouTubeSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Items) == 0 {
		return "No videos found", nil
	}

	var sb string
	sb += fmt.Sprintf("Found %d videos for '%s':\n\n", len(result.Items), params.Query)

	for i, item := range result.Items {
		videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", item.ID.VideoID)
		sb += fmt.Sprintf("%d. %s\n", i+1, item.Snippet.Title)
		sb += fmt.Sprintf("   Channel: %s\n", item.Snippet.ChannelTitle)
		sb += fmt.Sprintf("   Published: %s\n", item.Snippet.PublishedAt)
		sb += fmt.Sprintf("   Link: %s\n", videoURL)
		sb += fmt.Sprintf("   Description: %s\n\n", item.Snippet.Description)
	}

	return sb, nil
}

// Execute implements the Tool interface
func (t *YouTubeTool) Execute(methodName string, args json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, args)
}
