package neo4j

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNeo4jHTTPQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, _ := r.BasicAuth()
		if r.URL.Path != "/db/movies/query/v2" || u != "neo" || p != "pw" {
			w.WriteHeader(401)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch body["statement"] {
		case "CALL db.labels()":
			_, _ = w.Write([]byte(`{"data":{"fields":["label"],"values":[["Person"],["Movie"]]}}`))
		default:
			_, _ = w.Write([]byte(`{"data":{"fields":["name","age"],"values":[["Ann",30]]}}`))
		}
	}))
	defer srv.Close()
	n := NewNeo4jTools(WithURI(srv.URL), WithAuth("neo", "pw"), WithDatabase("movies"))
	labels, _ := n.ListLabels(NoParams{})
	if l := labels.([]interface{}); len(l) != 2 || l[0] != "Person" {
		t.Fatal(labels)
	}
	rows, _ := n.RunCypherQuery(CypherParams{Query: "MATCH (p) RETURN p.name AS name, p.age AS age"})
	if r := rows.([]map[string]interface{}); len(r) != 1 || r[0]["name"] != "Ann" {
		t.Fatal(rows)
	}
	bad := NewNeo4jTools(WithURI(srv.URL), WithAuth("x", "y"))
	if out, _ := bad.ListRelationshipTypes(NoParams{}); len(out.([]interface{})) != 0 {
		t.Fatal(out)
	}
	if got := NewNeo4jTools(WithURI("neo4j+s://db.example.com:7687")).HTTPBaseURL(); got != "https://db.example.com:7474" {
		t.Fatal(got)
	}
}
