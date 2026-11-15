package browserbase

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionLifecycle(t *testing.T) {
	var calls []string
	var releaseBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-BB-API-Key") != "k" {
			t.Errorf("bad api key %q", r.Header.Get("X-BB-API-Key"))
		}
		calls = append(calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["projectId"] != "p" {
				t.Errorf("bad create body %v", body)
			}
			_, _ = w.Write([]byte(`{"id":"s1","connectUrl":"wss://connect/s1","status":"RUNNING"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/sessions":
			_, _ = w.Write([]byte(`[{"id":"s1","status":"RUNNING"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/sessions/s1":
			_, _ = w.Write([]byte(`{"id":"s1","status":"RUNNING"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions/s1":
			_ = json.NewDecoder(r.Body).Decode(&releaseBody)
			_, _ = w.Write([]byte(`{"id":"s1","status":"COMPLETED"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	b := New(WithAPIKey("k"), WithProjectID("p"), WithBaseURL(srv.URL))
	if len(b.GetMethods()) != 4 {
		t.Fatalf("expected 4 methods, got %d", len(b.GetMethods()))
	}
	out, err := b.CreateSession(EmptyParams{})
	if err != nil || out != `{"connect_url":"wss://connect/s1","session_id":"s1"}` {
		t.Fatalf("unexpected create %v %v", out, err)
	}
	_, _ = b.CreateSession(EmptyParams{}) // reused, no new request
	if out, _ := b.ListSessions(ListSessionsParams{Status: "RUNNING"}); !strings.Contains(out.(string), `"s1"`) {
		t.Fatalf("unexpected list %v", out)
	}
	if out, _ := b.GetSession(SessionParams{SessionID: "s1"}); !strings.Contains(out.(string), "RUNNING") {
		t.Fatalf("unexpected get %v", out)
	}
	out, _ = b.CloseSession(CloseSessionParams{})
	if !strings.Contains(out.(string), `"status":"closed"`) || releaseBody["status"] != "REQUEST_RELEASE" || releaseBody["projectId"] != "p" {
		t.Fatalf("unexpected close %v body=%v", out, releaseBody)
	}
	want := []string{"POST /v1/sessions?", "GET /v1/sessions?status=RUNNING", "GET /v1/sessions/s1?", "POST /v1/sessions/s1?"}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected calls %v", calls)
	}
}

func TestMissingKey(t *testing.T) {
	t.Setenv("BROWSERBASE_API_KEY", "")
	if _, err := New(WithProjectID("p")).CreateSession(EmptyParams{}); err == nil {
		t.Fatal("expected missing key error")
	}
}
