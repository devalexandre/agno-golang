package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// HackerNewsTools provides access to HackerNews content
type HackerNewsTools struct {
	Name        string
	Description string
	MaxStories  int
	StoryType   string // "top", "new", "best", "ask", "show", "job"
	httpClient  *http.Client
}

// NewHackerNewsTools creates a new HackerNews tool
func NewHackerNewsTools(options ...HackerNewsOption) *HackerNewsTools {
	tool := &HackerNewsTools{
		Name:        "HackerNews",
		Description: "Get stories and insights from HackerNews",
		MaxStories:  10,
		StoryType:   "top",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range options {
		opt(tool)
	}

	return tool
}

// HackerNewsOption is a functional option for configuring HackerNewsTools
type HackerNewsOption func(*HackerNewsTools)

// WithHackerNewsMaxStories sets the maximum number of stories to fetch
func WithHackerNewsMaxStories(max int) HackerNewsOption {
	return func(h *HackerNewsTools) {
		h.MaxStories = max
	}
}

// WithHackerNewsStoryType sets the type of stories to fetch
func WithHackerNewsStoryType(storyType string) HackerNewsOption {
	return func(h *HackerNewsTools) {
		h.StoryType = storyType
	}
}

// GetName returns the tool name
func (h *HackerNewsTools) GetName() string {
	return h.Name
}

// GetDescription returns the tool description
func (h *HackerNewsTools) GetDescription() string {
	return h.Description
}

// GetTopStories fetches the top stories from HackerNews
func (h *HackerNewsTools) GetTopStories() ([]HNStory, error) {
	return h.GetTopStoriesWithContext(context.Background())
}

// GetTopStoriesWithContext fetches top stories with context
func (h *HackerNewsTools) GetTopStoriesWithContext(ctx context.Context) ([]HNStory, error) {
	// HackerNews API base URL
	baseURL := "https://hacker-news.firebaseio.com/v0"

	// Determine which endpoint to use based on story type
	endpoint := fmt.Sprintf("%s/%sstories.json", baseURL, h.StoryType)

	// Get story IDs
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch story IDs: %w", err)
	}
	defer resp.Body.Close()

	var storyIDs []int
	if err := json.NewDecoder(resp.Body).Decode(&storyIDs); err != nil {
		return nil, fmt.Errorf("failed to decode story IDs: %w", err)
	}

	// Limit to MaxStories
	if len(storyIDs) > h.MaxStories {
		storyIDs = storyIDs[:h.MaxStories]
	}

	// Fetch individual stories
	stories := make([]HNStory, 0, len(storyIDs))
	for _, id := range storyIDs {
		story, err := h.fetchStory(ctx, id)
		if err != nil {
			// Continue with other stories if one fails
			continue
		}
		stories = append(stories, *story)
	}

	return stories, nil
}

// fetchStory fetches a single story by ID
func (h *HackerNewsTools) fetchStory(ctx context.Context, id int) (*HNStory, error) {
	url := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", id)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var story HNStory
	if err := json.NewDecoder(resp.Body).Decode(&story); err != nil {
		return nil, err
	}

	return &story, nil
}

// SearchStories searches for stories matching a query
func (h *HackerNewsTools) SearchStories(query string) ([]HNStory, error) {
	return h.SearchStoriesWithContext(context.Background(), query)
}

// SearchStoriesWithContext searches stories with context
func (h *HackerNewsTools) SearchStoriesWithContext(ctx context.Context, query string) ([]HNStory, error) {
	// Use Algolia HN Search API
	searchURL := fmt.Sprintf("https://hn.algolia.com/api/v1/search?query=%s&tags=story&hitsPerPage=%d", query, h.MaxStories)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search stories: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var searchResult AlgoliaSearchResult
	if err := json.Unmarshal(body, &searchResult); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	// Convert Algolia results to HNStory format
	stories := make([]HNStory, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		story := HNStory{
			ID:          hit.ObjectID,
			Title:       hit.Title,
			URL:         hit.URL,
			Score:       hit.Points,
			By:          hit.Author,
			Time:        hit.CreatedAtI,
			Text:        hit.StoryText,
			Descendants: hit.NumComments,
		}
		stories = append(stories, story)
	}

	return stories, nil
}

