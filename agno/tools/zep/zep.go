// Package zep ports Agno's ZepTools: conversation memory and user knowledge
// graph search backed by the Zep Cloud REST API (https://api.getzep.com).
//
// As in Python, a session (thread) ID and a user ID are generated when not
// supplied; the user and thread are created lazily on the first tool call
// rather than in the constructor so construction never performs I/O.
package zep

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Zep Cloud API base URL.
const DefaultBaseURL = "https://api.getzep.com/api/v2"

// DefaultInstructions mirrors Python's DEFAULT_INSTRUCTIONS.
const DefaultInstructions = `You have access to the users memories stored in Zep. You can interact with them using the following tools:
- ` + "`add_zep_message`" + `: Add a message to the Zep session memory. Use this to add messages to the Zep session memory.
- ` + "`get_zep_memory`" + `: Get the memory for the current Zep session. Use this to get the memory for the current Zep session.
- ` + "`search_zep_memory`" + `: Search the Zep user graph for relevant facts. Use this to search the Zep user graph for relevant facts.

Guidelines:
- Use ` + "`add_zep_message`" + ` tool to add relevant messages to the users memories. You can use this tool multiple times to add multiple messages.
- Use ` + "`get_zep_memory`" + ` tool to get the memory for the current Zep session for additional context. This will give you a entire context of the user's memories with relevant facts.
`

// ZepTools manages Zep thread memory and graph search.
type ZepTools struct {
	toolkit.Toolkit
	apiKey                  string
	baseURL                 string
	sessionID               string
	userID                  string
	userProvided            bool
	ignoreAssistantMessages bool
	instructions            string
	httpClient              *http.Client

	enableAdd, enableGet, enableSearch bool

	mu          sync.Mutex
	initialized bool
}

// Option configures ZepTools.
type Option func(*ZepTools)

// WithZepAPIKey sets the API key (default ZEP_API_KEY).
func WithZepAPIKey(k string) Option { return func(z *ZepTools) { z.apiKey = k } }

// WithZepBaseURL overrides the API base URL.
func WithZepBaseURL(u string) Option { return func(z *ZepTools) { z.baseURL = u } }

// WithZepSessionID sets the thread (session) ID; one is generated otherwise.
func WithZepSessionID(id string) Option { return func(z *ZepTools) { z.sessionID = id } }

// WithZepUserID sets the user ID; a new user is created otherwise.
func WithZepUserID(id string) Option {
	return func(z *ZepTools) { z.userID, z.userProvided = id, id != "" }
}

// WithZepIgnoreAssistantMessages excludes assistant messages from graph ingestion.
func WithZepIgnoreAssistantMessages(b bool) Option {
	return func(z *ZepTools) { z.ignoreAssistantMessages = b }
}

// WithZepInstructions overrides the toolkit instructions.
func WithZepInstructions(s string) Option { return func(z *ZepTools) { z.instructions = s } }

// WithZepHTTPClient replaces the HTTP client.
func WithZepHTTPClient(c *http.Client) Option { return func(z *ZepTools) { z.httpClient = c } }

// WithEnableAddZepMessage toggles AddZepMessage (default true).
func WithEnableAddZepMessage(b bool) Option { return func(z *ZepTools) { z.enableAdd = b } }

// WithEnableGetZepMemory toggles GetZepMemory (default true).
func WithEnableGetZepMemory(b bool) Option { return func(z *ZepTools) { z.enableGet = b } }

// WithEnableSearchZepMemory toggles SearchZepMemory (default true).
func WithEnableSearchZepMemory(b bool) Option { return func(z *ZepTools) { z.enableSearch = b } }

// AddZepMessageParams are the parameters of AddZepMessage.
type AddZepMessageParams struct {
	Role    string `json:"role" description:"The role of the message sender (e.g., 'user', 'assistant', 'system')." required:"true"`
	Content string `json:"content" description:"The text content of the message." required:"true"`
}

