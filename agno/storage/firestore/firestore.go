// Package firestore provides a Google Cloud Firestore implementation of the
// storage contracts (storage.DB, storage.Storage, storage.KnowledgeStorage,
// session search and metrics), a port of Python agno/db/firestore. It uses
// the Firestore REST API (v1) directly, so no gRPC SDK is required, and it
// works with the Firestore emulator (FIRESTORE_EMULATOR_HOST).
//
// Each kvstore collection maps to the Firestore collection
// "{prefix}_{collection}". Documents keep the JSON record in the string
// field "data" and indexed fields as "idx_<field>" strings, which equality
// queries (runQuery) filter on.
package firestore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/oauth2/google"

	"github.com/devalexandre/agno-golang/agno/storage/kvstore"
)

// Config configures the Firestore backend.
type Config struct {
	// ProjectID is the GCP project (default GOOGLE_CLOUD_PROJECT).
	ProjectID string
	// Database is the database id (default "(default)").
	Database string
	// Prefix is the collection name prefix (default "agno").
	Prefix string
	// HTTPClient is an authenticated client. When nil, Application Default
	// Credentials are used — or a plain client with the emulator.
	HTTPClient *http.Client
	// Endpoint overrides the API base URL (default https://firestore.googleapis.com/v1,
	// or http://$FIRESTORE_EMULATOR_HOST/v1).
	Endpoint string
}

// Backend is a kvstore.Backend over Firestore.
type Backend struct {
	client *http.Client
	base   string // .../v1/projects/{p}/databases/{d}/documents
	prefix string
}

// NewBackend creates a Firestore backend.
func NewBackend(ctx context.Context, cfg Config) (*Backend, error) {
	project := cfg.ProjectID
	if project == "" {
		project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if project == "" {
		return nil, fmt.Errorf("firestore: project id is required (set GOOGLE_CLOUD_PROJECT)")
	}
	database := cfg.Database
	if database == "" {
		database = "(default)"
	}
	endpoint := cfg.Endpoint
	emulator := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if endpoint == "" && emulator != "" {
		endpoint = "http://" + strings.TrimPrefix(emulator, "http://") + "/v1"
	}
	if endpoint == "" {
		endpoint = "https://firestore.googleapis.com/v1"
	}
	client := cfg.HTTPClient
	if client == nil {
		if emulator != "" || cfg.Endpoint != "" {
			client = http.DefaultClient
		} else {
			c, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/datastore")
			if err != nil {
				return nil, fmt.Errorf("firestore: default credentials: %w", err)
			}
			client = c
		}
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "agno"
	}
	base := fmt.Sprintf("%s/projects/%s/databases/%s/documents", strings.TrimRight(endpoint, "/"), project, database)
	return &Backend{client: client, base: base, prefix: prefix}, nil
}

// New creates a Firestore-backed store implementing every storage contract.
func New(ctx context.Context, cfg Config, opts ...kvstore.Option) (*kvstore.Store, error) {
	b, err := NewBackend(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return kvstore.New(b, append([]kvstore.Option{kvstore.WithID("firestore")}, opts...)...), nil
}

func (b *Backend) collName(coll string) string { return b.prefix + "_" + coll }

func (b *Backend) docURL(coll, id string) string {
	return b.base + "/" + b.collName(coll) + "/" + url.PathEscape(id)
}

type value struct {
	StringValue *string `json:"stringValue,omitempty"`
}

type document struct {
	Name   string           `json:"name,omitempty"`
	Fields map[string]value `json:"fields"`
}

func toDocument(doc []byte, index map[string]string) document {
	s := string(doc)
	d := document{Fields: map[string]value{"data": {StringValue: &s}}}
	for k, v := range index {
		v := v
		d.Fields["idx_"+k] = value{StringValue: &v}
	}
	return d
}

func (d document) data() []byte {
	if v, ok := d.Fields["data"]; ok && v.StringValue != nil {
		return []byte(*v.StringValue)
	}
	return nil
}

func (b *Backend) do(ctx context.Context, method, u string, body interface{}) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return b.client.Do(req)
}

func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
}

func apiError(resp *http.Response, op string) error {
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("firestore: %s: %s: %s", op, resp.Status, strings.TrimSpace(string(msg)))
}

// Init is a no-op: Firestore collections are created on first write.
func (b *Backend) Init(context.Context) error { return nil }

// Put upserts a document (PATCH without precondition).
func (b *Backend) Put(ctx context.Context, coll, id string, doc []byte, index map[string]string) error {
	resp, err := b.do(ctx, http.MethodPatch, b.docURL(coll, id), toDocument(doc, index))
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode >= 300 {
		return apiError(resp, "patch")
	}
	return nil
}

