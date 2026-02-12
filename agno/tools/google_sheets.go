package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const sheetsBaseURL = "https://sheets.googleapis.com/v4/spreadsheets"

// GoogleSheetsTool provides Google Sheets automation via the Sheets API.
// Requires a Google OAuth2 access token with Sheets scopes.
type GoogleSheetsTool struct {
	toolkit.Toolkit
	accessToken string
	httpClient  *http.Client
}

type SheetsReadParams struct {
	SpreadsheetID string `json:"spreadsheet_id" description:"The spreadsheet ID." required:"true"`
	Range         string `json:"range" description:"The A1 notation range to read (e.g., 'Sheet1!A1:D10')." required:"true"`
}

type SheetsWriteParams struct {
	SpreadsheetID string     `json:"spreadsheet_id" description:"The spreadsheet ID." required:"true"`
	Range         string     `json:"range" description:"The A1 notation range to write (e.g., 'Sheet1!A1')." required:"true"`
	Values        [][]string `json:"values" description:"2D array of values to write." required:"true"`
}

type SheetsAppendParams struct {
	SpreadsheetID string     `json:"spreadsheet_id" description:"The spreadsheet ID." required:"true"`
	Range         string     `json:"range" description:"The A1 notation range to append to (e.g., 'Sheet1!A1')." required:"true"`
	Values        [][]string `json:"values" description:"2D array of values to append." required:"true"`
}

type SheetsCreateParams struct {
	Title string `json:"title" description:"The title for the new spreadsheet." required:"true"`
}

// NewGoogleSheetsTool creates a new Google Sheets tool.
// If accessToken is empty, it reads from the GOOGLE_ACCESS_TOKEN environment variable.
func NewGoogleSheetsTool(accessToken string) *GoogleSheetsTool {
	if accessToken == "" {
		accessToken = os.Getenv("GOOGLE_ACCESS_TOKEN")
	}

	t := &GoogleSheetsTool{
		accessToken: accessToken,
		httpClient:  &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "GoogleSheetsTool"
	tk.Description = "Google Sheets automation: read, write, append data, and create spreadsheets."

	t.Toolkit = tk
	t.Toolkit.Register("ReadSheet", "Read data from a Google Sheet range.", t, t.ReadSheet, SheetsReadParams{})
	t.Toolkit.Register("WriteSheet", "Write data to a Google Sheet range.", t, t.WriteSheet, SheetsWriteParams{})
	t.Toolkit.Register("AppendSheet", "Append data to a Google Sheet.", t, t.AppendSheet, SheetsAppendParams{})
	t.Toolkit.Register("CreateSpreadsheet", "Create a new Google Spreadsheet.", t, t.CreateSpreadsheet, SheetsCreateParams{})

	return t
}

func (t *GoogleSheetsTool) doRequest(method, endpoint string, body interface{}) (interface{}, error) {
	if t.accessToken == "" {
		return nil, fmt.Errorf("GOOGLE_ACCESS_TOKEN not set")
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google sheets request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("google sheets API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	return result, nil
}

func (t *GoogleSheetsTool) ReadSheet(params SheetsReadParams) (interface{}, error) {
	endpoint := fmt.Sprintf("%s/%s/values/%s",
		sheetsBaseURL, params.SpreadsheetID, url.PathEscape(params.Range))
	return t.doRequest("GET", endpoint, nil)
}

func (t *GoogleSheetsTool) WriteSheet(params SheetsWriteParams) (interface{}, error) {
	endpoint := fmt.Sprintf("%s/%s/values/%s?valueInputOption=USER_ENTERED",
		sheetsBaseURL, params.SpreadsheetID, url.PathEscape(params.Range))
	body := map[string]interface{}{
		"values": params.Values,
	}
	return t.doRequest("PUT", endpoint, body)
}

func (t *GoogleSheetsTool) AppendSheet(params SheetsAppendParams) (interface{}, error) {
	endpoint := fmt.Sprintf("%s/%s/values/%s:append?valueInputOption=USER_ENTERED",
		sheetsBaseURL, params.SpreadsheetID, url.PathEscape(params.Range))
	body := map[string]interface{}{
		"values": params.Values,
	}
	return t.doRequest("POST", endpoint, body)
}

func (t *GoogleSheetsTool) CreateSpreadsheet(params SheetsCreateParams) (interface{}, error) {
	body := map[string]interface{}{
		"properties": map[string]interface{}{
			"title": params.Title,
		},
	}
	return t.doRequest("POST", sheetsBaseURL, body)
}

func (t *GoogleSheetsTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}
