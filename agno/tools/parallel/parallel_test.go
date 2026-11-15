package parallel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParallel(t *testing.T) {
	var key string
	var searchBody map[string]interface{}
	var resultQuery, eventsQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("x-api-key")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/search":
			_ = json.NewDecoder(r.Body).Decode(&searchBody)
			_, _ = w.Write([]byte(`{"search_id":"s1","results":[{"url":"https://go.dev","title":"Go","excerpts":["x"]}]}`))
		case "GET /v1/tasks/runs/r1/result":
			resultQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"run":{"run_id":"r1","status":"completed","processor":"base"},"output":{"content":"answer","basis":[{"field":"f","confidence":"high","citations":[{"url":"https://a","title":"A","excerpts":["e"]}]}]}}`))
		case "GET /v1/monitors/m1/events":
			eventsQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"events":[{"event_id":"e1","event_type":"event","output":{"content":"c"}}],"next_cursor":"n"}`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithAll(), WithIncludeDomains("go.dev"))
	if len(p.GetMethods()) != 11 {
		t.Fatalf("expected 11 methods, got %d", len(p.GetMethods()))
	}
	out, _ := p.ParallelSearch(SearchParams{Objective: "go releases"})
	if !strings.Contains(out.(string), `"search_id":"s1"`) || key != "k" {
		t.Fatalf("unexpected search output: %v key=%s", out, key)
	}
	adv := searchBody["advanced_settings"].(map[string]interface{})
	if searchBody["mode"] != "advanced" || searchBody["search_queries"].([]interface{})[0] != "go releases" ||
		adv["max_results"] != float64(10) || adv["source_policy"] == nil {
		t.Fatalf("unexpected search body: %v", searchBody)
	}

	out, _ = p.GetTaskResult(RunIDParams{RunID: "r1"})
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &res)
	if resultQuery != "api_timeout=1800" || res["content"] != "answer" || res["status"] != "completed" {
		t.Fatalf("unexpected task result: %v (q=%s)", out, resultQuery)
	}

	out, _ = p.GetMonitorEvents(MonitorEventsParams{MonitorID: "m1", Limit: 500})
	if !strings.Contains(eventsQuery, "limit=100") || !strings.Contains(out.(string), `"has_more": true`) {
		t.Fatalf("unexpected events: %v (q=%s)", out, eventsQuery)
	}

	out, _ = p.ParallelSearch(SearchParams{})
	if !strings.Contains(out.(string), "objective or search_queries") {
		t.Fatalf("expected validation error, got %v", out)
	}
}
