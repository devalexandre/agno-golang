package resend

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendEmail(t *testing.T) {
	var gotAuth, gotPath, gotMethod string
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotMethod = r.Header.Get("Authorization"), r.URL.Path, r.Method
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &got)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()

	tk := New(WithAPIKey("k"), WithFromEmail("me@x.com"), WithBaseURL(srv.URL))
	out, err := tk.SendEmail(SendEmailParams{ToEmail: "you@x.com", Subject: "Hi", Body: "<p>hello</p>"})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotPath != "/emails" || gotAuth != "Bearer k" {
		t.Fatalf("bad request %s %s %s", gotMethod, gotPath, gotAuth)
	}
	if got["from"] != "me@x.com" || got["to"] != "you@x.com" || got["html"] != "<p>hello</p>" || got["subject"] != "Hi" {
		t.Fatalf("bad body %v", got)
	}
	if out != "Email sent to you@x.com successfully." {
		t.Fatalf("unexpected output %v", out)
	}
}

func TestSendEmailErrors(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	if out, _ := New().SendEmail(SendEmailParams{ToEmail: "a@b.c"}); out != "Please provide an API key" {
		t.Fatalf("got %v", out)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"bad"}`, 422)
	}))
	defer srv.Close()
	out, _ := New(WithAPIKey("k"), WithBaseURL(srv.URL)).SendEmail(SendEmailParams{ToEmail: "a@b.c"})
	if !strings.HasPrefix(out.(string), "Error:") {
		t.Fatalf("got %v", out)
	}
	if len(New(WithAPIKey("k"), WithEnableSendEmail(false)).GetMethods()) != 0 {
		t.Fatal("expected no methods")
	}
}
