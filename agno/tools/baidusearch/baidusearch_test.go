package baidusearch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBaiduSearchPagination(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/s" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		queries = append(queries, r.URL.Query().Get("wd"))
		pn := r.URL.Query().Get("pn")
		next := `<div id="page"><a class="n" href="/s?wd=golang&pn=10">下一页 &gt;</a></div>`
		if pn != "" {
			next = `<div id="page"><a class="n" href="/s?wd=golang">&lt; 上一页</a></div>`
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<html><body><div id="content_left">
<div class="result c-container"><h3><a href="https://www.baidu.com/link?url=a%s">Go 语言 %s</a></h3><div class="c-abstract">Go is a language</div></div>
<div class="result c-container"><h3><a href="https://www.baidu.com/link?url=b%s">Golang %s</a></h3><span class="content-right_x">Second abstract</span></div>
</div>%s</body></html>`, pn, pn, pn, pn, next)
	}))
	defer srv.Close()

	b := New(WithBaseURL(srv.URL))
	out, err := b.BaiduSearch(SearchParams{Query: "golang", MaxResults: 3})
	if err != nil {
		t.Fatal(err)
	}
	var res []map[string]string
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 || len(queries) != 2 || queries[0] != "golang" {
		t.Fatalf("unexpected: %d results, %d requests: %s", len(res), len(queries), out)
	}
	if res[0]["title"] != "Go 语言" || res[0]["abstract"] != "Go is a language" || res[1]["abstract"] != "Second abstract" ||
		res[2]["rank"] != "3" || res[2]["url"] != "https://www.baidu.com/link?url=a10" {
		t.Fatalf("unexpected output: %s", out)
	}
}
