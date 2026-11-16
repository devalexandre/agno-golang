package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// fakeTool is a minimal toolkit.Tool with one method.
type fakeTool struct{ calls int }

func (f *fakeTool) GetName() string        { return "weather" }
func (f *fakeTool) GetDescription() string { return "weather tools" }
func (f *fakeTool) GetParameterStruct(string) map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{"city": map[string]interface{}{"type": "string"}}}
}
func (f *fakeTool) GetMethods() map[string]toolkit.Method {
	return map[string]toolkit.Method{"get_weather": {Description: "Get the weather"}}
}
func (f *fakeTool) GetFunction(string) interface{}       { return nil }
func (f *fakeTool) GetDescriptionOfMethod(string) string { return "Get the weather" }
func (f *fakeTool) Execute(name string, in json.RawMessage) (interface{}, error) {
	f.calls++
	return "sunny in " + string(in), nil
}

type captured struct {
	path   string
	query  string
	header http.Header
	body   map[string]interface{}
}

// server replies with the given bodies in order and records requests.
func server(t *testing.T, replies ...string) (*httptest.Server, *[]captured) {
	t.Helper()
	var mu sync.Mutex
	var reqs []captured
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		reqs = append(reqs, captured{path: r.URL.Path, query: r.URL.RawQuery, header: r.Header.Clone(), body: body})
		reply := replies[len(replies)-1]
		if i < len(replies) {
			reply = replies[i]
		}
		i++
		mu.Unlock()
		if strings.HasPrefix(reply, "event:") || strings.HasPrefix(reply, "data:") {
			w.Header().Set("Content-Type", "text/event-stream")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs
}

const textReply = `{"id":"resp_1","model":"gpt-5.4-mini","status":"completed","output":[
 {"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"thinking"}]},
 {"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"hello"}]}]}`

const toolReply = `{"id":"resp_t","model":"gpt-5.4-mini","status":"completed","output":[
 {"type":"reasoning","id":"rs_t","summary":[],"encrypted_content":"enc"},
 {"type":"function_call","id":"fc_1","call_id":"call_1","name":"get_weather","arguments":"{\"city\":\"Rio\"}"}]}`

func TestInvokeDefaults(t *testing.T) {
	srv, reqs := server(t, textReply)
	t.Setenv(EnvAPIKey, "env-key")
	m, err := NewOpenAIResponses(models.WithBaseURL(srv.URL + "/v1"))
	if err != nil {
		t.Fatal(err)
	}
	if m.GetID() != DefaultModel {
		t.Fatalf("GetID = %q", m.GetID())
	}
	resp, err := m.Invoke(context.Background(), []models.Message{
		{Role: models.TypeSystemRole, Content: "be nice"},
		{Role: models.TypeUserRole, Content: "hi"},
	}, models.WithReasoningEffort("low"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" || resp.ReasoningContent != "thinking" {
		t.Fatalf("unexpected response %+v", resp)
	}
	req := (*reqs)[0]
	if req.path != "/v1/responses" {
		t.Fatalf("path = %q", req.path)
	}
	if req.header.Get("Authorization") != "Bearer env-key" {
		t.Fatalf("Authorization = %q", req.header.Get("Authorization"))
	}
	if req.body["model"] != DefaultModel {
		t.Fatalf("model = %v", req.body["model"])
	}
	input := req.body["input"].([]interface{})
	if input[0].(map[string]interface{})["role"] != "developer" {
		t.Fatalf("system message not mapped to developer: %v", input[0])
	}
	if req.body["reasoning"].(map[string]interface{})["effort"] != "low" {
		t.Fatalf("reasoning = %v", req.body["reasoning"])
	}
	if _, ok := req.body["temperature"]; ok {
		t.Fatalf("temperature must not be sent by default")
	}
	if rr, ok := m.(*Responses); !ok || rr.LastResponseID() != "resp_1" {
		t.Fatalf("LastResponseID not tracked")
	}
}

func TestToolLoopChainsPreviousResponseID(t *testing.T) {
	srv, reqs := server(t, toolReply, textReply)
	m, err := New(Config{APIKey: "k", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	tool := &fakeTool{}
	resp, err := m.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "weather?"}},
		models.WithTools([]toolkit.Tool{tool}))
	if err != nil {
		t.Fatal(err)
	}
	if tool.calls != 1 || resp.Content != "hello" || len(resp.ToolCalls) != 1 || len(resp.ToolResults) != 1 {
		t.Fatalf("unexpected result calls=%d resp=%+v", tool.calls, resp)
	}
	first := (*reqs)[0].body
	tools := first["tools"].([]interface{})
	if tools[0].(map[string]interface{})["name"] != "get_weather" || tools[0].(map[string]interface{})["type"] != "function" {
		t.Fatalf("tools = %v", tools)
	}
	second := (*reqs)[1].body
	if second["previous_response_id"] != "resp_t" {
		t.Fatalf("previous_response_id = %v", second["previous_response_id"])
	}
	input := second["input"].([]interface{})
	if len(input) != 1 || input[0].(map[string]interface{})["type"] != "function_call_output" ||
		input[0].(map[string]interface{})["call_id"] != "call_1" {
		t.Fatalf("chained input = %v", input)
	}
}

func TestToolLoopReplaysWhenStoreFalse(t *testing.T) {
	srv, reqs := server(t, toolReply, textReply)
	store := false
	m, _ := New(Config{APIKey: "k", BaseURL: srv.URL, Store: &store})
	_, err := m.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "weather?"}},
		models.WithTools([]toolkit.Tool{&fakeTool{}}))
	if err != nil {
		t.Fatal(err)
	}
	first := (*reqs)[0].body
	if first["store"] != false {
		t.Fatalf("store = %v", first["store"])
	}
	if inc := fmt.Sprint(first["include"]); !strings.Contains(inc, "reasoning.encrypted_content") {
		t.Fatalf("include = %v", inc)
	}
	second := (*reqs)[1].body
	if _, ok := second["previous_response_id"]; ok {
		t.Fatalf("must not chain when store=false")
	}
	var types []string
	for _, it := range second["input"].([]interface{}) {
		m := it.(map[string]interface{})
		if tp, ok := m["type"].(string); ok {
			types = append(types, tp)
		} else {
			types = append(types, "role:"+m["role"].(string))
		}
	}
	if strings.Join(types, ",") != "role:user,reasoning,function_call,function_call_output" {
		t.Fatalf("replayed input = %v", types)
	}
}

