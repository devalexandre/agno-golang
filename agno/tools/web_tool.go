package tools

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// WebTool provides web-related functionality
type WebTool struct {
	toolkit.Toolkit
}

// WebResponse represents the response from web operations
type WebResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	URL        string            `json:"url"`
	Success    bool              `json:"success"`
	Error      string            `json:"error,omitempty"`
}

// HttpRequestParams represents parameters for HTTP requests
type HttpRequestParams struct {
	URL     string            `json:"url" description:"The URL to make the request to" required:"true"`
	Method  string            `json:"method,omitempty" description:"HTTP method (GET, POST, PUT, DELETE). Default: GET"`
	Headers map[string]string `json:"headers,omitempty" description:"HTTP headers to include in the request"`
	Body    string            `json:"body,omitempty" description:"Request body for POST/PUT requests"`
	Timeout int               `json:"timeout,omitempty" description:"Request timeout in seconds. Default: 30"`
}

// ScrapeParams represents parameters for web scraping
type ScrapeParams struct {
	URL      string `json:"url" description:"The URL to scrape" required:"true"`
	Selector string `json:"selector,omitempty" description:"CSS selector to extract specific content (optional)"`
	Timeout  int    `json:"timeout,omitempty" description:"Request timeout in seconds. Default: 30"`
}

// SimpleUrlParams represents parameters for simple URL operations
type SimpleUrlParams struct {
	URL     string `json:"url" description:"The URL to process" required:"true"`
	Timeout int    `json:"timeout,omitempty" description:"Request timeout in seconds. Default: 30"`
}

// NewWebTool creates a new WebTool instance
func NewWebTool() toolkit.Tool {
	wt := &WebTool{}
	wt.Toolkit = toolkit.NewToolkit()
	wt.Toolkit.Name = "WebTool"
	wt.Toolkit.Description = "A comprehensive web tool for making HTTP requests and web scraping. Supports GET, POST, PUT, DELETE requests and can extract content from web pages using CSS selectors."

	// Register methods
	wt.Toolkit.Register("HttpRequest", "Make HTTP requests to any URL", wt, wt.HttpRequest, HttpRequestParams{})
	wt.Toolkit.Register("ScrapeContent", "Scrape content from web pages using CSS selectors", wt, wt.ScrapeContent, ScrapeParams{})
	wt.Toolkit.Register("GetPageText", "Extract all text content from a web page", wt, wt.GetPageText, SimpleUrlParams{})
	wt.Toolkit.Register("GetPageTitle", "Get the title of a web page", wt, wt.GetPageTitle, SimpleUrlParams{})

	return wt
}

// HttpRequest makes HTTP requests to any URL
func (wt *WebTool) HttpRequest(params HttpRequestParams) (interface{}, error) {
	// Validate URL
	if params.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}

	// Set default method
	if params.Method == "" {
		params.Method = "GET"
	} else {
		params.Method = strings.ToUpper(params.Method)
	}

	// Set default timeout
	if params.Timeout <= 0 {
		params.Timeout = 30
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(params.Timeout) * time.Second,
	}

	// Prepare request body
	var body io.Reader
	if params.Body != "" && (params.Method == "POST" || params.Method == "PUT") {
		body = strings.NewReader(params.Body)
	}

	// Create request
	req, err := http.NewRequest(params.Method, params.URL, body)
	if err != nil {
		return WebResponse{
			URL:     params.URL,
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	// Set User-Agent
	req.Header.Set("User-Agent", "Agno-Framework/1.0 (Web Tool)")

	// Set custom headers
	for key, value := range params.Headers {
		req.Header.Set(key, value)
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return WebResponse{
			URL:     params.URL,
			Success: false,
			Error:   fmt.Sprintf("request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return WebResponse{
			StatusCode: resp.StatusCode,
			URL:        params.URL,
			Success:    false,
			Error:      fmt.Sprintf("failed to read response: %v", err),
		}, nil
	}

	// Convert headers
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	result := WebResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(bodyBytes),
		URL:        params.URL,
		Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
	}

	return result, nil
}

// ScrapeContent scrapes content from web pages using CSS selectors
func (wt *WebTool) ScrapeContent(params ScrapeParams) (interface{}, error) {
	// Validate URL
	if params.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}

	// Set default timeout
	if params.Timeout <= 0 {
		params.Timeout = 30
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: time.Duration(params.Timeout) * time.Second,
	}

	// Make request
	req, err := http.NewRequest("GET", params.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("User-Agent", "Agno-Framework/1.0 (Web Tool)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	if params.Selector == "" {
		// Return basic page info if no selector provided
		title := doc.Find("title").First().Text()
		bodyText := doc.Find("body").Text()
		// Limit body text to avoid huge responses
		if len(bodyText) > 2000 {
			bodyText = bodyText[:2000] + "..."
		}

		return map[string]interface{}{
			"url":   params.URL,
			"title": strings.TrimSpace(title),
			"text":  strings.TrimSpace(bodyText),
		}, nil
	}

	// Use CSS selector
	elements := []map[string]interface{}{}
	doc.Find(params.Selector).Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			element := map[string]interface{}{
				"text": text,
			}
			// Add href if it's a link
			if href, exists := s.Attr("href"); exists {
				element["href"] = href
			}
			// Add src if it's an image
			if src, exists := s.Attr("src"); exists {
				element["src"] = src
			}
			elements = append(elements, element)
		}
	})

	return map[string]interface{}{
		"url":      params.URL,
		"selector": params.Selector,
		"elements": elements,
		"count":    len(elements),
	}, nil
}

// GetPageText extracts all text content from a web page
func (wt *WebTool) GetPageText(params SimpleUrlParams) (interface{}, error) {
	// Validate URL
	if params.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}

	// Set default timeout
	if params.Timeout <= 0 {
		params.Timeout = 30
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: time.Duration(params.Timeout) * time.Second,
	}

	// Make request
	req, err := http.NewRequest("GET", params.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("User-Agent", "Agno-Framework/1.0 (Web Tool)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	// Extract text content
	title := doc.Find("title").First().Text()
	bodyText := doc.Find("body").Text()

	// Clean up text
	title = strings.TrimSpace(title)
	bodyText = strings.TrimSpace(bodyText)

	// Limit text to avoid huge responses
	if len(bodyText) > 3000 {
		bodyText = bodyText[:3000] + "..."
	}

	return map[string]interface{}{
		"url":   params.URL,
		"title": title,
		"text":  bodyText,
	}, nil
}

// GetPageTitle gets the title of a web page
func (wt *WebTool) GetPageTitle(params SimpleUrlParams) (interface{}, error) {
	// Validate URL
	if params.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}

	// Set default timeout
	if params.Timeout <= 0 {
		params.Timeout = 30
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: time.Duration(params.Timeout) * time.Second,
	}

	// Make request
	req, err := http.NewRequest("GET", params.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("User-Agent", "Agno-Framework/1.0 (Web Tool)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	title := doc.Find("title").First().Text()
	title = strings.TrimSpace(title)

	return map[string]interface{}{
		"url":   params.URL,
		"title": title,
	}, nil
}
