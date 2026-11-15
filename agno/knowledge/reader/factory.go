package reader

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

// Reader factory. Port of agno-python knowledge/reader/reader_factory.py:
// readers are created by key (CreateReader), by file extension or MIME type
// (ReaderForExtension) or by URL (ReaderForURL); custom readers can be added
// with RegisterReader. Every call returns a fresh instance.

// ReaderInfo describes a registered reader.
type ReaderInfo struct {
	Key         string
	Name        string
	Description string
	Extensions  []string
	New         func() Reader
}

var (
	registryMu sync.RWMutex
	registry   = map[string]ReaderInfo{}
)

func init() {
	builtin := []ReaderInfo{
		{Key: "pdf", Name: "PdfReader", Description: "Processes PDF documents (one document per page)", Extensions: []string{".pdf"}, New: func() Reader { return NewPDFReader() }},
		{Key: "csv", Name: "CsvReader", Description: "Parses CSV/TSV files, one labeled document per row", Extensions: []string{".csv", ".tsv"}, New: func() Reader { return NewCSVReader() }},
		{Key: "field_labeled_csv", Name: "FieldLabeledCsvReader", Description: "Converts CSV rows to field-labeled documents", Extensions: []string{".csv"}, New: func() Reader { return NewFieldLabeledCSVReader() }},
		{Key: "excel", Name: "ExcelReader", Description: "Processes Excel workbooks, one document per sheet", Extensions: []string{".xlsx", ".xlsm", ".xls"}, New: func() Reader { return NewExcelReader() }},
		{Key: "docx", Name: "DocxReader", Description: "Extracts text from Word documents", Extensions: []string{".docx"}, New: func() Reader { return NewDocxReader() }},
		{Key: "pptx", Name: "PptxReader", Description: "Extracts text from PowerPoint presentations", Extensions: []string{".pptx"}, New: func() Reader { return NewPPTXReader() }},
		{Key: "json", Name: "JsonReader", Description: "Processes JSON data structures", Extensions: []string{".json"}, New: func() Reader { return NewJSONReader() }},
		{Key: "markdown", Name: "MarkdownReader", Description: "Processes Markdown documentation", Extensions: []string{".md", ".markdown", ".mdx"}, New: func() Reader { return NewMarkdownReader() }},
		{Key: "text", Name: "TextReader", Description: "Handles plain text files", Extensions: []string{".txt", ".text", ".log"}, New: func() Reader { return NewTextReader() }},
		{Key: "website", Name: "WebsiteReader", Description: "Fetches a web page and extracts its text", New: func() Reader { return NewWebsiteReader() }},
		{Key: "firecrawl", Name: "FirecrawlReader", Description: "Scrapes or crawls websites with the Firecrawl API (FIRECRAWL_API_KEY)", New: func() Reader { return NewFirecrawlReader("") }},
		{Key: "tavily", Name: "TavilyReader", Description: "Extracts web content with the Tavily Extract API (TAVILY_API_KEY)", New: func() Reader { return NewTavilyReader("") }},
		{Key: "youtube", Name: "YouTubeReader", Description: "Fetches YouTube video transcripts", New: func() Reader { return NewYouTubeReader() }},
		{Key: "arxiv", Name: "ArxivReader", Description: "Downloads and processes arXiv papers", New: func() Reader { return NewArxivReader(0) }},
		{Key: "wikipedia", Name: "WikipediaReader", Description: "Fetches Wikipedia articles", New: func() Reader { return NewWikipediaReader("en") }},
		{Key: "web_search", Name: "WebSearchReader", Description: "Searches the web (DuckDuckGo) and reads the result pages", New: func() Reader { return NewWebSearchReader() }},
		{Key: "llms_txt", Name: "LLMsTxtReader", Description: "Reads llms.txt files, discovers linked documentation URLs, and fetches their content", New: func() Reader { return NewLLMsTxtReader() }},
		{Key: "sitemap", Name: "SitemapReader", Description: "Discovers a website's sitemap and loads one document per page with its source URL", New: func() Reader { return NewSitemapReader() }},
		{Key: "docling", Name: "DoclingReader", Description: "Converts many formats through a docling-serve instance", New: func() Reader { return NewDoclingReader("") }},
		{Key: "s3", Name: "S3Reader", Description: "Reads objects from S3 or S3-compatible storage", New: func() Reader { return NewS3Reader() }},
	}
	for _, info := range builtin {
		registry[info.Key] = info
	}
}

// RegisterReader adds (or replaces) a reader under key.
func RegisterReader(info ReaderInfo) {
	if info.Key == "" || info.New == nil {
		panic("reader: RegisterReader needs a Key and a New func")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[info.Key] = info
}

// CreateReader returns a new reader for key (e.g. "pdf", "sitemap").
func CreateReader(key string) (Reader, error) {
	registryMu.RLock()
	info, ok := registry[key]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("reader: unknown reader key %q", key)
	}
	return info.New(), nil
}

// GetReaderInfo returns the registration of key.
func GetReaderInfo(key string) (ReaderInfo, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	info, ok := registry[key]
	return info, ok
}

var urlReaderPriority = []string{"website", "firecrawl", "tavily", "youtube"}

// ReaderKeys returns every registered key: URL readers first (website,
// firecrawl, tavily, youtube), then the rest alphabetically.
func ReaderKeys() []string {
	registryMu.RLock()
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	registryMu.RUnlock()
	prio := func(k string) int {
		for i, p := range urlReaderPriority {
			if p == k {
				return i
			}
		}
		return len(urlReaderPriority)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi, pj := prio(keys[i]), prio(keys[j])
		if pi != pj {
			return pi < pj
		}
		return keys[i] < keys[j]
	})
	return keys
}

// ReaderForExtension returns a file-based Reader for the given file extension
// (case-insensitive, leading dot optional) or MIME type, or nil if none
// matches. Network/query readers are selected with ReaderForURL / CreateReader.
func ReaderForExtension(ext string) Reader {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext != "" && !strings.HasPrefix(ext, ".") && !strings.Contains(ext, "/") {
		ext = "." + ext
	}
	switch ext {
	case ".pdf", "application/pdf":
		return NewPDFReader()
	case ".txt", ".text", ".log", "text/plain":
		return NewTextReader()
	case ".md", ".markdown", ".mdx", "text/markdown":
		return NewMarkdownReader()
	case ".csv", ".tsv", "text/csv":
		return NewCSVReader()
	case ".xlsx", ".xlsm", ".xls",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.ms-excel":
		return NewExcelReader()
	case ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return NewDocxReader()
	case ".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return NewPPTXReader()
	case ".json", "application/json":
		return NewJSONReader()
	default:
		return nil
	}
}

// ReaderForURL picks a reader for a URL: YouTube URLs get the transcript
// reader, sitemap URLs (sitemap*.xml[.gz]) the sitemap reader, s3:// URIs
// the S3 reader, anything else the website reader.
func ReaderForURL(rawURL string) Reader {
	lower := strings.ToLower(rawURL)
	switch {
	case strings.HasPrefix(lower, "s3://"):
		return NewS3Reader()
	case strings.Contains(lower, "youtube.com") || strings.Contains(lower, "youtu.be"):
		return NewYouTubeReader()
	case IsSitemapURL(rawURL):
		return NewSitemapReader()
	default:
		return NewWebsiteReader()
	}
}

// parseHTMLString parses an HTML document from a string (test/helper utility).
func parseHTMLString(s string) (*html.Node, error) {
	return html.Parse(strings.NewReader(s))
}
