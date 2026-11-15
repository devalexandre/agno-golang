// Package apify ports Agno's ApifyTools: every configured Apify Actor is
// registered as its own tool whose JSON schema comes from the Actor's input
// schema, and calls run the Actor through the Apify REST API
// (https://api.apify.com/v2).
package apify

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Apify API base URL.
const DefaultBaseURL = "https://api.apify.com/v2"

// MaxDescriptionLen caps Actor and parameter descriptions.
const MaxDescriptionLen = 350

// DefaultTimeout bounds Actor runs (Python's REQUESTS_TIMEOUT_SECS). Apify's
// synchronous run endpoint itself waits at most 300 seconds.
const DefaultTimeout = 300 * time.Second

// ApifyTools exposes Apify Actors as tools.
type ApifyTools struct {
	toolkit.Toolkit
	apiToken   string
	baseURL    string
	actors     []string
	httpClient *http.Client
	client     *restkit.Client
}

// Option configures ApifyTools.
type Option func(*ApifyTools)

// WithAPIToken sets the Apify API token (default: APIFY_API_TOKEN).
func WithAPIToken(token string) Option { return func(a *ApifyTools) { a.apiToken = token } }

// WithAPIKey is an alias of WithAPIToken.
func WithAPIKey(token string) Option { return WithAPIToken(token) }

// WithActors sets the Actor IDs (e.g. "apify/rag-web-browser") registered as tools.
func WithActors(actors ...string) Option {
	return func(a *ApifyTools) { a.actors = append(a.actors, actors...) }
}

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(a *ApifyTools) { a.baseURL = u } }

// WithTimeout sets the HTTP timeout (default 300s).
func WithTimeout(d time.Duration) Option { return func(a *ApifyTools) { a.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(a *ApifyTools) { a.httpClient = c } }

// ActorInput captures the free-form input of an Actor tool call.
type ActorInput struct {
	values map[string]interface{}
}

// UnmarshalJSON keeps every argument the model sent.
func (in *ActorInput) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	// Params are sometimes nested under a "kwargs" key.
	if inner, ok := m["kwargs"].(map[string]interface{}); ok && len(m) == 1 {
		m = inner
	}
	in.values = m
	return nil
}

// Values returns the Actor input.
func (in ActorInput) Values() map[string]interface{} {
	if in.values == nil {
		return map[string]interface{}{}
	}
	return in.values
}

// NewActorInput builds an ActorInput (useful when calling tools directly).
func NewActorInput(v map[string]interface{}) ActorInput { return ActorInput{values: v} }

// New creates the Apify toolkit and registers the configured Actors. Actors
// whose metadata cannot be fetched are logged and skipped, like in Python.
func New(opts ...Option) *ApifyTools {
	a := &ApifyTools{baseURL: DefaultBaseURL}
	a.Toolkit = toolkit.NewToolkit()
	a.Toolkit.Name = "ApifyTools"
	a.Toolkit.Description = "Run Apify Actors for web scraping, crawling and data extraction."
	a.Toolkit.Timeout = DefaultTimeout
	for _, o := range opts {
		o(a)
	}
	a.apiToken = restkit.Env(a.apiToken, "APIFY_API_TOKEN")
	if a.httpClient == nil {
		a.httpClient = a.Toolkit.HTTPClient()
	}
	a.client = &restkit.Client{
		HTTP:    a.httpClient,
		BaseURL: a.baseURL,
		Name:    "Apify API",
		Headers: map[string]string{"Authorization": "Bearer " + a.apiToken, "User-Agent": "agno-golang; Origin/agno"},
	}
	for _, id := range a.actors {
		if err := a.RegisterActor(id); err != nil {
			slog.Error("failed to register Apify Actor", "actor", id, "error", err)
		}
	}
	return a
}

