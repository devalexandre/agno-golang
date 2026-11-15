package reader

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func gz(t *testing.T, s string) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write([]byte(s))
	w.Close()
	return b.Bytes()
}

func newSitemapServer(t *testing.T) *httptest.Server {
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "User-agent: *\nSitemap: %s/sitemap_index.xml\n", srv.URL)
	})
	mux.HandleFunc("/sitemap_index.xml", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<?xml version="1.0"?><sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+
			`<sitemap><loc>%s/pages.xml.gz</loc></sitemap><sitemap><loc>%s/broken.xml</loc></sitemap>`+
			`<sitemap><loc>https://other.example/sitemap.xml</loc></sitemap></sitemapindex>`, srv.URL, srv.URL)
	})
	mux.HandleFunc("/pages.xml.gz", func(w http.ResponseWriter, r *http.Request) {
		w.Write(gz(t, fmt.Sprintf(`<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">`+
			`<url><loc>%[1]s/docs/</loc><image:image><image:loc>%[1]s/img.png</image:loc></image:image></url>`+
			`<url><loc>%[1]s/docs/index.html</loc></url>`+
			`<url><loc>%[1]s/notes.txt</loc></url>`+
			`<url><loc>%[1]s/missing</loc></url>`+
			`<url><loc>https://elsewhere.example/page</loc></url></urlset>`, srv.URL)))
	})
	mux.HandleFunc("/broken.xml", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 500) })
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Docs Home</title></head><body><nav>menu</nav><main><h1>Welcome</h1><p>Read the docs.</p></main><footer>foot</footer></body></html>`)
	})
	mux.HandleFunc("/notes.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "plain notes")
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestSitemapReader(t *testing.T) {
	srv := newSitemapServer(t)
	r := NewSitemapReader()
	r.SourceHeader = true
	docs, err := r.Read(context.Background(), srv.URL+"/docs/")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 3 {
		for _, d := range docs {
			t.Logf("%s %q %v", d.ID, d.Content, d.Metadata)
		}
		t.Fatalf("expected 3 pages (dedup + host filter), got %d", len(docs))
	}
	home := docs[0]
	if home.ID != srv.URL+"/docs" || home.Metadata["title"] != "Docs Home" || home.Metadata["source"] != "sitemap" {
		t.Fatalf("home doc: %s %v", home.ID, home.Metadata)
	}
	if !strings.HasPrefix(home.Content, "# Docs Home\nSource: "+srv.URL+"/docs\n\n") ||
		!strings.Contains(home.Content, "Welcome\nRead the docs.") || strings.Contains(home.Content, "menu") {
		t.Fatalf("home content: %q", home.Content)
	}
	if !strings.Contains(docs[1].Content, "plain notes") {
		t.Fatalf("notes: %q", docs[1].Content)
	}
	if docs[2].Metadata["error"] != "HTTP 404" || docs[2].Content != "" {
		t.Fatalf("missing page should carry its error: %v", docs[2].Metadata)
	}
	if home.Metadata["discovery_incomplete"] != true {
		t.Fatal("broken child sitemap should mark discovery incomplete")
	}

	r.MaxPages = 1
	pages, sm, incomplete := r.Discover(context.Background(), srv.URL+"/docs/")
	if len(pages) != 1 || sm != srv.URL+"/sitemap_index.xml" || !incomplete {
		t.Fatalf("capped discovery: %v %s %v", pages, sm, incomplete)
	}
}

func TestSitemapReaderNoSitemapAndPublicOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/page" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><article>Only page</article></body></html>`)
	}))
	defer srv.Close()
	r := NewSitemapReader()
	docs, err := r.Read(context.Background(), srv.URL+"/page")
	if err != nil || len(docs) != 1 || docs[0].Content != "Only page" || docs[0].Metadata["source"] != "page" {
		t.Fatalf("single page fallback: %v %v", docs, err)
	}

	// PublicOnly refuses loopback by default...
	pub := NewSitemapReader().WithPublicOnly(true)
	docs, _ = pub.Read(context.Background(), srv.URL+"/page")
	if len(docs) != 1 || docs[0].Metadata["error"] == nil {
		t.Fatalf("PublicOnly must refuse loopback: %v", docs[0].Metadata)
	}
	// ...and the dial-time check is what does it (URL checks disabled here).
	pub.policy = &publicPolicy{allowIP: IsPublicIP}
	if _, err := pub.get(context.Background(), srv.URL+"/page", nil); !errors.Is(err, ErrNonPublicAddress) {
		t.Fatalf("expected ErrNonPublicAddress, got %v", err)
	}
	pub.policy = &publicPolicy{allowIP: func(netip.Addr) bool { return true }}
	if res, err := pub.get(context.Background(), srv.URL+"/page", nil); err != nil || !strings.Contains(string(res.Body), "Only page") {
		t.Fatalf("permissive policy: %v", err)
	}
}

func TestNetOptionsAllowedHostsRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "secret") }))
	defer target.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(target.URL, "127.0.0.1", "localhost", 1), http.StatusFound)
	}))
	defer src.Close()
	o := NetOptions{AllowedHosts: []string{"127.0.0.1"}}
	if _, err := o.get(context.Background(), src.URL, nil); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("expected redirect refusal, got %v", err)
	}
	if _, err := o.get(context.Background(), "http://evil.example/", nil); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("expected host refusal, got %v", err)
	}
	o.MaxBytes = 3
	if _, err := o.get(context.Background(), target.URL, nil); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
	}
}

func TestLLMsTxtReader(t *testing.T) {
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/llms.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "# Project\n\n> A great project.\n\n## Docs\n\n- [Intro](/intro.md): Getting started\n- [API](%s/api.html)\n\n## Optional\n\n- [Extra](/extra.md)\n", srv.URL)
	})
	mux.HandleFunc("/intro.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		fmt.Fprint(w, "# Intro\nhello")
	})
	mux.HandleFunc("/api.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><header>h</header><main><p>API ref</p></main></body></html>`)
	})
	mux.HandleFunc("/extra.md", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "extra") })
	srv = httptest.NewServer(mux)
	defer srv.Close()

	r := NewLLMsTxtReader()
	r.SkipOptional = true
	docs, err := r.Read(context.Background(), srv.URL+"/llms.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected overview + 2 docs, got %d", len(docs))
	}
	if docs[0].Metadata["type"] != "llms_txt_overview" || !strings.Contains(docs[0].Content, "A great project") {
		t.Fatalf("overview: %q", docs[0].Content)
	}
	if docs[1].Name != "Intro" || docs[1].Content != "# Intro\nhello" || docs[1].Metadata["url"] != srv.URL+"/intro.md" ||
		docs[1].Metadata["description"] != "Getting started" || docs[1].Metadata["section"] != "Docs" {
		t.Fatalf("intro: %q %v", docs[1].Content, docs[1].Metadata)
	}
	if docs[2].Content != "API ref" {
		t.Fatalf("api: %q", docs[2].Content)
	}
	r.SkipOptional = false
	r.MaxURLs = 1
	_, entries := r.ParseLLMsTxt("## A\n- [x](http://a/1)\n## Optional\n- [y](/2)", "http://a/llms.txt")
	if len(entries) != 2 || entries[1].URL != "http://a/2" {
		t.Fatalf("entries: %v", entries)
	}
}

