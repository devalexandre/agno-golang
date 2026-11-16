package todoist

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTodoist(t *testing.T) {
	var gotAuth, gotMethod, gotPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotMethod, gotPath = r.Header.Get("Authorization"), r.Method, r.URL.Path
		gotBody = nil
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &gotBody)
		}
		switch {
		case r.URL.Path == "/tasks" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"results":[{"id":"1","content":"a","child_order":2,"added_by_uid":"u","labels":["x"],"due":{"date":"2025-01-01","string":"tomorrow","is_recurring":false}}],"next_cursor":null}`))
		case r.URL.Path == "/tasks":
			_, _ = w.Write([]byte(`{"id":"9","content":"Buy milk","priority":4,"labels":[]}`))
		case r.URL.Path == "/tasks/9/close":
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	td := New(WithAPIToken("tok"), WithBaseURL(srv.URL))
	out, err := td.CreateTask(CreateTaskParams{Content: "Buy milk", Priority: 4, DueString: "tomorrow"})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" || gotMethod != http.MethodPost || gotPath != "/tasks" || gotBody["content"] != "Buy milk" || gotBody["due_string"] != "tomorrow" {
		t.Fatalf("bad request: %s %s %s %v", gotAuth, gotMethod, gotPath, gotBody)
	}
	var task map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &task)
	if task["id"] != "9" || task["url"] != "https://app.todoist.com/app/task/9" {
		t.Fatalf("bad output: %s", out)
	}

	out, _ = td.GetActiveTasks(NoParams{})
	var tasks []map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &tasks)
	if len(tasks) != 1 || tasks[0]["order"].(float64) != 2 || tasks[0]["creator_id"] != "u" || tasks[0]["due"].(map[string]interface{})["string"] != "tomorrow" {
		t.Fatalf("bad tasks: %s", out)
	}

	out, _ = td.CloseTask(TaskParams{TaskID: "9"})
	if out != `{"success":true}` || gotPath != "/tasks/9/close" {
		t.Fatalf("bad close: %v %s", out, gotPath)
	}
	if len(td.GetMethods()) != 7 {
		t.Fatalf("expected 7 methods")
	}
}

func TestTodoistMissingToken(t *testing.T) {
	t.Setenv("TODOIST_API_TOKEN", "")
	if _, err := New().GetTask(TaskParams{TaskID: "1"}); err == nil {
		t.Fatal("expected error")
	}
}
