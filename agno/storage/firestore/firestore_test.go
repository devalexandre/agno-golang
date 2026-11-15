package firestore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/devalexandre/agno-golang/agno/storage/kvstore"
	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

const root = "/v1/projects/p/databases/(default)/documents"

// fakeFirestore implements the subset of the Firestore REST API used by the backend.
func fakeFirestore(t *testing.T) *httptest.Server {
	var mu sync.Mutex
	colls := map[string]map[string]document{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		path, _ := url.PathUnescape(r.URL.EscapedPath())
		if r.Method == http.MethodPost && path == root+":runQuery" {
			var body struct {
				StructuredQuery struct {
					From []struct {
						CollectionID string `json:"collectionId"`
					} `json:"from"`
					Where json.RawMessage `json:"where"`
				} `json:"structuredQuery"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			type ff struct {
				FieldFilter struct {
					Field struct {
						FieldPath string `json:"fieldPath"`
					} `json:"field"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"fieldFilter"`
				CompositeFilter struct {
					Filters []ff `json:"filters"`
				} `json:"compositeFilter"`
			}
			var where ff
			_ = json.Unmarshal(body.StructuredQuery.Where, &where)
			filters := where.CompositeFilter.Filters
			if where.FieldFilter.Field.FieldPath != "" {
				filters = []ff{where}
			}
			var out []map[string]interface{}
			for _, d := range colls[body.StructuredQuery.From[0].CollectionID] {
				ok := true
				for _, f := range filters {
					v := d.Fields[f.FieldFilter.Field.FieldPath].StringValue
					if v == nil || *v != f.FieldFilter.Value.StringValue {
						ok = false
					}
				}
				if ok {
					out = append(out, map[string]interface{}{"document": d})
				}
			}
			out = append(out, map[string]interface{}{"readTime": "x"})
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		rest := strings.TrimPrefix(path, root+"/")
		parts := strings.SplitN(rest, "/", 2)
		coll := parts[0]
		if colls[coll] == nil {
			colls[coll] = map[string]document{}
		}
		if len(parts) == 1 {
			switch r.Method {
			case http.MethodPost:
				id := r.URL.Query().Get("documentId")
				if _, ok := colls[coll][id]; ok {
					w.WriteHeader(http.StatusConflict)
					return
				}
				var d document
				_ = json.NewDecoder(r.Body).Decode(&d)
				d.Name = root + "/" + coll + "/" + id
				colls[coll][id] = d
				_ = json.NewEncoder(w).Encode(d)
			case http.MethodGet:
				ids := make([]string, 0)
				for id := range colls[coll] {
					ids = append(ids, id)
				}
				sort.Strings(ids)
				var docs []document
				for _, id := range ids {
					docs = append(docs, colls[coll][id])
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"documents": docs})
			}
			return
		}
		id := parts[1]
		switch r.Method {
		case http.MethodPatch:
			var d document
			_ = json.NewDecoder(r.Body).Decode(&d)
			d.Name = root + "/" + coll + "/" + id
			colls[coll][id] = d
			_ = json.NewEncoder(w).Encode(d)
		case http.MethodGet:
			d, ok := colls[coll][id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(d)
		case http.MethodDelete:
			delete(colls[coll], id)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newFakeStore(t *testing.T) *kvstore.Store {
	t.Helper()
	srv := fakeFirestore(t)
	s, err := New(context.Background(), Config{ProjectID: "p", Endpoint: srv.URL + "/v1", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFirestoreConformanceFake(t *testing.T) {
	storagetest.RunDBConformance(t, newFakeStore(t))
	storagetest.RunSessionSearchConformance(t, newFakeStore(t))
	storagetest.RunMetricsConformance(t, newFakeStore(t))
	storagetest.RunKnowledgeConformance(t, newFakeStore(t))
	s := newFakeStore(t)
	storagetest.RunStorageConformance(t, s)
	if err := s.DropTables(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestFirestoreEmulator runs against the emulator when FIRESTORE_EMULATOR_HOST is set:
// gcloud emulators firestore start --host-port=localhost:8080
// FIRESTORE_EMULATOR_HOST=localhost:8080 GOOGLE_CLOUD_PROJECT=demo
func TestFirestoreEmulator(t *testing.T) {
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST not set")
	}
	ctx := context.Background()
	s, err := New(ctx, Config{Prefix: "agno_conformance"})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.DropTables(ctx)
	defer s.DropTables(ctx)
	storagetest.RunDBConformance(t, s)
}
