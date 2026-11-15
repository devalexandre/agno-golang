// Package bitbucket ports Agno's BitbucketTools: repositories, commits, pull
// requests and issues of a Bitbucket Cloud workspace through the REST API 2.0
// (HTTP basic auth with username + app password or API token).
package bitbucket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultServerURL is the Bitbucket Cloud API host.
const DefaultServerURL = "api.bitbucket.org"

// DefaultAPIVersion is the Bitbucket API version.
const DefaultAPIVersion = "2.0"

// BitbucketTools works with one Bitbucket repository / workspace.
type BitbucketTools struct {
	toolkit.Toolkit
	serverURL  string
	apiVersion string
	baseURL    string
	username   string
	password   string
	token      string
	workspace  string
	repoSlug   string
	httpClient *http.Client
	client     *restkit.Client
}

// Option configures BitbucketTools.
type Option func(*BitbucketTools)

// WithServerURL sets the API host (default api.bitbucket.org); "https://" is
// prepended when no scheme is given.
func WithServerURL(s string) Option { return func(b *BitbucketTools) { b.serverURL = s } }

// WithAPIVersion sets the API version (default 2.0).
func WithAPIVersion(v string) Option { return func(b *BitbucketTools) { b.apiVersion = v } }

// WithBaseURL sets the full API base URL (overrides server URL and version).
func WithBaseURL(u string) Option { return func(b *BitbucketTools) { b.baseURL = u } }

// WithUsername sets the username (default: BITBUCKET_USERNAME).
func WithUsername(u string) Option { return func(b *BitbucketTools) { b.username = u } }

// WithPassword sets the app password (default: BITBUCKET_PASSWORD).
func WithPassword(p string) Option { return func(b *BitbucketTools) { b.password = p } }

// WithToken sets the API token, preferred over the password (default: BITBUCKET_TOKEN).
func WithToken(t string) Option { return func(b *BitbucketTools) { b.token = t } }

// WithWorkspace sets the workspace (required).
func WithWorkspace(w string) Option { return func(b *BitbucketTools) { b.workspace = w } }

