package webbrowser

import (
	"encoding/json"
	"testing"
)

func TestOpenPage(t *testing.T) {
	var gotURL string
	var gotNew bool
	w := New(WithOpener(func(u string, nw bool) error { gotURL, gotNew = u, nw; return nil }))
	out, err := w.Execute("webbrowser_tools_OpenPage", json.RawMessage(`{"url":"https://agno.com","new_window":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "https://agno.com" || !gotNew || out != "Opened https://agno.com in the web browser" {
		t.Fatalf("unexpected: %v %v %v", gotURL, gotNew, out)
	}
	if _, err := w.OpenPage(OpenPageParams{URL: "--help"}); err == nil {
		t.Fatal("expected error for option-like URL")
	}
}
