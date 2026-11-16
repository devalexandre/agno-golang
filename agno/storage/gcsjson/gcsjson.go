// Package gcsjson provides a Google Cloud Storage implementation of the
// storage contracts that keeps every record as a JSON blob, a port of Python
// agno/db/gcs_json. It talks to the GCS JSON API over plain HTTP (no SDK).
//
// Objects are named "{prefix}{collection}/{id}.json". Creates use
// ifGenerationMatch=0 so concurrent writers cannot overwrite each other.
// Listing downloads every blob of a collection, so this backend suits small
// and medium deployments (the same trade-off as the Python adapter).
package gcsjson

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

// Config configures the GCS backend.
type Config struct {
	// Bucket is the bucket name (default GCS_BUCKET).
	Bucket string
	// Prefix is the object name prefix (default "agno/").
	Prefix string
	// HTTPClient is an authenticated client. When nil, Application Default
	// Credentials are used — or a plain client when STORAGE_EMULATOR_HOST is set.
	HTTPClient *http.Client
	// Endpoint overrides the API base URL (default https://storage.googleapis.com,
	// or http://$STORAGE_EMULATOR_HOST).
	Endpoint string
}

// Backend is a kvstore.Backend over GCS.
type Backend struct {
	client   *http.Client
	endpoint string
	bucket   string
	prefix   string
}

// NewBackend creates a GCS backend.
func NewBackend(ctx context.Context, cfg Config) (*Backend, error) {
	bucket := cfg.Bucket
	if bucket == "" {
		bucket = os.Getenv("GCS_BUCKET")
	}
	if bucket == "" {
		return nil, fmt.Errorf("gcsjson: bucket is required (set GCS_BUCKET)")
	}
	endpoint := cfg.Endpoint
	emulator := os.Getenv("STORAGE_EMULATOR_HOST")
	if endpoint == "" && emulator != "" {
		endpoint = emulator
		if !strings.HasPrefix(endpoint, "http") {
			endpoint = "http://" + endpoint
		}
	}
	if endpoint == "" {
		endpoint = "https://storage.googleapis.com"
	}
	client := cfg.HTTPClient
	if client == nil {
		if emulator != "" || cfg.Endpoint != "" {
			client = http.DefaultClient
		} else {
			c, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/devstorage.read_write")
			if err != nil {
				return nil, fmt.Errorf("gcsjson: default credentials: %w", err)
			}
			client = c
		}
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "agno/"
	}
	return &Backend{client: client, endpoint: strings.TrimRight(endpoint, "/"), bucket: bucket, prefix: prefix}, nil
}

// New creates a GCS-backed store implementing every storage contract.
func New(ctx context.Context, cfg Config, opts ...kvstore.Option) (*kvstore.Store, error) {
	b, err := NewBackend(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return kvstore.New(b, append([]kvstore.Option{kvstore.WithID("gcs_json")}, opts...)...), nil
}

func (b *Backend) object(coll, id string) string { return b.prefix + coll + "/" + id + ".json" }

func (b *Backend) do(ctx context.Context, method, u string, body []byte) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
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
	return fmt.Errorf("gcsjson: %s: %s: %s", op, resp.Status, strings.TrimSpace(string(msg)))
}

// Init checks that the bucket is reachable.
func (b *Backend) Init(ctx context.Context) error {
	resp, err := b.do(ctx, http.MethodGet, fmt.Sprintf("%s/storage/v1/b/%s", b.endpoint, url.PathEscape(b.bucket)), nil)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode != http.StatusOK {
		return apiError(resp, "get bucket")
	}
	return nil
}

func (b *Backend) upload(ctx context.Context, name string, doc []byte, onlyIfAbsent bool) (int, error) {
	q := url.Values{"uploadType": {"media"}, "name": {name}}
	if onlyIfAbsent {
		q.Set("ifGenerationMatch", "0")
	}
	u := fmt.Sprintf("%s/upload/storage/v1/b/%s/o?%s", b.endpoint, url.PathEscape(b.bucket), q.Encode())
	resp, err := b.do(ctx, http.MethodPost, u, doc)
	if err != nil {
		return 0, err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusPreconditionFailed {
		return resp.StatusCode, nil
	}
	if resp.StatusCode >= 300 {
		return resp.StatusCode, apiError(resp, "upload "+name)
	}
	return resp.StatusCode, nil
}

// Put writes a document.
func (b *Backend) Put(ctx context.Context, coll, id string, doc []byte, _ map[string]string) error {
	_, err := b.upload(ctx, b.object(coll, id), doc, false)
	return err
}

// Insert writes a document only if absent (ifGenerationMatch=0).
func (b *Backend) Insert(ctx context.Context, coll, id string, doc []byte, _ map[string]string) (bool, error) {
	code, err := b.upload(ctx, b.object(coll, id), doc, true)
	if err != nil {
		return false, err
	}
	return code != http.StatusPreconditionFailed, nil
}

func (b *Backend) download(ctx context.Context, name string) ([]byte, error) {
	u := fmt.Sprintf("%s/storage/v1/b/%s/o/%s?alt=media", b.endpoint, url.PathEscape(b.bucket), url.PathEscape(name))
	resp, err := b.do(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp, "download "+name)
	}
	return io.ReadAll(resp.Body)
}

// Get reads a document.
func (b *Backend) Get(ctx context.Context, coll, id string) ([]byte, error) {
	return b.download(ctx, b.object(coll, id))
}

func (b *Backend) deleteObject(ctx context.Context, name string) error {
	u := fmt.Sprintf("%s/storage/v1/b/%s/o/%s", b.endpoint, url.PathEscape(b.bucket), url.PathEscape(name))
	resp, err := b.do(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode < 300 {
		return nil
	}
	return apiError(resp, "delete "+name)
}

// Delete removes a document.
func (b *Backend) Delete(ctx context.Context, coll, id string) error {
	return b.deleteObject(ctx, b.object(coll, id))
}

func (b *Backend) names(ctx context.Context, coll string) ([]string, error) {
	var names []string
	token := ""
	for {
		q := url.Values{"prefix": {b.prefix + coll + "/"}}
		if token != "" {
			q.Set("pageToken", token)
		}
		u := fmt.Sprintf("%s/storage/v1/b/%s/o?%s", b.endpoint, url.PathEscape(b.bucket), q.Encode())
		resp, err := b.do(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
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
		for _, it := range page.Items {
			if strings.HasSuffix(it.Name, ".json") {
				names = append(names, it.Name)
			}
		}
		if page.NextPageToken == "" {
			return names, nil
		}
		token = page.NextPageToken
	}
}

// List downloads every document of a collection (where is applied by the store).
func (b *Backend) List(ctx context.Context, coll string, _ map[string]string) ([][]byte, error) {
	names, err := b.names(ctx, coll)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, 0, len(names))
	for _, n := range names {
		doc, err := b.download(ctx, n)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			out = append(out, doc)
		}
	}
	return out, nil
}

// Drop deletes every blob of a collection.
func (b *Backend) Drop(ctx context.Context, coll string) error {
	names, err := b.names(ctx, coll)
	if err != nil {
		return err
	}
	for _, n := range names {
		if err := b.deleteObject(ctx, n); err != nil {
			return err
		}
	}
	return nil
}

var (
	_ kvstore.Backend  = (*Backend)(nil)
	_ kvstore.Inserter = (*Backend)(nil)
)