func TestYouTubeReader(t *testing.T) {
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/watch", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("v") != "dQw4w9WgXcQ" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `<html><script>ytcfg.set({"INNERTUBE_API_KEY": "KEY123"})</script></html>`)
	})
	mux.HandleFunc("/youtubei/v1/player", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Context struct {
				Client struct{ ClientName string } `json:"client"`
			} `json:"context"`
			VideoID string `json:"videoId"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Query().Get("key") != "KEY123" || body.VideoID != "dQw4w9WgXcQ" || body.Context.Client.ClientName != "ANDROID" {
			http.Error(w, "bad request", 400)
			return
		}
		fmt.Fprintf(w, `{"playabilityStatus":{"status":"OK"},"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[
			{"baseUrl":"%[1]s/api/timedtext?lang=en&kind=asr&fmt=srv3","languageCode":"en","kind":"asr"},
			{"baseUrl":"%[1]s/api/timedtext?lang=en&fmt=srv3","languageCode":"en"},
			{"baseUrl":"%[1]s/api/timedtext?lang=pt","languageCode":"pt"}]}}}`, srv.URL)
	})
	mux.HandleFunc("/api/timedtext", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fmt") != "" {
			http.Error(w, "srv3 should be stripped", 400)
			return
		}
		if r.URL.Query().Get("kind") == "asr" {
			fmt.Fprint(w, `<transcript><text start="0" dur="1">auto</text></transcript>`)
			return
		}
		fmt.Fprintf(w, `<?xml version="1.0"?><transcript><text start="0.0" dur="1.5">Hello &amp;amp; welcome</text>`+
			`<text start="1.5" dur="2">to the &lt;b&gt;show&lt;/b&gt;</text><text start="4" dur="1"></text><text start="5" dur="1">lang %s</text></transcript>`,
			r.URL.Query().Get("lang"))
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	r := NewYouTubeReader()
	r.BaseURL = srv.URL
	docs, err := r.Read(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatal(err)
	}
	if docs[0].Content != "Hello & welcome to the show lang en" {
		t.Fatalf("transcript: %q", docs[0].Content)
	}
	if docs[0].ID != "youtube_dQw4w9WgXcQ" || docs[0].Metadata["video_id"] != "dQw4w9WgXcQ" {
		t.Fatalf("metadata: %v", docs[0].Metadata)
	}
	r.Languages = []string{"de", "pt"}
	docs, err = r.Read(context.Background(), "dQw4w9WgXcQ")
	if err != nil || !strings.HasSuffix(docs[0].Content, "lang pt") || docs[0].Metadata["language"] != "pt" {
		t.Fatalf("language preference: %v %v", docs, err)
	}
	r.Languages = []string{"de"}
	if _, err := r.Read(context.Background(), "dQw4w9WgXcQ"); !errors.Is(err, ErrNoTranscript) {
		t.Fatalf("expected ErrNoTranscript, got %v", err)
	}
}

func TestFirecrawlReader(t *testing.T) {
	var polls int32
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/scrape", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if r.Header.Get("Authorization") != "Bearer fc-key" || body["url"] != "https://example.com" || body["onlyMainContent"] != true {
			http.Error(w, "bad", 400)
			return
		}
		fmt.Fprint(w, `{"success":true,"data":{"markdown":"# Example","metadata":{"title":"Example"}}}`)
	})
	mux.HandleFunc("/v1/crawl", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"success":true,"id":"job1"}`)
	})
	mux.HandleFunc("/v1/crawl/job1", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&polls, 1) == 1 {
			fmt.Fprint(w, `{"status":"scraping","data":[]}`)
			return
		}
		fmt.Fprintf(w, `{"status":"completed","data":[{"markdown":"page one","metadata":{"sourceURL":"https://example.com/1"}}],"next":"%s/v1/crawl/job1/next"}`, srv.URL)
	})
	mux.HandleFunc("/v1/crawl/job1/next", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"markdown":"page two","metadata":{"sourceURL":"https://example.com/2"}},{"markdown":""}]}`)
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	r := NewFirecrawlReader("fc-key")
	r.BaseURL = srv.URL
	r.Params = map[string]any{"onlyMainContent": true}
	docs, err := r.Read(context.Background(), "https://example.com")
	if err != nil || len(docs) != 1 || docs[0].Content != "# Example" || docs[0].Metadata["title"] != "Example" {
		t.Fatalf("scrape: %v %v", docs, err)
	}
	r.Mode = FirecrawlCrawl
	r.Params = nil
	r.PollInterval = time.Millisecond
	docs, err = r.Read(context.Background(), "https://example.com")
	if err != nil || len(docs) != 2 || docs[1].ID != "https://example.com/2" || docs[0].Name != "https://example.com" {
		t.Fatalf("crawl: %v %v", docs, err)
	}
	r.AllowedHosts = []string{"docs.example.com"}
	if _, err := r.Read(context.Background(), "https://example.com"); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("expected host refusal, got %v", err)
	}
}

func TestTavilyReader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/extract" || r.Header.Get("Authorization") != "Bearer tv-key" || body["extract_depth"] != "advanced" || body["format"] != "markdown" {
			http.Error(w, "bad", 400)
			return
		}
		u := body["urls"].([]any)[0].(string)
		if strings.Contains(u, "fail") {
			fmt.Fprintf(w, `{"results":[],"failed_results":[{"url":%q,"error":"blocked"}]}`, u)
			return
		}
		fmt.Fprintf(w, `{"results":[{"url":%q,"raw_content":"## Extracted"}]}`, u)
	}))
	defer srv.Close()
	r := NewTavilyReader("tv-key")
	r.BaseURL = srv.URL
	r.ExtractDepth = "advanced"
	docs, err := r.Read(context.Background(), "https://example.com/a")
	if err != nil || docs[0].Content != "## Extracted" || docs[0].ID != "https://example.com/a" {
		t.Fatalf("extract: %v %v", docs, err)
	}
	if _, err := r.Read(context.Background(), "https://example.com/fail"); err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected failure reason, got %v", err)
	}
}

func TestDoclingReader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/convert/source":
			var body struct {
				Sources []map[string]string `json:"sources"`
				Options map[string]any      `json:"options"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.Sources[0]["kind"] != "http" || body.Options["to_formats"].([]any)[0] != "md" {
				http.Error(w, "bad", 400)
				return
			}
			fmt.Fprint(w, `{"status":"success","document":{"md_content":"# Paper"}}`)
		case "/v1/convert/file":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			f, hdr, _ := r.FormFile("files")
			data, _ := io.ReadAll(f)
			if hdr.Filename != "notes.docx" || string(data) != "DOCX" || r.FormValue("to_formats") != "json" || r.Header.Get("X-Api-Key") != "k" {
				http.Error(w, "bad file", 400)
				return
			}
			fmt.Fprint(w, `{"status":"success","document":{"json_content":{"name":"notes","texts":[1,2]}}}`)
		}
	}))
	defer srv.Close()

	r := NewDoclingReader(srv.URL)
	docs, err := r.Read(context.Background(), "https://arxiv.org/pdf/2408.09869.pdf?x=1")
	if err != nil || docs[0].Content != "# Paper" || docs[0].Name != "2408.09869" {
		t.Fatalf("url: %v %v", docs, err)
	}
	r.OutputFormat = "yaml"
	r.APIKey = "k"
	docs, err = r.Read(context.Background(), writeFile(t, "notes.docx", "DOCX"))
	if err != nil || !strings.Contains(docs[0].Content, "name: notes") {
		t.Fatalf("file/yaml: %v %v", docs, err)
	}
	r.OutputFormat = "vtt"
	if _, err := r.Read(context.Background(), "https://example.com/a.pdf"); err == nil {
		t.Fatal("expected invalid format error")
	}
	r.OutputFormat = "markdown"
	r.PublicOnly = true
	if _, err := r.Read(context.Background(), "http://127.0.0.1/a.pdf"); !errors.Is(err, ErrNonPublicAddress) {
		t.Fatalf("expected public-only refusal, got %v", err)
	}
}

