package gcsjson

import (
	"context"
	"encoding/json"
	"io"
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

// fakeGCS implements the subset of the GCS JSON API used by the backend.
func fakeGCS(t *testing.T) *httptest.Server {
	var mu sync.Mutex
	objects := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		path := r.URL.EscapedPath()
		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(path, "/upload/storage/v1/b/test-bucket/o"):
			name := r.URL.Query().Get("name")
			if r.URL.Query().Get("ifGenerationMatch") == "0" {
				if _, ok := objects[name]; ok {
					w.WriteHeader(http.StatusPreconditionFailed)
					return
				}
			}
			body, _ := io.ReadAll(r.Body)
			objects[name] = body
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && path == "/storage/v1/b/test-bucket":
			_, _ = w.Write([]byte(`{"name":"test-bucket"}`))
		case r.Method == http.MethodGet && path == "/storage/v1/b/test-bucket/o":
			prefix := r.URL.Query().Get("prefix")
			var names []string
			for n := range objects {
				if strings.HasPrefix(n, prefix) {
					names = append(names, n)
				}
			}
			sort.Strings(names)
			// Two items per page to exercise pagination.
			start := 0
			if tok := r.URL.Query().Get("pageToken"); tok != "" {
				for i, n := range names {
					if n == tok {
						start = i
					}
				}
			}
			resp := map[string]interface{}{}
			var items []map[string]string
			for i := start; i < len(names) && i < start+2; i++ {
				items = append(items, map[string]string{"name": names[i]})
			}
			resp["items"] = items
			if start+2 < len(names) {
				resp["nextPageToken"] = names[start+2]
			}
			_ = json.NewEncoder(w).Encode(resp)
		case strings.HasPrefix(path, "/storage/v1/b/test-bucket/o/"):
			name, _ := url.PathUnescape(strings.TrimPrefix(path, "/storage/v1/b/test-bucket/o/"))
			doc, ok := objects[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodDelete {
				delete(objects, name)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			_, _ = w.Write(doc)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newFakeStore(t *testing.T) *kvstore.Store {
	t.Helper()
	srv := fakeGCS(t)
	s, err := New(context.Background(), Config{Bucket: "test-bucket", Endpoint: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestGCSConformanceFake(t *testing.T) {
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

// TestGCSRealBucket runs against a real bucket (or fake-gcs-server via
// STORAGE_EMULATOR_HOST) when GCS_BUCKET is set.
func TestGCSRealBucket(t *testing.T) {
	if os.Getenv("GCS_BUCKET") == "" {
		t.Skip("GCS_BUCKET not set")
	}
	ctx := context.Background()
	s, err := New(ctx, Config{Prefix: "agno_conformance/"})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.DropTables(ctx)
	defer s.DropTables(ctx)
	storagetest.RunDBConformance(t, s)
}
