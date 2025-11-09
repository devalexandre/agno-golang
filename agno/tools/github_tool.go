package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// GitHubTool provides GitHub integration capabilities
type GitHubTool struct {
	client *github.Client
	owner  string
	repo   string
}

// NewGitHubTool creates a new GitHub tool instance
func NewGitHubTool(token, owner, repo string) *GitHubTool {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return &GitHubTool{
		client: client,
		owner:  owner,
		repo:   repo,
	}
}

// GetName returns the tool name
func (g *GitHubTool) GetName() string {
	return "github"
}

// GetDescription returns the tool description
func (g *GitHubTool) GetDescription() string {
	return "GitHub integration tool for repository management, issues, pull requests, and code search"
}

// GetMethods returns available methods
func (g *GitHubTool) GetMethods() map[string]toolkit.Method {
	return map[string]toolkit.Method{
		"create_issue": {
			Receiver:  g,
			Function:  g.CreateIssue,
			ParamType: nil,
		},
		"list_issues": {
			Receiver:  g,
			Function:  g.ListIssues,
			ParamType: nil,
		},
		"get_issue": {
			Receiver:  g,
			Function:  g.GetIssue,
			ParamType: nil,
		},
		"update_issue": {
			Receiver:  g,
			Function:  g.UpdateIssue,
			ParamType: nil,
		},
		"close_issue": {
			Receiver:  g,
			Function:  g.CloseIssue,
			ParamType: nil,
		},
		"create_pull_request": {
			Receiver:  g,
			Function:  g.CreatePullRequest,
			ParamType: nil,
		},
		"list_pull_requests": {
			Receiver:  g,
			Function:  g.ListPullRequests,
			ParamType: nil,
		},
		"merge_pull_request": {
			Receiver:  g,
			Function:  g.MergePullRequest,
			ParamType: nil,
		},
		"search_code": {
			Receiver:  g,
			Function:  g.SearchCode,
			ParamType: nil,
		},
		"get_file_content": {
			Receiver:  g,
			Function:  g.GetFileContent,
			ParamType: nil,
		},
		"create_or_update_file": {
			Receiver:  g,
			Function:  g.CreateOrUpdateFile,
			ParamType: nil,
		},
		"list_commits": {
			Receiver:  g,
			Function:  g.ListCommits,
			ParamType: nil,
		},
		"get_repository_info": {
			Receiver:  g,
			Function:  g.GetRepositoryInfo,
			ParamType: nil,
		},
	}
}

// GetFunction returns a specific function
func (g *GitHubTool) GetFunction(methodName string) interface{} {
	methods := g.GetMethods()
	if method, exists := methods[methodName]; exists {
		return method.Function
	}
	return nil
}

// GetParameterStruct returns parameter structure for a method
func (g *GitHubTool) GetParameterStruct(methodName string) map[string]interface{} {
	schemas := map[string]map[string]interface{}{
		"create_issue": {
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Issue title",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Issue description",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"description": "Issue labels",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"assignees": map[string]interface{}{
					"type":        "array",
					"description": "Issue assignees",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []string{"title"},
		},
		"list_issues": {
			"type": "object",
			"properties": map[string]interface{}{
				"state": map[string]interface{}{
					"type":        "string",
					"description": "Issue state: open, closed, all",
					"default":     "open",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"description": "Filter by labels",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of issues to return",
					"default":     30,
				},
			},
		},
		"get_issue": {
			"type": "object",
			"properties": map[string]interface{}{
				"number": map[string]interface{}{
					"type":        "integer",
					"description": "Issue number",
				},
			},
			"required": []string{"number"},
		},
		"search_code": {
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results",
					"default":     10,
				},
			},
			"required": []string{"query"},
		},
	}

	if schema, exists := schemas[methodName]; exists {
		return schema
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// Execute executes a method with JSON input
func (g *GitHubTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	var params map[string]interface{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal input: %w", err)
		}
	}

	switch methodName {
	case "create_issue":
		return g.CreateIssue(params)
	case "list_issues":
		return g.ListIssues(params)
	case "get_issue":
		return g.GetIssue(params)
	case "update_issue":
		return g.UpdateIssue(params)
	case "close_issue":
		return g.CloseIssue(params)
	case "create_pull_request":
		return g.CreatePullRequest(params)
	case "list_pull_requests":
		return g.ListPullRequests(params)
	case "merge_pull_request":
		return g.MergePullRequest(params)
	case "search_code":
		return g.SearchCode(params)
	case "get_file_content":
		return g.GetFileContent(params)
	case "create_or_update_file":
		return g.CreateOrUpdateFile(params)
	case "list_commits":
		return g.ListCommits(params)
	case "get_repository_info":
		return g.GetRepositoryInfo(params)
	default:
		return nil, fmt.Errorf("unknown method: %s", methodName)
	}
}

