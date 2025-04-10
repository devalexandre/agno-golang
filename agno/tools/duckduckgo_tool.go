package tools

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// DuckDuckGoSearchResponse represents the expected structure of DuckDuckGo search API response.
type DuckDuckGoSearchResponse []struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Href  string `json:"href"`
}

// GetDuckDuckGoSearchHandler performs a DuckDuckGo search based on provided query parameters.
func GetDuckDuckGoSearchHandler(queryParams map[string]interface{}) (string, error) {
	baseURL := "https://duckduckgo.com/ac/"
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("error parsing base URL: %v", err)
	}

	// Build query
	q := u.Query()
	if query, ok := queryParams["query"].(string); ok {
		q.Set("q", query)
	} else {
		return "", fmt.Errorf("missing required 'query' parameter")
	}
	u.RawQuery = q.Encode()

	// Perform the request
	resp, err := http.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("error fetching data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("invalid HTTP status: %d. Response: %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var results DuckDuckGoSearchResponse
	err = json.Unmarshal(body, &results)
	if err != nil {
		// If there is an error unmarshaling, return raw response.
		return string(body), nil
	}

	// Format the response
	type SearchResult struct {
		Query    string      `json:"query"`
		Results  interface{} `json:"results"`
		Response string      `json:"summary"`
	}

	response := SearchResult{
		Query:   queryParams["query"].(string),
		Results: results,
		Response: fmt.Sprintf(
			"DuckDuckGo search for '%v' returned %d results.",
			queryParams["query"].(string), len(results),
		),
	}

	output, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("error formatting output JSON: %v", err)
	}

	return string(output), nil
}

// DuckDuckGoTool implements the Tool interface for DuckDuckGo search.
type DuckDuckGoTool struct{}

// Description returns a short description of the tool.
func (dt DuckDuckGoTool) Description() string {
	return "Search for information using DuckDuckGo. Provide a query string to retrieve search suggestions and results."
}

// Execute performs the search operation based on input parameters.
func (dt DuckDuckGoTool) Execute(input json.RawMessage) (interface{}, error) {
	var params map[string]interface{}
	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, err
	}
	result, err := GetDuckDuckGoSearchHandler(params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetParameterStruct returns the expected parameters for the tool.
func (dt DuckDuckGoTool) GetParameterStruct() interface{} {
	return map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "The search query to use in DuckDuckGo.",
		},
	}
}

// Name returns the name of the tool.
func (dt DuckDuckGoTool) Name() string {
	return "DuckDuckGoTool"
}
