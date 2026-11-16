package sandbox

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// S3CheckpointStore keeps objects in an S3-compatible bucket (AWS S3, MinIO,
// R2, ...) using plain REST calls signed with AWS Signature V4. Objects are
// named <Prefix><sha256>.
type S3CheckpointStore struct {
	// Endpoint, e.g. "https://s3.us-east-1.amazonaws.com" or "http://localhost:9000".
	Endpoint     string
	Region       string // default "us-east-1"
	Bucket       string
	Prefix       string // default "checkpoints/"
	AccessKey    string
	SecretKey    string
	SessionToken string
	// PathStyle uses <endpoint>/<bucket>/<key> (needed for MinIO); otherwise virtual-hosted style.
	PathStyle bool
	MaxBytes  int64 // default 1 GiB
	Client    *http.Client
	now       func() time.Time
}

func (s *S3CheckpointStore) objectURL(ref CheckpointRef) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(s.Endpoint, "/"))
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid S3 endpoint %q", s.Endpoint)
	}
	prefix := s.Prefix
	if prefix == "" {
		prefix = "checkpoints/"
	}
	key := prefix + ref.SHA256
	if s.PathStyle {
		u.Path = "/" + s.Bucket + "/" + key
	} else {
		u.Host = s.Bucket + "." + u.Host
		u.Path = "/" + key
	}
	return u, nil
}

func (s *S3CheckpointStore) do(ctx context.Context, method string, ref CheckpointRef, body []byte) (*http.Response, error) {
	u, err := s.objectURL(ref)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(body))
	s.sign(req, body)
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func uriEncode(s string, keepSlash bool) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		switch {
		case (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~':
			b.WriteByte(c)
		case c == '/' && keepSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// sign applies AWS Signature Version 4 headers.
func (s *S3CheckpointStore) sign(req *http.Request, body []byte) {
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}
	region := s.Region
	if region == "" {
		region = "us-east-1"
	}
	amzDate := now.Format("20060102T150405Z")
	day := now.Format("20060102")
	sum := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(sum[:])
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	if s.SessionToken != "" {
		req.Header.Set("x-amz-security-token", s.SessionToken)
	}
	headers := map[string]string{"host": req.URL.Host}
	for k := range req.Header {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "x-amz-") || lk == "content-type" {
			headers[lk] = strings.TrimSpace(req.Header.Get(k))
		}
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	var canonHeaders strings.Builder
	for _, k := range names {
		canonHeaders.WriteString(k + ":" + headers[k] + "\n")
	}
	signed := strings.Join(names, ";")
	query := req.URL.Query()
	qk := make([]string, 0, len(query))
	for k := range query {
		qk = append(qk, k)
	}
	sort.Strings(qk)
	var qs []string
	for _, k := range qk {
		for _, v := range query[k] {
			qs = append(qs, uriEncode(k, false)+"="+uriEncode(v, false))
		}
	}
	canonical := strings.Join([]string{req.Method, uriEncode(req.URL.Path, true), strings.Join(qs, "&"),
		canonHeaders.String(), signed, payloadHash}, "\n")
	scope := day + "/" + region + "/s3/aws4_request"
	creq := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(creq[:])
	k := hmacSHA256([]byte("AWS4"+s.SecretKey), day)
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, "s3")
	k = hmacSHA256(k, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(k, toSign))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.AccessKey, scope, signed, sig))
}

// Put implements CheckpointStore (content-addressed: re-putting is harmless).
func (s *S3CheckpointStore) Put(ctx context.Context, r io.Reader) (CheckpointRef, error) {
	max := s.MaxBytes
	if max <= 0 {
		max = 1 << 30
	}
	var buf bytes.Buffer
	ref, err := hashTo(r, &buf, max)
	if err != nil {
		return ref, err
	}
	resp, err := s.do(ctx, http.MethodPut, ref, buf.Bytes())
	if err != nil {
		return ref, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return ref, fmt.Errorf("s3 put: %s: %s", resp.Status, msg)
	}
	return ref, nil
}

// Open implements CheckpointStore, verifying the digest.
func (s *S3CheckpointStore) Open(ctx context.Context, ref CheckpointRef) (io.ReadCloser, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	resp, err := s.do(ctx, http.MethodGet, ref, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrCheckpointNotFound
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("s3 get: %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, ref.Size+1))
	if err != nil {
		return nil, err
	}
	if err := verifyBytes(b, ref); err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

// Delete implements CheckpointStore.
func (s *S3CheckpointStore) Delete(ctx context.Context, ref CheckpointRef) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	resp, err := s.do(ctx, http.MethodDelete, ref, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("s3 delete: %s", resp.Status)
	}
	return nil
}
