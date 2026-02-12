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

const driveBaseURL = "https://www.googleapis.com/drive/v3"

// GoogleDriveTool provides Google Drive integration for managing files and folders.
// Requires a Google OAuth2 access token with Drive scopes.
type GoogleDriveTool struct {
	toolkit.Toolkit
	accessToken string
	httpClient  *http.Client
}

type DriveListFilesParams struct {
	Query      string `json:"query,omitempty" description:"Drive search query (e.g., \"name contains 'report'\", \"mimeType = 'application/pdf'\")."`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of files. Default: 10."`
}

type DriveDownloadParams struct {
	FileID string `json:"file_id" description:"The Google Drive file ID." required:"true"`
}

type DriveCreateFolderParams struct {
	Name     string `json:"name" description:"Folder name." required:"true"`
	ParentID string `json:"parent_id,omitempty" description:"Parent folder ID. If empty, creates in root."`
}

type DriveDeleteParams struct {
	FileID string `json:"file_id" description:"The file or folder ID to delete." required:"true"`
}

// NewGoogleDriveTool creates a new Google Drive tool.
// If accessToken is empty, it reads from GOOGLE_ACCESS_TOKEN environment variable.
func NewGoogleDriveTool(accessToken string) *GoogleDriveTool {
	if accessToken == "" {
		accessToken = os.Getenv("GOOGLE_ACCESS_TOKEN")
	}

	t := &GoogleDriveTool{
		accessToken: accessToken,
		httpClient:  &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "GoogleDriveTool"
	tk.Description = "Google Drive file management: list, download, create folders, and delete files."

	t.Toolkit = tk
	t.Toolkit.Register("ListFiles", "List files in Google Drive.", t, t.ListFiles, DriveListFilesParams{})
	t.Toolkit.Register("DownloadFile", "Get metadata for a Google Drive file.", t, t.DownloadFile, DriveDownloadParams{})
	t.Toolkit.Register("CreateFolder", "Create a folder in Google Drive.", t, t.CreateFolder, DriveCreateFolderParams{})
	t.Toolkit.RegisterWithOptions("DeleteFile", "Delete a file from Google Drive.", t, t.DeleteFile, DriveDeleteParams{},
		toolkit.WithConfirmation(),
	)

	return t
}

func (t *GoogleDriveTool) doRequest(method, endpoint string, body interface{}) (interface{}, error) {
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
		return nil, fmt.Errorf("google drive request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("google drive API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if len(respBody) == 0 {
		return map[string]interface{}{"status": "ok"}, nil
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	return result, nil
}

func (t *GoogleDriveTool) ListFiles(params DriveListFilesParams) (interface{}, error) {
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}

	endpoint := fmt.Sprintf("%s/files?pageSize=%d&fields=files(id,name,mimeType,size,modifiedTime)", driveBaseURL, maxResults)
	if params.Query != "" {
		endpoint += "&q=" + url.QueryEscape(params.Query)
	}

	return t.doRequest("GET", endpoint, nil)
}

func (t *GoogleDriveTool) DownloadFile(params DriveDownloadParams) (interface{}, error) {
	endpoint := fmt.Sprintf("%s/files/%s?fields=id,name,mimeType,size,modifiedTime,webViewLink,webContentLink",
		driveBaseURL, params.FileID)
	return t.doRequest("GET", endpoint, nil)
}

func (t *GoogleDriveTool) CreateFolder(params DriveCreateFolderParams) (interface{}, error) {
	body := map[string]interface{}{
		"name":     params.Name,
		"mimeType": "application/vnd.google-apps.folder",
	}
	if params.ParentID != "" {
		body["parents"] = []string{params.ParentID}
	}

	return t.doRequest("POST", driveBaseURL+"/files", body)
}

func (t *GoogleDriveTool) DeleteFile(params DriveDeleteParams) (interface{}, error) {
	endpoint := fmt.Sprintf("%s/files/%s", driveBaseURL, params.FileID)
	return t.doRequest("DELETE", endpoint, nil)
}

func (t *GoogleDriveTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}