func TestWebSearchReader(t *testing.T) {
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/html/", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.PostForm.Get("q") != "golang agents" {
			http.Error(w, "bad", 400)
			return
		}
		fmt.Fprintf(w, `<html><body>
<div class="result results_links result--ad"><a class="result__a" href="%[1]s/ad">Ad</a></div>
<div class="result"><a class="result__a" href="//duckduckgo.com/l/?uddg=%[2]s&rut=x">First Result</a><a class="result__snippet">First snippet</a></div>
<div class="result"><a class="result__a" href="%[1]s/b">Second</a><a class="result__snippet">Second snippet</a></div>
<div class="result"><a class="result__a" href="ftp://nope">FTP</a></div>
</body></html>`, srv.URL, url.QueryEscape(srv.URL+"/a"))
	})
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><script>x()</script><body><p>Page   A</p>\n<p>text</p></body></html>")
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "raw b") })
	srv = httptest.NewServer(mux)
	defer srv.Close()

	r := NewWebSearchReader()
	r.Searcher = &DuckDuckGoSearcher{BaseURL: srv.URL + "/html/"}
	r.DelayBetweenRequests = 0
	docs, err := r.Read(context.Background(), "golang agents")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}
	if docs[0].Name != "First Result" || docs[0].Content != "Page A text" || docs[0].Metadata["search_description"] != "First snippet" {
		t.Fatalf("doc0: %q %q %v", docs[0].Name, docs[0].Content, docs[0].Metadata)
	}
	if docs[1].Content != "raw b" {
		t.Fatalf("doc1: %q", docs[1].Content)
	}
	r.MaxResults = 1
	if docs, _ = r.Read(context.Background(), "golang agents"); len(docs) != 1 {
		t.Fatalf("max results: %d", len(docs))
	}
	if _, err := r.Read(context.Background(), " "); err == nil {
		t.Fatal("empty query must fail")
	}
}