// ActorIDToToolName turns an Actor ID into a valid tool name.
func ActorIDToToolName(actorID string) string {
	var b strings.Builder
	b.WriteString("apify_actor_")
	for _, r := range actorID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// actorPath encodes an Actor ID for URLs ("user/name" -> "user~name").
func actorPath(actorID string) string {
	return url.PathEscape(strings.ReplaceAll(actorID, "/", "~"))
}

// RegisterActor fetches an Actor's latest build and registers it as a tool.
func (a *ApifyTools) RegisterActor(actorID string) error {
	if a.apiToken == "" {
		return fmt.Errorf("APIFY_API_TOKEN environment variable or api token option must be set")
	}
	build, err := a.latestBuild(actorID)
	if err != nil {
		return err
	}
	def := restkit.Map(build, "actorDefinition")
	desc := restkit.Str(def, "description")
	if len([]rune(desc)) > MaxDescriptionLen {
		desc = restkit.Truncate(desc, MaxDescriptionLen) + "...(TRUNCATED, TOO LONG)"
	}
	input := restkit.Map(def, "input")
	if input == nil {
		return fmt.Errorf("input schema not found in the Actor build for Actor: %s", actorID)
	}
	props, required := pruneActorInputSchema(input)

	var doc strings.Builder
	doc.WriteString(desc)
	doc.WriteString("\n\nArgs:\n")
	for _, name := range sortedKeys(props) {
		p := props[name].(map[string]interface{})
		typ := restkit.Str(p, "type")
		if typ == "" {
			typ = "any"
		}
		pd := restkit.Str(p, "description")
		if pd == "" {
			pd = "No description available"
		}
		marker := "(optional)"
		if contains(required, name) {
			marker = "(required)"
		}
		fmt.Fprintf(&doc, "    %s (%s): %s %s\n", name, typ, marker, pd)
	}
	doc.WriteString("\nReturns:\n    str: JSON string containing the Actor's output dataset\n")

	toolName := ActorIDToToolName(actorID)
	fn := func(in ActorInput) (interface{}, error) { return a.RunActor(actorID, in.Values()), nil }
	a.Toolkit.Register(toolName, doc.String(), a, fn, ActorInput{})

	// Replace the reflected schema with the Actor's own input schema.
	methods := a.Toolkit.GetMethods()
	full := a.Toolkit.Name + "_" + toolName
	if m, ok := methods[full]; ok {
		m.Schema = propsToJSONSchema(props, required)
		methods[full] = m
	}
	return nil
}

func (a *ApifyTools) latestBuild(actorID string) (map[string]interface{}, error) {
	var actor struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := a.client.Into(http.MethodGet, "/acts/"+actorPath(actorID), nil, nil, &actor); err != nil {
		return nil, fmt.Errorf("actor %s not found: %w", actorID, err)
	}
	objID := restkit.Str(actor.Data, "id")
	if objID == "" {
		return nil, fmt.Errorf("failed to get the Actor object ID for %s", actorID)
	}
	var build struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := a.client.Into(http.MethodGet, "/acts/"+url.PathEscape(objID)+"/builds/default", nil, nil, &build); err != nil {
		return nil, fmt.Errorf("failed to get the latest build of the Actor %s: %w", actorID, err)
	}
	if build.Data == nil {
		return nil, fmt.Errorf("failed to get the latest build data of the Actor %s", actorID)
	}
	return build.Data, nil
}

// RunActor runs an Actor synchronously and returns its dataset items as JSON.
// Errors are returned as [{"error": "..."}] like the Python toolkit.
func (a *ApifyTools) RunActor(actorID string, input map[string]interface{}) string {
	if input == nil {
		input = map[string]interface{}{}
	}
	q := url.Values{"clean": {"true"}}
	items, err := a.client.JSON(http.MethodPost, "/acts/"+actorPath(actorID)+"/run-sync-get-dataset-items", q, input)
	if err != nil {
		msg := fmt.Sprintf("Error running Apify Actor %s: %v", actorID, err)
		return restkit.CompactJSON([]map[string]string{{"error": msg}})
	}
	if items == nil {
		items = []interface{}{}
	}
	return restkit.CompactJSON(items)
}

func pruneActorInputSchema(input map[string]interface{}) (map[string]interface{}, []string) {
	props := restkit.Map(input, "properties")
	var required []string
	if req, ok := input["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}
	out := map[string]interface{}{}
	for name, metaV := range props {
		meta, _ := metaV.(map[string]interface{})
		p := map[string]interface{}{}
		if d := restkit.Str(meta, "description"); d != "" {
			if len([]rune(d)) > MaxDescriptionLen {
				d = restkit.Truncate(d, MaxDescriptionLen) + "..."
			}
			p["description"] = d
		}
		for _, k := range []string{"type", "default", "prefill", "enum"} {
			if v, ok := meta[k]; ok && truthy(v) {
				p[k] = v
			}
		}
		out[name] = p
	}
	return out, required
}

func truthy(v interface{}) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String, reflect.Slice, reflect.Map:
		return rv.Len() > 0
	case reflect.Bool:
		return rv.Bool()
	case reflect.Float64:
		return rv.Float() != 0
	}
	return true
}

