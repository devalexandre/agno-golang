// Package surrealdb provides a SurrealDB implementation of the storage
// contracts (storage.DB, storage.Storage, storage.KnowledgeStorage, session
// search and metrics), a port of Python agno/db/surrealdb. It talks to the
// SurrealDB HTTP /sql endpoint (SurrealDB 2.x+), so no SDK is needed.
//
// Each kvstore collection maps to the table "{prefix}_{collection}"; record
// ids are the record ids, the JSON document lives in the string field "data"
// and indexed fields in the object field "idx".
package surrealdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/storage/kvstore"
)

// Config configures the SurrealDB backend.
type Config struct {
	// URL is the server base URL (default SURREALDB_URL, then http://localhost:8000).
	URL string
	// Namespace and Database select the SurrealDB namespace/database
	// (defaults SURREALDB_NS / SURREALDB_DB, then "agno").
	Namespace string
	Database  string
	// User/Password for basic auth (defaults SURREALDB_USER / SURREALDB_PASS).
	User     string
	Password string
	// Token is a bearer token used instead of basic auth.
	Token string
	// Prefix is the table name prefix (default "agno").
	Prefix string
	// HTTPClient overrides the HTTP client.
	HTTPClient *http.Client
}

// Backend is a kvstore.Backend over SurrealDB.
type Backend struct {
	cfg    Config
	client *http.Client
}

func envOr(v, env, def string) string {
	if v != "" {
		return v
	}
	if e := os.Getenv(env); e != "" {
		return e
	}
	return def
}

