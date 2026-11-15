// Package mem0 ports Agno's Mem0Tools: user memory storage backed by the Mem0
// Platform REST API (https://api.mem0.ai).
//
// Python can also run Mem0 in-process ("Memory.from_config"); the Go port only
// supports the hosted platform API, which requires MEM0_API_KEY. Because Go
// tools receive no run context, the user ID is set on the toolkit
// (WithMem0UserID); a call made without it returns an error message, as in
// Python when no user_id can be resolved.
package mem0

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Mem0 Platform API base URL.
const DefaultBaseURL = "https://api.mem0.ai"

// Mem0Tools stores and retrieves user memories through Mem0.
type Mem0Tools struct {
	toolkit.Toolkit
	apiKey     string
	baseURL    string
	userID     string
	orgID      string
	projectID  string
	infer      bool
	httpClient *http.Client

	enableAdd, enableSearch, enableGetAll, enableDeleteAll bool
}

// Option configures Mem0Tools.
type Option func(*Mem0Tools)

// WithMem0APIKey sets the API key (default MEM0_API_KEY).
func WithMem0APIKey(k string) Option { return func(m *Mem0Tools) { m.apiKey = k } }

// WithMem0BaseURL overrides the API base URL.
func WithMem0BaseURL(u string) Option { return func(m *Mem0Tools) { m.baseURL = u } }

// WithMem0UserID sets the user whose memories are managed.
func WithMem0UserID(id string) Option { return func(m *Mem0Tools) { m.userID = id } }

// WithMem0OrgID sets the organisation ID (default MEM0_ORG_ID).
func WithMem0OrgID(id string) Option { return func(m *Mem0Tools) { m.orgID = id } }

// WithMem0ProjectID sets the project ID (default MEM0_PROJECT_ID).
func WithMem0ProjectID(id string) Option { return func(m *Mem0Tools) { m.projectID = id } }

// WithMem0Infer toggles Mem0 fact inference when adding memories (default true).
func WithMem0Infer(b bool) Option { return func(m *Mem0Tools) { m.infer = b } }

// WithMem0HTTPClient replaces the HTTP client.
func WithMem0HTTPClient(c *http.Client) Option { return func(m *Mem0Tools) { m.httpClient = c } }

// WithEnableAddMemory enables/disables AddMemory (default true).
func WithEnableAddMemory(b bool) Option { return func(m *Mem0Tools) { m.enableAdd = b } }

// WithEnableSearchMemory enables/disables SearchMemory (default true).
func WithEnableSearchMemory(b bool) Option { return func(m *Mem0Tools) { m.enableSearch = b } }

// WithEnableGetAllMemories enables/disables GetAllMemories (default true).
func WithEnableGetAllMemories(b bool) Option { return func(m *Mem0Tools) { m.enableGetAll = b } }

// WithEnableDeleteAllMemories enables/disables DeleteAllMemories (default true).
func WithEnableDeleteAllMemories(b bool) Option { return func(m *Mem0Tools) { m.enableDeleteAll = b } }

// AddMemoryParams are the parameters of AddMemory.
type AddMemoryParams struct {
	Content interface{} `json:"content" description:"The facts that should be stored: a string (e.g. 'I live in NYC') or an object of key/value facts." required:"true"`
}

// SearchMemoryParams are the parameters of SearchMemory.
type SearchMemoryParams struct {
	Query string `json:"query" description:"Semantic search query across the user's stored memories." required:"true"`
}

// NoParams is used by methods without arguments.
type NoParams struct{}

// NewMem0Tools creates the Mem0 toolkit.
func NewMem0Tools(opts ...Option) *Mem0Tools {
	m := &Mem0Tools{
		apiKey:          os.Getenv("MEM0_API_KEY"),
		orgID:           os.Getenv("MEM0_ORG_ID"),
		projectID:       os.Getenv("MEM0_PROJECT_ID"),
		baseURL:         DefaultBaseURL,
		infer:           true,
		enableAdd:       true,
		enableSearch:    true,
		enableGetAll:    true,
		enableDeleteAll: true,
	}
	m.Toolkit = toolkit.NewToolkit()
	m.Toolkit.Name = "mem0_tools"
	m.Toolkit.Description = "Store, search and manage long-term user memories with Mem0."
	for _, o := range opts {
		o(m)
	}
	if m.httpClient == nil {
		m.httpClient = m.Toolkit.HTTPClient()
	}
	if m.enableAdd {
		m.Toolkit.Register("AddMemory", "Add facts to the user's memory. Content may be a string or an object of facts.", m, m.AddMemory, AddMemoryParams{})
	}
	if m.enableSearch {
		m.Toolkit.Register("SearchMemory", "Semantic search for query across the user's stored memories.", m, m.SearchMemory, SearchMemoryParams{})
	}
	if m.enableGetAll {
		m.Toolkit.Register("GetAllMemories", "Return all memories for the current user as a JSON string.", m, m.GetAllMemories, NoParams{})
	}
	if m.enableDeleteAll {
		m.Toolkit.RegisterWithOptions("DeleteAllMemories", "Delete all memories associated with the current user.", m, m.DeleteAllMemories, NoParams{}, toolkit.WithConfirmation())
	}
	return m
}