// CreateIssue creates a new issue
func (g *GitHubTool) CreateIssue(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	title, _ := params["title"].(string)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	body, _ := params["body"].(string)

	issueRequest := &github.IssueRequest{
		Title: &title,
		Body:  &body,
	}

	// Add labels if provided
	if labelsRaw, ok := params["labels"].([]interface{}); ok {
		labels := make([]string, 0, len(labelsRaw))
		for _, l := range labelsRaw {
			if label, ok := l.(string); ok {
				labels = append(labels, label)
			}
		}
		issueRequest.Labels = &labels
	}

	// Add assignees if provided
	if assigneesRaw, ok := params["assignees"].([]interface{}); ok {
		assignees := make([]string, 0, len(assigneesRaw))
		for _, a := range assigneesRaw {
			if assignee, ok := a.(string); ok {
				assignees = append(assignees, assignee)
			}
		}
		issueRequest.Assignees = &assignees
	}

	issue, _, err := g.client.Issues.Create(ctx, g.owner, g.repo, issueRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return map[string]interface{}{
		"number":  issue.GetNumber(),
		"title":   issue.GetTitle(),
		"url":     issue.GetHTMLURL(),
		"state":   issue.GetState(),
		"created": issue.GetCreatedAt(),
	}, nil
}

// ListIssues lists issues
func (g *GitHubTool) ListIssues(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	state := "open"
	if s, ok := params["state"].(string); ok {
		state = s
	}

	limit := 30
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	opts := &github.IssueListByRepoOptions{
		State: state,
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	}

	// Add label filter if provided
	if labelsRaw, ok := params["labels"].([]interface{}); ok {
		labels := make([]string, 0, len(labelsRaw))
		for _, l := range labelsRaw {
			if label, ok := l.(string); ok {
				labels = append(labels, label)
			}
		}
		opts.Labels = labels
	}

	issues, _, err := g.client.Issues.ListByRepo(ctx, g.owner, g.repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}

	result := make([]map[string]interface{}, 0, len(issues))
	for _, issue := range issues {
		if issue.IsPullRequest() {
			continue // Skip pull requests
		}

		labels := make([]string, 0, len(issue.Labels))
		for _, label := range issue.Labels {
			labels = append(labels, label.GetName())
		}

		result = append(result, map[string]interface{}{
			"number":  issue.GetNumber(),
			"title":   issue.GetTitle(),
			"state":   issue.GetState(),
			"labels":  labels,
			"url":     issue.GetHTMLURL(),
			"created": issue.GetCreatedAt(),
			"updated": issue.GetUpdatedAt(),
		})
	}

	return result, nil
}

// GetIssue gets a specific issue
func (g *GitHubTool) GetIssue(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	number, ok := params["number"].(float64)
	if !ok {
		return nil, fmt.Errorf("number is required")
	}

	issue, _, err := g.client.Issues.Get(ctx, g.owner, g.repo, int(number))
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		labels = append(labels, label.GetName())
	}

	return map[string]interface{}{
		"number":   issue.GetNumber(),
		"title":    issue.GetTitle(),
		"body":     issue.GetBody(),
		"state":    issue.GetState(),
		"labels":   labels,
		"url":      issue.GetHTMLURL(),
		"created":  issue.GetCreatedAt(),
		"updated":  issue.GetUpdatedAt(),
		"comments": issue.GetComments(),
	}, nil
}

