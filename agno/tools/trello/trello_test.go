package trello

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrello(t *testing.T) {
	var reqs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("key") != "k" || q.Get("token") != "tok" {
			t.Errorf("missing auth: %s", r.URL.RawQuery)
		}
		reqs = append(reqs, r.Method+" "+r.URL.Path)
		switch r.Method + " " + r.URL.Path {
		case "GET /boards/b1/lists":
			_, _ = w.Write([]byte(`[{"id":"l1","name":"To Do","cards":[{"id":"c"},{"id":"d"}]},{"id":"l2","name":"Done"}]`))
		case "POST /cards":
			if q.Get("idList") != "l1" || q.Get("name") != "Write docs" || q.Get("desc") != "d" {
				t.Errorf("bad card query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"id":"c1","name":"Write docs","url":"https://trello.com/c/c1"}`))
		case "GET /members/me/boards":
			_, _ = w.Write([]byte(`[{"id":"b1","name":"Board","desc":"x","url":"u","closed":false,"idOrganization":null}]`))
		}
	}))
	defer srv.Close()

	tr := New(WithAPIKey("k"), WithToken("tok"), WithBaseURL(srv.URL))
	out, _ := tr.CreateCard(CreateCardParams{BoardID: "b1", ListName: "to do", CardTitle: "Write docs", Description: "d"})
	if out != `{"id":"c1","list":"to do","name":"Write docs","url":"https://trello.com/c/c1"}` {
		t.Fatalf("bad card: %v", out)
	}
	out, _ = tr.CreateCard(CreateCardParams{BoardID: "b1", ListName: "nope", CardTitle: "x"})
	if out != "List 'nope' not found on board" {
		t.Fatalf("bad missing list: %v", out)
	}
	out, _ = tr.GetBoardLists(BoardParams{BoardID: "b1"})
	if !strings.Contains(out.(string), `"cards_count":2`) {
		t.Fatalf("bad lists: %v", out)
	}
	out, _ = tr.ListBoards(ListBoardsParams{})
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &res)
	if res["filter_used"] != "all" || res["total_boards"].(float64) != 1 {
		t.Fatalf("bad boards: %v", out)
	}
	if len(tr.GetMethods()) != 7 {
		t.Fatal("expected 7 methods")
	}
}
