package awsses

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendEmail(t *testing.T) {
	var auth, path, method string
	var body struct {
		FromEmailAddress string
		Destination      struct{ ToAddresses []string }
		Content          struct {
			Simple struct {
				Subject struct{ Data, Charset string }
				Body    struct{ Text struct{ Data string } }
			}
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path, method = r.Header.Get("Authorization"), r.URL.Path, r.Method
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &body)
		_, _ = w.Write([]byte(`{"MessageId":"m1"}`))
	}))
	defer srv.Close()

	s := New(WithStaticCredentials("AKID", "SECRET", ""), WithSenderEmail("me@x.com"), WithSenderName("Me"), WithBaseURL(srv.URL))
	out, err := s.SendEmail(SendEmailParams{Subject: "Hi", Body: "hello", ReceiverEmail: "you@x.com"})
	if err != nil || out != "Email sent successfully!" {
		t.Fatalf("out=%v err=%v", out, err)
	}
	if method != "POST" || path != "/v2/email/outbound-emails" || !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=AKID/") || !strings.Contains(auth, "/us-east-1/ses/aws4_request") {
		t.Fatalf("bad request %s %s %s", method, path, auth)
	}
	if body.FromEmailAddress != "Me <me@x.com>" || body.Destination.ToAddresses[0] != "you@x.com" ||
		body.Content.Simple.Subject.Data != "Hi" || body.Content.Simple.Subject.Charset != "UTF-8" || body.Content.Simple.Body.Text.Data != "hello" {
		t.Fatalf("bad body %+v", body)
	}
	if out, _ := s.SendEmail(SendEmailParams{Body: "x"}); out != "Email subject cannot be empty." {
		t.Fatalf("got %v", out)
	}
}

func TestSendEmailAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Email address is not verified"}`, 400)
	}))
	defer srv.Close()
	_, err := New(WithStaticCredentials("A", "S", ""), WithBaseURL(srv.URL)).SendEmail(SendEmailParams{Subject: "s", Body: "b", ReceiverEmail: "r@x.com"})
	if err == nil || !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("got %v", err)
	}
}
