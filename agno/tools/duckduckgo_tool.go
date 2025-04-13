package tools

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DuckDuckGoSearchResponse represents the expected structure of DuckDuckGo search API response.
type DuckDuckGoSearchResponse []struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Href  string `json:"href"`
}

// DuckDuckGoToolInput defines the input parameters for the DuckDuckGoTool.
type DuckDuckGoToolInput struct {
	Query string `json:"query" description:"The search query to use in DuckDuckGo." required:"true"`
}

// GetDuckDuckGoSearchHandler performs a DuckDuckGo search based on provided input parameters.
func GetDuckDuckGoSearchHandler(params DuckDuckGoToolInput) (string, error) {
	baseURL := "https://duckduckgo.com/ac/"
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("error parsing base URL: %v", err)
	}

	// Build query
	q := u.Query()
	q.Set("q", params.Query)
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
		Query:   params.Query,
		Results: results,
		Response: fmt.Sprintf(
			"DuckDuckGo search for '%v' returned %d results.",
			params.Query, len(results),
		),
	}

	output, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("error formatting output JSON: %v", err)
	}

	return string(output), nil
}

// DuckDuckGoTool implements the Tool interface for DuckDuckGo search.
type DuckDuckGoTool struct {
	toolkit.Toolkit
}

func NewDuckDuckGoTool() *DuckDuckGoTool {
	dt := &DuckDuckGoTool{}
	tk := toolkit.NewToolkit()
	tk.Name = "DuckDuckGoTool"
	tk.Description = "Searches DuckDuckGo for the given query."
	dt.Toolkit = tk
	dt.Toolkit.Register("Search", dt, dt.Search, DuckDuckGoToolInput{})
	return dt
}

// Execute performs the search operation based on input parameters.
func (dt DuckDuckGoTool) Search(input DuckDuckGoToolInput) (interface{}, error) {

	result, err := GetDuckDuckGoSearchHandler(input)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// // GetParameterStruct dynamically generates the parameter schema for DuckDuckGoTool.
// func (dt DuckDuckGoTool) GetParameterStruct() interface{} {
// 	return utils.GenerateJSONSchema(DuckDuckGoToolInput{})
// }
