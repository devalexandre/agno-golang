package trafilatura

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const page = `<html lang="en"><head><title>Doc Title</title>
<meta name="author" content="Jane Doe"><meta property="og:site_name" content="Example">
<meta name="description" content="A test page"><meta name="keywords" content="go, agno">
<script>var x = 1;</script></head>
<body><nav>Menu Home About</nav><header>Site header</header>
<article><h1>Main Heading</h1><p>First paragraph.</p><p>Second <b>bold</b> paragraph.</p>
<div class="comments">A comment</div><a href="/other">Other page</a></article>
<footer>Copyright</footer></body></html>`

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
}

func TestExtractText(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	tk := New(WithIncludeComments(false))
	out, err := tk.ExtractText(ExtractTextParams{URL: srv.URL + "/a"})
	if err != nil {
		t.Fatal(err)
	}
	s := out.(string)
	for _, want := range []string{"Main Heading", "First paragraph.", "Second bold paragraph."} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %q", want, s)
		}
	}
	for _, bad := range []string{"Menu", "Copyright", "var x", "A comment", "Site header"} {
		if strings.Contains(s, bad) {
			t.Fatalf("unexpected %q in %q", bad, s)
		}
	}
	md, _ := tk.ExtractText(ExtractTextParams{URL: srv.URL + "/a", OutputFormat: "markdown"})
	if !strings.Contains(md.(string), "# Main Heading") {
		t.Fatalf("markdown heading missing: %q", md)
	}
	miss, _ := tk.ExtractText(ExtractTextParams{URL: srv.URL + "/missing"})
	if !strings.HasPrefix(miss.(string), "Error: Could not fetch content from URL") {
		t.Fatalf("unexpected: %v", miss)
	}
}

func TestExtractMetadataOnlyViaExecute(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	tk := New()
	out, err := tk.Execute("trafilatura_tools_ExtractMetadataOnly", json.RawMessage(`{"url":"`+srv.URL+`/a"}`))
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(out.(string)), &meta); err != nil {
		t.Fatalf("expected JSON by default: %v (%s)", err, out)
	}
	if meta["title"] != "Doc Title" || meta["author"] != "Jane Doe" || meta["sitename"] != "Example" || meta["language"] != "en" {
		t.Fatalf("unexpected metadata: %v", meta)
	}
	if tags, _ := meta["tags"].([]interface{}); len(tags) != 2 {
		t.Fatalf("unexpected tags: %v", meta["tags"])
	}
}

func TestHtmlToTextAndBatchAndCrawl(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	tk := New(WithAll())
	txt, _ := tk.Execute("trafilatura_tools_HtmlToText", json.RawMessage(`{"html_content":"<p>Hello <script>x()</script>world</p>"}`))
	if txt != "Hello world" {
		t.Fatalf("unexpected: %q", txt)
	}
	out, _ := tk.ExtractBatch(ExtractBatchParams{URLs: []string{srv.URL + "/a", srv.URL + "/missing"}})
	var batch struct {
		Successful int      `json:"successful_extractions"`
		Failed     int      `json:"failed_extractions"`
		FailedURLs []string `json:"failed_urls"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Successful != 1 || batch.Failed != 1 || batch.FailedURLs[0] != srv.URL+"/missing" {
		t.Fatalf("unexpected batch: %s", out)
	}
	crawl, _ := tk.CrawlWebsite(CrawlWebsiteParams{HomepageURL: srv.URL + "/"})
	if !strings.Contains(crawl.(string), srv.URL+"/other") {
		t.Fatalf("crawl missing link: %s", crawl)
	}
}