// UpdateIssue updates an issue
func (g *GitHubTool) UpdateIssue(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	number, ok := params["number"].(float64)
	if !ok {
		return nil, fmt.Errorf("number is required")
	}

	issueRequest := &github.IssueRequest{}

	if title, ok := params["title"].(string); ok {
		issueRequest.Title = &title
	}

	if body, ok := params["body"].(string); ok {
		issueRequest.Body = &body
	}

	if state, ok := params["state"].(string); ok {
		issueRequest.State = &state
	}

	issue, _, err := g.client.Issues.Edit(ctx, g.owner, g.repo, int(number), issueRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to update issue: %w", err)
	}

	return map[string]interface{}{
		"number":  issue.GetNumber(),
		"title":   issue.GetTitle(),
		"state":   issue.GetState(),
		"updated": issue.GetUpdatedAt(),
	}, nil
}

// CloseIssue closes an issue
func (g *GitHubTool) CloseIssue(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	number, ok := params["number"].(float64)
	if !ok {
		return nil, fmt.Errorf("number is required")
	}

	state := "closed"
	issueRequest := &github.IssueRequest{
		State: &state,
	}

	issue, _, err := g.client.Issues.Edit(ctx, g.owner, g.repo, int(number), issueRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to close issue: %w", err)
	}

	return map[string]interface{}{
		"number": issue.GetNumber(),
		"state":  issue.GetState(),
		"closed": issue.GetClosedAt(),
	}, nil
}

// CreatePullRequest creates a new pull request
func (g *GitHubTool) CreatePullRequest(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	title, _ := params["title"].(string)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	head, _ := params["head"].(string)
	if head == "" {
		return nil, fmt.Errorf("head branch is required")
	}

	base, _ := params["base"].(string)
	if base == "" {
		base = "main"
	}

	body, _ := params["body"].(string)

	newPR := &github.NewPullRequest{
		Title: &title,
		Head:  &head,
		Base:  &base,
		Body:  &body,
	}

	pr, _, err := g.client.PullRequests.Create(ctx, g.owner, g.repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	return map[string]interface{}{
		"number":  pr.GetNumber(),
		"title":   pr.GetTitle(),
		"url":     pr.GetHTMLURL(),
		"state":   pr.GetState(),
		"created": pr.GetCreatedAt(),
	}, nil
}

// ListPullRequests lists pull requests
func (g *GitHubTool) ListPullRequests(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	state := "open"
	if s, ok := params["state"].(string); ok {
		state = s
	}

	opts := &github.PullRequestListOptions{
		State: state,
	}

	prs, _, err := g.client.PullRequests.List(ctx, g.owner, g.repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests: %w", err)
	}

	result := make([]map[string]interface{}, 0, len(prs))
	for _, pr := range prs {
		result = append(result, map[string]interface{}{
			"number":  pr.GetNumber(),
			"title":   pr.GetTitle(),
			"state":   pr.GetState(),
			"url":     pr.GetHTMLURL(),
			"created": pr.GetCreatedAt(),
			"updated": pr.GetUpdatedAt(),
		})
	}

	return result, nil
}

// MergePullRequest merges a pull request
func (g *GitHubTool) MergePullRequest(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	number, ok := params["number"].(float64)
	if !ok {
		return nil, fmt.Errorf("number is required")
	}

	commitMessage, _ := params["commit_message"].(string)

	opts := &github.PullRequestOptions{
		CommitTitle: commitMessage,
	}

	result, _, err := g.client.PullRequests.Merge(ctx, g.owner, g.repo, int(number), commitMessage, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to merge pull request: %w", err)
	}

	return map[string]interface{}{
		"merged":  result.GetMerged(),
		"message": result.GetMessage(),
		"sha":     result.GetSHA(),
	}, nil
}

// SearchCode searches code in the repository
func (g *GitHubTool) SearchCode(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Add repo qualifier to query
	fullQuery := fmt.Sprintf("%s repo:%s/%s", query, g.owner, g.repo)

	limit := 10
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	}

	results, _, err := g.client.Search.Code(ctx, fullQuery, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search code: %w", err)
	}

	codeResults := make([]map[string]interface{}, 0, len(results.CodeResults))
	for _, result := range results.CodeResults {
		codeResults = append(codeResults, map[string]interface{}{
			"name":       result.GetName(),
			"path":       result.GetPath(),
			"url":        result.GetHTMLURL(),
			"repository": result.GetRepository().GetFullName(),
		})
	}

	return map[string]interface{}{
		"total_count": results.GetTotal(),
		"results":     codeResults,
	}, nil
}