func TestS3Reader(t *testing.T) {
	objects := map[string][]byte{
		"docs/readme.txt":  []byte("hello from s3"),
		"docs/data.json":   []byte(`[{"a":1},{"b":2}]`),
		"docs/My File.md":  []byte("# Title"),
		"other/ignore.txt": []byte("no"),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/bucket")
		if r.URL.Query().Get("list-type") == "2" {
			prefix := r.URL.Query().Get("prefix")
			var b strings.Builder
			b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>bucket</Name><IsTruncated>false</IsTruncated>`)
			for _, k := range []string{"docs/", "docs/data.json", "docs/My File.md", "docs/readme.txt", "other/ignore.txt"} {
				if strings.HasPrefix(k, prefix) {
					fmt.Fprintf(&b, `<Contents><Key>%s</Key><Size>1</Size></Contents>`, k)
				}
			}
			b.WriteString(`</ListBucketResult>`)
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, b.String())
			return
		}
		key, _ := url.PathUnescape(strings.TrimPrefix(path, "/"))
		data, ok := objects[key]
		if !ok {
			w.WriteHeader(404)
			fmt.Fprint(w, `<Error><Code>NoSuchKey</Code></Error>`)
			return
		}
		w.Write(data)
	}))
	defer srv.Close()

	client := s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(srv.URL),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("AK", "SK", ""),
	})
	r := NewS3Reader()
	r.Client = client
	docs, err := r.Read(context.Background(), "s3://bucket/docs/readme.txt")
	if err != nil || len(docs) != 1 || docs[0].Content != "hello from s3" || docs[0].Name != "readme" ||
		docs[0].Metadata["s3_key"] != "docs/readme.txt" || docs[0].Source != "s3://bucket/docs/readme.txt" {
		t.Fatalf("single: %v %v", docs, err)
	}
	docs, err = r.Read(context.Background(), "s3://bucket/docs/")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 4 { // 2 json docs + md + txt
		t.Fatalf("prefix: expected 4 docs, got %d", len(docs))
	}
	if docs[2].Name != "My_File" {
		t.Fatalf("name: %q", docs[2].Name)
	}
	if _, err := r.Read(context.Background(), "s3://bucket/docs/missing.txt"); err == nil {
		t.Fatal("expected missing object error")
	}
	if _, _, err := ParseS3URI("https://x"); err == nil {
		t.Fatal("expected bad URI error")
	}
}

func TestHTTPPageFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write(samplePDF(t, "PDF page text"))
		case "/bin":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte{0x89, 'P', 'N', 'G'})
		case "/empty":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body><script>x</script></body></html>")
		}
	}))
	defer srv.Close()
	f := NewHTTPPageFetcher()
	pages := f.FetchMany(context.Background(), []string{srv.URL + "/pdf", srv.URL + "/bin", srv.URL + "/empty", "http://other.example/x"}, []string{"127.0.0.1"})
	if !pages[0].OK() || !strings.Contains(pages[0].Content, "PDF page text") {
		t.Fatalf("pdf page: %+v", pages[0])
	}
	if pages[1].Error != "not-html (image/png)" || pages[2].Error != "empty" || pages[3].Error != "off-host" {
		t.Fatalf("errors: %q %q %q", pages[1].Error, pages[2].Error, pages[3].Error)
	}
	if len(pages[1].Attempts) != 1 || pages[1].Attempts[0].Extractor != "http" {
		t.Fatalf("attempts: %+v", pages[1].Attempts)
	}
}
