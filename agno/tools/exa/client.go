package exa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type Client struct {
	APIKey string
}

func NewClient(apiKey string) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("EXA_API_KEY")
	}
	return &Client{APIKey: apiKey}
}

// Payloads
type SearchRequest struct {
	Query              string   `json:"query"`
	NumResults         int      `json:"num_results,omitempty"`
	Highlights         bool     `json:"highlights,omitempty"`
	Text               bool     `json:"text,omitempty"`
	Summary            bool     `json:"summary,omitempty"`
	Category           string   `json:"category,omitempty"`
	IncludeDomains     []string `json:"include_domains,omitempty"`
	ExcludeDomains     []string `json:"exclude_domains,omitempty"`
	UseAutoPrompt      bool     `json:"use_autoprompt,omitempty"`
	StartCrawlDate     string   `json:"start_crawl_date,omitempty"`
	EndCrawlDate       string   `json:"end_crawl_date,omitempty"`
	StartPublishedDate string   `json:"start_published_date,omitempty"`
	EndPublishedDate   string   `json:"end_published_date,omitempty"`
}

func (c *Client) Search(req SearchRequest) (map[string]interface{}, error) {
	return c.doRequest("https://api.exa.ai/search", req)
}

func (c *Client) GetContents(urls []string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"urls": urls,
		"text": true,
	}
	return c.doRequest("https://api.exa.ai/contents", req)
}

func (c *Client) FindSimilar(url string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"url":  url,
		"text": true,
	}
	return c.doRequest("https://api.exa.ai/findSimilar", req)
}

func (c *Client) Answer(query string, model string, text bool) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"query": query,
		"model": model,
		"text":  text,
	}
	return c.doRequest("https://api.exa.ai/answer", req)
}

func (c *Client) doRequest(url string, payload interface{}) (map[string]interface{}, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}

	// ⚠️ Corrigido para Authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := ioutil.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}

	return result, nil
}