// GetZepMemoryParams are the parameters of GetZepMemory.
type GetZepMemoryParams struct {
	MemoryType string `json:"memory_type,omitempty" description:"The type of memory to retrieve ('context', 'messages'). Default: context."`
}

// SearchZepMemoryParams are the parameters of SearchZepMemory.
type SearchZepMemoryParams struct {
	Query       string `json:"query" description:"The search term to find relevant facts or nodes." required:"true"`
	SearchScope string `json:"search_scope,omitempty" description:"The scope of the search: 'edges' (facts, default) or 'nodes'."`
}

// NewZepTools creates the Zep toolkit.
func NewZepTools(opts ...Option) *ZepTools {
	z := &ZepTools{
		apiKey:       os.Getenv("ZEP_API_KEY"),
		baseURL:      DefaultBaseURL,
		enableAdd:    true,
		enableGet:    true,
		enableSearch: true,
	}
	z.Toolkit = toolkit.NewToolkit()
	z.Toolkit.Name = "zep_tools"
	z.Toolkit.Description = "Store conversation messages in Zep and retrieve user memory context and facts."
	for _, o := range opts {
		o(z)
	}
	if z.instructions == "" {
		z.instructions = "<Memory Instructions>\n" + DefaultInstructions + "\n</Memory Instructions>"
	}
	if z.sessionID == "" {
		z.sessionID = newUUID()
	}
	if z.userID == "" {
		z.userID = "user-" + newUUID()
	}
	if z.httpClient == nil {
		z.httpClient = z.Toolkit.HTTPClient()
	}
	if z.enableAdd {
		z.Toolkit.Register("AddZepMessage", "Adds a message to the current Zep session memory.", z, z.AddZepMessage, AddZepMessageParams{})
	}
	if z.enableGet {
		z.Toolkit.Register("GetZepMemory", "Retrieves the memory for the current Zep session ('context' or 'messages').", z, z.GetZepMemory, GetZepMemoryParams{})
	}
	if z.enableSearch {
		z.Toolkit.Register("SearchZepMemory", "Searches the Zep knowledge graph for relevant facts (edges) or nodes.", z, z.SearchZepMemory, SearchZepMemoryParams{})
	}
	return z
}

// Instructions returns the toolkit instructions.
func (z *ZepTools) Instructions() string { return z.instructions }

// SessionID returns the Zep thread ID used by the toolkit.
func (z *ZepTools) SessionID() string { return z.sessionID }

// UserID returns the Zep user ID used by the toolkit.
func (z *ZepTools) UserID() string { return z.userID }

func (z *ZepTools) client() *restapi.Client {
	return restapi.New(z.baseURL, z.httpClient, map[string]string{"Authorization": "Api-Key " + z.apiKey})
}

// initialize ensures the user and thread exist (idempotent).
func (z *ZepTools) initialize(ctx context.Context) error {
	z.mu.Lock()
	defer z.mu.Unlock()
	if z.initialized {
		return nil
	}
	if z.apiKey == "" {
		return errors.New("no Zep API key provided; set ZEP_API_KEY")
	}
	c := z.client()
	createUser := !z.userProvided
	if z.userProvided {
		_, err := c.Do(ctx, http.MethodGet, "/users/"+url.PathEscape(z.userID), nil, nil)
		var apiErr *restapi.APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			createUser = true
		} else if err != nil {
			return err
		}
	}
	if createUser {
		if _, err := c.Do(ctx, http.MethodPost, "/users", nil, map[string]string{"user_id": z.userID}); err != nil {
			return fmt.Errorf("failed to create user %s: %w", z.userID, err)
		}
	}
	// The thread may already exist; ignore errors as Python does.
	_, _ = c.Do(ctx, http.MethodPost, "/threads", nil, map[string]string{"thread_id": z.sessionID, "user_id": z.userID})
	z.initialized = true
	return nil
}