func jsonTypeOf(v interface{}) string {
	switch v.(type) {
	case string:
		return "string"
	case float64, int:
		return "number"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case bool:
		return "boolean"
	case nil:
		return "null"
	}
	return "string"
}

func inferArrayItemType(p map[string]interface{}) string {
	if t := restkit.Str(restkit.Map(p, "items"), "type"); t != "" {
		return t
	}
	for _, k := range []string{"prefill", "default"} {
		if arr, ok := p[k].([]interface{}); ok && len(arr) > 0 {
			return jsonTypeOf(arr[0])
		}
	}
	return "string"
}

func propsToJSONSchema(props map[string]interface{}, required []string) map[string]interface{} {
	if required == nil {
		required = []string{}
	}
	properties := map[string]interface{}{}
	for key, vv := range props {
		value := vv.(map[string]interface{})
		ps := map[string]interface{}{}
		propType := restkit.Str(value, "type")
		if e, ok := value["enum"]; ok {
			ps["enum"] = e
		}
		if d, ok := value["default"]; ok {
			ps["default"] = d
		} else if d, ok := value["prefill"]; ok {
			ps["default"] = d
		}
		if d, ok := value["description"]; ok {
			ps["description"] = d
		}
		editor := restkit.Str(value, "editor")
		switch {
		case propType == "object" && editor == "proxy":
			ps["type"] = "object"
			ps["properties"] = map[string]interface{}{
				"useApifyProxy": map[string]interface{}{
					"title":       "Use Apify Proxy",
					"type":        "boolean",
					"description": "Whether to use Apify Proxy - ALWAYS SET TO TRUE.",
					"default":     true,
					"examples":    []interface{}{true},
				},
			}
			ps["required"] = []string{"useApifyProxy"}
		case propType == "array" && editor == "requestListSources":
			ps["type"] = "array"
			ps["items"] = map[string]interface{}{
				"type":        "object",
				"title":       "Request list source",
				"description": "Request list source",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"title": "URL", "type": "string", "description": "URL of the request list source"},
				},
			}
		case propType == "array":
			title := restkit.Str(value, "title")
			if title == "" {
				title = "Item"
			}
			ps["type"] = "array"
			ps["items"] = map[string]interface{}{"type": inferArrayItemType(value), "title": title, "description": "Item"}
		case propType == "object":
			ps["type"] = "object"
			if def, ok := value["default"]; ok {
				src, _ := value["properties"].(map[string]interface{})
				if src == nil {
					src, _ = def.(map[string]interface{})
				}
				sub := map[string]interface{}{}
				for k, v := range src {
					t := jsonTypeOf(v)
					if m, ok := v.(map[string]interface{}); ok {
						t = restkit.Str(m, "type")
					}
					sub[k] = map[string]interface{}{"type": t}
				}
				ps["properties"] = sub
			}
		default:
			if propType != "" {
				ps["type"] = propType
			}
		}
		properties[key] = ps
	}
	return map[string]interface{}{"type": "object", "properties": properties, "required": required}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Execute implements toolkit.Tool.
func (a *ApifyTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return a.Toolkit.Execute(methodName, input)
}
