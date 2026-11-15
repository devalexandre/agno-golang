package clickup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClickUp(t *testing.T) {
	var created map[string]interface{}
	var deleted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "pk_123" {
			t.Errorf("bad auth %q", r.Header.Get("Authorization"))
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /team/team1/space":
			_, _ = w.Write([]byte(`{"spaces":[{"id":"s1","name":"Marketing"},{"id":"s2","name":"Engineering"}]}`))
		case "GET /space/s2/list":
			_, _ = w.Write([]byte(`{"lists":[{"id":"l1","name":"Backlog"},{"id":"l2","name":"Sprint"}]}`))
		case "GET /list/l1/task":
			_, _ = w.Write([]byte(`{"tasks":[{"id":"t1","name":"A"}]}`))
		case "GET /list/l2/task":
			_, _ = w.Write([]byte(`{"tasks":[{"id":"t2","name":"B"}]}`))
		case "POST /list/l1/task":
			_ = json.NewDecoder(r.Body).Decode(&created)
			_, _ = w.Write([]byte(`{"id":"t3","name":"New"}`))
		case "DELETE /task/t9":
			deleted = true
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(WithAPIKey("pk_123"), WithMasterSpaceID("team1"), WithBaseURL(srv.URL))
	out, err := c.ListTasks(SpaceParams{SpaceName: "engineering"})
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Tasks []map[string]string `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Tasks) != 2 || res.Tasks[1]["list_name"] != "Sprint" {
		t.Fatalf("unexpected tasks: %s", out)
	}

	if _, err := c.CreateTask(CreateTaskParams{SpaceName: "Eng", TaskName: "New", TaskDescription: "Desc"}); err != nil {
		t.Fatal(err)
	}
	if created["name"] != "New" || created["description"] != "Desc" {
		t.Fatalf("unexpected create body: %v", created)
	}

	out, _ = c.DeleteTask(TaskParams{TaskID: "t9"})
	if !deleted || out != "{\n  \"message\": \"Task t9 deleted successfully\",\n  \"success\": true\n}" {
		t.Fatalf("unexpected delete output: %v", out)
	}

	out, _ = c.ListLists(SpaceParams{SpaceName: "Sales"})
	if out != "{\n  \"error\": \"Space 'Sales' not found\"\n}" {
		t.Fatalf("unexpected: %v", out)
	}
}