func TestStreaming(t *testing.T) {
	sse := "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hel\"}\n\n" +
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"lo\"}\n\n" +
		"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":" + strings.ReplaceAll(textReply, "\n", "") + "}\n\n"
	srv, reqs := server(t, sse)
	m, _ := New(Config{APIKey: "k", BaseURL: srv.URL, ID: "gpt-4.1"})

	var got strings.Builder
	err := m.InvokeStream(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "hi"}},
		models.WithStreamingFunc(func(_ context.Context, b []byte) error { got.Write(b); return nil }))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "hello" {
		t.Fatalf("streamed %q", got.String())
	}
	if (*reqs)[0].body["stream"] != true {
		t.Fatalf("stream flag not sent")
	}
	if _, ok := (*reqs)[0].body["store"]; ok {
		t.Fatalf("store must be omitted for non-reasoning models by default")
	}

	ch, errCh := m.AInvokeStream(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "hi"}})
	var async strings.Builder
	for c := range ch {
		async.WriteString(c.Content)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	if async.String() != "hello" {
		t.Fatalf("async streamed %q", async.String())
	}
}

func TestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"bad things","type":"invalid_request_error"}}`)
	}))
	defer srv.Close()
	m, _ := New(Config{APIKey: "k", BaseURL: srv.URL})
	_, err := m.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "x"}})
	if err == nil || !strings.Contains(err.Error(), "bad things") {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenResponsesIsStateless(t *testing.T) {
	srv, reqs := server(t, textReply)
	m, err := NewOpenResponses(models.WithBaseURL(srv.URL), models.WithID("gpt-5-mini"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "x"}}); err != nil {
		t.Fatal(err)
	}
	body := (*reqs)[0].body
	if body["store"] != false {
		t.Fatalf("store = %v", body["store"])
	}
	if _, ok := body["include"]; ok {
		t.Fatalf("include must not be added for Open Responses: %v", body["include"])
	}
}
