package plivo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlivo(t *testing.T) {
	var last *http.Request
	var lastBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last = r
		lastBody = nil
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &lastBody)
		if u, p, ok := r.BasicAuth(); !ok || u != "MA1" || p != "tok" {
			http.Error(w, "unauthorized", 401)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /Account/MA1/Message/":
			_, _ = w.Write([]byte(`{"message_uuid":["m-1"]}`))
		case "POST /Account/MA1/Call/":
			_, _ = w.Write([]byte(`{"request_uuid":"r-1"}`))
		case "GET /Account/MA1/Call/c-1/":
			_, _ = w.Write([]byte(`{"to_number":"+1","from_number":"+2","call_state":"ANSWER","call_duration":5}`))
		case "GET /Account/MA1/Message/":
			_, _ = w.Write([]byte(`{"objects":[{"message_uuid":"m-1","to_number":"+1","message_direction":"outbound"}]}`))
		case "GET /Account/MA1/Call/":
			_, _ = w.Write([]byte(`{"objects":[{"call_uuid":"c-1","call_direction":"inbound"}]}`))
		case "GET /Number/+15551234567":
			_, _ = w.Write([]byte(`{"phone_number":"+15551234567","country":{"iso2":"US"},"format":{},"carrier":{"name":"X"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := New(WithAuthID("MA1"), WithAuthToken("tok"), WithBaseURL(srv.URL), WithLookupBaseURL(srv.URL))
	if out, _ := p.SendSms(SendSmsParams{To: "+15551234567", From: "+15557654321", Body: "hi"}); out != "Message sent successfully. UUID: m-1" {
		t.Fatalf("got %v", out)
	}
	if lastBody["src"] != "+15557654321" || lastBody["dst"] != "+15551234567" || lastBody["text"] != "hi" {
		t.Fatalf("bad body %v", lastBody)
	}
	if out, _ := p.MakeCall(MakeCallParams{To: "+15551234567", From: "+15557654321", AnswerURL: "https://a/x"}); out != "Call placed successfully. request_uuid: r-1" {
		t.Fatalf("got %v", out)
	}
	if lastBody["answer_method"] != "POST" {
		t.Fatalf("bad body %v", lastBody)
	}
	call, _ := p.GetCallDetails(GetCallDetailsParams{CallUUID: "c-1"})
	if call.(map[string]interface{})["state"] != "ANSWER" {
		t.Fatalf("got %v", call)
	}
	msgs, _ := p.ListMessages(ListMessagesParams{Limit: 50, MessageDirection: "outbound"})
	if len(msgs.([]map[string]interface{})) != 1 || last.URL.Query().Get("limit") != "20" || last.URL.Query().Get("message_direction") != "outbound" {
		t.Fatalf("got %v %s", msgs, last.URL.RawQuery)
	}
	calls, _ := p.ListCalls(ListCallsParams{})
	if calls.([]map[string]interface{})[0]["call_uuid"] != "c-1" {
		t.Fatalf("got %v", calls)
	}
	lk, _ := p.LookupNumber(LookupNumberParams{Number: "+15551234567"})
	if lk.(map[string]interface{})["number"] != "+15551234567" || last.URL.Query().Get("type") != "carrier" {
		t.Fatalf("got %v", lk)
	}
	bad, _ := p.ListCalls(ListCallsParams{CallDirection: "sideways"})
	if bad.([]map[string]interface{})[0]["error"] == nil {
		t.Fatal("expected error")
	}
}
