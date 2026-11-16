package ibm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

type fakeTool struct{ calls int }

func (f *fakeTool) GetName() string        { return "calc" }
func (f *fakeTool) GetDescription() string { return "calc" }
func (f *fakeTool) GetParameterStruct(string) map[string]interface{} {
	return map[string]interface{}{"type": "object"}
}
func (f *fakeTool) GetMethods() map[string]toolkit.Method {
	return map[string]toolkit.Method{"add": {Description: "add"}}
}
func (f *fakeTool) GetFunction(string) interface{}       { return nil }
func (f *fakeTool) GetDescriptionOfMethod(string) string { return "add numbers" }
func (f *fakeTool) Execute(string, json.RawMessage) (interface{}, error) {
	f.calls++
	return 3, nil
}

type rec struct {
	path, query, auth string
	body              map[string]interface{}
}

func newServer(t *testing.T, chatReplies ...string) (*httptest.Server, *[]rec, *int) {
	var mu sync.Mutex
	var reqs []rec
	iamCalls := 0
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/identity/token" {
			iamCalls++
			_ = r.ParseForm()
			if r.Form.Get("apikey") != "ibm-key" || r.Form.Get("grant_type") != "urn:ibm:params:oauth:grant-type:apikey" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = io.WriteString(w, `{"access_token":"iam-token","expires_in":3600}`)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		reqs = append(reqs, rec{path: r.URL.Path, query: r.URL.RawQuery, auth: r.Header.Get("Authorization"), body: body})
		reply := chatReplies[len(chatReplies)-1]
		if i < len(chatReplies) {
			reply = chatReplies[i]
		}
		i++
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs, &iamCalls
}

const textReply = `{"model_id":"m","choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`
const toolReply = `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"add","arguments":"{\"a\":1,\"b\":2}"}}]},"finish_reason":"tool_calls"}]}`

func TestInvokeWithToolsAndIAM(t *testing.T) {
	srv, reqs, iam := newServer(t, toolReply, textReply)
	t.Setenv(EnvAPIKey, "ibm-key")
	t.Setenv(EnvProjectID, "proj-1")
	t.Setenv(EnvURL, "")

	m, err := NewWatsonXChat(models.WithBaseURL(srv.URL),
		models.WithClientParams(map[string]interface{}{"iam_url": srv.URL + "/identity/token"}))
	if err != nil {
		t.Fatal(err)
	}
	if m.GetID() != DefaultModel {
		t.Fatalf("GetID = %q", m.GetID())
	}
	tool := &fakeTool{}
	resp, err := m.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "1+2?"}},
		models.WithTools([]toolkit.Tool{tool}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" || tool.calls != 1 || len(resp.ToolCalls) != 1 {
		t.Fatalf("resp = %+v calls=%d", resp, tool.calls)
	}
	if *iam != 1 {
		t.Fatalf("IAM token must be cached, got %d exchanges", *iam)
	}
	first := (*reqs)[0]
	if first.path != "/ml/v1/text/chat" || first.query != "version="+DefaultAPIVersion {
		t.Fatalf("path=%q query=%q", first.path, first.query)
	}
	if first.auth != "Bearer iam-token" {
		t.Fatalf("Authorization = %q", first.auth)
	}
	if first.body["model_id"] != DefaultModel || first.body["project_id"] != "proj-1" {
		t.Fatalf("body = %v", first.body)
	}
	msgs := (*reqs)[1].body["messages"].([]interface{})
	last := msgs[len(msgs)-1].(map[string]interface{})
	if last["role"] != "tool" || last["tool_call_id"] != "c1" || last["content"] != "3" {
		t.Fatalf("tool message = %v", last)
	}
}

func TestStream(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\ndata: [DONE]\n\n"
	srv, reqs, _ := newServer(t, sse)
	m, err := New(Config{BearerToken: "tok", ProjectID: "p", URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	err = m.InvokeStream(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "hi"}},
		models.WithStreamingFunc(func(_ context.Context, b []byte) error { got.Write(b); return nil }))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "hello" || (*reqs)[0].path != "/ml/v1/text/chat_stream" {
		t.Fatalf("got %q path %q", got.String(), (*reqs)[0].path)
	}
}

func TestRequiresProject(t *testing.T) {
	t.Setenv(EnvProjectID, "")
	if _, err := New(Config{APIKey: "k"}); err == nil {
		t.Fatal("expected error")
	}
}
