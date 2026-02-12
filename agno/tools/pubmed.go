package tools

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const (
	pubmedSearchURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi"
	pubmedFetchURL  = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/efetch.fcgi"
)

// PubMedTool provides search for scientific/medical articles on PubMed.
type PubMedTool struct {
	toolkit.Toolkit
	maxResults int
	httpClient *http.Client
}

// PubMedSearchParams defines the parameters for the Search method.
type PubMedSearchParams struct {
	Query      string `json:"query" description:"The search query for PubMed articles." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of results. Default: 5."`
}

// NewPubMedTool creates a new PubMed tool.
func NewPubMedTool(maxResults int) *PubMedTool {
	if maxResults <= 0 {
		maxResults = 5
	}

	t := &PubMedTool{
		maxResults: maxResults,
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "PubMedTool"
	tk.Description = "Search for scientific and medical articles on PubMed."

	t.Toolkit = tk
	t.Toolkit.Register("Search", "Search PubMed for scientific articles.", t, t.Search, PubMedSearchParams{})

	return t
}

type pubmedSearchResult struct {
	IDList struct {
		ID []string `xml:"Id"`
	} `xml:"IdList"`
}

type pubmedArticleSet struct {
	Articles []pubmedArticle `xml:"PubmedArticle"`
}

type pubmedArticle struct {
	MedlineCitation struct {
		PMID struct {
			Value string `xml:",chardata"`
		} `xml:"PMID"`
		Article struct {
			ArticleTitle string `xml:"ArticleTitle"`
			Abstract     struct {
				AbstractText []string `xml:"AbstractText"`
			} `xml:"Abstract"`
			AuthorList struct {
				Author []struct {
					LastName string `xml:"LastName"`
					ForeName string `xml:"ForeName"`
				} `xml:"Author"`
			} `xml:"AuthorList"`
			Journal struct {
				Title string `xml:"Title"`
				PubDate struct {
					Year  string `xml:"Year"`
					Month string `xml:"Month"`
				} `xml:"JournalIssue>PubDate"`
			} `xml:"Journal"`
		} `xml:"Article"`
	} `xml:"MedlineCitation"`
}

// Search searches PubMed for articles matching the query.
func (t *PubMedTool) Search(params PubMedSearchParams) (interface{}, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = t.maxResults
	}

	// Step 1: Search for article IDs
	searchURL := fmt.Sprintf("%s?db=pubmed&term=%s&retmax=%d&retmode=xml",
		pubmedSearchURL, url.QueryEscape(params.Query), maxResults)

	resp, err := t.httpClient.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("pubmed search failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var searchResult pubmedSearchResult
	if err := xml.Unmarshal(body, &searchResult); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	if len(searchResult.IDList.ID) == 0 {
		return "No articles found.", nil
	}

	// Step 2: Fetch article details
	ids := strings.Join(searchResult.IDList.ID, ",")
	fetchURL := fmt.Sprintf("%s?db=pubmed&id=%s&retmode=xml", pubmedFetchURL, ids)

	resp2, err := t.httpClient.Get(fetchURL)
	if err != nil {
		return nil, fmt.Errorf("pubmed fetch failed: %w", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read fetch response: %w", err)
	}

	var articleSet pubmedArticleSet
	if err := xml.Unmarshal(body2, &articleSet); err != nil {
		return nil, fmt.Errorf("failed to parse articles: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d articles for '%s':\n\n", len(articleSet.Articles), params.Query))

	for i, art := range articleSet.Articles {
		pmid := art.MedlineCitation.PMID.Value
		title := art.MedlineCitation.Article.ArticleTitle
		journal := art.MedlineCitation.Article.Journal.Title
		year := art.MedlineCitation.Article.Journal.PubDate.Year

		var authors []string
		for _, a := range art.MedlineCitation.Article.AuthorList.Author {
			authors = append(authors, a.ForeName+" "+a.LastName)
		}

		abstract := strings.Join(art.MedlineCitation.Article.Abstract.AbstractText, " ")
		if len(abstract) > 300 {
			abstract = abstract[:297] + "..."
		}

		sb.WriteString(strconv.Itoa(i+1) + ". " + title + "\n")
		sb.WriteString("   Authors: " + strings.Join(authors, ", ") + "\n")
		sb.WriteString("   Journal: " + journal + "\n")
		if year != "" {
			sb.WriteString("   Year: " + year + "\n")
		}
		sb.WriteString("   PMID: " + pmid + "\n")
		sb.WriteString("   Link: https://pubmed.ncbi.nlm.nih.gov/" + pmid + "/\n")
		if abstract != "" {
			sb.WriteString("   Abstract: " + abstract + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// Execute implements the toolkit.Tool interface.
func (t *PubMedTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}
