package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/devalexandre/agno-golang/agno/document"
)

// DoclingReader converts documents (PDF, DOCX, PPTX, XLSX, HTML, images,
// audio, ...) through a docling-serve instance
// (https://github.com/docling-project/docling-serve). Port of agno-python
// knowledge/reader/docling_reader.py, which runs docling in-process; Go talks
// to the server's HTTP API instead: POST /v1/convert/source for http(s) URLs
// and POST /v1/convert/file (multipart) for local files.
type DoclingReader struct {
	BaseReader
	// BaseURL of docling-serve (default http://localhost:5001).
	BaseURL string
	// APIKey sent as X-Api-Key when docling-serve requires one.
	APIKey string
	// OutputFormat: markdown (default), text, json, yaml, html,
	// html_split_page or doctags.
	OutputFormat string
	// Options are merged into the conversion options (e.g. {"do_ocr": false}).
	Options map[string]any
	// AllowedHosts, when non-nil, refuses URL sources whose host is not listed.
	AllowedHosts []string
	// PublicOnly refuses URL sources that are not public http(s) URLs
	// (docling-serve fetches the URL itself, so only the URL is validated).
	PublicOnly bool
	HTTPClient *http.Client
}

// NewDoclingReader creates a docling-serve reader.
func NewDoclingReader(baseURL string) *DoclingReader {
	if baseURL == "" {
		baseURL = "http://localhost:5001"
	}
	return &DoclingReader{
		BaseReader: BaseReader{ID: "docling", Extensions: []string{
			".pdf", ".docx", ".dotx", ".docm", ".dotm", ".pptx", ".potx", ".ppsx", ".pptm", ".potm", ".ppsm",
			".md", ".html", ".htm", ".xhtml", ".xml", ".adoc", ".asciidoc", ".asc", ".csv", ".xlsx", ".xlsm",
			".tex", ".latex", ".vtt", ".png", ".jpg", ".jpeg", ".tif", ".tiff", ".bmp", ".webp",
			".wav", ".mp3", ".m4a", ".aac", ".ogg", ".flac", ".mp4", ".avi", ".mov",
		}},
		BaseURL:      baseURL,
		OutputFormat: "markdown",
	}
}

// docling-serve to_formats value and response field for each output format.
var doclingFormats = map[string][2]string{
	"markdown":        {"md", "md_content"},
	"text":            {"text", "text_content"},
	"json":            {"json", "json_content"},
	"yaml":            {"json", "json_content"},
	"html":            {"html", "html_content"},
	"html_split_page": {"html_split_page", "html_content"},
	"doctags":         {"doctags", "doctags_content"},
}

func (r *DoclingReader) format() (string, [2]string, error) {
	f := strings.ToLower(r.OutputFormat)
	if f == "" {
		f = "markdown"
	}
	spec, ok := doclingFormats[f]
	if !ok {
		return "", spec, fmt.Errorf("docling: invalid output format %q (valid: markdown, text, json, yaml, html, html_split_page, doctags)", r.OutputFormat)
	}
	return f, spec, nil
}

// Read converts source (an http(s) URL or a local file path).
func (r *DoclingReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	f, spec, err := r.format()
	if err != nil {
		return nil, err
	}
	options := map[string]any{"to_formats": []string{spec[0]}}
	for k, v := range r.Options {
		options[k] = v
	}
	base := strings.TrimRight(r.BaseURL, "/")
	if base == "" {
		base = "http://localhost:5001"
	}
	client := apiClient(r.HTTPClient, 10*time.Minute)
	headers := map[string]string{}
	if r.APIKey != "" {
		headers["X-Api-Key"] = r.APIKey
	}

	var resp doclingResponse
	var name string
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		if !IsHostAllowed(source, r.AllowedHosts) {
			return nil, fmt.Errorf("docling: %w: %s", ErrHostNotAllowed, source)
		}
		if r.PublicOnly {
			if _, err := ValidatePublicURL(source); err != nil {
				return nil, fmt.Errorf("docling: %w", err)
			}
		}
		urlPath := strings.SplitN(source, "?", 2)[0]
		name = strings.TrimSuffix(path.Base(urlPath), path.Ext(urlPath))
		body := map[string]any{
			"sources": []map[string]any{{"kind": "http", "url": source}},
			"options": options,
		}
		if err := doJSON(ctx, client, http.MethodPost, base+"/v1/convert/source", headers, body, &resp); err != nil {
			return nil, fmt.Errorf("docling: %w", err)
		}
	} else {
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, err
		}
		name = strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
		if err := r.convertFile(ctx, client, base, headers, filepath.Base(source), data, options, &resp); err != nil {
			return nil, err
		}
	}
	if resp.Status != "" && resp.Status != "success" && resp.Status != "partial_success" {
		return nil, fmt.Errorf("docling: conversion %s: %v", resp.Status, resp.Errors)
	}
	raw, ok := resp.Document[spec[1]]
	if !ok || string(raw) == "null" {
		return nil, fmt.Errorf("docling: response has no %s", spec[1])
	}
	var content string
	switch f {
	case "json":
		content = string(raw)
	case "yaml":
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("docling: decode json_content: %w", err)
		}
		out, err := yaml.Marshal(v)
		if err != nil {
			return nil, err
		}
		content = string(out)
	default:
		if err := json.Unmarshal(raw, &content); err != nil {
			return nil, fmt.Errorf("docling: decode %s: %w", spec[1], err)
		}
	}
	d := document.NewDocument(content)
	d.Name = name
	d.Source = source
	d.ContentType = map[string]string{
		"markdown": "text/markdown", "text": "text/plain", "json": "application/json", "yaml": "application/yaml",
		"html": "text/html", "html_split_page": "text/html", "doctags": "text/plain",
	}[f]
	d.Metadata["source"] = source
	d.Metadata["output_format"] = f
	return []*document.Document{d}, nil
}

type doclingResponse struct {
	Document map[string]json.RawMessage `json:"document"`
	Status   string                     `json:"status"`
	Errors   []any                      `json:"errors"`
}

func (r *DoclingReader) convertFile(ctx context.Context, client *http.Client, base string, headers map[string]string,
	filename string, data []byte, options map[string]any, out *doclingResponse) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("files", filename)
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, bytes.NewReader(data)); err != nil {
		return err
	}
	for k, v := range options {
		switch val := v.(type) {
		case []string:
			for _, s := range val {
				_ = mw.WriteField(k, s)
			}
		case string:
			_ = mw.WriteField(k, val)
		default:
			b, _ := json.Marshal(val)
			_ = mw.WriteField(k, string(b))
		}
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/convert/file", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if err := sendJSON(client, req, out); err != nil {
		return fmt.Errorf("docling: %w", err)
	}
	return nil
}

var _ Reader = (*DoclingReader)(nil)