// Execute implements the Tool interface
func (h *HackerNewsTools) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Check if there's a query parameter for search
	if query, ok := args["query"].(string); ok && query != "" {
		stories, err := h.SearchStoriesWithContext(ctx, query)
		if err != nil {
			return nil, err
		}
		return h.formatStories(stories, fmt.Sprintf("HackerNews search results for '%s'", query)), nil
	}

	// Otherwise, get top stories
	stories, err := h.GetTopStoriesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return h.formatStories(stories, fmt.Sprintf("Top %d HackerNews stories", len(stories))), nil
}

// formatStories formats stories for agent consumption
func (h *HackerNewsTools) formatStories(stories []HNStory, header string) string {
	var formatted strings.Builder
	formatted.WriteString(fmt.Sprintf("%s:\n\n", header))

	// Sort by score (descending)
	sort.Slice(stories, func(i, j int) bool {
		return stories[i].Score > stories[j].Score
	})

	for i, story := range stories {
		formatted.WriteString(fmt.Sprintf("%d. %s\n", i+1, story.Title))

		if story.URL != "" {
			formatted.WriteString(fmt.Sprintf("   URL: %s\n", story.URL))
		} else {
			formatted.WriteString(fmt.Sprintf("   HN Link: https://news.ycombinator.com/item?id=%s\n", story.ID))
		}

		formatted.WriteString(fmt.Sprintf("   Score: %d points | By: %s | Comments: %d\n",
			story.Score, story.By, story.Descendants))

		if story.Text != "" {
			// Truncate text if too long
			text := story.Text
			if len(text) > 200 {
				text = text[:197] + "..."
			}
			formatted.WriteString(fmt.Sprintf("   %s\n", text))
		}

		formatted.WriteString(fmt.Sprintf("   Posted: %s\n", time.Unix(int64(story.Time), 0).Format("2006-01-02 15:04")))
		formatted.WriteString("\n")
	}

	return formatted.String()
}

// GetSchema returns the tool's schema for function calling
func (h *HackerNewsTools) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "get_hackernews_stories",
		"description": h.Description,
		"parameters": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Optional search query. If not provided, returns top stories",
				},
				"story_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of stories to fetch: top, new, best, ask, show, job",
					"enum":        []string{"top", "new", "best", "ask", "show", "job"},
				},
			},
		},
	}
}

// HNStory represents a HackerNews story
type HNStory struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url,omitempty"`
	Score       int    `json:"score"`
	By          string `json:"by"`
	Time        int    `json:"time"`
	Text        string `json:"text,omitempty"`
	Type        string `json:"type"`
	Descendants int    `json:"descendants"`
	Kids        []int  `json:"kids,omitempty"`
}

// AlgoliaSearchResult represents search results from Algolia HN API
type AlgoliaSearchResult struct {
	Hits []AlgoliaHit `json:"hits"`
}

// AlgoliaHit represents a single hit from Algolia search
type AlgoliaHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Points      int    `json:"points"`
	Author      string `json:"author"`
	CreatedAtI  int    `json:"created_at_i"`
	StoryText   string `json:"story_text"`
	NumComments int    `json:"num_comments"`
}

// GetTrendingTopics analyzes stories to extract trending topics
func (h *HackerNewsTools) GetTrendingTopics(stories []HNStory) []string {
	// Simple keyword extraction (in production, use NLP)
	keywords := make(map[string]int)

	commonWords := map[string]bool{
		"the": true, "and": true, "a": true, "an": true, "is": true,
		"it": true, "to": true, "of": true, "in": true, "for": true,
		"on": true, "with": true, "as": true, "by": true, "at": true,
		"from": true, "up": true, "about": true, "into": true, "through": true,
		"during": true, "before": true, "after": true, "above": true, "below": true,
	}

	for _, story := range stories {
		words := strings.Fields(strings.ToLower(story.Title))
		for _, word := range words {
			// Clean word
			word = strings.Trim(word, ".,!?;:'\"()[]{}|-")

			// Skip common words and short words
			if len(word) < 3 || commonWords[word] {
				continue
			}

			keywords[word]++
		}
	}

	// Sort keywords by frequency
	type kv struct {
		Key   string
		Value int
	}

	var sortedKeywords []kv
	for k, v := range keywords {
		sortedKeywords = append(sortedKeywords, kv{k, v})
	}

	sort.Slice(sortedKeywords, func(i, j int) bool {
		return sortedKeywords[i].Value > sortedKeywords[j].Value
	})

	// Return top 10 trending topics
	topics := make([]string, 0, 10)
	for i, kv := range sortedKeywords {
		if i >= 10 {
			break
		}
		topics = append(topics, kv.Key)
	}

	return topics
}