// GetFileContent gets file content from repository
func (g *GitHubTool) GetFileContent(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	path, ok := params["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}

	ref := ""
	if r, ok := params["ref"].(string); ok {
		ref = r
	}

	opts := &github.RepositoryContentGetOptions{
		Ref: ref,
	}

	fileContent, _, _, err := g.client.Repositories.GetContents(ctx, g.owner, g.repo, path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %w", err)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, fmt.Errorf("failed to decode content: %w", err)
	}

	return map[string]interface{}{
		"path":    fileContent.GetPath(),
		"content": content,
		"sha":     fileContent.GetSHA(),
		"size":    fileContent.GetSize(),
	}, nil
}

// CreateOrUpdateFile creates or updates a file in the repository
func (g *GitHubTool) CreateOrUpdateFile(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	path, ok := params["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}

	content, ok := params["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content is required")
	}

	message, ok := params["message"].(string)
	if !ok || message == "" {
		message = fmt.Sprintf("Update %s", path)
	}

	opts := &github.RepositoryContentFileOptions{
		Message: &message,
		Content: []byte(content),
	}

	// Get current file SHA if it exists (for updates)
	if sha, ok := params["sha"].(string); ok && sha != "" {
		opts.SHA = &sha
	}

	// Try to get existing file SHA if not provided
	if opts.SHA == nil {
		fileContent, _, _, err := g.client.Repositories.GetContents(ctx, g.owner, g.repo, path, nil)
		if err == nil && fileContent != nil {
			sha := fileContent.GetSHA()
			opts.SHA = &sha
		}
	}

	result, _, err := g.client.Repositories.CreateFile(ctx, g.owner, g.repo, path, opts)
	if err != nil {
		// If file exists, try update
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "sha") {
			result, _, err = g.client.Repositories.UpdateFile(ctx, g.owner, g.repo, path, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to update file: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to create file: %w", err)
		}
	}

	return map[string]interface{}{
		"path":    result.GetContent().GetPath(),
		"sha":     result.GetContent().GetSHA(),
		"url":     result.GetContent().GetHTMLURL(),
		"message": "File created/updated successfully",
	}, nil
}

// ListCommits lists commits in the repository
func (g *GitHubTool) ListCommits(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	opts := &github.CommitsListOptions{}

	if ref, ok := params["ref"].(string); ok {
		opts.SHA = ref
	}

	if limit, ok := params["limit"].(float64); ok {
		opts.ListOptions.PerPage = int(limit)
	} else {
		opts.ListOptions.PerPage = 30
	}

	commits, _, err := g.client.Repositories.ListCommits(ctx, g.owner, g.repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list commits: %w", err)
	}

	result := make([]map[string]interface{}, 0, len(commits))
	for _, commit := range commits {
		result = append(result, map[string]interface{}{
			"sha":     commit.GetSHA(),
			"message": commit.GetCommit().GetMessage(),
			"author":  commit.GetCommit().GetAuthor().GetName(),
			"date":    commit.GetCommit().GetAuthor().GetDate(),
			"url":     commit.GetHTMLURL(),
		})
	}

	return result, nil
}

// GetRepositoryInfo gets repository information
func (g *GitHubTool) GetRepositoryInfo(params map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	repo, _, err := g.client.Repositories.Get(ctx, g.owner, g.repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	return map[string]interface{}{
		"name":           repo.GetName(),
		"full_name":      repo.GetFullName(),
		"description":    repo.GetDescription(),
		"url":            repo.GetHTMLURL(),
		"stars":          repo.GetStargazersCount(),
		"forks":          repo.GetForksCount(),
		"open_issues":    repo.GetOpenIssuesCount(),
		"language":       repo.GetLanguage(),
		"default_branch": repo.GetDefaultBranch(),
		"created":        repo.GetCreatedAt(),
		"updated":        repo.GetUpdatedAt(),
	}, nil
}
