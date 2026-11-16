// Package parallel ports Agno's ParallelTools: AI-optimised web search and
// extraction, deep-research tasks and monitors through the Parallel API
// (https://api.parallel.ai).
package parallel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Parallel API base URL.
const DefaultBaseURL = "https://api.parallel.ai"

// ParallelTools exposes Parallel's Search, Extract, Task and Monitor APIs.
type ParallelTools struct {
	toolkit.Toolkit
	apiKey                  string
	baseURL                 string
	httpClient              *http.Client
	client                  *restkit.Client
	taskClient              *restkit.Client
	enableSearch            bool
	enableExtract           bool
	enableTask              bool
	enableMonitor           bool
	all                     bool
	maxResults              int
	maxCharsPerResult       int
	mode                    string
	includeDomains          []string
	excludeDomains          []string
	maxAgeSeconds           *int
	disableCacheFallback    *bool
	defaultProcessor        string
	defaultMonitorProcessor string
	defaultMonitorFrequency string
	defaultTimeout          int
	defaultOutputSchema     interface{}
}

// Option configures ParallelTools.
type Option func(*ParallelTools)

// WithAPIKey sets the API key (default: PARALLEL_API_KEY).
func WithAPIKey(key string) Option { return func(p *ParallelTools) { p.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(p *ParallelTools) { p.baseURL = u } }

// WithTimeout sets the HTTP timeout. GetTaskResult blocks server-side for up
// to WithDefaultTimeout seconds and uses a longer timeout when needed.
func WithTimeout(d time.Duration) Option { return func(p *ParallelTools) { p.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(p *ParallelTools) { p.httpClient = c } }

// WithEnableSearch toggles ParallelSearch (default true).
func WithEnableSearch(v bool) Option { return func(p *ParallelTools) { p.enableSearch = v } }

// WithEnableExtract toggles ParallelExtract (default true).
func WithEnableExtract(v bool) Option { return func(p *ParallelTools) { p.enableExtract = v } }

// WithEnableTask toggles the Task API tools (default false).
func WithEnableTask(v bool) Option { return func(p *ParallelTools) { p.enableTask = v } }

// WithEnableMonitor toggles the Monitor API tools (default false).
func WithEnableMonitor(v bool) Option { return func(p *ParallelTools) { p.enableMonitor = v } }

// WithAll enables every tool.
func WithAll() Option { return func(p *ParallelTools) { p.all = true } }

// WithMaxResults sets the default search result count (default 10).
func WithMaxResults(n int) Option { return func(p *ParallelTools) { p.maxResults = n } }

// WithMaxCharsPerResult sets the default excerpt size (default 10000).
func WithMaxCharsPerResult(n int) Option { return func(p *ParallelTools) { p.maxCharsPerResult = n } }

// WithMode sets the search mode: "turbo", "basic" or "advanced" (default).
func WithMode(m string) Option { return func(p *ParallelTools) { p.mode = m } }

// WithIncludeDomains restricts search results to these domains.
func WithIncludeDomains(d ...string) Option { return func(p *ParallelTools) { p.includeDomains = d } }

// WithExcludeDomains excludes these domains from search results.
func WithExcludeDomains(d ...string) Option { return func(p *ParallelTools) { p.excludeDomains = d } }

// WithMaxAgeSeconds sets the fetch-policy cache age threshold (minimum 600).
func WithMaxAgeSeconds(n int) Option { return func(p *ParallelTools) { p.maxAgeSeconds = &n } }

// WithDisableCacheFallback sets the fetch-policy cache fallback behaviour.
func WithDisableCacheFallback(v bool) Option {
	return func(p *ParallelTools) { p.disableCacheFallback = &v }
}

// WithDefaultProcessor sets the Task API processor (default "base").
func WithDefaultProcessor(s string) Option { return func(p *ParallelTools) { p.defaultProcessor = s } }

// WithDefaultMonitorProcessor sets the Monitor API processor: "lite" (default) or "base".
func WithDefaultMonitorProcessor(s string) Option {
	return func(p *ParallelTools) { p.defaultMonitorProcessor = s }
}

// WithDefaultMonitorFrequency sets the monitor frequency: "1h", "1d" (default), "1w", "30d".
func WithDefaultMonitorFrequency(s string) Option {
	return func(p *ParallelTools) { p.defaultMonitorFrequency = s }
}

// WithDefaultTimeout sets the task-result wait in seconds (default 1800).
func WithDefaultTimeout(seconds int) Option {
	return func(p *ParallelTools) { p.defaultTimeout = seconds }
}

// WithDefaultOutputSchema sets the structured-output schema used by tasks and
// monitors: a string description or a map such as {"type": "auto"}.
func WithDefaultOutputSchema(schema interface{}) Option {
	return func(p *ParallelTools) { p.defaultOutputSchema = schema }
}

// SearchParams are the parameters of ParallelSearch.
type SearchParams struct {
	Objective         string   `json:"objective,omitempty" description:"Natural-language description of what the web search is trying to find."`
	SearchQueries     []string `json:"search_queries,omitempty" description:"Traditional keyword queries with optional search operators."`
	MaxResults        int      `json:"max_results,omitempty" description:"Upper bound on results returned. Overrides constructor default."`
	MaxCharsPerResult int      `json:"max_chars_per_result,omitempty" description:"Upper bound on total characters per url for excerpts."`
}

// ExtractParams are the parameters of ParallelExtract.
type ExtractParams struct {
	URLs                   []string `json:"urls" description:"List of public URLs to extract content from." required:"true"`
	Objective              string   `json:"objective,omitempty" description:"Search focus to guide content extraction."`
	SearchQueries          []string `json:"search_queries,omitempty" description:"Keywords for targeting relevant content."`
	MaxCharsPerExcerpt     int      `json:"max_chars_per_excerpt,omitempty" description:"Upper bound on characters per URL for excerpts."`
	FullContent            bool     `json:"full_content,omitempty" description:"Include complete page text."`
	MaxCharsForFullContent int      `json:"max_chars_for_full_content,omitempty" description:"Limit on characters per url. Only used when full_content is True."`
}

// CreateTaskParams are the parameters of CreateTask and CreateMonitor.
type CreateTaskParams struct {
	Query string `json:"query" description:"Natural language research query (e.g., \"What is Anthropic's latest funding?\")" required:"true"`
}

// RunIDParams identify a task run.
type RunIDParams struct {
	RunID string `json:"run_id" description:"The task run identifier from create_task()" required:"true"`
}

// CreateMonitorParams are the parameters of CreateMonitor.
type CreateMonitorParams struct {
	Query string `json:"query" description:"Search query to monitor (e.g., \"AI startup funding rounds\")" required:"true"`
}

// MonitorIDParams identify a monitor.
type MonitorIDParams struct {
	MonitorID string `json:"monitor_id" description:"The monitor's unique identifier" required:"true"`
}

// UpdateMonitorParams are the parameters of UpdateMonitor.
type UpdateMonitorParams struct {
	MonitorID string `json:"monitor_id" description:"The monitor's unique identifier" required:"true"`
	Frequency string `json:"frequency,omitempty" description:"New frequency (e.g., \"1h\", \"1d\", \"1w\", \"30d\")"`
	Query     string `json:"query,omitempty" description:"New search query (only for event_stream monitors)"`
}

// ListMonitorsParams are the parameters of ListMonitors.
type ListMonitorsParams struct {
	Status      string `json:"status,omitempty" description:"Filter by status: active or cancelled"`
	MonitorType string `json:"monitor_type,omitempty" description:"Filter by type: event_stream or snapshot"`
	Limit       int    `json:"limit,omitempty" description:"Maximum number of monitors to return (default: 100)"`
}

// MonitorEventsParams are the parameters of GetMonitorEvents.
type MonitorEventsParams struct {
	MonitorID          string `json:"monitor_id" description:"The monitor's unique identifier" required:"true"`
	IncludeCompletions bool   `json:"include_completions,omitempty" description:"Include runs with no changes (default: False)"`
	Limit              int    `json:"limit,omitempty" description:"Maximum number of events to return (default: 20, max: 100)"`
}

// New creates the Parallel toolkit.
func New(opts ...Option) *ParallelTools {
	p := &ParallelTools{
		baseURL:                 DefaultBaseURL,
		enableSearch:            true,
		enableExtract:           true,
		maxResults:              10,
		maxCharsPerResult:       10000,
		defaultProcessor:        "base",
		defaultMonitorProcessor: "lite",
		defaultMonitorFrequency: "1d",
		defaultTimeout:          1800,
	}
	p.Toolkit = toolkit.NewToolkit()
	p.Toolkit.Name = "parallel_tools"
	p.Toolkit.Description = "Search the web, extract pages, run deep-research tasks and monitors with Parallel."
	for _, o := range opts {
		o(p)
	}
	p.apiKey = restkit.Env(p.apiKey, "PARALLEL_API_KEY")
	if p.httpClient == nil {
		p.httpClient = p.Toolkit.HTTPClient()
	}
	p.client = &restkit.Client{
		HTTP:    p.httpClient,
		BaseURL: p.baseURL,
		Name:    "Parallel API",
		Headers: map[string]string{"x-api-key": p.apiKey},
	}
	// Task results block server-side for up to default_timeout seconds, so they
	// use a copy of the HTTP client with enough headroom.
	taskHTTP := *p.httpClient
	if need := time.Duration(p.defaultTimeout+30) * time.Second; taskHTTP.Timeout > 0 && taskHTTP.Timeout < need {
		taskHTTP.Timeout = need
	}
	tc := *p.client
	tc.HTTP = &taskHTTP
	p.taskClient = &tc
	tk := &p.Toolkit
	if p.all || p.enableSearch {
		tk.Register("ParallelSearch", "Search the web using Parallel's Search API with a natural language objective. You must provide at least one of objective or search_queries.", p, p.ParallelSearch, SearchParams{})
	}
	if p.all || p.enableExtract {
		tk.Register("ParallelExtract", "Extract content from specific URLs using Parallel's Extract API.", p, p.ParallelExtract, ExtractParams{})
	}
	if p.all || p.enableTask {
		tk.Register("CreateTask", "Create a research task without waiting for results. Use get_task_result() to retrieve later.", p, p.CreateTask, CreateTaskParams{})
		tk.Register("GetTaskStatus", "Check the status of a task without waiting for completion.", p, p.GetTaskStatus, RunIDParams{})
		tk.Register("GetTaskResult", "Get the result of a task. Blocks until task completes or default_timeout is reached.", p, p.GetTaskResult, RunIDParams{})
	}
	if p.all || p.enableMonitor {
		tk.Register("CreateMonitor", "Create a monitor to track a search query for changes over time.", p, p.CreateMonitor, CreateMonitorParams{})
		tk.Register("GetMonitor", "Retrieve a specific monitor by ID.", p, p.GetMonitor, MonitorIDParams{})
		tk.Register("UpdateMonitor", "Update a monitor's frequency or query. Cannot update cancelled monitors.", p, p.UpdateMonitor, UpdateMonitorParams{})
		tk.Register("ListMonitors", "List all monitors with optional filters.", p, p.ListMonitors, ListMonitorsParams{})
		tk.Register("GetMonitorEvents", "Get events (changes) detected by a monitor.", p, p.GetMonitorEvents, MonitorEventsParams{})
		tk.Register("CancelMonitor", "Cancel a monitor permanently. This action cannot be undone.", p, p.CancelMonitor, MonitorIDParams{})
	}
	return p
}

type obj = map[string]interface{}

func errJSON(msg string) string { return restkit.JSONString(obj{"error": msg}) }

func (p *ParallelTools) call(method, path string, q url.Values, body interface{}) (obj, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("PARALLEL_API_KEY not set. Please set the PARALLEL_API_KEY environment variable")
	}
	var out obj
	if err := p.client.Into(method, path, q, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *ParallelTools) fetchPolicy() obj {
	fp := obj{}
	if p.maxAgeSeconds != nil {
		fp["max_age_seconds"] = *p.maxAgeSeconds
	}
	if p.disableCacheFallback != nil {
		fp["disable_cache_fallback"] = *p.disableCacheFallback
	}
	return fp
}

// ParallelSearch runs a Parallel web search.
func (p *ParallelTools) ParallelSearch(params SearchParams) (interface{}, error) {
	if params.Objective == "" && len(params.SearchQueries) == 0 {
		return errJSON("Please provide at least one of: objective or search_queries"), nil
	}
	maxResults := p.maxResults
	if params.MaxResults > 0 {
		maxResults = params.MaxResults
	}
	body := obj{}
	if params.Objective != "" {
		body["objective"] = params.Objective
	}
	if len(params.SearchQueries) > 0 {
		body["search_queries"] = params.SearchQueries
	} else {
		body["search_queries"] = []string{params.Objective}
	}
	body["mode"] = "advanced"
	if p.mode != "" {
		body["mode"] = p.mode
	}
	adv := obj{"max_results": maxResults}
	maxChars := p.maxCharsPerResult
	if params.MaxCharsPerResult > 0 {
		maxChars = params.MaxCharsPerResult
	}
	if maxChars > 0 {
		adv["excerpt_settings"] = obj{"max_chars_per_result": maxChars}
	}
	sp := obj{}
	if len(p.includeDomains) > 0 {
		sp["include_domains"] = p.includeDomains
	}
	if len(p.excludeDomains) > 0 {
		sp["exclude_domains"] = p.excludeDomains
	}
	if len(sp) > 0 {
		adv["source_policy"] = sp
	}
	if fp := p.fetchPolicy(); len(fp) > 0 {
		adv["fetch_policy"] = fp
	}
	body["advanced_settings"] = adv
	res, err := p.call(http.MethodPost, "/v1/search", nil, body)
	if err != nil {
		return errJSON("Search failed: " + err.Error()), nil
	}
	return restkit.CompactJSON(res), nil
}

// ParallelExtract extracts content from URLs.
func (p *ParallelTools) ParallelExtract(params ExtractParams) (interface{}, error) {
	if len(params.URLs) == 0 {
		return errJSON("Please provide at least one URL to extract"), nil
	}
	body := obj{"urls": params.URLs}
	if params.Objective != "" {
		body["objective"] = params.Objective
	}
	if len(params.SearchQueries) > 0 {
		body["search_queries"] = params.SearchQueries
	}
	adv := obj{}
	excerpt := p.maxCharsPerResult
	if params.MaxCharsPerExcerpt > 0 {
		excerpt = params.MaxCharsPerExcerpt
	}
	if excerpt > 0 {
		adv["excerpt_settings"] = obj{"max_chars_per_result": excerpt}
	}
	if params.FullContent {
		if params.MaxCharsForFullContent > 0 {
			adv["full_content"] = obj{"max_chars_per_result": params.MaxCharsForFullContent}
		} else {
			adv["full_content"] = true
		}
	}
	if fp := p.fetchPolicy(); len(fp) > 0 {
		adv["fetch_policy"] = fp
	}
	if len(adv) > 0 {
		body["advanced_settings"] = adv
	}
	res, err := p.call(http.MethodPost, "/v1/extract", nil, body)
	if err != nil {
		return errJSON("Extract failed: " + err.Error()), nil
	}
	return restkit.CompactJSON(res), nil
}

// CreateTask starts a deep-research task run.
func (p *ParallelTools) CreateTask(params CreateTaskParams) (interface{}, error) {
	body := obj{"input": params.Query, "processor": p.defaultProcessor}
	if p.defaultOutputSchema != nil {
		body["task_spec"] = obj{"output_schema": p.defaultOutputSchema}
	}
	run, err := p.call(http.MethodPost, "/v1/tasks/runs", nil, body)
	if err != nil {
		return errJSON("Create task failed: " + err.Error()), nil
	}
	return restkit.JSONString(obj{
		"run_id":         run["run_id"],
		"status":         run["status"],
		"interaction_id": run["interaction_id"],
		"processor":      run["processor"],
		"is_active":      run["is_active"],
	}), nil
}

// GetTaskResult blocks until a task completes (or default_timeout elapses).
func (p *ParallelTools) GetTaskResult(params RunIDParams) (interface{}, error) {
	if p.apiKey == "" {
		return errJSON("Get result failed: PARALLEL_API_KEY not set. Please set the PARALLEL_API_KEY environment variable"), nil
	}
	q := url.Values{"api_timeout": {strconv.Itoa(p.defaultTimeout)}}
	var res obj
	err := p.taskClient.Into(http.MethodGet, "/v1/tasks/runs/"+url.PathEscape(params.RunID)+"/result", q, nil, &res)
	if err != nil {
		return errJSON("Get result failed: " + err.Error()), nil
	}
	run := restkit.Map(res, "run")
	out := obj{"run_id": params.RunID, "status": run["status"], "processor": run["processor"]}
	output := restkit.Map(res, "output")
	if v, ok := output["content"]; ok {
		out["content"] = v
	}
	if _, ok := output["basis"]; ok {
		out["basis"] = formatBasis(restkit.Slice(output, "basis"), true)
	}
	return restkit.JSONString(out), nil
}

func formatBasis(basis []map[string]interface{}, withExcerpts bool) []obj {
	out := []obj{}
	for _, b := range basis {
		cites := []obj{}
		for _, c := range restkit.Slice(b, "citations") {
			ci := obj{"url": c["url"], "title": c["title"]}
			if withExcerpts {
				ci["excerpts"] = c["excerpts"]
			}
			cites = append(cites, ci)
		}
		out = append(out, obj{"field": b["field"], "confidence": b["confidence"], "citations": cites})
	}
	return out
}

// GetTaskStatus returns a task run's status without waiting.
func (p *ParallelTools) GetTaskStatus(params RunIDParams) (interface{}, error) {
	run, err := p.call(http.MethodGet, "/v1/tasks/runs/"+url.PathEscape(params.RunID), nil, nil)
	if err != nil {
		return errJSON("Get status failed: " + err.Error()), nil
	}
	return restkit.JSONString(obj{
		"run_id":      run["run_id"],
		"status":      run["status"],
		"processor":   run["processor"],
		"is_active":   run["is_active"],
		"created_at":  run["created_at"],
		"modified_at": run["modified_at"],
	}), nil
}

func monitorInfo(m obj) obj {
	info := obj{
		"monitor_id":  m["monitor_id"],
		"type":        m["type"],
		"status":      m["status"],
		"frequency":   m["frequency"],
		"processor":   m["processor"],
		"created_at":  restkit.Str(m, "created_at"),
		"last_run_at": m["last_run_at"],
	}
	if restkit.Str(m, "type") == "event_stream" {
		if s := restkit.Map(m, "settings"); s != nil {
			if q, ok := s["query"]; ok {
				info["query"] = q
			}
		}
	}
	return info
}

// CreateMonitor creates an event-stream monitor for a query.
func (p *ParallelTools) CreateMonitor(params CreateMonitorParams) (interface{}, error) {
	settings := obj{"query": params.Query}
	if p.defaultOutputSchema != nil {
		settings["output_schema"] = p.defaultOutputSchema
	}
	m, err := p.call(http.MethodPost, "/v1/monitors", nil, obj{
		"type":      "event_stream",
		"frequency": p.defaultMonitorFrequency,
		"processor": p.defaultMonitorProcessor,
		"settings":  settings,
	})
	if err != nil {
		return errJSON("Create monitor failed: " + err.Error()), nil
	}
	info := monitorInfo(m)
	info["query"] = params.Query
	return restkit.JSONString(info), nil
}

// GetMonitor retrieves a monitor.
func (p *ParallelTools) GetMonitor(params MonitorIDParams) (interface{}, error) {
	m, err := p.call(http.MethodGet, "/v1/monitors/"+url.PathEscape(params.MonitorID), nil, nil)
	if err != nil {
		return errJSON("Get monitor failed: " + err.Error()), nil
	}
	return restkit.JSONString(monitorInfo(m)), nil
}

// UpdateMonitor changes a monitor's frequency or query.
func (p *ParallelTools) UpdateMonitor(params UpdateMonitorParams) (interface{}, error) {
	body := obj{}
	if params.Frequency != "" {
		body["frequency"] = params.Frequency
	}
	if params.Query != "" {
		body["type"] = "event_stream"
		body["settings"] = obj{"query": params.Query}
	}
	if len(body) == 0 {
		return errJSON("At least one of frequency or query must be provided"), nil
	}
	m, err := p.call(http.MethodPost, "/v1/monitors/"+url.PathEscape(params.MonitorID)+"/update", nil, body)
	if err != nil {
		return errJSON("Update monitor failed: " + err.Error()), nil
	}
	info := monitorInfo(m)
	delete(info, "created_at")
	delete(info, "last_run_at")
	info["updated"] = true
	return restkit.JSONString(info), nil
}

// ListMonitors lists monitors.
func (p *ParallelTools) ListMonitors(params ListMonitorsParams) (interface{}, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	q := url.Values{"limit": {strconv.Itoa(limit)}}
	if params.Status != "" {
		q.Set("status", params.Status)
	}
	if params.MonitorType != "" {
		q.Set("type", params.MonitorType)
	}
	res, err := p.call(http.MethodGet, "/v1/monitors", q, nil)
	if err != nil {
		return errJSON("List monitors failed: " + err.Error()), nil
	}
	monitors := []obj{}
	for _, m := range restkit.Slice(res, "monitors") {
		monitors = append(monitors, monitorInfo(m))
	}
	return restkit.JSONString(obj{"monitors": monitors, "has_more": res["next_cursor"] != nil}), nil
}

// CancelMonitor cancels a monitor permanently.
func (p *ParallelTools) CancelMonitor(params MonitorIDParams) (interface{}, error) {
	m, err := p.call(http.MethodPost, "/v1/monitors/"+url.PathEscape(params.MonitorID)+"/cancel", nil, nil)
	if err != nil {
		return errJSON("Cancel monitor failed: " + err.Error()), nil
	}
	return restkit.JSONString(obj{"monitor_id": m["monitor_id"], "status": m["status"], "cancelled": true}), nil
}

// GetMonitorEvents lists the events detected by a monitor.
func (p *ParallelTools) GetMonitorEvents(params MonitorEventsParams) (interface{}, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q := url.Values{"include_completions": {strconv.FormatBool(params.IncludeCompletions)}, "limit": {strconv.Itoa(limit)}}
	res, err := p.call(http.MethodGet, "/v1/monitors/"+url.PathEscape(params.MonitorID)+"/events", q, nil)
	if err != nil {
		return errJSON("Get events failed: " + err.Error()), nil
	}
	events := []obj{}
	for _, e := range restkit.Slice(res, "events") {
		ev := obj{
			"event_id":       e["event_id"],
			"event_type":     e["event_type"],
			"event_group_id": e["event_group_id"],
			"event_date":     e["event_date"],
		}
		if out := restkit.Map(e, "output"); out != nil {
			if v, ok := out["content"]; ok {
				ev["content"] = v
			}
			if _, ok := out["basis"]; ok {
				ev["basis"] = formatBasis(restkit.Slice(out, "basis"), false)
			}
		}
		events = append(events, ev)
	}
	return restkit.JSONString(obj{"events": events, "has_more": res["next_cursor"] != nil}), nil
}

// Execute implements toolkit.Tool.
func (p *ParallelTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return p.Toolkit.Execute(methodName, input)
}