// NewBackend creates a SurrealDB backend.
func NewBackend(cfg Config) (*Backend, error) {
	cfg.URL = strings.TrimRight(envOr(cfg.URL, "SURREALDB_URL", "http://localhost:8000"), "/")
	cfg.URL = strings.Replace(strings.Replace(cfg.URL, "ws://", "http://", 1), "wss://", "https://", 1)
	cfg.URL = strings.TrimSuffix(strings.TrimSuffix(cfg.URL, "/rpc"), "/sql")
	cfg.Namespace = envOr(cfg.Namespace, "SURREALDB_NS", "agno")
	cfg.Database = envOr(cfg.Database, "SURREALDB_DB", "agno")
	cfg.User = envOr(cfg.User, "SURREALDB_USER", "")
	cfg.Password = envOr(cfg.Password, "SURREALDB_PASS", "")
	if cfg.Prefix == "" {
		cfg.Prefix = "agno"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &Backend{cfg: cfg, client: client}, nil
}

// New creates a SurrealDB-backed store implementing every storage contract.
func New(cfg Config, opts ...kvstore.Option) (*kvstore.Store, error) {
	b, err := NewBackend(cfg)
	if err != nil {
		return nil, err
	}
	return kvstore.New(b, append([]kvstore.Option{kvstore.WithID("surrealdb")}, opts...)...), nil
}

// lit renders a Go value as a SurrealQL literal (JSON is valid SurrealQL).
func lit(v interface{}) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func (b *Backend) table(coll string) string { return b.cfg.Prefix + "_" + coll }

func (b *Backend) thing(coll, id string) string {
	return fmt.Sprintf("type::thing(%s, %s)", lit(b.table(coll)), lit(id))
}

// statement is one result of a /sql request.
type statement struct {
	Status string          `json:"status"`
	Result json.RawMessage `json:"result"`
}

// Exec runs one SurrealQL query and returns the per-statement results.
func (b *Backend) Exec(ctx context.Context, query string) ([]statement, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.cfg.URL+"/sql", bytes.NewBufferString(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	for _, h := range []string{"Surreal-NS", "NS"} {
		req.Header.Set(h, b.cfg.Namespace)
	}
	for _, h := range []string{"Surreal-DB", "DB"} {
		req.Header.Set(h, b.cfg.Database)
	}
	if b.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+b.cfg.Token)
	} else if b.cfg.User != "" {
		req.SetBasicAuth(b.cfg.User, b.cfg.Password)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("surrealdb: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out []statement
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("surrealdb: decode response: %w", err)
	}
	return out, nil
}

func stmtErr(st statement) error {
	var msg string
	_ = json.Unmarshal(st.Result, &msg)
	return fmt.Errorf("surrealdb: %s", msg)
}

func (b *Backend) exec1(ctx context.Context, query string) (statement, error) {
	res, err := b.Exec(ctx, query)
	if err != nil {
		return statement{}, err
	}
	if len(res) == 0 {
		return statement{}, fmt.Errorf("surrealdb: empty response")
	}
	if res[0].Status != "OK" {
		return res[0], stmtErr(res[0])
	}
	return res[0], nil
}

// Init checks connectivity.
func (b *Backend) Init(ctx context.Context) error {
	_, err := b.exec1(ctx, "RETURN true;")
	return err
}

func content(doc []byte, index map[string]string) string {
	if index == nil {
		index = map[string]string{}
	}
	return lit(map[string]interface{}{"data": string(doc), "idx": index})
}

// PutQuery builds the upsert statement.
func (b *Backend) PutQuery(coll, id string, doc []byte, index map[string]string) string {
	return fmt.Sprintf("UPSERT %s CONTENT %s;", b.thing(coll, id), content(doc, index))
}

// Put upserts a document.
func (b *Backend) Put(ctx context.Context, coll, id string, doc []byte, index map[string]string) error {
	_, err := b.exec1(ctx, b.PutQuery(coll, id, doc, index))
	return err
}

// Insert creates a document, reporting false when it already exists.
func (b *Backend) Insert(ctx context.Context, coll, id string, doc []byte, index map[string]string) (bool, error) {
	st, err := b.exec1(ctx, fmt.Sprintf("CREATE %s CONTENT %s;", b.thing(coll, id), content(doc, index)))
	if err != nil {
		if strings.Contains(strings.ToLower(string(st.Result)), "already exists") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func decodeData(raw json.RawMessage) ([][]byte, error) {
	var rows []struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("surrealdb: decode rows: %w", err)
	}
	out := make([][]byte, 0, len(rows))
	for _, r := range rows {
		out = append(out, []byte(r.Data))
	}
	return out, nil
}

// Get reads a document.
func (b *Backend) Get(ctx context.Context, coll, id string) ([]byte, error) {
	st, err := b.exec1(ctx, fmt.Sprintf("SELECT data FROM %s;", b.thing(coll, id)))
	if err != nil {
		return nil, err
	}
	docs, err := decodeData(st.Result)
	if err != nil || len(docs) == 0 {
		return nil, err
	}
	return docs[0], nil
}

// Delete removes a document.
func (b *Backend) Delete(ctx context.Context, coll, id string) error {
	_, err := b.exec1(ctx, fmt.Sprintf("DELETE %s;", b.thing(coll, id)))
	return err
}

// ListQuery builds the select statement for where.
func (b *Backend) ListQuery(coll string, where map[string]string) string {
	q := fmt.Sprintf("SELECT data, id FROM type::table(%s)", lit(b.table(coll)))
	var conds []string
	for k, v := range where {
		conds = append(conds, fmt.Sprintf("idx[%s] = %s", lit(k), lit(v)))
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	return q + " ORDER BY id;"
}

// List returns documents matching where.
func (b *Backend) List(ctx context.Context, coll string, where map[string]string) ([][]byte, error) {
	st, err := b.exec1(ctx, b.ListQuery(coll, where))
	if err != nil {
		return nil, err
	}
	return decodeData(st.Result)
}

// Drop deletes every record of a table.
func (b *Backend) Drop(ctx context.Context, coll string) error {
	_, err := b.exec1(ctx, fmt.Sprintf("DELETE type::table(%s);", lit(b.table(coll))))
	return err
}

var (
	_ kvstore.Backend  = (*Backend)(nil)
	_ kvstore.Inserter = (*Backend)(nil)
)