// WithRepoSlug sets the repository slug (required).
func WithRepoSlug(r string) Option { return func(b *BitbucketTools) { b.repoSlug = r } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(b *BitbucketTools) { b.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(b *BitbucketTools) { b.httpClient = c } }

// CountParams are the parameters of the list methods.
type CountParams struct {
	Count int `json:"count,omitempty" description:"The number of items to retrieve. Default is 10, maximum 50."`
}

// NoParams is used by methods without arguments.
type NoParams struct{}

// CreateRepositoryParams are the parameters of CreateRepository.
type CreateRepositoryParams struct {
	Name        string `json:"name" description:"The name of the new repository." required:"true"`
	Project     string `json:"project,omitempty" description:"The key of the project to create the repository in."`
	IsPrivate   bool   `json:"is_private,omitempty" description:"Whether the repository is private."`
	Description string `json:"description,omitempty" description:"A short description of the repository."`
	Language    string `json:"language,omitempty" description:"The primary language of the repository."`
	HasIssues   bool   `json:"has_issues,omitempty" description:"Whether the repository has issues enabled."`
	HasWiki     bool   `json:"has_wiki,omitempty" description:"Whether the repository has a wiki enabled."`
}

// ListPullRequestsParams are the parameters of ListAllPullRequests.
type ListPullRequestsParams struct {
	State string `json:"state,omitempty" description:"The state of the pull requests to retrieve: OPEN, MERGED, DECLINED or SUPERSEDED. Default is OPEN."`
}

// PullRequestParams identify a pull request.
type PullRequestParams struct {
	PullRequestID int `json:"pull_request_id" description:"The ID of the pull request to retrieve." required:"true"`
}

// New creates the Bitbucket toolkit.
func New(opts ...Option) *BitbucketTools {
	b := &BitbucketTools{serverURL: DefaultServerURL, apiVersion: DefaultAPIVersion}
	b.Toolkit = toolkit.NewToolkit()
	b.Toolkit.Name = "bitbucket"
	b.Toolkit.Description = "Work with Bitbucket Cloud repositories, commits, pull requests and issues."
	for _, o := range opts {
		o(b)
	}
	b.username = restkit.Env(b.username, "BITBUCKET_USERNAME")
	b.password = restkit.Env(b.password, "BITBUCKET_PASSWORD")
	b.token = restkit.Env(b.token, "BITBUCKET_TOKEN")
	if b.baseURL == "" {
		server := b.serverURL
		if server == "" {
			server = DefaultServerURL
		}
		version := b.apiVersion
		if version == "" {
			version = DefaultAPIVersion
		}
		if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
			server = "https://" + server
		}
		b.baseURL = strings.TrimRight(server, "/") + "/" + version
	}
	if b.httpClient == nil {
		b.httpClient = b.Toolkit.HTTPClient()
	}
	authPassword := b.token
	if authPassword == "" {
		authPassword = b.password
	}
	b.client = &restkit.Client{HTTP: b.httpClient, BaseURL: b.baseURL, Name: "Bitbucket API", BasicUser: b.username, BasicPass: authPassword}

	b.Toolkit.Register("ListRepositories", "Get repositories in the workspace. Returns JSON.", b, b.ListRepositories, CountParams{})
	b.Toolkit.Register("GetRepositoryDetails", "Retrieve information about the configured repository. Returns JSON.", b, b.GetRepositoryDetails, NoParams{})
	b.Toolkit.Register("CreateRepository", "Create a new repository in the workspace. Returns JSON.", b, b.CreateRepository, CreateRepositoryParams{})
	b.Toolkit.Register("ListRepositoryCommits", "Retrieve commits of the repository. Returns JSON.", b, b.ListRepositoryCommits, CountParams{})
	b.Toolkit.Register("ListAllPullRequests", "Retrieve pull requests of the repository. Returns JSON.", b, b.ListAllPullRequests, ListPullRequestsParams{})
	b.Toolkit.Register("GetPullRequestDetails", "Retrieve a pull request of the repository. Returns JSON.", b, b.GetPullRequestDetails, PullRequestParams{})
	b.Toolkit.Register("GetPullRequestChanges", "Retrieve the diff of a pull request.", b, b.GetPullRequestChanges, PullRequestParams{})
	b.Toolkit.Register("ListIssues", "Retrieve issues of the repository. Returns JSON.", b, b.ListIssues, CountParams{})
	return b
}

func (b *BitbucketTools) check() string {
	switch {
	case b.username == "" || (b.token == "" && b.password == ""):
		return restkit.ErrorJSON("Username and password or token are required (BITBUCKET_USERNAME and BITBUCKET_TOKEN/BITBUCKET_PASSWORD)")
	case b.workspace == "":
		return restkit.ErrorJSON("Workspace is required")
	case b.repoSlug == "":
		return restkit.ErrorJSON("Repo slug is required")
	}
	return ""
}

func (b *BitbucketTools) repoPath(suffix string) string {
	return "/repositories/" + url.PathEscape(b.workspace) + "/" + url.PathEscape(b.repoSlug) + suffix
}

func (b *BitbucketTools) request(method, path string, q url.Values, body interface{}) (interface{}, string) {
	if msg := b.check(); msg != "" {
		return nil, msg
	}
	v, err := b.client.JSON(method, path, q, body)
	if err != nil {
		return nil, restkit.ErrorJSON(err.Error())
	}
	if v == nil {
		v = map[string]interface{}{}
	}
	return v, ""
}

func render(v interface{}, errStr string) (interface{}, error) {
	if errStr != "" {
		return errStr, nil
	}
	return restkit.JSONString(v), nil
}

func clampCount(n int) int {
	if n <= 0 {
		n = 10
	}
	if n > 50 {
		n = 50
	}
	return n
}

// ListRepositories lists repositories in the workspace.
func (b *BitbucketTools) ListRepositories(params CountParams) (interface{}, error) {
	count := clampCount(params.Count)
	q := url.Values{"page": {"1"}, "pagelen": {strconv.Itoa(count)}}
	return render(b.request(http.MethodGet, "/repositories/"+url.PathEscape(b.workspace), q, nil))
}

// GetRepositoryDetails returns the configured repository.
func (b *BitbucketTools) GetRepositoryDetails(NoParams) (interface{}, error) {
	return render(b.request(http.MethodGet, b.repoPath(""), nil, nil))
}

