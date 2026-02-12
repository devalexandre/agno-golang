package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"

	"golang.org/x/net/html"
)

// NewspaperTool provides article extraction and parsing from news websites.
// It fetches an article page and extracts the title, authors, text, published date, and images.
type NewspaperTool struct {
	toolkit.Toolkit
	httpClient *http.Client
}

// NewspaperGetArticleParams defines the parameters for the GetArticle method.
type NewspaperGetArticleParams struct {
	URL string `json:"url" description:"The URL of the news article to extract." required:"true"`
}

// Article represents the extracted content from a news article.
type Article struct {
	URL           string   `json:"url"`
	Title         string   `json:"title"`
	Authors       []string `json:"authors,omitempty"`
	Text          string   `json:"text"`
	PublishedDate string   `json:"published_date,omitempty"`
	TopImage      string   `json:"top_image,omitempty"`
	Images        []string `json:"images,omitempty"`
}

// NewNewspaperTool creates a new Newspaper tool.
func NewNewspaperTool() *NewspaperTool {
	t := &NewspaperTool{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "NewspaperTool"
	tk.Description = "Extract articles from news websites: title, authors, text, date, and images."

	t.Toolkit = tk
	t.Toolkit.Register("GetArticle", "Extract an article's content from a URL.", t, t.GetArticle, NewspaperGetArticleParams{})

	return t
}

// GetArticle extracts article content from a news URL.
func (t *NewspaperTool) GetArticle(params NewspaperGetArticleParams) (interface{}, error) {
	if params.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	resp, err := t.httpClient.Get(params.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch article: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error %d fetching article", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	article := Article{URL: params.URL}

	// Extract title
	article.Title = newspaperExtractTitle(doc)

	// Extract meta info
	article.Authors = newspaperExtractAuthors(doc)
	article.PublishedDate = newspaperExtractDate(doc)

	// Extract top image
	article.TopImage = htmlExtractMetaContent(doc, "og:image")

	// Extract all images
	article.Images = htmlExtractAllImages(doc)

	// Extract body text (from <article> or <p> tags)
	if articleNode := htmlFindElement(doc, "article"); articleNode != nil {
		article.Text = htmlExtractParagraphs(articleNode)
	} else if bodyNode := htmlFindElement(doc, "body"); bodyNode != nil {
		article.Text = htmlExtractParagraphs(bodyNode)
	}

	output, err := json.Marshal(article)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal article: %w", err)
	}

	return string(output), nil
}

func newspaperExtractTitle(doc *html.Node) string {
	if title := htmlExtractTitle(doc); title != "" {
		if idx := strings.LastIndex(title, " | "); idx > 0 {
			return strings.TrimSpace(title[:idx])
		}
		if idx := strings.LastIndex(title, " - "); idx > 0 {
			return strings.TrimSpace(title[:idx])
		}
		return title
	}
	return htmlExtractMetaContent(doc, "og:title")
}

func newspaperExtractAuthors(doc *html.Node) []string {
	var authors []string
	for _, prop := range []string{"author", "article:author", "dc.creator"} {
		if author := htmlExtractMetaContent(doc, prop); author != "" {
			authors = append(authors, author)
		}
	}
	return authors
}

func newspaperExtractDate(doc *html.Node) string {
	for _, prop := range []string{"article:published_time", "datePublished", "date", "pubdate"} {
		if date := htmlExtractMetaContent(doc, prop); date != "" {
			return date
		}
	}
	return ""
}

// Execute implements the toolkit.Tool interface.
func (t *NewspaperTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}