func (m *Mem0Tools) client() *restapi.Client {
	return restapi.New(m.baseURL, m.httpClient, map[string]string{"Authorization": "Token " + m.apiKey})
}

func (m *Mem0Tools) scope(v map[string]interface{}) {
	if m.orgID != "" {
		v["org_id"] = m.orgID
	}
	if m.projectID != "" {
		v["project_id"] = m.projectID
	}
}

func (m *Mem0Tools) scopeQuery() url.Values {
	q := url.Values{"user_id": {m.userID}}
	if m.orgID != "" {
		q.Set("org_id", m.orgID)
	}
	if m.projectID != "" {
		q.Set("project_id", m.projectID)
	}
	return q
}

func (m *Mem0Tools) userErr(method string) string {
	if m.userID == "" {
		return fmt.Sprintf("Error in %s: A user_id must be provided in the method call.", method)
	}
	if m.apiKey == "" {
		return fmt.Sprintf("Error in %s: MEM0_API_KEY not set.", method)
	}
	return ""
}

// AddMemory adds facts to the user's memory.
func (m *Mem0Tools) AddMemory(p AddMemoryParams) (interface{}, error) {
	if e := m.userErr("add_memory"); e != "" {
		return e, nil
	}
	var content string
	switch c := p.Content.(type) {
	case string:
		content = c
	case nil:
		content = ""
	default:
		b, _ := json.Marshal(c)
		content = string(b)
	}
	body := map[string]interface{}{
		"messages": []map[string]string{{"role": "user", "content": content}},
		"user_id":  m.userID,
		"infer":    m.infer,
	}
	m.scope(body)
	data, err := m.client().Do(context.Background(), http.MethodPost, "/v1/memories/", nil, body)
	if err != nil {
		return fmt.Sprintf("Error adding memory: %v", err), nil
	}
	return compact(data), nil
}

// SearchMemory searches the user's memories.
func (m *Mem0Tools) SearchMemory(p SearchMemoryParams) (interface{}, error) {
	if e := m.userErr("search_memory"); e != "" {
		return e, nil
	}
	body := map[string]interface{}{"query": p.Query, "user_id": m.userID}
	m.scope(body)
	data, err := m.client().Do(context.Background(), http.MethodPost, "/v1/memories/search/", nil, body)
	if err != nil {
		return fmt.Sprintf("Error searching memory: %v", err), nil
	}
	return resultsList(data), nil
}

// GetAllMemories returns all memories of the user.
func (m *Mem0Tools) GetAllMemories(_ NoParams) (interface{}, error) {
	if e := m.userErr("get_all_memories"); e != "" {
		return e, nil
	}
	data, err := m.client().Do(context.Background(), http.MethodGet, "/v1/memories/", m.scopeQuery(), nil)
	if err != nil {
		return fmt.Sprintf("Error getting all memories: %v", err), nil
	}
	return resultsList(data), nil
}

// DeleteAllMemories deletes all memories of the user.
func (m *Mem0Tools) DeleteAllMemories(_ NoParams) (interface{}, error) {
	if e := m.userErr("delete_all_memories"); e != "" {
		return "Error deleting all memories: " + e, nil
	}
	if _, err := m.client().Do(context.Background(), http.MethodDelete, "/v1/memories/", m.scopeQuery(), nil); err != nil {
		return fmt.Sprintf("Error deleting all memories: %v", err), nil
	}
	return fmt.Sprintf("Successfully deleted all memories for user_id: %s.", m.userID), nil
}

func compact(data []byte) string {
	var v interface{}
	if json.Unmarshal(data, &v) != nil {
		return strings.TrimSpace(string(data))
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// resultsList unwraps {"results": [...]} or returns a list as-is, else "[]".
func resultsList(data []byte) string {
	var v interface{}
	if json.Unmarshal(data, &v) != nil {
		return "[]"
	}
	switch t := v.(type) {
	case []interface{}:
		b, _ := json.Marshal(t)
		return string(b)
	case map[string]interface{}:
		if r, ok := t["results"].([]interface{}); ok {
			b, _ := json.Marshal(r)
			return string(b)
		}
	}
	return "[]"
}