// Insert creates a document, reporting false when it already exists.
func (b *Backend) Insert(ctx context.Context, coll, id string, doc []byte, index map[string]string) (bool, error) {
	u := b.base + "/" + b.collName(coll) + "?documentId=" + url.QueryEscape(id)
	resp, err := b.do(ctx, http.MethodPost, u, toDocument(doc, index))
	if err != nil {
		return false, err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusConflict {
		return false, nil
	}
	if resp.StatusCode >= 300 {
		return false, apiError(resp, "create")
	}
	return true, nil
}

// Get reads a document.
func (b *Backend) Get(ctx context.Context, coll, id string) ([]byte, error) {
	resp, err := b.do(ctx, http.MethodGet, b.docURL(coll, id), nil)
	if err != nil {
		return nil, err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp, "get")
	}
	var d document
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}
	return d.data(), nil
}

// Delete removes a document.
func (b *Backend) Delete(ctx context.Context, coll, id string) error {
	resp, err := b.do(ctx, http.MethodDelete, b.docURL(coll, id), nil)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return apiError(resp, "delete")
	}
	return nil
}

// RunQueryBody builds the structuredQuery for an equality filter.
func (b *Backend) RunQueryBody(coll string, where map[string]string) map[string]interface{} {
	var filters []interface{}
	for k, v := range where {
		filters = append(filters, map[string]interface{}{"fieldFilter": map[string]interface{}{
			"field": map[string]string{"fieldPath": "idx_" + k},
			"op":    "EQUAL",
			"value": map[string]string{"stringValue": v},
		}})
	}
	q := map[string]interface{}{"from": []map[string]string{{"collectionId": b.collName(coll)}}}
	if len(filters) == 1 {
		q["where"] = filters[0]
	} else if len(filters) > 1 {
		q["where"] = map[string]interface{}{"compositeFilter": map[string]interface{}{"op": "AND", "filters": filters}}
	}
	return map[string]interface{}{"structuredQuery": q}
}

func (b *Backend) listAll(ctx context.Context, coll string) ([]document, error) {
	var out []document
	token := ""
	for {
		u := b.base + "/" + b.collName(coll) + "?pageSize=300"
		if token != "" {
			u += "&pageToken=" + url.QueryEscape(token)
		}
		resp, err := b.do(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Documents     []document `json:"documents"`
			NextPageToken string     `json:"nextPageToken"`
		}
		if resp.StatusCode != http.StatusOK {
			err = apiError(resp, "list")
		} else {
			err = json.NewDecoder(resp.Body).Decode(&page)
		}
		drain(resp)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Documents...)
		if page.NextPageToken == "" {
			return out, nil
		}
		token = page.NextPageToken
	}
}

// List returns the documents of a collection, pushing equality filters down via runQuery.
func (b *Backend) List(ctx context.Context, coll string, where map[string]string) ([][]byte, error) {
	var docs []document
	if len(where) == 0 {
		var err error
		if docs, err = b.listAll(ctx, coll); err != nil {
			return nil, err
		}
	} else {
		resp, err := b.do(ctx, http.MethodPost, b.base+":runQuery", b.RunQueryBody(coll, where))
		if err != nil {
			return nil, err
		}
		var results []struct {
			Document *document `json:"document"`
		}
		if resp.StatusCode != http.StatusOK {
			err = apiError(resp, "runQuery")
		} else {
			err = json.NewDecoder(resp.Body).Decode(&results)
		}
		drain(resp)
		if err != nil {
			return nil, err
		}
		for _, r := range results {
			if r.Document != nil {
				docs = append(docs, *r.Document)
			}
		}
	}
	out := make([][]byte, 0, len(docs))
	for _, d := range docs {
		if data := d.data(); data != nil {
			out = append(out, data)
		}
	}
	return out, nil
}

// Drop deletes every document of a collection.
func (b *Backend) Drop(ctx context.Context, coll string) error {
	docs, err := b.listAll(ctx, coll)
	if err != nil {
		return err
	}
	for _, d := range docs {
		id := d.Name[strings.LastIndex(d.Name, "/")+1:]
		if unescaped, err := url.PathUnescape(id); err == nil {
			id = unescaped
		}
		if err := b.Delete(ctx, coll, id); err != nil {
			return err
		}
	}
	return nil
}

var (
	_ kvstore.Backend  = (*Backend)(nil)
	_ kvstore.Inserter = (*Backend)(nil)
)
