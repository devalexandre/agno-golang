package exa

import (
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// ExaTool is the Tool wrapper to use in Agent
type ExaTool struct {
	client *Client
	toolkit.Toolkit
}

// NewExaTool initializes the tool and registers the methods
func NewExaTool(apiKey string) toolkit.Tool {
	tk := toolkit.NewToolkit()
	tk.Name = "ExaTool"
	tk.Description = "Toolkit for Exa API: search, get contents, find similar documents, and answer generation."

	exaTool := &ExaTool{
		client:  NewClient(apiKey),
		Toolkit: tk,
	}

	// ✅ Register all methods with their specific input types
	exaTool.Toolkit.Register("SearchExa", "Search Exa API for documents", exaTool, exaTool.SearchExa, SearchExaInput{})
	exaTool.Toolkit.Register("GetContents", "Get contents from URLs", exaTool, exaTool.GetContents, GetContentsInput{})
	exaTool.Toolkit.Register("FindSimilar", "Find similar documents", exaTool, exaTool.FindSimilar, FindSimilarInput{})
	exaTool.Toolkit.Register("ExaAnswer", "Generate answers using Exa", exaTool, exaTool.ExaAnswer, ExaAnswerInput{})

	return exaTool
}

// ========================= Input Structs =========================

type SearchExaInput struct {
	Query      string `json:"query" description:"The search query." required:"true"`
	NumResults int    `json:"num_results,omitempty" description:"Number of results to retrieve."`
	Category   string `json:"category,omitempty" description:"Category filter for the search."`
	Text       bool   `json:"text,omitempty" description:"Include full text in results."`
}

type GetContentsInput struct {
	URLs []string `json:"urls" description:"List of URLs to fetch contents from." required:"true"`
}

type FindSimilarInput struct {
	URL string `json:"url" description:"A single URL to find related documents." required:"true"`
}

type ExaAnswerInput struct {
	Query string `json:"query" description:"Question to answer using Exa." required:"true"`
	Model string `json:"model,omitempty" description:"Model to use. Options: exa, exa-pro."`
	Text  bool   `json:"text,omitempty" description:"Include full text in answer results."`
}

// ========================= Methods =========================

func (e *ExaTool) SearchExa(params SearchExaInput) (interface{}, error) {
	req := SearchRequest{
		Query:      params.Query,
		NumResults: params.NumResults,
		Category:   params.Category,
		Text:       params.Text,
	}

	return e.client.Search(req)
}

func (e *ExaTool) GetContents(params GetContentsInput) (interface{}, error) {
	return e.client.GetContents(params.URLs)
}

func (e *ExaTool) FindSimilar(params FindSimilarInput) (interface{}, error) {
	return e.client.FindSimilar(params.URL)
}

func (e *ExaTool) ExaAnswer(params ExaAnswerInput) (interface{}, error) {
	return e.client.Answer(params.Query, params.Model, params.Text)
}