// AddZepMessage adds a message to the current thread.
func (z *ZepTools) AddZepMessage(p AddZepMessageParams) (interface{}, error) {
	ctx := context.Background()
	if err := z.initialize(ctx); err != nil {
		return "Error: Zep client/session not initialized.", nil
	}
	body := map[string]interface{}{
		"messages": []map[string]string{{"role": p.Role, "content": p.Content, "role_type": p.Role}},
	}
	if z.ignoreAssistantMessages {
		body["ignore_roles"] = []string{"assistant"}
	}
	if _, err := z.client().Do(ctx, http.MethodPost, "/threads/"+url.PathEscape(z.sessionID)+"/messages", nil, body); err != nil {
		return fmt.Sprintf("Error adding message: %v", err), nil
	}
	return fmt.Sprintf("Message from '%s' added successfully to session %s.", p.Role, z.sessionID), nil
}

// GetZepMemory returns the thread context or messages.
func (z *ZepTools) GetZepMemory(p GetZepMemoryParams) (interface{}, error) {
	ctx := context.Background()
	if err := z.initialize(ctx); err != nil {
		return "Error: Zep client/session not initialized.", nil
	}
	memoryType := p.MemoryType
	if memoryType == "" {
		memoryType = "context"
	}
	thread := "/threads/" + url.PathEscape(z.sessionID)
	switch memoryType {
	case "context":
		var out struct {
			Context string `json:"context"`
		}
		if err := z.client().JSON(ctx, http.MethodGet, thread+"/context", url.Values{"mode": {"basic"}}, nil, &out); err != nil {
			return fmt.Sprintf("Error getting memory for session %s", z.sessionID), nil
		}
		if out.Context == "" {
			return "No context available.", nil
		}
		return out.Context, nil
	case "messages":
		var out struct {
			Messages []map[string]interface{} `json:"messages"`
		}
		if err := z.client().JSON(ctx, http.MethodGet, thread+"/messages", nil, nil, &out); err != nil {
			return fmt.Sprintf("Error getting memory for session %s", z.sessionID), nil
		}
		if len(out.Messages) == 0 {
			return "No messages available.", nil
		}
		b, _ := json.Marshal(out.Messages)
		return string(b), nil
	default:
		return fmt.Sprintf("Unsupported memory_type requested: %s. Returning empty string.", memoryType), nil
	}
}

// SearchZepMemory searches the user's knowledge graph.
func (z *ZepTools) SearchZepMemory(p SearchZepMemoryParams) (interface{}, error) {
	ctx := context.Background()
	if err := z.initialize(ctx); err != nil {
		return "Error: Zep client/user not initialized.", nil
	}
	scope := p.SearchScope
	if scope == "" {
		scope = "edges"
	}
	var out struct {
		Edges []struct {
			Fact string `json:"fact"`
		} `json:"edges"`
		Nodes []struct {
			Name    string `json:"name"`
			Summary string `json:"summary"`
		} `json:"nodes"`
	}
	body := map[string]string{"query": p.Query, "user_id": z.userID, "scope": scope}
	if err := z.client().JSON(ctx, http.MethodPost, "/graph/search", nil, body, &out); err != nil {
		return fmt.Sprintf("Error searching graph: %v", err), nil
	}
	var sb strings.Builder
	switch {
	case scope == "edges" && len(out.Edges) > 0:
		fmt.Fprintf(&sb, "Found %d facts:", len(out.Edges))
		for _, e := range out.Edges {
			sb.WriteString("\n- " + e.Fact)
		}
		return sb.String(), nil
	case scope == "nodes" && len(out.Nodes) > 0:
		fmt.Fprintf(&sb, "Found %d nodes:", len(out.Nodes))
		for _, n := range out.Nodes {
			fmt.Fprintf(&sb, "\n- %s: %s", n.Name, n.Summary)
		}
		return sb.String(), nil
	}
	return fmt.Sprintf("No %s found for query: %s", scope, p.Query), nil
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