var slugInvalid = regexp.MustCompile(`[^a-z0-9._-]+`)

// CreateRepository creates a repository in the workspace. The repository slug
// is derived from name (Python posts to the configured repo_slug, which only
// works when it equals the new name).
func (b *BitbucketTools) CreateRepository(params CreateRepositoryParams) (interface{}, error) {
	if params.Name == "" {
		return restkit.ErrorJSON("name is required"), nil
	}
	payload := map[string]interface{}{
		"name":        params.Name,
		"scm":         "git",
		"is_private":  params.IsPrivate,
		"description": nullable(params.Description),
		"language":    nullable(params.Language),
		"has_issues":  params.HasIssues,
		"has_wiki":    params.HasWiki,
	}
	if params.Project != "" {
		payload["project"] = map[string]string{"key": params.Project}
	}
	slug := strings.Trim(slugInvalid.ReplaceAllString(strings.ToLower(params.Name), "-"), "-")
	path := "/repositories/" + url.PathEscape(b.workspace) + "/" + url.PathEscape(slug)
	return render(b.request(http.MethodPost, path, nil, payload))
}

func nullable(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ListRepositoryCommits lists commits, following pagination up to count.
func (b *BitbucketTools) ListRepositoryCommits(params CountParams) (interface{}, error) {
	count := clampCount(params.Count)
	v, errStr := b.request(http.MethodGet, b.repoPath("/commits"), url.Values{"pagelen": {strconv.Itoa(count)}}, nil)
	if errStr != "" {
		return errStr, nil
	}
	page, ok := v.(map[string]interface{})
	if !ok || restkit.Str(page, "next") == "" {
		return restkit.JSONString(v), nil
	}
	collected, _ := page["values"].([]interface{})
	for len(collected) < count && restkit.Str(page, "next") != "" {
		next := restkit.Str(page, "next")
		query := ""
		if i := strings.Index(next, "?"); i >= 0 {
			query = next[i+1:]
		}
		nv, errStr := b.request(http.MethodGet, b.repoPath("/commits?"+query), nil, nil)
		if errStr != "" {
			return errStr, nil
		}
		np, ok := nv.(map[string]interface{})
		if !ok {
			break
		}
		page = np
		if vals, ok := np["values"].([]interface{}); ok {
			collected = append(collected, vals...)
		}
	}
	if len(collected) > count {
		collected = collected[:count]
	}
	page["values"] = collected
	return restkit.JSONString(page), nil
}

// ListAllPullRequests lists pull requests by state.
func (b *BitbucketTools) ListAllPullRequests(params ListPullRequestsParams) (interface{}, error) {
	state := params.State
	switch state {
	case "OPEN", "MERGED", "DECLINED", "SUPERSEDED":
	default:
		state = "OPEN"
	}
	return render(b.request(http.MethodGet, b.repoPath("/pullrequests"), url.Values{"state": {state}}, nil))
}

// GetPullRequestDetails returns a pull request.
func (b *BitbucketTools) GetPullRequestDetails(params PullRequestParams) (interface{}, error) {
	return render(b.request(http.MethodGet, b.repoPath(fmt.Sprintf("/pullrequests/%d", params.PullRequestID)), nil, nil))
}

// GetPullRequestChanges returns the diff of a pull request (plain text).
func (b *BitbucketTools) GetPullRequestChanges(params PullRequestParams) (interface{}, error) {
	v, errStr := b.request(http.MethodGet, b.repoPath(fmt.Sprintf("/pullrequests/%d/diff", params.PullRequestID)), nil, nil)
	if errStr != "" {
		return errStr, nil
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	return restkit.JSONString(v), nil
}

// ListIssues lists repository issues.
func (b *BitbucketTools) ListIssues(params CountParams) (interface{}, error) {
	count := clampCount(params.Count)
	return render(b.request(http.MethodGet, b.repoPath("/issues"), url.Values{"pagelen": {strconv.Itoa(count)}}, nil))
}

// Execute implements toolkit.Tool.
func (b *BitbucketTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return b.Toolkit.Execute(methodName, input)
}
